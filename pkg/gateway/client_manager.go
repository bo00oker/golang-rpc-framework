package gateway

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rpc-framework/core/pkg/config"
	"github.com/rpc-framework/core/pkg/logger"
	"github.com/rpc-framework/core/pkg/registry"
	"github.com/rpc-framework/core/proto/order"
	"github.com/rpc-framework/core/proto/user"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// ClientManager RPC客户端管理器
type ClientManager struct {
	config       *config.Config
	registry     registry.Registry
	clients      map[string]*ServiceClient
	mu           sync.RWMutex
	logger       logger.Logger
	stopCh       chan struct{}
	watcherStops []func()
}

// ServiceClient 服务客户端
type ServiceClient struct {
	ServiceName string
	Connections []*grpc.ClientConn
	Clients     []interface{}
	CurrentIdx  int
	mu          sync.RWMutex
	healthy     bool
}

// NewClientManager 创建客户端管理器
func NewClientManager(cfg *config.Config, registry registry.Registry) (*ClientManager, error) {
	cm := &ClientManager{
		config:   cfg,
		registry: registry,
		clients:  make(map[string]*ServiceClient),
		logger:   logger.GetGlobalLogger(),
		stopCh:   make(chan struct{}),
	}

	return cm, nil
}

// Start 启动客户端管理器
func (cm *ClientManager) Start() error {
	cm.logger.Info("Starting RPC client manager...")

	// 初始化服务客户端
	services := []string{
		"user.UserService",
		"order.OrderService",
	}

	for _, serviceName := range services {
		if err := cm.initServiceClient(serviceName); err != nil {
			cm.logger.Errorf("Failed to initialize client for service %s: %v", serviceName, err)
			return err
		}

		// 启动服务发现监听
		if err := cm.watchService(serviceName); err != nil {
			cm.logger.Errorf("Failed to watch service %s: %v", serviceName, err)
			return err
		}
	}

	cm.logger.Info("RPC client manager started successfully")
	return nil
}

