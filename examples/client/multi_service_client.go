package main

import (
	"context"
	"fmt"
	"time"

	"github.com/rpc-framework/core/pkg/client"
	"github.com/rpc-framework/core/pkg/logger"
	"github.com/rpc-framework/core/pkg/registry"
	"github.com/rpc-framework/core/pkg/trace"
	"github.com/rpc-framework/core/proto/hello"
	"github.com/rpc-framework/core/proto/order"
	"github.com/rpc-framework/core/proto/user"
	"google.golang.org/grpc"
)

// MultiServiceClient 多服务客户端
type MultiServiceClient struct {
	cli         *client.Client
	helloConn   *grpc.ClientConn
	userConn    *grpc.ClientConn
	orderConn   *grpc.ClientConn
	helloClient hello.HelloServiceClient
	userClient  user.UserServiceClient
	orderClient order.OrderServiceClient
}

// NewMultiServiceClient 创建多服务客户端
func NewMultiServiceClient(cli *client.Client, nacosReg registry.Registry, tracer *trace.Tracer) (*MultiServiceClient, error) {
	ctx := context.Background()

	// 连接Hello服务
	helloConn, err := cli.DialService(ctx, "hello.HelloService", nacosReg)
	if err != nil {
		return nil, fmt.Errorf("failed to dial hello service: %w", err)
	}

	// 连接User服务
	userConn, err := cli.DialService(ctx, "user.UserService", nacosReg)
	if err != nil {
		helloConn.Close()
		return nil, fmt.Errorf("failed to dial user service: %w", err)
	}

	// 连接Order服务
	orderConn, err := cli.DialService(ctx, "order.OrderService", nacosReg)
	if err != nil {
		helloConn.Close()
		userConn.Close()
		return nil, fmt.Errorf("failed to dial order service: %w", err)
	}

	return &MultiServiceClient{
		cli:         cli,
		helloConn:   helloConn,
		userConn:    userConn,
		orderConn:   orderConn,
		helloClient: hello.NewHelloServiceClient(helloConn),
		userClient:  user.NewUserServiceClient(userConn),
		orderClient: order.NewOrderServiceClient(orderConn),
	}, nil
}

// Close 关闭所有连接
func (m *MultiServiceClient) Close() {
	if m.helloConn != nil {
		m.helloConn.Close()
	}
	if m.userConn != nil {
		m.userConn.Close()
	}
	if m.orderConn != nil {
		m.orderConn.Close()
	}
}

// TestHelloService 测试Hello服务
func (m *MultiServiceClient) TestHelloService(ctx context.Context) {
	log := logger.GetGlobalLogger()
	log.Info("=== Testing Hello Service ===")

	// 测试普通调用
	resp, err := m.helloClient.SayHello(ctx, &hello.HelloRequest{
		Name:    "MultiServiceClient",
		Message: "Hello from multi-service client!",
	})
	if err != nil {
		log.Errorf("Hello service call failed: %v", err)
		return
	}
	log.Infof("Hello response: %s", resp.Message)
}

// TestUserService 测试用户服务
func (m *MultiServiceClient) TestUserService(ctx context.Context) {
	log := logger.GetGlobalLogger()
	log.Info("=== Testing User Service ===")

	// 创建用户
	createResp, err := m.userClient.CreateUser(ctx, &user.CreateUserRequest{
		Username: "testuser1",
		Email:    "test1@example.com",
		Phone:    "13800138001",
		Age:      25,
		Address:  "北京市朝阳区",
	})
	if err != nil {
		log.Errorf("Create user failed: %v", err)
		return
	}
	log.Infof("User created: ID=%d, Username=%s", createResp.User.Id, createResp.User.Username)

	userID := createResp.User.Id

	// 创建第二个用户
	createResp2, err := m.userClient.CreateUser(ctx, &user.CreateUserRequest{
		Username: "testuser2",
		Email:    "test2@example.com",
		Phone:    "13800138002",
		Age:      30,
		Address:  "上海市浦东新区",
	})
	if err != nil {
		log.Errorf("Create second user failed: %v", err)
		return
	}
	log.Infof("Second user created: ID=%d, Username=%s", createResp2.User.Id, createResp2.User.Username)

	// 查询用户
	getResp, err := m.userClient.GetUser(ctx, &user.GetUserRequest{
		UserId: userID,
	})
	if err != nil {
		log.Errorf("Get user failed: %v", err)
		return
	}
	log.Infof("User retrieved: %s", getResp.Message)
	if getResp.User != nil {
		log.Infof("User details: ID=%d, Username=%s, Email=%s",
			getResp.User.Id, getResp.User.Username, getResp.User.Email)
	}

	// 查询用户列表
	listResp, err := m.userClient.ListUsers(ctx, &user.ListUsersRequest{
		Page:     1,
		PageSize: 10,
		Keyword:  "test",
	})
	if err != nil {
		log.Errorf("List users failed: %v", err)
		return
	}
	log.Infof("Users listed: %s, Total=%d", listResp.Message, listResp.Total)
	for _, u := range listResp.Users {
		log.Infof("  - ID=%d, Username=%s, Email=%s", u.Id, u.Username, u.Email)
	}

	// 更新用户
	updateResp, err := m.userClient.UpdateUser(ctx, &user.UpdateUserRequest{
		UserId:  userID,
		Age:     26,
		Address: "北京市海淀区",
	})
	if err != nil {
		log.Errorf("Update user failed: %v", err)
		return
	}
	log.Infof("User updated: %s", updateResp.Message)
	if updateResp.User != nil {
		log.Infof("Updated user: Age=%d, Address=%s", updateResp.User.Age, updateResp.User.Address)
	}
}

