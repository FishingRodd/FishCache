package etcd

import (
	"FishCache/consistent"
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/naming/endpoints"
)

// Register 函数用于注册指定服务的地址 addr。
// 在正常服务提供期间，该函数不会返回。只有在应用程序停止、租约续订失败或 etcd 连接丢失时才会返回。
func Register(stop chan error, registerAddress string, update chan struct{}) error {
	// 创建 etcd 客户端
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   consistent.Conf.Etcd.Address,
		DialTimeout: consistent.Conf.Etcd.Timeout,
	})
	if err != nil {
		log.Fatalf("err: %v", err)
		return err
	}

	// 创建一个 5 秒的租约
	ctx, cancel := context.WithTimeout(context.Background(), consistent.Conf.Etcd.Timeout)
	defer cancel()
	leaseGrantResp, err := cli.Grant(ctx, 5)
	if err != nil {
		return fmt.Errorf("grant creates a new lease failed: %v", err)
	}
	leaseId := leaseGrantResp.ID // 获取租约ID
	//log.Infof("租约 ID (十六进制): %x\n", leaseId)

	// 将服务地址与租约关联，如果租约过期，将从 etcd 中删除服务地址信息
	err = etcdAddEndpoint(cli, leaseId, registerAddress)
	if err != nil {
		return fmt.Errorf("failed to add services as endpoint to etcd endpoint Manager: %v", err)
	}

	// KeepAlive 尝试保持租约有效
	alive, err := cli.KeepAlive(context.Background(), leaseId)
	if err != nil {
		return fmt.Errorf("set keepalive for lease failed: %v", err)
	}

	// 注册成功
	update <- struct{}{} // 通知一次更新

	// 监测停止信号、etcd 客户端状态和租约保持响应
	for {
		select {
		case err = <-stop: // 应用级停止信号
			return err
		case <-cli.Ctx().Done(): // etcd 客户端断开
			return fmt.Errorf("etcd client connect broken")
		case _, ok := <-alive: // 租约保持响应
			if !ok {
				log.Error("keepalive channel closed, revoke given lease")
				// 从 etcd 删除端点
				if err = etcdDelEndpoint(cli, registerAddress); err != nil {
					log.Errorf("Failed to delete endpoint: %v", err)
				}
				return fmt.Errorf("keepalive channel closed, revoke given lease")
			}
		default:
			time.Sleep(2 * time.Second) // 防止资源过度使用
		}
	}
}

// etcdAddEndpoint 函数将服务的注册信息存储在 etcd 中，键的形式为 {service}/{addr}，值的形式为 {addr, metadata}。
func etcdAddEndpoint(client *clientv3.Client, leaseId clientv3.LeaseID, address string) error {
	endpointsManager, err := endpoints.NewManager(client, consistent.Conf.Etcd.ServiceName)
	if err != nil {
		return err
	}

	// 使用字符串元数据确保可比性
	metadata := endpoints.Endpoint{
		Addr:     address,
		Metadata: "weight:10;version:v1.0.0",
	}

	// 将服务地址和元数据添加到 etcd
	return endpointsManager.AddEndpoint(context.TODO(),
		fmt.Sprintf("%s/%s", consistent.Conf.Etcd.ServiceName, address), // 键的形式
		metadata,
		clientv3.WithLease(leaseId)) // 绑定租约
}

// etcdDelEndpoint 函数从 etcd 删除指定服务的地址
func etcdDelEndpoint(client *clientv3.Client, address string) error {
	endpointsManager, err := endpoints.NewManager(client, consistent.Conf.Etcd.ServiceName)
	if err != nil {
		return err
	}
	// 根据键 ({service}/{address}) 删除端点
	return endpointsManager.DeleteEndpoint(client.Ctx(), fmt.Sprintf("%s/%s", consistent.Conf.Etcd.ServiceName, address), nil)
}
