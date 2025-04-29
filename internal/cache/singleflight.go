package cache

import (
	"sync"
	"time"
)

// result 封装了缓存值和可能的错误
type result struct {
	Value interface{}
	Err   error
}

// call 表示一个正在进行的函数调用，多个goroutine可以等待该调用的完成
type call struct {
	done chan struct{} // 用于通知调用完成的通道
	res  result        // 调用结果
}

// cacheEntry 表示缓存中的一个条目，包含结果和过期时间
type cacheEntry struct {
	result  result    // 缓存结果
	expires time.Time // 过期时间
}

// SingleFlight 提供缓存和并发调用合并功能
type SingleFlight struct {
	mu    sync.RWMutex          // 读写锁，保护calls和cache
	calls map[string]*call      // 正在进行的调用集合
	cache map[string]cacheEntry // 缓存条目集合
	ttl   time.Duration         // 缓存有效期
}

// NewFlightGroup 创建一个新的SingleFlight实例
// ttl: 缓存有效期，若<=0则默认3秒
func NewFlightGroup(ttl time.Duration) *SingleFlight {
	if ttl <= 0 {
		ttl = 3 * time.Second
	}

	return &SingleFlight{
		calls: make(map[string]*call),
		cache: make(map[string]cacheEntry),
		ttl:   ttl,
	}
}

// Do 执行并返回给定key对应的结果
// 如果缓存有效则直接返回，否则合并并发请求并执行fn获取结果
func (sf *SingleFlight) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	// 1. 首先检查有效缓存
	if value, ok := sf.getCache(key); ok {
		return value.Value, value.Err
	}

	// 2. 获取或创建调用对象
	c, created := sf.getCall(key)
	if !created {
		<-c.done // 等待已有调用完成
		return c.res.Value, c.res.Err
	}

	// 3. 当前goroutine负责执行函数并设置结果
	defer func() {
		sf.mu.Lock()
		delete(sf.calls, key) // 调用完成后移除call记录
		sf.mu.Unlock()
		close(c.done) // 通知所有等待的goroutine
	}()

	// 执行实际函数
	value, err := fn()

	// 4. 更新缓存（加写锁保证原子性）
	sf.mu.Lock()
	sf.cache[key] = cacheEntry{
		result:  result{Value: value, Err: err},
		expires: time.Now().Add(sf.ttl),
	}
	sf.mu.Unlock()

	// 5. 设置调用结果并返回
	c.res.Value = value
	c.res.Err = err
	return value, err
}

// getCache 获取有效缓存（自动清理过期缓存）
func (sf *SingleFlight) getCache(key string) (result, bool) {
	sf.mu.RLock()
	entry, exists := sf.cache[key]
	sf.mu.RUnlock()

	// 发现过期缓存时异步清理
	if exists && time.Now().After(entry.expires) {
		go func() {
			sf.mu.Lock()
			defer sf.mu.Unlock()
			// 二次检查防止并发场景下的误删
			if entry, exists := sf.cache[key]; exists && time.Now().After(entry.expires) {
				delete(sf.cache, key)
			}
		}()
		return result{}, false
	}

	return entry.result, exists
}

// getCall 获取或创建调用对象（双重检查锁保证并发安全）
func (sf *SingleFlight) getCall(key string) (*call, bool) {
	// 第一重检查（读锁）
	sf.mu.RLock()
	if c, ok := sf.calls[key]; ok {
		sf.mu.RUnlock()
		return c, false
	}
	sf.mu.RUnlock()

	// 第二重检查（写锁）
	sf.mu.Lock()
	defer sf.mu.Unlock()

	if c, ok := sf.calls[key]; ok {
		return c, false
	}

	// 创建新的调用对象
	c := &call{done: make(chan struct{})}
	sf.calls[key] = c
	return c, true
}