// TestOrderService 测试订单服务
func (m *MultiServiceClient) TestOrderService(ctx context.Context) {
	log := logger.GetGlobalLogger()
	log.Info("=== Testing Order Service ===")

	// 创建订单
	createResp, err := m.orderClient.CreateOrder(ctx, &order.CreateOrderRequest{
		UserId:      1,
		Amount:      299.99,
		Description: "测试订单1",
		Items: []*order.OrderItem{
			{
				ProductId:   1,
				ProductName: "iPhone 15",
				Price:       299.99,
				Quantity:    1,
			},
		},
	})
	if err != nil {
		log.Errorf("Create order failed: %v", err)
		return
	}
	log.Infof("Order created: ID=%d, OrderNo=%s, Amount=%.2f",
		createResp.Order.Id, createResp.Order.OrderNo, createResp.Order.Amount)

	orderID := createResp.Order.Id

	// 创建第二个订单
	createResp2, err := m.orderClient.CreateOrder(ctx, &order.CreateOrderRequest{
		UserId:      2,
		Amount:      199.99,
		Description: "测试订单2",
		Items: []*order.OrderItem{
			{
				ProductId:   2,
				ProductName: "MacBook Air",
				Price:       199.99,
				Quantity:    1,
			},
		},
	})
	if err != nil {
		log.Errorf("Create second order failed: %v", err)
		return
	}
	log.Infof("Second order created: ID=%d, OrderNo=%s, Amount=%.2f",
		createResp2.Order.Id, createResp2.Order.OrderNo, createResp2.Order.Amount)

	// 查询订单
	getResp, err := m.orderClient.GetOrder(ctx, &order.GetOrderRequest{
		OrderId: orderID,
	})
	if err != nil {
		log.Errorf("Get order failed: %v", err)
		return
	}
	log.Infof("Order retrieved: %s", getResp.Message)
	if getResp.Order != nil {
		log.Infof("Order details: ID=%d, OrderNo=%s, Amount=%.2f, Status=%s",
			getResp.Order.Id, getResp.Order.OrderNo, getResp.Order.Amount, getResp.Order.Status)
	}

	// 查询订单列表
	listResp, err := m.orderClient.ListOrders(ctx, &order.ListOrdersRequest{
		Page:     1,
		PageSize: 10,
		Status:   "pending",
	})
	if err != nil {
		log.Errorf("List orders failed: %v", err)
		return
	}
	log.Infof("Orders listed: %s, Total=%d", listResp.Message, listResp.Total)
	for _, o := range listResp.Orders {
		log.Infof("  - ID=%d, OrderNo=%s, Amount=%.2f, Status=%s",
			o.Id, o.OrderNo, o.Amount, o.Status)
	}

	// 更新订单状态
	updateResp, err := m.orderClient.UpdateOrder(ctx, &order.UpdateOrderRequest{
		OrderId:     orderID,
		Status:      "completed",
		Description: "订单已完成",
	})
	if err != nil {
		log.Errorf("Update order failed: %v", err)
		return
	}
	log.Infof("Order updated: %s", updateResp.Message)
	if updateResp.Order != nil {
		log.Infof("Updated order: Status=%s, Description=%s",
			updateResp.Order.Status, updateResp.Order.Description)
	}

	// 根据用户ID查询订单
	userOrdersResp, err := m.orderClient.GetOrdersByUser(ctx, &order.GetOrdersByUserRequest{
		UserId:   1,
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		log.Errorf("Get orders by user failed: %v", err)
		return
	}
	log.Infof("User orders: %s, Total=%d", userOrdersResp.Message, userOrdersResp.Total)
	for _, o := range userOrdersResp.Orders {
		log.Infof("  - ID=%d, OrderNo=%s, Amount=%.2f, Status=%s",
			o.Id, o.OrderNo, o.Amount, o.Status)
	}
}

// TestAllServices 测试所有服务
func (m *MultiServiceClient) TestAllServices(ctx context.Context) {
	log := logger.GetGlobalLogger()
	log.Info("🚀 Starting multi-service test...")

	// 测试Hello服务
	m.TestHelloService(ctx)
	time.Sleep(1 * time.Second)

	// 测试用户服务
	m.TestUserService(ctx)
	time.Sleep(1 * time.Second)

	// 测试订单服务
	m.TestOrderService(ctx)

	log.Info("✅ Multi-service test completed!")
}
