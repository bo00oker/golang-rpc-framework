package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/rpc-framework/core/internal/order/model"
	"github.com/rpc-framework/core/internal/order/repository"
	"github.com/rpc-framework/core/pkg/logger"
	"github.com/rpc-framework/core/pkg/trace"
)

var (
	ErrInvalidRequest = errors.New("invalid request")
	ErrOrderNotFound  = errors.New("order not found")
)

// OrderService 订单服务接口
type OrderService interface {
	CreateOrder(ctx context.Context, req *model.CreateOrderRequest) (*model.Order, error)
	GetOrder(ctx context.Context, id int64) (*model.Order, error)
	UpdateOrder(ctx context.Context, req *model.UpdateOrderRequest) (*model.Order, error)
	DeleteOrder(ctx context.Context, id int64) error
	ListOrders(ctx context.Context, req *model.ListOrdersRequest) (*model.ListOrdersResponse, error)
	GetOrdersByUser(ctx context.Context, userID int64, page, pageSize int32) (*model.ListOrdersResponse, error)
}

// orderService 订单服务实现
type orderService struct {
	orderRepo repository.OrderRepository
	tracer    *trace.Tracer
	logger    logger.Logger
}

// NewOrderService 创建订单服务实例
func NewOrderService(orderRepo repository.OrderRepository, tracer *trace.Tracer) OrderService {
	return &orderService{
		orderRepo: orderRepo,
		tracer:    tracer,
		logger:    logger.GetGlobalLogger(),
	}
}

func (s *orderService) CreateOrder(ctx context.Context, req *model.CreateOrderRequest) (*model.Order, error) {
	ctx, span := s.tracer.StartSpan(ctx, "OrderService.CreateOrder")
	defer span.End()

	s.logger.Infof("Creating order for user: %d, amount: %.2f", req.UserID, req.Amount)

	// 验证请求参数
	if err := s.validateCreateOrderRequest(req); err != nil {
		s.logger.Errorf("Invalid create order request: %v", err)
		return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}

	// 创建订单
	order := &model.Order{
		UserID:      req.UserID,
		Amount:      req.Amount,
		Description: req.Description,
		Items:       req.Items,
	}

	createdOrder, err := s.orderRepo.Create(ctx, order)
	if err != nil {
		s.logger.Errorf("Failed to create order: %v", err)
		return nil, err
	}

	s.logger.Infof("Order created successfully: ID=%d, UserID=%d, OrderNo=%s, Amount=%.2f",
		createdOrder.ID, createdOrder.UserID, createdOrder.OrderNo, createdOrder.Amount)

	return createdOrder, nil
}

func (s *orderService) GetOrder(ctx context.Context, id int64) (*model.Order, error) {
	ctx, span := s.tracer.StartSpan(ctx, "OrderService.GetOrder")
	defer span.End()

	s.logger.Infof("Getting order: ID=%d", id)

	if id <= 0 {
		return nil, fmt.Errorf("%w: invalid order ID", ErrInvalidRequest)
	}

	order, err := s.orderRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrOrderNotFound) {
			return nil, ErrOrderNotFound
		}
		s.logger.Errorf("Failed to get order: %v", err)
		return nil, err
	}

	s.logger.Infof("Order retrieved successfully: ID=%d", id)
	return order, nil
}

func (s *orderService) UpdateOrder(ctx context.Context, req *model.UpdateOrderRequest) (*model.Order, error) {
	ctx, span := s.tracer.StartSpan(ctx, "OrderService.UpdateOrder")
	defer span.End()

	s.logger.Infof("Updating order: ID=%d", req.ID)

	// 验证请求参数
	if err := s.validateUpdateOrderRequest(req); err != nil {
		s.logger.Errorf("Invalid update order request: %v", err)
		return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}

	// 检查订单是否存在
	_, err := s.orderRepo.GetByID(ctx, req.ID)
	if err != nil {
		if errors.Is(err, repository.ErrOrderNotFound) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}

	// 更新订单
	order := &model.Order{
		ID:          req.ID,
		Amount:      req.Amount,
		Status:      req.Status,
		Description: req.Description,
	}

	updatedOrder, err := s.orderRepo.Update(ctx, order)
	if err != nil {
		s.logger.Errorf("Failed to update order: %v", err)
		return nil, err
	}

	s.logger.Infof("Order updated successfully: ID=%d", req.ID)
	return updatedOrder, nil
}

