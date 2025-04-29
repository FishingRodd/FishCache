package eviction

import (
	"sync"
	"testing"
	"time"
)

type String string

func (d String) Len() int {
	return len(d)
}

// TestCacheUseLRU_Basic 测试基础功能
func TestCacheUseLRU_Basic(t *testing.T) {
	// 定义一个测试用例，名称为"creation"
	t.Run("creation", func(t *testing.T) {
		// 创建一个大小为100的LRU缓存
		lru := NewLRUCache(100, nil)
		// 检查缓存是否成功创建
		if lru == nil {
			t.Error("未能创建LRU缓存")
		}
		// 检查新创建的缓存是否为空
		if lru.Len() != 0 {
			t.Errorf("新缓存应该为空，实际长度为 %d", lru.Len())
		}
	})

	// 定义一个测试用例，名称为"basic operations"
	t.Run("basic operations", func(t *testing.T) {
		// 创建一个大小为1024的LRU缓存
		lru := NewLRUCache(1024, nil)

		// 测试添加（Add）和获取（Get）操作
		lru.Add("key1", String("value1")) // 将键"key1"添加到缓存，并赋值"value1"
		// 检查从缓存中获取"key1"的值是否正确
		if v, _, ok := lru.Get("key1"); !ok || string(v.(String)) != "value1" {
			t.Errorf("获取键'key1'失败，实际值为 %v，期望值为 'value1'", v)
		}

		// 测试获取不存在的键
		if _, _, ok := lru.Get("missing"); ok {
			t.Error("获取不存在的键应返回false")
		}

		// 测试更新已有键的值
		lru.Add("key1", String("value2")) // 更新键"key1"的值为"value2"
		// 检查从缓存中获取"key1"的值是否更新成功
		if v, _, ok := lru.Get("key1"); !ok || string(v.(String)) != "value2" {
			t.Errorf("更新后获取键'key1'失败，实际值为 %v，期望值为 'value2'", v)
		}
	})
}

// TestCacheUseLRU_CleanUp 用于测试 LRU 缓存的清理功能
func TestCacheUseLRU_CleanUp(t *testing.T) {
	// 创建一个新的 LRU 缓存，大小为 1024，清理函数为 nil
	lru := NewLRUCache(1024, nil)

	// 设置清理间隔为 50 毫秒
	lru.SetCleanupInterval(50 * time.Millisecond)

	// 设置 TTL（生存时间）为 100 毫秒
	lru.SetTTL(100 * time.Millisecond)

	// 向缓存中添加一些条目
	lru.Add("k1", String("v1"))
	lru.Add("k2", String("v2"))
	lru.Add("k3", String("v3"))

	// 访问一些条目，以改变它们的最后访问时间
	time.Sleep(20 * time.Millisecond) // 等待 20 毫秒
	_, _, ok1 := lru.Get("k1")        // 获取 k1 的值
	time.Sleep(20 * time.Millisecond) // 等待 20 毫秒
	_, _, ok2 := lru.Get("k2")        // 获取 k2 的值
	time.Sleep(20 * time.Millisecond) // 等待 20 毫秒
	_, _, ok3 := lru.Get("k3")        // 获取 k3 的值

	// 检查获取的条目是否存在
	if !ok1 || !ok2 || !ok3 {
		t.Fatal("获取的条目应该存在，但失败了")
	}

	// 等待清理操作发生
	time.Sleep(150 * time.Millisecond)

	// 所有条目应该被清除
	if lru.Len() != 0 {
		t.Errorf("清理失败，缓存应该为空，当前长度为 %d", lru.Len())
	}
}

func TestCacheUseLRU_Concurrent(t *testing.T) {
	lru := NewLRUCache(1024, nil)
	var wg sync.WaitGroup
	numOps := 1000
	numGoroutines := 10

	// 并行读写，启动20个goroutine
	for i := 0; i < numGoroutines; i++ {
		wg.Add(2)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				key := "k" + string(rune(j%100+'0'))
				value := String("v" + string(rune(j%100+'0')))
				lru.Add(key, value)
			}
		}(i)

		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				key := "k" + string(rune(j%100+'0'))
				lru.Get(key)
				//v, _, ok := lru.Get(key)
				//if ok {
				//	fmt.Print("获取到", v, ", ")
				//}
			}
		}(i)
	}
	wg.Wait()
	//fmt.Println("\n共存在", lru.Len(), "个缓存数据")
}
