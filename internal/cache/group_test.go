package cache

import (
	"fmt"
	"log"
	"testing"
)

// 假定一个数据库
var db = map[string]string{
	"Tom":  "630",
	"Jack": "589",
	"Sam":  "567",
}

func TestGroup_basic(t *testing.T) {

	var getFromLocal Getter = GetterFunc(func(key string) ([]byte, error) {
		log.Println("[testDB] local load key", key)
		if value, exists := db[key]; exists {
			return []byte(value), nil
		}
		return nil, fmt.Errorf("%s not exist", key)
	})
	mygrp := NewGroup("testGroup", 2<<10, getFromLocal)

	// 测试一下group实例查询缓存
	if _, err := mygrp.Get("Tom"); err != nil {
		t.Log(err)
	}
	if _, err := mygrp.Get("Jack"); err != nil {
		t.Log(err)
	}
	if _, err := mygrp.Get("Sam"); err != nil {
		t.Log(err)
	}
	// 查找数据库中不存在的数据
	if _, err := mygrp.Get("Tam"); err != nil {
		t.Log(err)
	}
	// 第二次Get不需要再从数据库中获取，直接走缓存
	if _, err := mygrp.Get("Tom"); err != nil {
		t.Log(err)
	}
}
