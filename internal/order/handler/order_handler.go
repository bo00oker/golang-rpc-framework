package handler

import (
	"context"
	"errors"

	"github.com/rpc-framework/core/internal/order/model"
	"github.com/rpc-framework/core/internal/order/service"
	"github.com/rpc-framework/core/pkg/logger"
	"github.com/rpc-framework/core/pkg/trace"
	"github.com/rpc-framework/core/proto/order"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// OrderHandler 订单gRPC处理器
type OrderHandler struct {
	order.UnimplementedOrderServiceServer
	orderService service.OrderService
	tracer       *trace.Tracer
	logger       logger.Logger
}

// NewOrderHandler 创建订单处理器
func NewOrderHandler(orderService service.OrderService, tracer *trace.Tracer) *OrderHandler {
	return &OrderHandler{
		orderService: orderService,
		tracer:       tracer,
		logger:       logger.GetGlobalLogger(),
	}
}

// CreateOrder 创建订单
func (h *OrderHandler) CreateOrder(ctx context.Context, req *order.CreateOrderRequest) (*order.CreateOrderResponse, error) {
	ctx, span := h.tracer.StartSpan(ctx, "OrderHandler.CreateOrder")
	defer span.End()

	// 转换请求参数
	items := make([]*model.OrderItem, len(req.Items))
	for i, item := range req.Items {
		items[i] = &model.OrderItem{
			ProductID:   item.ProductId,
			ProductName: item.ProductName,
			Price:       item.Price,
			Quantity:    item.Quantity,
		}
	}

	createReq := &model.CreateOrderRequest{
		UserID:      req.UserId,
		Amount:      req.Amount,
		Description: req.Description,
		Items:       items,
	}

	// 调用服务层
	createdOrder, err := h.orderService.CreateOrder(ctx, createReq)
	if err != nil {
		h.logger.Errorf("Failed to create order: %v", err)

		if errors.Is(err, service.ErrInvalidRequest) {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid request: %v", err)
		}

		return nil, status.Errorf(codes.Internal, "Internal error: %v", err)
	}

	// 转换响应
	return &order.CreateOrderResponse{
		Order:   h.convertOrderToProto(createdOrder),
		Message: "订单创建成功",
	}, nil
}

// GetOrder 查询订单
func (h *OrderHandler) GetOrder(ctx context.Context, req *order.GetOrderRequest) (*order.GetOrderResponse, error) {
	ctx, span := h.tracer.StartSpan(ctx, "OrderHandler.GetOrder")
	defer span.End()

	// 调用服务层
	foundOrder, err := h.orderService.GetOrder(ctx, req.OrderId)
	if err != nil {
		h.logger.Errorf("Failed to get order: %v", err)

		if errors.Is(err, service.ErrInvalidRequest) {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid request: %v", err)
		}
		if errors.Is(err, service.ErrOrderNotFound) {
			return &order.GetOrderResponse{
				Message: "订单不存在",
			}, nil
		}

		return nil, status.Errorf(codes.Internal, "Internal error: %v", err)
	}

	// 转换响应
	return &order.GetOrderResponse{
		Order:   h.convertOrderToProto(foundOrder),
		Message: "查询成功",
	}, nil
}

// UpdateOrder 更新订单
func (h *OrderHandler) UpdateOrder(ctx context.Context, req *order.UpdateOrderRequest) (*order.UpdateOrderResponse, error) {
	ctx, span := h.tracer.StartSpan(ctx, "OrderHandler.UpdateOrder")
	defer span.End()

	// 转换请求参数
	updateReq := &model.UpdateOrderRequest{
		ID:          req.OrderId,
		Amount:      req.Amount,
		Status:      req.Status,
		Description: req.Description,
	}

	// 调用服务层
	updatedOrder, err := h.orderService.UpdateOrder(ctx, updateReq)
	if err != nil {
		h.logger.Errorf("Failed to update order: %v", err)

		if errors.Is(err, service.ErrInvalidRequest) {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid request: %v", err)
		}
		if errors.Is(err, service.ErrOrderNotFound) {
			return &order.UpdateOrderResponse{
				Message: "订单不存在",
			}, nil
		}

		return nil, status.Errorf(codes.Internal, "Internal error: %v", err)
	}

	// 转换响应
	return &order.UpdateOrderResponse{
		Order:   h.convertOrderToProto(updatedOrder),
		Message: "订单更新成功",
	}, nil
}

// DeleteOrder 删除订单
func (h *OrderHandler) DeleteOrder(ctx context.Context, req *order.DeleteOrderRequest) (*order.DeleteOrderResponse, error) {
	ctx, span := h.tracer.StartSpan(ctx, "OrderHandler.DeleteOrder")
	defer span.End()

	// 调用服务层
	err := h.orderService.DeleteOrder(ctx, req.OrderId)
	if err != nil {
		h.logger.Errorf("Failed to delete order: %v", err)

		if errors.Is(err, service.ErrInvalidRequest) {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid request: %v", err)
		}
		if errors.Is(err, service.ErrOrderNotFound) {
			return &order.DeleteOrderResponse{
				Success: false,
				Message: "订单不存在",
			}, nil
		}

		return nil, status.Errorf(codes.Internal, "Internal error: %v", err)
	}

	// 返回响应
	return &order.DeleteOrderResponse{
		Success: true,
		Message: "订单删除成功",
	}, nil
}

// ListOrders 查询订单列表
func (h *OrderHandler) ListOrders(ctx context.Context, req *order.ListOrdersRequest) (*order.ListOrdersResponse, error) {
	ctx, span := h.tracer.StartSpan(ctx, "OrderHandler.ListOrders")
	defer span.End()

	// 转换请求参数
	listReq := &model.ListOrdersRequest{
		Page:     req.Page,
		PageSize: req.PageSize,
		Status:   req.Status,
		UserID:   req.UserId,
	}

	// 设置默认值
	if listReq.Page <= 0 {
		listReq.Page = 1
	}
	if listReq.PageSize <= 0 {
		listReq.PageSize = 10
	}

	// 调用服务层
	response, err := h.orderService.ListOrders(ctx, listReq)
	if err != nil {
		h.logger.Errorf("Failed to list orders: %v", err)

		if errors.Is(err, service.ErrInvalidRequest) {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid request: %v", err)
		}

		return nil, status.Errorf(codes.Internal, "Internal error: %v", err)
	}

	// 转换响应
	protoOrders := make([]*order.Order, len(response.Orders))
	for i, o := range response.Orders {
		protoOrders[i] = h.convertOrderToProto(o)
	}

	return &order.ListOrdersResponse{
		Orders:   protoOrders,
		Total:    response.Total,
		Page:     response.Page,
		PageSize: response.PageSize,
		Message:  "查询成功",
	}, nil
}

// GetOrdersByUser 根据用户ID查询订单
func (h *OrderHandler) GetOrdersByUser(ctx context.Context, req *order.GetOrdersByUserRequest) (*order.GetOrdersByUserResponse, error) {
	ctx, span := h.tracer.StartSpan(ctx, "OrderHandler.GetOrdersByUser")
	defer span.End()

	// 设置默认值
	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	// 调用服务层
	response, err := h.orderService.GetOrdersByUser(ctx, req.UserId, page, pageSize)
	if err != nil {
		h.logger.Errorf("Failed to get orders by user: %v", err)

		if errors.Is(err, service.ErrInvalidRequest) {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid request: %v", err)
		}

		return nil, status.Errorf(codes.Internal, "Internal error: %v", err)
	}

	// 转换响应
	protoOrders := make([]*order.Order, len(response.Orders))
	for i, o := range response.Orders {
		protoOrders[i] = h.convertOrderToProto(o)
	}

	return &order.GetOrdersByUserResponse{
		Orders:   protoOrders,
		Total:    response.Total,
		Page:     response.Page,
		PageSize: response.PageSize,
		Message:  "查询成功",
	}, nil
}

// convertOrderToProto 将领域模型转换为protobuf格式
func (h *OrderHandler) convertOrderToProto(o *model.Order) *order.Order {
	return &order.Order{
		Id:          o.ID,
		UserId:      o.UserID,
		OrderNo:     o.OrderNo,
		Amount:      o.Amount,
		Status:      o.Status,
		Description: o.Description,
		CreatedAt:   o.CreatedAt.Unix(),
		UpdatedAt:   o.UpdatedAt.Unix(),
	}
}
