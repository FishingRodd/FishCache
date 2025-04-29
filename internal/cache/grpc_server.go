package cache

import (
	pb "FishCache/api/groupcachepb"
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"net"
	"strings"
	"sync"
)

const (
	defaultRpcAddr           = "127.0.0.1:23333" // 默认地址
	defaultRpcClientReplicas = 50                // 默认副本数
)

// Server 服务器为分布式缓存提供基于gRPC的点对点通信。
type Server struct {
	pb.UnimplementedCacheServiceServer // 嵌入未实现的 gRPC 服务器接口

	address     string                 // 服务器地址
	isRunning   bool                   // 服务器运行状态
	mu          sync.RWMutex           // 读写锁，保护并发访问
	consistHash *ConsistentMap         // 一致性哈希映射
	clients     map[string]*grpcGetter // 每一个远程节点对应一个 client
}

func NewRPCServer(address string) (*Server, error) {
	if address == "" {
		address = defaultRpcAddr
	}

	//if !validate.ValidPeerAddr(addr) { // 验证地址格式
	//	return nil, fmt.Errorf("invalid peer address %s", addr)
	//}

	return &Server{address: address}, nil
}

// Get 作为server根据client请求的 group name 和 key 返回对应缓存数据
func (s *Server) Get(_ context.Context, req *pb.GetRequest) (*pb.GetResponse, error) {
	group := GetGroup(req.Group)
	if group == nil {
		return &pb.GetResponse{}, fmt.Errorf("group name is nil")
	}
	// 从缓存组中获取指定key的值
	view, err := group.Get(req.Key)
	if err != nil {
		return &pb.GetResponse{}, fmt.Errorf("search %s error: %v", req.Key, err)
	}

	value := view.ByteSlice()
	return &pb.GetResponse{
		Value: value,
	}, nil
}

// 初始化服务器
func (s *Server) initServer() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		return fmt.Errorf("server already running")
	}

	s.isRunning = true
	return nil
}

// 设置监听器
func (s *Server) setupListener() (net.Listener, error) {
	// 从地址中提取端口号
	port := strings.Split(s.address, ":")[1]
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on port %s: %w", port, err)
	}
	return lis, nil
}

// 设置gRPC服务器
func (s *Server) setupGRPCServer() *grpc.Server {
	// 创建新的gRPC服务器
	grpcServer := grpc.NewServer()
	// 注册缓存服务
	pb.RegisterCacheServiceServer(grpcServer, s)
	reflection.Register(grpcServer)
	return grpcServer
}

// 启动gRPC服务器并处理请求
func (s *Server) serveRequests(grpcServer *grpc.Server, lis net.Listener) error {
	// 启动gRPC服务器，监听请求
	if err := grpcServer.Serve(lis); err != nil && s.isRunning {
		// 如果服务器启动失败且当前仍在运行，返回错误
		return fmt.Errorf("failed to serve on %s: %w", s.address, err)
	}
	return nil
}

// Run 启动服务器的主逻辑
func (s *Server) Run() error {
	// 初始化服务器
	if err := s.initServer(); err != nil {
		return fmt.Errorf("failed to initialize server: %w", err)
	}
	// 设置监听器
	lis, err := s.setupListener()
	if err != nil {
		return fmt.Errorf("failed to setup listener: %w", err)
	}
	// 设置gRPC服务器
	grpcServer := s.setupGRPCServer()
	// 启动服务器并处理请求
	log.Infof("Run grpc server on %s", s.address)
	if err = s.serveRequests(grpcServer, lis); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}

	return nil
}

// SetPeers 设置客户端节点在哈希环中的位置
func (s *Server) SetPeers(peers []string, groups []*Group) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(peers) == 0 {
		peers = []string{s.address}
	}
	nodes := append(peers, s.address) // 哈希环加入自身

	s.consistHash = NewConsistentHash(defaultRpcClientReplicas, nil)
	s.consistHash.AddNodes(nodes...)
	s.clients = make(map[string]*grpcGetter, len(peers))
	for _, peerAddress := range peers {
		// 根据传入的节点，为每一个node创建一个grpc客户端
		s.clients[peerAddress] = &grpcGetter{addr: peerAddress}
	}
	for _, group := range groups {
		group.RegisterPeers(s)
	}
}

// PickPeer 根据具体的 key，选择节点
func (s *Server) PickPeer(key string) (PeerGetter, bool) {
	if key == "" {
		return nil, false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	peer := s.consistHash.GetNode(key)
	if peer == "" {
		return nil, false
	}
	// 目前暂时统一规定传入的客户端节点名称为address（因为存在一个可能导致哈希环不一致的问题，即主机网卡可以存在不同IP的表现，而两个node中保存的IP不同会导致部分hash结果不一致，而死循环）
	// 判断当前选择的节点为自身时返回nil
	if peer == s.address {
		return nil, false
	}
	return s.clients[peer], true
}
