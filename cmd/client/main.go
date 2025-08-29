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

		// 高并发优化配置
		MaxConnections:          clientCfg.MaxConnections,
		MaxIdleConns:            clientCfg.MaxIdleConns,
		ConnTimeout:             clientCfg.ConnTimeout,
		IdleTimeout:             clientCfg.IdleTimeout,
		MaxRetries:              clientCfg.MaxRetries,
		RetryBackoff:            clientCfg.RetryBackoff,
		CircuitBreakerThreshold: clientCfg.CircuitBreakerThreshold,
		CircuitBreakerTimeout:   clientCfg.CircuitBreakerTimeout,

		// 缓存配置
		EnableCache:  clientCfg.EnableCache,
		CacheTTL:     clientCfg.CacheTTL,
		CacheMaxSize: clientCfg.CacheMaxSize,

		// 异步处理配置
		EnableAsync:      clientCfg.EnableAsync,
		AsyncWorkerCount: clientCfg.AsyncWorkerCount,
		AsyncQueueSize:   clientCfg.AsyncQueueSize,
	})
	defer cli.Close()

	// 测试模式选择
	testMode := "all" // 默认测试所有服务
	if len(os.Args) > 1 {
		testMode = os.Args[1]
	}

	switch testMode {
	case "user":
		testUserService(cli, nacosReg, tracer)
	case "order":
		testOrderService(cli, nacosReg, tracer)
	case "all":
		testAllServices(cli, nacosReg, tracer)
	default:
		log.Infof("Unknown test mode: %s, using all services test", testMode)
		testAllServices(cli, nacosReg, tracer)
	}

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info("Client stopped")
}

// testUserService 测试用户服务
func testUserService(cli *client.Client, nacosReg registry.Registry, tracer *trace.Tracer) {
	log := logger.GetGlobalLogger()
	log.Info("=== Testing User Service ===")

	userClient, err := NewUserServiceClient(cli, nacosReg, tracer)
	if err != nil {
		log.Errorf("Failed to create user client: %v", err)
		return
	}
	defer userClient.Close()

	userClient.TestUserOperations(context.Background())
}

// testOrderService 测试订单服务
func testOrderService(cli *client.Client, nacosReg registry.Registry, tracer *trace.Tracer) {
	log := logger.GetGlobalLogger()
	log.Info("=== Testing Order Service ===")

	orderClient, err := NewOrderServiceClient(cli, nacosReg, tracer)
	if err != nil {
		log.Errorf("Failed to create order client: %v", err)
		return
	}
	defer orderClient.Close()

	orderClient.TestOrderOperations(context.Background())
}

// testAllServices 测试所有服务
func testAllServices(cli *client.Client, nacosReg registry.Registry, tracer *trace.Tracer) {
	log := logger.GetGlobalLogger()
	log.Info("=== Testing All Services ===")

	// 测试用户服务
	log.Info("--- Testing User Service ---")
	testUserService(cli, nacosReg, tracer)

	time.Sleep(2 * time.Second)

	// 测试订单服务
	log.Info("--- Testing Order Service ---")
	testOrderService(cli, nacosReg, tracer)

	time.Sleep(2 * time.Second)

	// 测试跨服务场景
	log.Info("--- Testing Cross-Service Scenarios ---")
	testCrossServiceScenarios(cli, nacosReg, tracer)
}

// testCrossServiceScenarios 测试跨服务场景
func testCrossServiceScenarios(cli *client.Client, nacosReg registry.Registry, tracer *trace.Tracer) {
	log := logger.GetGlobalLogger()
	log.Info("Testing cross-service scenarios...")

	// 创建客户端
	userClient, err := NewUserServiceClient(cli, nacosReg, tracer)
	if err != nil {
		log.Errorf("Failed to create user client: %v", err)
		return
	}
	defer userClient.Close()

	orderClient, err := NewOrderServiceClient(cli, nacosReg, tracer)
	if err != nil {
		log.Errorf("Failed to create order client: %v", err)
		return
	}
	defer orderClient.Close()

	// 跨服务测试场景：创建用户后创建订单
	ctx := context.Background()

	// 1. 创建用户
	log.Info("Step 1: Creating a new user...")
	userID, err := userClient.CreateTestUser(ctx, "cross_service_user", "cross@example.com")
	if err != nil {
		log.Errorf("Failed to create user: %v", err)
		return
	}
	log.Infof("User created with ID: %d", userID)

	// 2. 为该用户创建订单
	log.Info("Step 2: Creating orders for the user...")
	orderID, err := orderClient.CreateTestOrder(ctx, userID, 100.50, "Cross-service test order")
	if err != nil {
		log.Errorf("Failed to create order: %v", err)
		return
	}
	log.Infof("Order created with ID: %d", orderID)

	// 3. 查询用户的订单
	log.Info("Step 3: Querying user's orders...")
	orders, err := orderClient.GetOrdersByUser(ctx, userID)
	if err != nil {
		log.Errorf("Failed to get user orders: %v", err)
		return
	}
	log.Infof("Found %d orders for user %d", len(orders), userID)

	log.Info("Cross-service scenario test completed successfully!")
}
