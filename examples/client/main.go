package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rpc-framework/core/pkg/client"
	"github.com/rpc-framework/core/pkg/config"
	"github.com/rpc-framework/core/pkg/logger"
	"github.com/rpc-framework/core/pkg/registry"
	"github.com/rpc-framework/core/pkg/trace"
	"github.com/rpc-framework/core/proto/hello"
)

func main() {
	// 加载配置
	cfg := config.New()
	if err := cfg.LoadFromFile("./configs/app.yaml"); err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 获取客户端配置
	clientCfg, err := cfg.GetClientConfig()
	if err != nil {
		fmt.Printf("Failed to get client config: %v\n", err)
		os.Exit(1)
	}

	// 获取日志配置
	logCfg, err := cfg.GetLogConfig()
	if err != nil {
		fmt.Printf("Failed to get log config: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	log, err := logger.NewLogger(&logger.Config{
		Level:      logCfg.Level,
		Format:     logCfg.Format,
		Output:     logCfg.Output,
		FilePath:   logCfg.Filename,
		MaxSize:    logCfg.MaxSize,
		MaxBackups: logCfg.MaxBackups,
		MaxAge:     logCfg.MaxAge,
		Compress:   logCfg.Compress,
	})
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	logger.SetGlobalLogger(log)

	// 获取追踪配置
	traceCfg, err := cfg.GetTraceConfig()
	if err != nil {
		log.Errorf("Failed to get trace config: %v", err)
		os.Exit(1)
	}

	// 初始化追踪器
	tracer, err := trace.NewTracer(&trace.Config{
		ServiceName:    traceCfg.ServiceName + "-client",
		ServiceVersion: traceCfg.ServiceVersion,
		Environment:    traceCfg.Environment,
		JaegerEndpoint: traceCfg.JaegerEndpoint,
		SampleRate:     traceCfg.SampleRate,
		EnableConsole:  traceCfg.EnableConsole,
	})
	if err != nil {
		log.Errorf("Failed to create tracer: %v", err)
		os.Exit(1)
	}
	defer tracer.Shutdown(context.Background())

	// 获取Nacos配置
	nacosCfg, err := cfg.GetNacosConfig()
	if err != nil {
		log.Errorf("Failed to get nacos config: %v", err)
		os.Exit(1)
	}

	// 创建Nacos注册中心
	nacosReg, err := registry.NewNacosRegistry(&registry.NacosOptions{
		Endpoints:   []string{nacosCfg.ServerAddr},
		NamespaceID: nacosCfg.Namespace,
		Group:       nacosCfg.Group,
		TimeoutMs:   uint64(nacosCfg.Timeout.Milliseconds()),
		Username:    nacosCfg.Username,
		Password:    nacosCfg.Password,
	})
	if err != nil {
		log.Errorf("Failed to create nacos registry: %v", err)
		os.Exit(1)
	}
	defer nacosReg.Close()

	// 创建客户端
	cli := client.NewClient(&client.ClientOptions{
		Timeout:         clientCfg.Timeout,
		KeepAlive:       clientCfg.KeepAlive,
		MaxRecvMsgSize:  clientCfg.MaxRecvMsgSize,
		MaxSendMsgSize:  clientCfg.MaxSendMsgSize,
		RetryAttempts:   clientCfg.RetryAttempts,
		RetryDelay:      clientCfg.RetryDelay,
		LoadBalanceType: clientCfg.LoadBalanceType,
		Tracer:          tracer,
	})
	defer cli.Close()

	// 测试模式选择
	testMode := "multi" // 默认测试多服务模式
	if len(os.Args) > 1 {
		testMode = os.Args[1]
	}

	switch testMode {
	case "hello":
		testHelloService(cli, nacosReg, tracer)
	case "user":
		testUserService(cli, nacosReg, tracer)
	case "order":
		testOrderService(cli, nacosReg, tracer)
	case "multi":
		testMultiServices(cli, nacosReg, tracer)
	default:
		log.Infof("Unknown test mode: %s, using multi-service test", testMode)
		testMultiServices(cli, nacosReg, tracer)
	}

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info("Client stopped")
}

// testHelloService 测试Hello服务
func testHelloService(cli *client.Client, nacosReg registry.Registry, tracer *trace.Tracer) {
	log := logger.GetGlobalLogger()
	log.Info("=== Testing Hello Service Only ===")

	// 服务名称
	serviceName := "hello.HelloService"

	// 按服务名拨号（自动发现+负载均衡+动态更新）
	conn, err := cli.DialService(context.Background(), serviceName, nacosReg)
	if err != nil {
		log.Errorf("Failed to dial service: %v", err)
		return
	}
	defer conn.Close()

	// 创建Hello服务客户端
	helloClient := hello.NewHelloServiceClient(conn)

	// 测试各种调用方式
	log.Info("Starting Hello service tests...")

	// 测试普通调用
	testSayHello(context.Background(), helloClient, tracer)

	// 等待一下
	time.Sleep(time.Second)

	// 测试流式调用
	testSayHelloStream(context.Background(), helloClient, tracer)

	// 等待一下
	time.Sleep(time.Second)

	// 测试双向流式调用
	testSayHelloBidirectional(context.Background(), helloClient, tracer)

	log.Info("Hello service tests completed!")
}

