package etcd

import (
	"FishCache/consistent"
	"context"
	log "github.com/sirupsen/logrus"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/naming/endpoints"
)

// ListServicePeers 根据服务名称从服务注册中心获取可用服务节点列表
func ListServicePeers() ([]string, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   consistent.Conf.Etcd.Address,
		DialTimeout: consistent.Conf.Etcd.Timeout,
	})
	if err != nil {
		log.Errorf("连接etcd失败，错误: %v", err)
		return []string{}, err
	}

	// Endpoints 实际上是 ip:port 的组合，也可以视为 Unix 中的 socket。
	endpointsManager, err := endpoints.NewManager(cli, consistent.Conf.Etcd.ServiceName)
	if err != nil {
		log.Errorf("创建端点管理器失败，%v", err)
		return []string{}, err
	}

	// List 返回当前服务的所有端点，形式为一个映射
	ctx, cancel := context.WithTimeout(context.Background(), consistent.Conf.Etcd.Timeout)
	defer cancel()

	Key2EndpointMap, err := endpointsManager.List(ctx)
	if err != nil {
		log.Errorf("获取目标服务的端点节点列表失败，错误: %s", err.Error())
		return []string{}, err
	}

	var peersAddr []string
	for _, endpoint := range Key2EndpointMap {
		peersAddr = append(peersAddr, endpoint.Addr) // Addr 是将要建立连接的服务器地址
		//log.Infof("找到端点地址: %s (%s):(%v)", key, endpoint.Addr, endpoint.Metadata)
	}

	return peersAddr, nil
}

// DynamicServices 提供动态构建全局哈希视图的能力
// 便于缓存系统的二级视图收敛
func DynamicServices(update chan struct{}) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   consistent.Conf.Etcd.Address,
		DialTimeout: consistent.Conf.Etcd.Timeout,
	})
	if err != nil {
		log.Errorf("连接etcd失败，错误: %v", err)
		return
	}
	defer cli.Close()
	watchChan := cli.Watch(context.Background(), consistent.Conf.Etcd.ServiceName, clientv3.WithPrefix())

	// 每当用户向指定服务添加或删除实例地址时，watchChan 后台守护进程
	// 可以通过 WithPrefix() 扫描实例数量的变化，并将其作为 watchResp.Events 事件返回
	for watchResp := range watchChan {
		for _, event := range watchResp.Events {
			switch event.Type {
			case clientv3.EventTypePut:
				// 处理添加或更新事件
				//log.Warnf("Put: %s %s\n", string(event.Kv.Key), string(event.Kv.Value))
				update <- struct{}{}
			case clientv3.EventTypeDelete:
				// 处理删除事件
				//log.Warnf("Delete: %s\n", string(event.Kv.Key))
				update <- struct{}{}
			}
		}
	}
}
