package cache

import (
	"FishCache/internal/cache/eviction"
	"fmt"
	"sync"
)

type cache struct {
	mu       sync.RWMutex
	strategy *eviction.CacheUseLRU
	maxBytes int64
}

func NewCache(maxBytes int64) (*cache, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("cache size must be positive, got %d", maxBytes)
	}

	onEvicted := func(key string, val eviction.Value) {
		fmt.Printf("Cache entry evicted: key=%s\n", key)
	}
	return &cache{
		maxBytes: maxBytes,
		strategy: eviction.NewLRUCache(maxBytes, onEvicted),
	}, nil
}

func (c *cache) get(key string) (ByteView, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if v, _, exists := c.strategy.Get(key); exists {
		// 类型断言，将接口类型的变量 v *Value 转换为具体的类型 ByteView
		if bv, ok := v.(ByteView); ok {
			return bv, ok
		}
	}

	return ByteView{}, false
}

func (c *cache) add(key string, value ByteView) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.strategy.Add(key, value)
}
