package main

import (
	"FishCache/internal/cache"
	"flag"
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// 假定一个数据库
var testDB = map[string]string{
	"Tom":  "630",
	"Jack": "589",
	"Sam":  "567",
}

func createScoresGroup() *cache.Group {
	return cache.NewGroup("scores", 2<<10, cache.GetterFunc(
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
	var port int
	var peers []string
	flag.IntVar(&port, "port", 23333, "FishCache server port") // 服务运行端口
	flag.Func("peers", "A list of peers separated by commas", func(s string) error {
		// 将输入的字符串按照逗号分隔并转换为切片
		peers = strings.Split(s, ",")
		return nil
	})
	flag.Parse()
	// 日志初始化
	logInit()
	// 缓存组初始化
	var groups []*cache.Group
	scoresGroup := createScoresGroup()
	groups = append(groups, scoresGroup)
	// RPC服务初始化
	addr := "127.0.0.1:" + strconv.Itoa(port)
	svr, err := cache.NewRPCServer(addr)
	if err != nil {
		log.Fatalf("acquire grpc server instance failed, %v", err)
	}
	// 设置节点与缓存组的一致性
	svr.SetPeers(peers, groups)
	// 运行服务
	err = svr.Run()
	// grpcurl -plaintext -d "{\"group\": \"scores\", \"key\": \"Tom\"}" 127.0.0.1:23333 fishcache.CacheService/Get
	if err != nil {
		log.Fatalf("failed to run server: %v", err)
		return
	}
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
