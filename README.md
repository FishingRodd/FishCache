# 介绍

基于 Go 1.23.0 实现的高可用、分布式缓存应用。

支持：

1. LRU缓存淘汰算法、缓存TTL机制
2. consistenthash一致性哈希、负载均衡
3. gRPC协议进行节点间传输
4. etcd服务注册与发现、动态节点管理
5. 并发访问控制、singleFlight

# 获取

```
git clone https://github.com/FishingRodd/FishCache.git
```

# 使用

在`11.0.1.1:23333`启动一个缓存节点，向`11.0.1.111:2379`的etcd服务发起注册

```
go run main.go -host 11.0.1.1:23333 -etcd 11.0.1.111:2379
```

