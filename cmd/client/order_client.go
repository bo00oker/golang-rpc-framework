package main

import (
	"context"
	"fmt"

	"github.com/rpc-framework/core/pkg/client"
	"github.com/rpc-framework/core/pkg/logger"
	"github.com/rpc-framework/core/pkg/registry"
	"github.com/rpc-framework/core/pkg/trace"
	"github.com/rpc-framework/core/proto/order"
	"google.golang.org/grpc"
)

const OrderServiceName = "order.OrderService"

// OrderServiceClient 订单服务客户端封装
type OrderServiceClient struct {
	conn   *grpc.ClientConn
	client order.OrderServiceClient
	tracer *trace.Tracer
	logger logger.Logger
}

// NewOrderServiceClient 创建订单服务客户端
func NewOrderServiceClient(cli *client.Client, registry registry.Registry, tracer *trace.Tracer) (*OrderServiceClient, error) {
	// 通过服务发现获取连接
	conn, err := cli.DialService(context.Background(), OrderServiceName, registry)
	if err != nil {
		return nil, fmt.Errorf("failed to dial order service: %w", err)
	}

	return &OrderServiceClient{
		conn:   conn,
		client: order.NewOrderServiceClient(conn),
		tracer: tracer,
		logger: logger.GetGlobalLogger(),
	}, nil
}

