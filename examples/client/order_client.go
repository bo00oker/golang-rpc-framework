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

// OrderClient 订单服务客户端
type OrderClient struct {
	cli         *client.Client
	conn        *grpc.ClientConn
	orderClient order.OrderServiceClient
}

// NewOrderClient 创建订单服务客户端
func NewOrderClient(cli *client.Client, nacosReg registry.Registry, tracer *trace.Tracer) (*OrderClient, error) {
	ctx := context.Background()

	// 连接订单服务
	conn, err := cli.DialService(ctx, "order.OrderService", nacosReg)
	if err != nil {
		return nil, fmt.Errorf("failed to dial order service: %w", err)
	}

	return &OrderClient{
		cli:         cli,
		conn:        conn,
		orderClient: order.NewOrderServiceClient(conn),
	}, nil
}

// Close 关闭连接
func (o *OrderClient) Close() {
	if o.conn != nil {
		o.conn.Close()
	}
}

// TestOrderService 测试订单服务
func (o *OrderClient) TestOrderService(ctx context.Context) {
	log := logger.GetGlobalLogger()
	log.Info("=== Testing Order Service ===")

	// 创建订单
	createResp, err := o.orderClient.CreateOrder(ctx, &order.CreateOrderRequest{
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
	createResp2, err := o.orderClient.CreateOrder(ctx, &order.CreateOrderRequest{
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
	getResp, err := o.orderClient.GetOrder(ctx, &order.GetOrderRequest{
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
	listResp, err := o.orderClient.ListOrders(ctx, &order.ListOrdersRequest{
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
	updateResp, err := o.orderClient.UpdateOrder(ctx, &order.UpdateOrderRequest{
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
	userOrdersResp, err := o.orderClient.GetOrdersByUser(ctx, &order.GetOrdersByUserRequest{
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

	log.Info("✅ Order service test completed!")
}
