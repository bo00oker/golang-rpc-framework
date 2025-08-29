package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/rpc-framework/core/pkg/config"
	"github.com/rpc-framework/core/pkg/logger"
	"github.com/rpc-framework/core/pkg/registry"
	"github.com/rpc-framework/core/pkg/server"
	"github.com/rpc-framework/core/pkg/trace"
	"github.com/rpc-framework/core/proto/order"
)

// OrderServiceImpl 订单服务实现
type OrderServiceImpl struct {
	order.UnimplementedOrderServiceServer
	tracer *trace.Tracer
	orders map[int64]*order.Order
	mu     sync.RWMutex
	nextID int64
}

// NewOrderService 创建订单服务实例
func NewOrderService(tracer *trace.Tracer) *OrderServiceImpl {
	return &OrderServiceImpl{
		tracer: tracer,
		orders: make(map[int64]*order.Order),
		nextID: 1,
	}
}

// CreateOrder 创建订单
func (s *OrderServiceImpl) CreateOrder(ctx context.Context, req *order.CreateOrderRequest) (*order.CreateOrderResponse, error) {
	ctx, span := s.tracer.StartSpan(ctx, "CreateOrder")
	defer span.End()

	log := logger.GetGlobalLogger()
	log.Infof("Creating order for user: %d, amount: %.2f", req.UserId, req.Amount)

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()
	orderNo := fmt.Sprintf("ORD%d%d", now, s.nextID)

	newOrder := &order.Order{
		Id:          s.nextID,
		UserId:      req.UserId,
		OrderNo:     orderNo,
		Amount:      req.Amount,
		Status:      "pending",
		Description: req.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	s.orders[newOrder.Id] = newOrder
	s.nextID++

	log.Infof("Order created successfully: ID=%d, OrderNo=%s", newOrder.Id, newOrder.OrderNo)

	return &order.CreateOrderResponse{
		Order:   newOrder,
		Message: "订单创建成功",
	}, nil
}

// GetOrder 查询订单
func (s *OrderServiceImpl) GetOrder(ctx context.Context, req *order.GetOrderRequest) (*order.GetOrderResponse, error) {
	ctx, span := s.tracer.StartSpan(ctx, "GetOrder")
	defer span.End()

	log := logger.GetGlobalLogger()
	log.Infof("Getting order: ID=%d", req.OrderId)

	s.mu.RLock()
	defer s.mu.RUnlock()

	o, exists := s.orders[req.OrderId]
	if !exists {
		return &order.GetOrderResponse{
			Message: "订单不存在",
		}, nil
	}

	return &order.GetOrderResponse{
		Order:   o,
		Message: "查询成功",
	}, nil
}

// ListOrders 查询订单列表
func (s *OrderServiceImpl) ListOrders(ctx context.Context, req *order.ListOrdersRequest) (*order.ListOrdersResponse, error) {
	ctx, span := s.tracer.StartSpan(ctx, "ListOrders")
	defer span.End()

	log := logger.GetGlobalLogger()
	log.Infof("Listing orders: page=%d, page_size=%d, status=%s, user_id=%d",
		req.Page, req.PageSize, req.Status, req.UserId)

	s.mu.RLock()
	defer s.mu.RUnlock()

	var filteredOrders []*order.Order
	for _, o := range s.orders {
		// 按状态过滤
		if req.Status != "" && o.Status != req.Status {
			continue
		}
		// 按用户ID过滤
		if req.UserId > 0 && o.UserId != req.UserId {
			continue
		}
		filteredOrders = append(filteredOrders, o)
	}

	// 分页处理
	total := int32(len(filteredOrders))
	start := (req.Page - 1) * req.PageSize
	end := start + req.PageSize

	if start >= total {
		return &order.ListOrdersResponse{
			Orders:   []*order.Order{},
			Total:    total,
			Page:     req.Page,
			PageSize: req.PageSize,
			Message:  "查询成功",
		}, nil
	}

	if end > total {
		end = total
	}

	return &order.ListOrdersResponse{
		Orders:   filteredOrders[start:end],
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Message:  "查询成功",
	}, nil
}

// UpdateOrder 更新订单
func (s *OrderServiceImpl) UpdateOrder(ctx context.Context, req *order.UpdateOrderRequest) (*order.UpdateOrderResponse, error) {
	ctx, span := s.tracer.StartSpan(ctx, "UpdateOrder")
	defer span.End()

	log := logger.GetGlobalLogger()
	log.Infof("Updating order: ID=%d", req.OrderId)

	s.mu.Lock()
	defer s.mu.Unlock()

	o, exists := s.orders[req.OrderId]
	if !exists {
		return &order.UpdateOrderResponse{
			Message: "订单不存在",
		}, nil
	}

	// 更新字段
	if req.Amount > 0 {
		o.Amount = req.Amount
	}
	if req.Status != "" {
		o.Status = req.Status
	}
	if req.Description != "" {
		o.Description = req.Description
	}
	o.UpdatedAt = time.Now().Unix()

	log.Infof("Order updated successfully: ID=%d", req.OrderId)

	return &order.UpdateOrderResponse{
		Order:   o,
		Message: "订单更新成功",
	}, nil
}

// DeleteOrder 删除订单
func (s *OrderServiceImpl) DeleteOrder(ctx context.Context, req *order.DeleteOrderRequest) (*order.DeleteOrderResponse, error) {
	ctx, span := s.tracer.StartSpan(ctx, "DeleteOrder")
	defer span.End()

	log := logger.GetGlobalLogger()
	log.Infof("Deleting order: ID=%d", req.OrderId)

	s.mu.Lock()
	defer s.mu.Unlock()

	_, exists := s.orders[req.OrderId]
	if !exists {
		return &order.DeleteOrderResponse{
			Success: false,
			Message: "订单不存在",
		}, nil
	}

	delete(s.orders, req.OrderId)

	log.Infof("Order deleted successfully: ID=%d", req.OrderId)

	return &order.DeleteOrderResponse{
		Success: true,
		Message: "订单删除成功",
	}, nil
}

// GetOrdersByUser 根据用户ID查询订单
func (s *OrderServiceImpl) GetOrdersByUser(ctx context.Context, req *order.GetOrdersByUserRequest) (*order.GetOrdersByUserResponse, error) {
	ctx, span := s.tracer.StartSpan(ctx, "GetOrdersByUser")
	defer span.End()

	log := logger.GetGlobalLogger()
	log.Infof("Getting orders for user: ID=%d, page=%d, page_size=%d",
		req.UserId, req.Page, req.PageSize)

	s.mu.RLock()
	defer s.mu.RUnlock()

	var userOrders []*order.Order
	for _, o := range s.orders {
		if o.UserId == req.UserId {
			userOrders = append(userOrders, o)
		}
	}

	// 分页处理
	total := int32(len(userOrders))
	start := (req.Page - 1) * req.PageSize
	end := start + req.PageSize

	if start >= total {
		return &order.GetOrdersByUserResponse{
			Orders:   []*order.Order{},
			Total:    total,
			Page:     req.Page,
			PageSize: req.PageSize,
			Message:  "查询成功",
		}, nil
	}

	if end > total {
		end = total
	}

	return &order.GetOrdersByUserResponse{
		Orders:   userOrders[start:end],
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Message:  "查询成功",
	}, nil
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
		ServiceName:    traceCfg.ServiceName + "-order",
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

	// 注册订单服务
	orderService := NewOrderService(tracer)
	srv.RegisterService(&order.OrderService_ServiceDesc, orderService)

	// 启动服务器
	if err := srv.Start(); err != nil {
		log.Errorf("Failed to start server: %v", err)
		os.Exit(1)
	}

	log.Infof("Order service started on %s", serverCfg.Address)

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

	// 注册订单服务到Nacos
	serviceID := fmt.Sprintf("%s-order-%s", traceCfg.ServiceName, serverCfg.Address)
	serviceInfo := &registry.ServiceInfo{
		ID:      serviceID,
		Name:    "order.OrderService",
		Version: traceCfg.ServiceVersion,
		Address: "127.0.0.1",
		Port:    serverCfg.Port,
		Metadata: map[string]string{
			"environment": traceCfg.Environment,
			"description": "订单服务",
		},
		TTL: 30,
	}

	if err := nacosReg.Register(context.Background(), serviceInfo); err != nil {
		log.Errorf("Failed to register order service: %v", err)
		os.Exit(1)
	}

	log.Infof("Order service registered with ID: %s", serviceID)

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutting down order service...")

	// 注销服务
	if err := nacosReg.Deregister(context.Background(), serviceID); err != nil {
		log.Errorf("Failed to deregister order service: %v", err)
	}

	// 停止服务器
	if err := srv.Stop(); err != nil {
		log.Errorf("Failed to stop server: %v", err)
	}

	log.Info("Order service stopped")
}
