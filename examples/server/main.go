package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rpc-framework/core/pkg/config"
	"github.com/rpc-framework/core/pkg/logger"
	"github.com/rpc-framework/core/pkg/registry"
	"github.com/rpc-framework/core/pkg/server"
	"github.com/rpc-framework/core/pkg/trace"
	"github.com/rpc-framework/core/proto/hello"
	"github.com/rpc-framework/core/proto/order"
	"github.com/rpc-framework/core/proto/user"
	"google.golang.org/grpc"
)

// HelloServiceImpl Hello服务实现
type HelloServiceImpl struct {
	hello.UnimplementedHelloServiceServer
	tracer *trace.Tracer
}

// NewHelloService 创建Hello服务实例
func NewHelloService(tracer *trace.Tracer) *HelloServiceImpl {
	return &HelloServiceImpl{
		tracer: tracer,
	}
}

// SayHello 实现SayHello方法
func (s *HelloServiceImpl) SayHello(ctx context.Context, req *hello.HelloRequest) (*hello.HelloResponse, error) {
	// 创建自定义span
	ctx, span := s.tracer.StartSpan(ctx, "SayHello")
	defer span.End()

	log := logger.GetGlobalLogger()
	log.Infof("Received SayHello request: %s", req.Name)

	return &hello.HelloResponse{
		Message: fmt.Sprintf("Hello, %s! Your message: %s", req.Name, req.Message),
	}, nil
}

// SayHelloStream 实现SayHelloStream方法
func (s *HelloServiceImpl) SayHelloStream(req *hello.HelloRequest, stream grpc.ServerStreamingServer[hello.HelloResponse]) error {
	// 创建自定义span
	_, span := s.tracer.StartSpan(stream.Context(), "SayHelloStream")
	defer span.End()

	log := logger.GetGlobalLogger()
	log.Infof("Received SayHelloStream request: %s", req.Name)

	// 发送多个响应
	for i := 0; i < 5; i++ {
		response := &hello.HelloResponse{
			Message: fmt.Sprintf("Stream Hello %d, %s! Your message: %s", i+1, req.Name, req.Message),
		}

		if err := stream.Send(response); err != nil {
			log.Errorf("Failed to send stream response: %v", err)
			return err
		}
	}

	return nil
}

// SayHelloBidirectional 实现SayHelloBidirectional方法
func (s *HelloServiceImpl) SayHelloBidirectional(stream grpc.BidiStreamingServer[hello.HelloRequest, hello.HelloResponse]) error {
	// 创建自定义span
	_, span := s.tracer.StartSpan(stream.Context(), "SayHelloBidirectional")
	defer span.End()

	log := logger.GetGlobalLogger()
	log.Info("Started bidirectional stream")

	for {
		// 接收请求
		req, err := stream.Recv()
		if err != nil {
			if err.Error() == "EOF" {
				log.Info("Client closed stream")
				break
			}
			log.Errorf("Failed to receive request: %v", err)
			return err
		}

		log.Infof("Received bidirectional request: %s", req.Name)

		// 发送响应
		response := &hello.HelloResponse{
			Message: fmt.Sprintf("Bidirectional Hello, %s! Your message: %s", req.Name, req.Message),
		}

		if err := stream.Send(response); err != nil {
			log.Errorf("Failed to send bidirectional response: %v", err)
			return err
		}
	}

	return nil
}

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
		ServiceName:    traceCfg.ServiceName,
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

	// 创建服务器
	srv := server.New(&server.Options{
		Address:        serverCfg.Address,
		MaxRecvMsgSize: serverCfg.MaxRecvMsgSize,
		MaxSendMsgSize: serverCfg.MaxSendMsgSize,
		Tracer:         tracer,
	})

	// 注册Hello服务
	helloService := NewHelloService(tracer)
	srv.RegisterService(&hello.HelloService_ServiceDesc, helloService)

	// 注册User服务
	userService := NewUserService(tracer)
	srv.RegisterService(&user.UserService_ServiceDesc, userService)

	// 注册Order服务
	orderService := NewOrderService(tracer)
	srv.RegisterService(&order.OrderService_ServiceDesc, orderService)

	// 启动服务器
	if err := srv.Start(); err != nil {
		log.Errorf("Failed to start server: %v", err)
		os.Exit(1)
	}

	log.Infof("Server started on %s", serverCfg.Address)
	log.Info("Registered services: HelloService, UserService, OrderService")

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

	// 注册多个服务到Nacos
	services := []struct {
		name string
		desc string
	}{
		{"hello.HelloService", "Hello服务"},
		{"user.UserService", "用户服务"},
		{"order.OrderService", "订单服务"},
	}

	for _, service := range services {
		serviceID := fmt.Sprintf("%s-%s-%s", traceCfg.ServiceName, service.name, serverCfg.Address)
		serviceInfo := &registry.ServiceInfo{
			ID:      serviceID,
			Name:    service.name,
			Version: traceCfg.ServiceVersion,
			Address: "127.0.0.1",
			Port:    serverCfg.Port,
			Metadata: map[string]string{
				"environment": traceCfg.Environment,
				"description": service.desc,
			},
			TTL: 30,
		}

		if err := nacosReg.Register(context.Background(), serviceInfo); err != nil {
			log.Errorf("Failed to register service %s: %v", service.name, err)
			os.Exit(1)
		}

		log.Infof("Service registered: %s (ID: %s)", service.name, serviceID)
	}

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutting down server...")

	// 注销所有服务
	for _, service := range services {
		serviceID := fmt.Sprintf("%s-%s-%s", traceCfg.ServiceName, service.name, serverCfg.Address)
		if err := nacosReg.Deregister(context.Background(), serviceID); err != nil {
			log.Errorf("Failed to deregister service %s: %v", service.name, err)
		} else {
			log.Infof("Service deregistered: %s", service.name)
		}
	}

	// 停止服务器
	if err := srv.Stop(); err != nil {
		log.Errorf("Failed to stop server: %v", err)
	}

	log.Info("Server stopped")
}
