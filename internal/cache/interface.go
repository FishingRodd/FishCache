package cache

// Getter 用于加载指定键的数据。
type Getter interface {
	Get(key string) ([]byte, error) // Get 方法接受一个字符串类型的键，并返回相应的数据和可能发生的错误
}

// GetterFunc 类型实现了 Getter 接口，通过一个函数来实现。
type GetterFunc func(key string) ([]byte, error) // GetterFunc 是一个函数类型，符合 Getter 接口

// Get 实现了 Getter 接口中的函数
func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key) // 调用 GetterFunc 类型的函数 f，并传入键，返回数据和错误
}
