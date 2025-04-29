package eviction

import (
	"container/list"
	"hash/fnv"
	"sync"
	"time"
)

const (
	// 缓存的TTL时间
	defaultTTL = 10 * time.Minute
	// 清理缓存的定时器时间
	defaultCleanupInterval = 2 * time.Minute
	// 缓存管理器的缓存分片数量
	defaultNumSegments = 16
)

// 缓存的实际存储单元，负责存储和管理缓存分片，提供LRU策略的实现。支持查找、新增或更新、删除过期缓存、删除缓存
type segment struct {
	// 读写锁
	mu sync.RWMutex
	// 允许使用的最大内存
	maxBytes int64
	// nowbytes是当前已使用的内存
	nowBytes int64
	// 双向链表，存储所有value，被访问时移动到队尾O(1)
	ll *list.List
	// 哈希表，映射键值对
	cache map[string]*list.Element
	// 记录被移除时的回调函数
	OnEvicted func(key string, value Value)
}

// CacheUseLRU 负责管理缓存分片，支持查找、新增或更新，不直接支持删除
// 分片设计，将整个缓存划分为多个独立的分片。
type CacheUseLRU struct {
	// 缓存分片集合
	segments []*segment
	// 表示缓存分片的数量
	numSegments int
	// TTL
	ttl time.Duration
	// 管理器读写锁
	mu              sync.RWMutex
	stopCleanup     chan struct{}
	cleanupInterval time.Duration
}

func NewLRUCache(maxBytes int64, onEvicted func(string, Value)) *CacheUseLRU {
	cache := &CacheUseLRU{
		segments:        make([]*segment, defaultNumSegments),
		numSegments:     defaultNumSegments,
		cleanupInterval: defaultCleanupInterval,
		ttl:             defaultTTL,
		stopCleanup:     make(chan struct{}),
	}
	// 由整体maxBytes定义缓存分片的平均maxBytes
	segmentMaxBytes := maxBytes / int64(defaultNumSegments)
	for i := 0; i < defaultNumSegments; i++ {
		cache.segments[i] = &segment{
			maxBytes:  segmentMaxBytes,
			ll:        list.New(),
			cache:     make(map[string]*list.Element),
			OnEvicted: onEvicted,
		}
	}
	// 开启定时清理过期缓存的任务
	go cache.cleanUpRoutine()

	return cache
}

// 根据键值(key)确定其所属的缓存段(segment)；通过FNV-1a哈希算法将键均匀分布到多个分段，减少全局锁竞争
func (cache *CacheUseLRU) getSegment(key string) *segment {
	// FNV(Fowler-Noll-Vo)是一种非加密哈希算法，特点是哈希速度快，适合高并发场景，并且有低碰撞率，能均匀分散键值分布
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	// 计算哈希值，取模确定段索引
	return cache.segments[h.Sum32()%uint32(cache.numSegments)]
}

// Get 在管理器中查找对应缓存段中的value, 并移动value至队列尾部
func (cache *CacheUseLRU) Get(key string) (value Value, updateAt time.Time, ok bool) {
	seg := cache.getSegment(key)
	seg.mu.RLock()
	defer seg.mu.RUnlock()

	if elm, ok := seg.cache[key]; ok {
		seg.ll.MoveToBack(elm)
		kv := elm.Value.(*Entry)
		kv.UpdatedTTLTime() // 更新TTL时间
		return kv.value, kv.updateAt, true
	}
	return nil, time.Time{}, false
}

// Add 新增或更新缓存段中的value
func (cache *CacheUseLRU) Add(key string, value Value) {
	seg := cache.getSegment(key)
	seg.mu.Lock()
	defer seg.mu.Unlock()
	// 计算最新的k+v比特大小
	newBytes := int64(len(key)) + int64(value.Len())
	// 尝试在缓存段中根据key获取value
	if elm, ok := seg.cache[key]; ok {
		// 修改
		entry := elm.Value.(*Entry)
		oldBytes := int64(len(entry.key)) + int64(entry.value.Len())
		entry.value = value
		entry.UpdatedTTLTime() // 更新TTL时间
		seg.nowBytes = seg.nowBytes - oldBytes + newBytes
		seg.ll.MoveToBack(elm)
	} else {
		// 添加
		entry := &Entry{
			key:      key,
			value:    value,
			updateAt: time.Now(),
		}
		elm = seg.ll.PushBack(entry)
		seg.cache[key] = elm
		seg.nowBytes += newBytes
	}

	for seg.maxBytes != 0 && seg.nowBytes > seg.maxBytes {
		// nowBytes 超出了 maxBytes 时，执行LRU淘汰策略的清理
		seg.removeOldest()
	}
}

// 删除缓存段中的最近最少使用(队头)数据
func (seg *segment) removeOldest() {
	if ele := seg.ll.Front(); ele != nil {
		seg.removeElement(ele)
	}
}

// 删除缓存段中的缓存数据
func (seg *segment) removeElement(elm *list.Element) {
	seg.ll.Remove(elm)
	entry := elm.Value.(*Entry)
	delete(seg.cache, entry.key)                                     // 从哈希表中删除对应key
	seg.nowBytes -= int64(len(entry.key)) + int64(entry.value.Len()) // 计算缓存段中的nowBytes

	if seg.OnEvicted != nil {
		seg.OnEvicted(entry.key, entry.value)
	}
}

// 定时触发TTL缓存队列检查
func (cache *CacheUseLRU) cleanUpRoutine() {
	ticker := time.NewTicker(cache.cleanupInterval) // 新建一个定时器
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 到达定时器触发时间，清除一次TTL过期的缓存
			cache.cleanUPSegments()
		case <-cache.stopCleanup:
			return
		}
	}
}

// 遍历缓存段，触发TTL清理方法
func (cache *CacheUseLRU) cleanUPSegments() {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	for _, seg := range cache.segments {
		seg.cleanUpExpired(cache.ttl)
	}
}

// 清理过期缓存
func (seg *segment) cleanUpExpired(ttl time.Duration) {
	seg.mu.Lock()
	defer seg.mu.Unlock()

	var next *list.Element
	// 遍历缓存队列
	for e := seg.ll.Front(); e != nil; e = next {
		next = e.Next()
		if e.Value == nil {
			continue
		}
		// 如果TTL过期则删除
		if e.Value.(*Entry).Expired(ttl) {
			seg.removeElement(e)
		}
	}
}

// Len 对外提供 计算当前缓存段集的总缓存数据个数
func (cache *CacheUseLRU) Len() int {
	total := 0
	for _, seg := range cache.segments {
		seg.mu.RLock()
		total += seg.ll.Len()
		seg.mu.RUnlock()
	}
	return total
}

// SetTTL 对外提供 设置缓存管理器TTL时间的方法
func (cache *CacheUseLRU) SetTTL(ttl time.Duration) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.ttl = ttl
}

// SetCleanupInterval 对外提供 设置缓存管理器定时器时间的方法
func (cache *CacheUseLRU) SetCleanupInterval(interval time.Duration) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	close(cache.stopCleanup)

	// 根据新的时间创建新定时器任务
	cache.stopCleanup = make(chan struct{})
	cache.cleanupInterval = interval

	go cache.cleanUpRoutine()
}

// Stop 停止当前的清理goroutine
func (cache *CacheUseLRU) Stop() {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	close(cache.stopCleanup)
}
