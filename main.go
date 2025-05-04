package main

import (
	"FishCache/consistent"
	"FishCache/internal/cache"
	"flag"
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// 假定一个数据库
var testDB = map[string]string{
	"Tom":  "630",
	"Jack": "589",
	"Sam":  "567",
}

func createScoresGroup() {
	cache.NewGroup("scores", 2<<10, cache.GetterFunc(
		func(key string) ([]byte, error) {
			if value, exists := testDB[key]; exists {
				log.Printf("Load local key: %s\n", key)
				return []byte(value), nil
			}
			log.Printf("Load local key: %s failed\n", key)
			return []byte(""), nil
		}),
	)
}

func main() {
	// 外部传参
	var addr string            // 服务运行地址 ip:port
	var peers []string         // 邻居节点，使用","分割
	var etcdServersIP []string // etcd服务地址，使用","分割
	var etcdServiceName string
	flag.Func("peers", "A list of peers separated by commas", func(s string) error {
		peers = strings.Split(s, ",")
		return nil
	})
	flag.Func("etcd", "register etcd server", func(s string) error {
		etcdServersIP = strings.Split(s, ",")
		return nil
	})
	flag.StringVar(&addr, "host", "", "FishCache node server host")
	flag.StringVar(&etcdServiceName, "service", "", "service name")
	flag.Parse()

	// 目前支持手动设置peers和etcd注册发现模式
	if len(peers) == 0 && len(etcdServersIP) == 0 {
		log.Errorf("请 手动设置邻居 或 传递etcdIP获取邻居\n")
		return
	}
	// 设置节点的通信源IP端口
	if addr == "" {
		log.Fatalf("FishCache node server host is empty")
		return
	}
	// 服务名称，在etcd中key的prefix体现
	if etcdServiceName == "" {
		etcdServiceName = consistent.DefaultServiceName
	}

	// 日志初始化
	logInit()

	// 缓存组初始化
	createScoresGroup()

	// RPC服务初始化
	svr, err := cache.NewRPCServer(addr)
	if err != nil {
		log.Fatalf("acquire grpc server instance failed, %v", err)
	}

	// 设置节点与缓存组的一致性
	if len(peers) != 0 {
		svr.SetPeers(peers)
	}

	// 初始化服务器
	if err = svr.InitServer(); err != nil {
		log.Fatalf("failed to initialize server: %v", err)
		return
	}
	// 发起服务注册，并定义服务停止时行为
	if len(etcdServersIP) != 0 {
		// 定义consistent中配置信息
		consistent.Conf = &consistent.Config{
			Etcd: &consistent.Etcd{
				Address:     etcdServersIP,
				Timeout:     5 * time.Second,
				ServiceName: etcdServiceName,
			},
		}
		go func() {
			defer func() {
				if svr != nil {
					if err = svr.StopServer(); err != nil {
						log.Errorf("Failed to stop server: %v", err)
					}
				}
			}()
			svr.RegisterEtcd()
		}()
	}
	// 运行服务
	err = svr.RunServer()
	if err != nil {
		log.Fatalf("failed to run server: %v", err)
		return
	}
	// grpcurl -plaintext -d "{\"group\": \"scores\", \"key\": \"Tom\"}" 127.0.0.1:23333 fishcache.CacheService/Get
}

func logInit() {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.InfoLevel)
	log.SetReportCaller(true) // 启用调用者信息
	log.SetFormatter(&log.TextFormatter{
		TimestampFormat: "2006-01-02 15:04:05", // 等价于 %(asctime)s
		FullTimestamp:   true,
		CallerPrettyfier: func(f *runtime.Frame) (string, string) { // 处理调用者信息
			filename := filepath.Base(f.File)     // 获取文件名
			funcName := filepath.Base(f.Function) // 获取函数名
			// 格式化为 [filename:行号:funcName]
			return "", fmt.Sprintf(" [%s:%d:%s]", filename, f.Line, funcName)
		},
		// 自定义格式
		ForceColors:  true,
		ForceQuote:   true,
		DisableQuote: false,
	})
}