func (s *orderService) DeleteOrder(ctx context.Context, id int64) error {
	ctx, span := s.tracer.StartSpan(ctx, "OrderService.DeleteOrder")
	defer span.End()

	s.logger.Infof("Deleting order: ID=%d", id)

	if id <= 0 {
		return fmt.Errorf("%w: invalid order ID", ErrInvalidRequest)
	}

	err := s.orderRepo.Delete(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrOrderNotFound) {
			return ErrOrderNotFound
		}
		s.logger.Errorf("Failed to delete order: %v", err)
		return err
	}

	s.logger.Infof("Order deleted successfully: ID=%d", id)
	return nil
}

func (s *orderService) ListOrders(ctx context.Context, req *model.ListOrdersRequest) (*model.ListOrdersResponse, error) {
	ctx, span := s.tracer.StartSpan(ctx, "OrderService.ListOrders")
	defer span.End()

	s.logger.Infof("Listing orders: page=%d, page_size=%d, user_id=%d, status=%s",
		req.Page, req.PageSize, req.UserID, req.Status)

	// 验证请求参数
	if err := s.validateListOrdersRequest(req); err != nil {
		s.logger.Errorf("Invalid list orders request: %v", err)
		return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}

	response, err := s.orderRepo.List(ctx, req)
	if err != nil {
		s.logger.Errorf("Failed to list orders: %v", err)
		return nil, err
	}

	s.logger.Infof("Orders listed successfully: total=%d", response.Total)
	return response, nil
}

func (s *orderService) GetOrdersByUser(ctx context.Context, userID int64, page, pageSize int32) (*model.ListOrdersResponse, error) {
	ctx, span := s.tracer.StartSpan(ctx, "OrderService.GetOrdersByUser")
	defer span.End()

	s.logger.Infof("Getting orders by user: user_id=%d, page=%d, page_size=%d", userID, page, pageSize)

	if userID <= 0 {
		return nil, fmt.Errorf("%w: invalid user ID", ErrInvalidRequest)
	}

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	response, err := s.orderRepo.GetByUserID(ctx, userID, page, pageSize)
	if err != nil {
		s.logger.Errorf("Failed to get orders by user: %v", err)
		return nil, err
	}

	s.logger.Infof("Orders by user retrieved successfully: user_id=%d, total=%d", userID, response.Total)
	return response, nil
}

// 验证创建订单请求
func (s *orderService) validateCreateOrderRequest(req *model.CreateOrderRequest) error {
	if req.UserID <= 0 {
		return errors.New("user ID is required")
	}
	if req.Amount < 0 {
		return errors.New("amount must be greater than or equal to 0")
	}
	if len(req.Items) == 0 {
		return errors.New("order items are required")
	}

	// 验证订单项
	for i, item := range req.Items {
		if item.ProductName == "" {
			return fmt.Errorf("product name is required for item %d", i+1)
		}
		if item.Quantity <= 0 {
			return fmt.Errorf("quantity must be greater than 0 for item %d", i+1)
		}
		if item.Price < 0 {
			return fmt.Errorf("price must be greater than or equal to 0 for item %d", i+1)
		}
	}

	return nil
}

// 验证更新订单请求
func (s *orderService) validateUpdateOrderRequest(req *model.UpdateOrderRequest) error {
	if req.ID <= 0 {
		return errors.New("order ID is required")
	}
	if req.Amount < 0 {
		return errors.New("amount must be greater than or equal to 0")
	}
	if req.Status != "" && !model.IsValidStatus(req.Status) {
		return errors.New("invalid order status")
	}
	return nil
}

// 验证查询订单列表请求
func (s *orderService) validateListOrdersRequest(req *model.ListOrdersRequest) error {
	if req.Page <= 0 {
		return errors.New("page must be greater than 0")
	}
	if req.PageSize <= 0 || req.PageSize > 100 {
		return errors.New("page_size must be between 1 and 100")
	}
	return nil
}