// Close 关闭客户端连接
func (c *OrderServiceClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// CreateTestOrder 创建测试订单
func (c *OrderServiceClient) CreateTestOrder(ctx context.Context, userID int64, amount float64, description string) (int64, error) {
	ctx, span := c.tracer.StartSpan(ctx, "OrderServiceClient.CreateTestOrder")
	defer span.End()

	c.logger.Infof("Creating test order for user %d, amount: %.2f", userID, amount)

	items := []*order.OrderItem{
		{
			ProductId:   1,
			ProductName: "Test Product",
			Price:       amount,
			Quantity:    1,
		},
	}

	resp, err := c.client.CreateOrder(ctx, &order.CreateOrderRequest{
		UserId:      userID,
		Amount:      amount,
		Description: description,
		Items:       items,
	})

	if err != nil {
		c.logger.Errorf("Failed to create order: %v", err)
		return 0, err
	}

	if resp.Order != nil {
		c.logger.Infof("Order created successfully: ID=%d, OrderNo=%s", resp.Order.Id, resp.Order.OrderNo)
		return resp.Order.Id, nil
	}

	return 0, fmt.Errorf("order creation failed: %s", resp.Message)
}

// GetOrdersByUser 获取用户的订单列表
func (c *OrderServiceClient) GetOrdersByUser(ctx context.Context, userID int64) ([]*order.Order, error) {
	ctx, span := c.tracer.StartSpan(ctx, "OrderServiceClient.GetOrdersByUser")
	defer span.End()

	c.logger.Infof("Getting orders for user: %d", userID)

	resp, err := c.client.GetOrdersByUser(ctx, &order.GetOrdersByUserRequest{
		UserId:   userID,
		Page:     1,
		PageSize: 10,
	})

	if err != nil {
		c.logger.Errorf("Failed to get orders by user: %v", err)
		return nil, err
	}

	c.logger.Infof("Found %d orders for user %d", len(resp.Orders), userID)
	return resp.Orders, nil
}

// TestOrderOperations 测试订单服务的所有操作
func (c *OrderServiceClient) TestOrderOperations(ctx context.Context) {
	c.logger.Info("Starting order service operations test...")

	// 1. 创建订单
	c.logger.Info("1. Testing CreateOrder...")
	items := []*order.OrderItem{
		{
			ProductId:   1,
			ProductName: "iPhone 15",
			Price:       999.99,
			Quantity:    1,
		},
		{
			ProductId:   2,
			ProductName: "iPhone Case",
			Price:       29.99,
			Quantity:    2,
		},
	}

	createResp, err := c.client.CreateOrder(ctx, &order.CreateOrderRequest{
		UserId:      12345,
		Amount:      1059.97, // 999.99 + 29.99*2
		Description: "Test order with multiple items",
		Items:       items,
	})
	if err != nil {
		c.logger.Errorf("CreateOrder failed: %v", err)
		return
	}
	c.logger.Infof("CreateOrder success: %s", createResp.Message)
	if createResp.Order == nil {
		c.logger.Error("No order returned from CreateOrder")
		return
	}

	orderID := createResp.Order.Id
	c.logger.Infof("Created order with ID: %d, OrderNo: %s", orderID, createResp.Order.OrderNo)

	// 2. 查询订单
	c.logger.Info("2. Testing GetOrder...")
	getOrderResp, err := c.client.GetOrder(ctx, &order.GetOrderRequest{
		OrderId: orderID,
	})
	if err != nil {
		c.logger.Errorf("GetOrder failed: %v", err)
		return
	}
	c.logger.Infof("GetOrder success: %s", getOrderResp.Message)
	if getOrderResp.Order != nil {
		o := getOrderResp.Order
		c.logger.Infof("Found order: %s (ID: %d, Amount: %.2f, Status: %s)",
			o.OrderNo, o.Id, o.Amount, o.Status)
	}

	// 3. 更新订单
	c.logger.Info("3. Testing UpdateOrder...")
	updateResp, err := c.client.UpdateOrder(ctx, &order.UpdateOrderRequest{
		OrderId:     orderID,
		Status:      "processing",
		Description: "Updated test order",
	})
	if err != nil {
		c.logger.Errorf("UpdateOrder failed: %v", err)
		return
	}
	c.logger.Infof("UpdateOrder success: %s", updateResp.Message)

	// 4. 查询订单列表
	c.logger.Info("4. Testing ListOrders...")
	listResp, err := c.client.ListOrders(ctx, &order.ListOrdersRequest{
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		c.logger.Errorf("ListOrders failed: %v", err)
		return
	}
	c.logger.Infof("ListOrders success: %s, Total: %d", listResp.Message, listResp.Total)
	for i, o := range listResp.Orders {
		c.logger.Infof("Order %d: %s (ID: %d, Amount: %.2f, Status: %s)",
			i+1, o.OrderNo, o.Id, o.Amount, o.Status)
	}

	// 5. 按用户查询订单
	c.logger.Info("5. Testing GetOrdersByUser...")
	userOrdersResp, err := c.client.GetOrdersByUser(ctx, &order.GetOrdersByUserRequest{
		UserId:   12345,
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		c.logger.Errorf("GetOrdersByUser failed: %v", err)
		return
	}
	c.logger.Infof("GetOrdersByUser success: %s, Total: %d", userOrdersResp.Message, userOrdersResp.Total)

	// 6. 按状态查询订单
	c.logger.Info("6. Testing ListOrders with status filter...")
	statusFilterResp, err := c.client.ListOrders(ctx, &order.ListOrdersRequest{
		Page:     1,
		PageSize: 10,
		Status:   "processing",
	})
	if err != nil {
		c.logger.Errorf("ListOrders with status filter failed: %v", err)
		return
	}
	c.logger.Infof("Status filter result: Total: %d", statusFilterResp.Total)

	// 7. 更新订单状态为已完成
	c.logger.Info("7. Testing order completion...")
	completeResp, err := c.client.UpdateOrder(ctx, &order.UpdateOrderRequest{
		OrderId: orderID,
		Status:  "completed",
	})
	if err != nil {
		c.logger.Errorf("Complete order failed: %v", err)
		return
	}
	c.logger.Infof("Order completion success: %s", completeResp.Message)

	// 8. 删除订单
	c.logger.Info("8. Testing DeleteOrder...")
	deleteResp, err := c.client.DeleteOrder(ctx, &order.DeleteOrderRequest{
		OrderId: orderID,
	})
	if err != nil {
		c.logger.Errorf("DeleteOrder failed: %v", err)
		return
	}
	c.logger.Infof("DeleteOrder success: %s", deleteResp.Message)

	// 9. 验证订单已删除
	c.logger.Info("9. Verifying order deletion...")
	_, err = c.client.GetOrder(ctx, &order.GetOrderRequest{
		OrderId: orderID,
	})
	if err != nil {
		c.logger.Infof("Order deletion verified (GetOrder returned error as expected)")
	} else {
		c.logger.Warn("Order still exists after deletion")
	}

	c.logger.Info("Order service operations test completed!")
}
