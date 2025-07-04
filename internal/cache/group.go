package cache

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

var (
	mu           sync.RWMutex
	GroupManager = make(map[string]*Group) // 组管理器，包含多个group组
)

type Group struct {
	name   string         // 一个 Group 可以认为是一个缓存的命名空间，每个 Group 拥有一个唯一的名称 name
	cache  *Cache         // 缓存值
	getter Getter         //缓存未命中时获取源数据的回调(callback)
	peers  HashPeerPicker // 包含一致性哈希的节点选择器
	flight *SingleFlight  // 防止瞬时高并发的数据结构
}

func NewGroup(name string, maxBytes int64, getter Getter) *Group {
	if getter == nil {
		panic("getter is nil")
	}
	mu.Lock()
	defer mu.Unlock()

	cache, err := NewCache(maxBytes)
	if err != nil {
		panic(err)
	}

	group := &Group{
		name:   name,
		getter: getter,
		cache:  cache,
		flight: NewFlightGroup(5 * time.Second),
	}
	GroupManager[name] = group

	return group
}

// GetGroup 从组管理器中根据name获取group
func GetGroup(name string) *Group {
	mu.RLock()
	g := GroupManager[name]
	mu.RUnlock()
	return g
}

// RegisterPeers 注入哈希环到Group中
func (g *Group) RegisterPeers(peers HashPeerPicker) {
	//if g.peers != nil {
	//	panic("RegisterPeerPicker called more than once")
	//}
	g.peers = peers
}

// Get 从组中获取缓存数据
func (g *Group) Get(key string) (ByteView, error) {
	if key == "" {
		return ByteView{}, fmt.Errorf("key is empty")
	}
	// 从缓存中查找值
	if v, ok := g.cache.get(key); ok {
		return v, nil
	}
	// 不存在则该数据还没缓存到该内存服务器，调用load
	return g.load(key)
}

func (g *Group) load(key string) (ByteView, error) {
	// flight Do封装获取方法，避免高峰请求，实现类单例功能
	viewi, err := g.flight.Do(key, func() (interface{}, error) {
		if g.peers != nil {
			// 由一致性哈希环判断当前key所在的节点
			if peer, ok := g.peers.PickPeer(key); ok {
				// 从远程节点获取
				if value, err := g.getFromPeer(peer, key); err == nil {
					log.Printf("Load remote key: %s\n", key)
					return value, err
				} else {
					log.Fatalln(err)
				}
			}
		}

		return g.getLocally(key)
	})

	if err != nil {
		return ByteView{}, err
	}
	return viewi.(ByteView), nil
}

func (g *Group) getLocally(key string) (ByteView, error) {
	bytes, err := g.getter.Get(key)
	// 调用自定义的get方法
	if err != nil {
		return ByteView{}, err
	}

	value := ByteView{b: cloneBytes(bytes)}
	// 将源数据添加到缓存中
	g.cache.add(key, value)
	return value, nil
}

// 从远程grpc节点获取缓存
func (g *Group) getFromPeer(peer PeerGetter, key string) (ByteView, error) {
	bytes, err := peer.Get(g.name, key)
	if err != nil {
		return ByteView{}, err
	}
	return ByteView{b: bytes}, nil
}