// Stop 停止客户端管理器
func (cm *ClientManager) Stop() error {
	cm.logger.Info("Stopping RPC client manager...")

	close(cm.stopCh)

	// 停止所有watcher
	for _, stop := range cm.watcherStops {
		stop()
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 关闭所有连接
	for serviceName, serviceClient := range cm.clients {
		for _, conn := range serviceClient.Connections {
			if err := conn.Close(); err != nil {
				cm.logger.Errorf("Failed to close connection for service %s: %v", serviceName, err)
			}
		}
	}

	cm.logger.Info("RPC client manager stopped")
	return nil
}

// initServiceClient 初始化服务客户端
func (cm *ClientManager) initServiceClient(serviceName string) error {
	// 从服务注册中心发现服务实例
	instances, err := cm.registry.Discover(context.Background(), serviceName)
	if err != nil {
		return fmt.Errorf("failed to discover service %s: %w", serviceName, err)
	}

	if len(instances) == 0 {
		return fmt.Errorf("no instances found for service %s", serviceName)
	}

	serviceClient := &ServiceClient{
		ServiceName: serviceName,
		Connections: make([]*grpc.ClientConn, 0),
		Clients:     make([]interface{}, 0),
		healthy:     true,
	}

	// 为每个实例创建连接
	for _, instance := range instances {
		conn, err := cm.createConnection(instance.Address)
		if err != nil {
			cm.logger.Errorf("Failed to connect to %s at %s: %v", serviceName, instance.Address, err)
			continue
		}

		serviceClient.Connections = append(serviceClient.Connections, conn)

		// 根据服务类型创建对应的客户端
		var client interface{}
		switch serviceName {
		case "user.UserService":
			client = user.NewUserServiceClient(conn)
		case "order.OrderService":
			client = order.NewOrderServiceClient(conn)
		default:
			conn.Close()
			return fmt.Errorf("unknown service: %s", serviceName)
		}

		serviceClient.Clients = append(serviceClient.Clients, client)
		cm.logger.Infof("Connected to %s at %s", serviceName, instance.Address)
	}

	if len(serviceClient.Connections) == 0 {
		return fmt.Errorf("failed to connect to any instance of service %s", serviceName)
	}

	cm.mu.Lock()
	cm.clients[serviceName] = serviceClient
	cm.mu.Unlock()

	return nil
}

// createConnection 创建gRPC连接
func (cm *ClientManager) createConnection(address string) (*grpc.ClientConn, error) {
	// 直接使用grpc.Dial创建连接
	conn, err := grpc.Dial(address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithTimeout(getDurationWithDefault(cm.config.GetDuration("client.conn_timeout"), 5*time.Second)),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                30 * time.Second,
			Timeout:             5 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", address, err)
	}

	return conn, nil
}

// watchService 监听服务变化
func (cm *ClientManager) watchService(serviceName string) error {
	ctx, cancel := context.WithCancel(context.Background())
	cm.watcherStops = append(cm.watcherStops, cancel)

	// 监听服务实例变化
	watchCh, err := cm.registry.Watch(ctx, serviceName)
	if err != nil {
		return fmt.Errorf("failed to watch service %s: %w", serviceName, err)
	}

	go func() {
		for {
			select {
			case instances := <-watchCh:
				cm.logger.Infof("Service %s instances updated: %d instances", serviceName, len(instances))
				// 更新服务客户端
				if err := cm.updateServiceClient(serviceName, instances); err != nil {
					cm.logger.Errorf("Failed to update service client %s: %v", serviceName, err)
				}
			case <-cm.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

// updateServiceClient 更新服务客户端
func (cm *ClientManager) updateServiceClient(serviceName string, instances []*registry.ServiceInfo) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	serviceClient, exists := cm.clients[serviceName]
	if !exists {
		return fmt.Errorf("service client %s not found", serviceName)
	}

	// 关闭现有连接
	for _, conn := range serviceClient.Connections {
		conn.Close()
	}

	// 重置客户端
	serviceClient.Connections = make([]*grpc.ClientConn, 0)
	serviceClient.Clients = make([]interface{}, 0)
	serviceClient.CurrentIdx = 0

	// 创建新连接
	for _, instance := range instances {
		conn, err := cm.createConnection(instance.Address)
		if err != nil {
			cm.logger.Errorf("Failed to connect to %s at %s: %v", serviceName, instance.Address, err)
			continue
		}

		serviceClient.Connections = append(serviceClient.Connections, conn)

		// 创建客户端
		var client interface{}
		switch serviceName {
		case "user.UserService":
			client = user.NewUserServiceClient(conn)
		case "order.OrderService":
			client = order.NewOrderServiceClient(conn)
		}

		serviceClient.Clients = append(serviceClient.Clients, client)
	}

	serviceClient.healthy = len(serviceClient.Connections) > 0
	return nil
}

// GetUserClient 获取用户服务客户端（负载均衡）
func (cm *ClientManager) GetUserClient() (user.UserServiceClient, error) {
	client, err := cm.getServiceClient("user.UserService")
	if err != nil {
		return nil, err
	}

	userClient, ok := client.(user.UserServiceClient)
	if !ok {
		return nil, fmt.Errorf("invalid user service client type")
	}

	return userClient, nil
}

// GetOrderClient 获取订单服务客户端（负载均衡）
func (cm *ClientManager) GetOrderClient() (order.OrderServiceClient, error) {
	client, err := cm.getServiceClient("order.OrderService")
	if err != nil {
		return nil, err
	}

	orderClient, ok := client.(order.OrderServiceClient)
	if !ok {
		return nil, fmt.Errorf("invalid order service client type")
	}

	return orderClient, nil
}

// getServiceClient 获取服务客户端（轮询负载均衡）
func (cm *ClientManager) getServiceClient(serviceName string) (interface{}, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	serviceClient, exists := cm.clients[serviceName]
	if !exists {
		return nil, fmt.Errorf("service client %s not found", serviceName)
	}

	if !serviceClient.healthy || len(serviceClient.Clients) == 0 {
		return nil, fmt.Errorf("service %s is not healthy", serviceName)
	}

	// 轮询负载均衡
	serviceClient.mu.Lock()
	client := serviceClient.Clients[serviceClient.CurrentIdx]
	serviceClient.CurrentIdx = (serviceClient.CurrentIdx + 1) % len(serviceClient.Clients)
	serviceClient.mu.Unlock()

	return client, nil
}

// IsHealthy 检查所有服务是否健康
func (cm *ClientManager) IsHealthy() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	for _, serviceClient := range cm.clients {
		if !serviceClient.healthy {
			return false
		}
	}

	return true
}

// GetActiveConnections 获取活跃连接数
func (cm *ClientManager) GetActiveConnections() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	total := 0
	for _, serviceClient := range cm.clients {
		total += len(serviceClient.Connections)
	}

	return total
}

// GetServiceStatus 获取服务状态
func (cm *ClientManager) GetServiceStatus() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	status := make(map[string]interface{})
	for serviceName, serviceClient := range cm.clients {
		status[serviceName] = map[string]interface{}{
			"healthy":          serviceClient.healthy,
			"connection_count": len(serviceClient.Connections),
			"current_index":    serviceClient.CurrentIdx,
		}
	}

	return status
}
