package cache

import (
	"hash/crc32"
	"sort"
	"strconv"
	"sync"
)

// Hash maps bytes to uint32
// 定义函数类型Hash，采取依赖注入方式
// 允许用于替换成自定义的 Hash 函数，默认为 crc32.ChecksumIEEE 算法。
type Hash func(data []byte) uint32

// ConsistentMap 一致性哈希算法的主数据结构,确保键值对的处理节点统一
type ConsistentMap struct {
	mu sync.RWMutex
	//Hash函数hash
	hash Hash
	//虚拟节点倍数replicas
	replicas int
	//哈希环
	keys []int // Sorted
	//虚拟节点与真实节点的映射表 hashMap，键是虚拟节点的哈希值，值是真实节点的名称。
	hashMap map[int]string
}

// NewConsistentHash 允许自定义虚拟节点倍数和 Hash 函数。
func NewConsistentHash(replicas int, fn Hash) *ConsistentMap {
	m := &ConsistentMap{
		replicas: replicas,
		hash:     fn,
		hashMap:  make(map[int]string),
	}
	if m.hash == nil {
		m.hash = crc32.ChecksumIEEE
	}
	return m
}

// AddNodes 函数允许传入 0 或 多个真实节点的名称
func (m *ConsistentMap) AddNodes(nodes ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.keys = nil                     // 清空之前的keys
	m.hashMap = make(map[int]string) // 清空之前的hashMap

	// 对节点名称进行排序
	sort.Strings(nodes)

	// 对每一个真实节点 node，对应创建 m.replicas 个虚拟节点。缓解真实节点少时的数据倾斜问题。
	for _, nodeName := range nodes {
		for i := 0; i < m.replicas; i++ {
			// 虚拟节点的名称是：strconv.Itoa(i) + nodeName，通过添加编号的方式区分不同虚拟节点。
			vnodeHash := int(m.hash([]byte(strconv.Itoa(i) + nodeName)))
			m.keys = append(m.keys, vnodeHash)
			//在 hashMap 中增加虚拟节点和真实节点的映射关系。
			m.hashMap[vnodeHash] = nodeName
		}
	}
	// 排序哈希环
	sort.Ints(m.keys)
}

// GetNode 返回指定key的对应节点
func (m *ConsistentMap) GetNode(key string) string {
	if len(m.keys) == 0 {
		return ""
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// 第一步，计算 key 的哈希值。
	hash := int(m.hash([]byte(key)))
	// 第二步，顺时针找到第一个匹配的虚拟节点的下标 idx
	index := sort.Search(len(m.keys), func(i int) bool {
		// 当前节点hash >= 当前key hash时，则为该key对应的处理节点
		return m.keys[i] >= hash
	})
	// 边界判断，下标达到边界时，统一返回第一个节点
	if index == len(m.keys) {
		index = 0
	}
	// 第三步，通过 hashMap 映射得到真实的节点。
	return m.hashMap[m.keys[index]]
}
