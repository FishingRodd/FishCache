package main

import (
	"FishCache/internal/cache"
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"runtime"
)

// 假定一个数据库
var testDB = map[string]string{
	"Tom":  "630",
	"Jack": "589",
	"Sam":  "567",
}

func createGroup() *cache.Group {
	return cache.NewGroup("scores", 2<<10, cache.GetterFunc(
		func(key string) ([]byte, error) {
			if value, exists := testDB[key]; exists {
				log.Printf("[testDB] load key %s\n", key)
				return []byte(value), nil
			}
			log.Printf("[testDB] load key %s failed\n", key)
			return nil, fmt.Errorf("%s not exist", key)
		}),
	)
}

func main() {
	logInit()
	createGroup()
	addr := "localhost:23333"
	svr, err := cache.NewRPCServer(addr)
	if err != nil {
		log.Fatalf("acquire grpc server instance failed, %v", err)
	}
	//svr.SetPeers()

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
