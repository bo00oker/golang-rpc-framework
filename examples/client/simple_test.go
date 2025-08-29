package main

import (
	"context"
	"fmt"
	"log"

	"github.com/rpc-framework/core/pkg/client"
	"github.com/rpc-framework/core/pkg/logger"
	"github.com/rpc-framework/core/pkg/registry"
	"github.com/rpc-framework/core/pkg/trace"
	"github.com/rpc-framework/core/proto/hello"
)

// SimpleTest 简单测试
func SimpleTest() {
	fmt.Println("=== Simple Multi-Service Test ===")

	// 创建简单的日志
	logger.SetGlobalLogger(&SimpleLogger{})

	// 创建简单的追踪器
	tracer, _ := trace.NewTracer(&trace.Config{
		ServiceName:    "test-client",
		ServiceVersion: "1.0.0",
		Environment:    "test",
		EnableConsole:  true,
	})

	// 创建客户端
	cli := client.NewClient(&client.ClientOptions{
		Tracer: tracer,
	})

	// 创建Nacos注册中心（这里只是示例，实际需要真实的Nacos服务）
	nacosReg, err := registry.NewNacosRegistry(&registry.NacosOptions{
		Endpoints:   []string{"127.0.0.1:8848"},
		NamespaceID: "public",
		Group:       "DEFAULT_GROUP",
		TimeoutMs:   5000,
	})
	if err != nil {
		log.Printf("Failed to create nacos registry: %v", err)
		return
	}

	// 尝试连接Hello服务
	ctx := context.Background()
	conn, err := cli.DialService(ctx, "hello.HelloService", nacosReg)
	if err != nil {
		log.Printf("Failed to dial hello service: %v", err)
		return
	}
	defer conn.Close()

	// 创建Hello服务客户端
	helloClient := hello.NewHelloServiceClient(conn)

	// 测试调用
	resp, err := helloClient.SayHello(ctx, &hello.HelloRequest{
		Name:    "TestUser",
		Message: "Hello from simple test!",
	})
	if err != nil {
		log.Printf("SayHello failed: %v", err)
		return
	}

	fmt.Printf("Hello response: %s\n", resp.Message)
	fmt.Println("✅ Simple test completed!")
}

// SimpleLogger 简单日志实现
type SimpleLogger struct{}

func (l *SimpleLogger) Debug(args ...interface{})                 { log.Println(args...) }
func (l *SimpleLogger) Debugf(format string, args ...interface{}) { log.Printf(format, args...) }
func (l *SimpleLogger) Info(args ...interface{})                  { log.Println(args...) }
func (l *SimpleLogger) Infof(format string, args ...interface{})  { log.Printf(format, args...) }
func (l *SimpleLogger) Warn(args ...interface{})                  { log.Println(args...) }
func (l *SimpleLogger) Warnf(format string, args ...interface{})  { log.Printf(format, args...) }
func (l *SimpleLogger) Error(args ...interface{})                 { log.Println(args...) }
func (l *SimpleLogger) Errorf(format string, args ...interface{}) { log.Printf(format, args...) }
func (l *SimpleLogger) Fatal(args ...interface{})                 { log.Fatal(args...) }
func (l *SimpleLogger) Fatalf(format string, args ...interface{}) { log.Fatalf(format, args...) }
func (l *SimpleLogger) WithField(key string, value interface{}) logger.Logger {
	return l
}
func (l *SimpleLogger) WithFields(fields map[string]interface{}) logger.Logger {
	return l
}