// testUserService 测试用户服务
func testUserService(cli *client.Client, nacosReg registry.Registry, tracer *trace.Tracer) {
	log := logger.GetGlobalLogger()
	log.Info("=== Testing User Service Only ===")

	// 创建用户服务客户端
	userClient, err := NewUserClient(cli, nacosReg, tracer)
	if err != nil {
		log.Errorf("Failed to create user client: %v", err)
		return
	}
	defer userClient.Close()

	// 测试用户服务
	userClient.TestUserService(context.Background())
}

// testOrderService 测试订单服务
func testOrderService(cli *client.Client, nacosReg registry.Registry, tracer *trace.Tracer) {
	log := logger.GetGlobalLogger()
	log.Info("=== Testing Order Service Only ===")

	// 创建订单服务客户端
	orderClient, err := NewOrderClient(cli, nacosReg, tracer)
	if err != nil {
		log.Errorf("Failed to create order client: %v", err)
		return
	}
	defer orderClient.Close()

	// 测试订单服务
	orderClient.TestOrderService(context.Background())
}

// testMultiServices 测试多服务
func testMultiServices(cli *client.Client, nacosReg registry.Registry, tracer *trace.Tracer) {
	log := logger.GetGlobalLogger()
	log.Info("=== Testing Multiple Services ===")

	// 创建多服务客户端
	multiClient, err := NewMultiServiceClient(cli, nacosReg, tracer)
	if err != nil {
		log.Errorf("Failed to create multi-service client: %v", err)
		return
	}
	defer multiClient.Close()

	// 测试所有服务
	multiClient.TestAllServices(context.Background())
}

// testSayHello 测试普通调用
func testSayHello(ctx context.Context, client hello.HelloServiceClient, tracer *trace.Tracer) {
	log := logger.GetGlobalLogger()
	log.Info("Testing SayHello...")

	// 创建自定义span
	ctx, span := tracer.StartSpan(ctx, "testSayHello")
	defer span.End()

	// 发送请求
	resp, err := client.SayHello(ctx, &hello.HelloRequest{
		Name:    "World",
		Message: "Hello from client!",
	})
	if err != nil {
		log.Errorf("SayHello failed: %v", err)
		return
	}

	log.Infof("SayHello response: %s", resp.Message)
}

// testSayHelloStream 测试流式调用
func testSayHelloStream(ctx context.Context, client hello.HelloServiceClient, tracer *trace.Tracer) {
	log := logger.GetGlobalLogger()
	log.Info("Testing SayHelloStream...")

	// 创建自定义span
	ctx, span := tracer.StartSpan(ctx, "testSayHelloStream")
	defer span.End()

	// 发送请求
	stream, err := client.SayHelloStream(ctx, &hello.HelloRequest{
		Name:    "Stream World",
		Message: "Hello from stream client!",
	})
	if err != nil {
		log.Errorf("SayHelloStream failed: %v", err)
		return
	}

	// 接收响应
	for {
		resp, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			log.Errorf("Failed to receive stream response: %v", err)
			return
		}
		log.Infof("Stream response: %s", resp.Message)
	}
}

// testSayHelloBidirectional 测试双向流式调用
func testSayHelloBidirectional(ctx context.Context, client hello.HelloServiceClient, tracer *trace.Tracer) {
	log := logger.GetGlobalLogger()
	log.Info("Testing SayHelloBidirectional...")

	// 创建自定义span
	ctx, span := tracer.StartSpan(ctx, "testSayHelloBidirectional")
	defer span.End()

	// 创建双向流
	stream, err := client.SayHelloBidirectional(ctx)
	if err != nil {
		log.Errorf("SayHelloBidirectional failed: %v", err)
		return
	}

	// 发送多个请求
	messages := []string{"First", "Second", "Third", "Fourth", "Fifth"}
	for _, msg := range messages {
		req := &hello.HelloRequest{
			Name:    "Bidirectional World",
			Message: msg,
		}
		if err := stream.Send(req); err != nil {
			log.Errorf("Failed to send bidirectional request: %v", err)
			return
		}
		log.Infof("Sent: %s", msg)
	}

	// 关闭发送
	if err := stream.CloseSend(); err != nil {
		log.Errorf("Failed to close send: %v", err)
		return
	}

	// 接收响应
	for {
		resp, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			log.Errorf("Failed to receive bidirectional response: %v", err)
			return
		}
		log.Infof("Bidirectional response: %s", resp.Message)
	}
}
