package cache

import "time"

// ByteView 抽象一个只读数据结构 ByteView 用来表示缓存值
type ByteView struct {
	b        []byte    // 选择 byte 类型是为了能够支持任意的数据类型的存储，例如字符串、图片等。
	expireAt time.Time // 过期时间，零值表示永不过期
}

// Len 实现缓存对象中必须实现的Value的接口，返回其所占的内存大小
func (v ByteView) Len() int {
	return len(v.b)
}

// ByteSlice 深拷贝，防止缓存值被外部程序修改
func (v ByteView) ByteSlice() []byte {
	return cloneBytes(v.b)
}

func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}

// String 返回字符串类型的value
func (v ByteView) String() string {
	return string(v.b)
}

// IsExpired 检查值是否已过期
func (v ByteView) IsExpired() bool {
	// 零值时间表示永不过期
	// expireAt < now 即 time.Now() 在 expireAt 之后，表示已过期，返回true
	return !v.expireAt.IsZero() && time.Now().After(v.expireAt)
}
