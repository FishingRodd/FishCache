package eviction

import "time"

// Entry 缓存的基础元数据
type Entry struct {
	key      string    //键
	value    Value     //值
	updateAt time.Time // 上次访问或修改该值的时间
}

// Value 支持的方法
type Value interface {
	Len() int // 返回值所占用的内存大小
}

// UpdatedTTLTime 更新time to live时间
func (e *Entry) UpdatedTTLTime() {
	e.updateAt = time.Now()
}

// Expired 判断该缓存是否过期
func (e *Entry) Expired(duration time.Duration) bool {
	// 未设置过期时间则永不过期
	if e.updateAt.IsZero() {
		return false
	}
	// 如果缓存的updateAt，加上TTL时间后已经过去了当前时间，则返回true
	// 表示这个条目已经过期 (即: 已经过去了TTL分钟)
	return e.updateAt.Add(duration).Before(time.Now())
}
