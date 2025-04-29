package cache

import (
	pb "FishCache/api/groupcachepb"
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"time"
)

// HashPeerPicker 用于根据传入的 key 选择相应节点。
type HashPeerPicker interface {
	PickPeer(key string) (peer PeerGetter, ok bool)
}

// PeerGetter 用于从对应 group 查找缓存值。
type PeerGetter interface {
	Get(group string, key string) ([]byte, error)
}

type grpcGetter struct {
	addr string
}

func (g *grpcGetter) Get(group string, key string) ([]byte, error) {
	// 创建一个带有超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// 设置连接选项
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()), // 显式使用不安全的凭证
	}
	// 调用 NewClient 创建连接
	conn, _ := grpc.NewClient(g.addr, opts...)
	defer func() {
		err := conn.Close()
		if err != nil {
			log.Errorf("grpc connection close error: %s", err.Error())
		}
	}()

	grpcClient := pb.NewCacheServiceClient(conn)
	resp, err := grpcClient.Get(ctx, &pb.GetRequest{
		Group: group,
		Key:   key,
	})
	if err != nil {
		return nil, fmt.Errorf("could not get %s/%s from peer %s", group, key, g.addr)
	}

	return resp.Value, nil
}
