package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/rpc-framework/core/internal/order/handler"
	"github.com/rpc-framework/core/internal/order/repository"
	"github.com/rpc-framework/core/internal/order/service"
	"github.com/rpc-framework/core/pkg/config"
	"github.com/rpc-framework/core/pkg/logger"
	"github.com/rpc-framework/core/pkg/registry"
	"github.com/rpc-framework/core/pkg/server"
	"github.com/rpc-framework/core/pkg/trace"
	"github.com/rpc-framework/core/proto/order"
)

const ServiceName = "order.OrderService"

func main() {
	// 加载配置
	cfg := config.New()
	if err := cfg.LoadFromFile("./configs/app.yaml"); err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 获取服务器配置
	serverCfg, err := cfg.GetServerConfig()
	if err != nil {
		fmt.Printf("Failed to get server config: %v\n", err)
		os.Exit(1)
	}

	// 修改端口以避免冲突
	serverCfg.Address = ":50052"

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
		ServiceName:    ServiceName,
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

	// 创建服务器
	srv := server.New(&server.Options{
		Address:        serverCfg.Address,
		MaxRecvMsgSize: serverCfg.MaxRecvMsgSize,
		MaxSendMsgSize: serverCfg.MaxSendMsgSize,
		Tracer:         tracer,

		// 高并发优化配置
		MaxConcurrentRequests: serverCfg.MaxConcurrentRequests,
		RequestTimeout:        serverCfg.RequestTimeout,
		MaxConnections:        serverCfg.MaxConnections,
		ConnectionTimeout:     serverCfg.ConnectionTimeout,
		KeepAliveTime:         serverCfg.KeepAliveTime,
		KeepAliveTimeout:      serverCfg.KeepAliveTimeout,
		RateLimit:             serverCfg.RateLimit,
		EnableMetrics:         serverCfg.EnableMetrics,

		// 内存池配置
		EnableMemoryPool:  serverCfg.EnableMemoryPool,
		MemoryPoolSize:    serverCfg.MemoryPoolSize,
		MemoryPoolMaxSize: serverCfg.MemoryPoolMaxSize,

		// 异步处理配置
		EnableAsync:      serverCfg.EnableAsync,
		AsyncWorkerCount: serverCfg.AsyncWorkerCount,
		AsyncQueueSize:   serverCfg.AsyncQueueSize,

		// 健康检查配置
		EnableHealthCheck:   serverCfg.EnableHealthCheck,
		HealthCheckInterval: serverCfg.HealthCheckInterval,
		HealthCheckTimeout:  serverCfg.HealthCheckTimeout,
	})

	// 创建依赖实例
	orderRepo := repository.NewMemoryOrderRepository()
	orderSvc := service.NewOrderService(orderRepo, tracer)
	orderHandler := handler.NewOrderHandler(orderSvc, tracer)

	// 注册服务
	srv.RegisterService(&order.OrderService_ServiceDesc, orderHandler)

	// 启动服务器
	go func() {
		if err := srv.Start(); err != nil {
			log.Errorf("Failed to start server: %v", err)
			os.Exit(1)
		}
	}()

	// 注册服务到注册中心
	listener, err := net.Listen("tcp", serverCfg.Address)
	if err != nil {
		log.Errorf("Failed to listen: %v", err)
		os.Exit(1)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	serviceInfo := &registry.ServiceInfo{
		ID:      fmt.Sprintf("%s-%d", ServiceName, port),
		Name:    ServiceName,
		Address: "127.0.0.1",
		Port:    port,
		Metadata: map[string]string{
			"version":  "1.0.0",
			"protocol": "grpc",
			"weight":   "100",
		},
	}

	if err := nacosReg.Register(context.Background(), serviceInfo); err != nil {
		log.Errorf("Failed to register service: %v", err)
		os.Exit(1)
	}

	log.Infof("Order service started successfully on %s", serverCfg.Address)
	log.Infof("Service registered: %s", serviceInfo.ID)

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutting down order service...")

	// 注销服务
	if err := nacosReg.Deregister(context.Background(), serviceInfo.ID); err != nil {
		log.Errorf("Failed to deregister service: %v", err)
	}

	// 停止服务器
	srv.Stop()

	log.Info("Order service stopped")
}
