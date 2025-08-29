package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rpc-framework/core/pkg/logger"
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
