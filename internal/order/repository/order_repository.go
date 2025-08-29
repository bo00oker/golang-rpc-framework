package repository
package repository

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/rpc-framework/core/internal/order/model"
)

var (
	ErrOrderNotFound  = errors.New("order not found")
	ErrInvalidOrderID = errors.New("invalid order ID")
)

// OrderRepository 订单仓储接口
type OrderRepository interface {
	Create(ctx context.Context, order *model.Order) (*model.Order, error)
	GetByID(ctx context.Context, id int64) (*model.Order, error)
	Update(ctx context.Context, order *model.Order) (*model.Order, error)
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, req *model.ListOrdersRequest) (*model.ListOrdersResponse, error)
	GetByUserID(ctx context.Context, userID int64, page, pageSize int32) (*model.ListOrdersResponse, error)
}

// memoryOrderRepository 内存订单仓储实现
type memoryOrderRepository struct {
	mu     sync.RWMutex
	orders map[int64]*model.Order
	nextID int64
}

// NewMemoryOrderRepository 创建内存订单仓储
func NewMemoryOrderRepository() OrderRepository {
	return &memoryOrderRepository{
		orders: make(map[int64]*model.Order),
		nextID: 1,
	}
}

func (r *memoryOrderRepository) Create(ctx context.Context, order *model.Order) (*model.Order, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	newOrder := &model.Order{
		ID:          r.nextID,
		UserID:      order.UserID,
		OrderNo:     model.GenerateOrderNo(),
		Amount:      order.Amount,
		Status:      model.OrderStatusPending,
		Description: order.Description,
		CreatedAt:   now,
		UpdatedAt:   now,
		Items:       order.Items,
	}

	r.orders[newOrder.ID] = newOrder
	r.nextID++

	return r.copyOrder(newOrder), nil
}

func (r *memoryOrderRepository) GetByID(ctx context.Context, id int64) (*model.Order, error) {
	if id <= 0 {
		return nil, ErrInvalidOrderID
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	order, exists := r.orders[id]
	if !exists {
		return nil, ErrOrderNotFound
	}

	return r.copyOrder(order), nil
}

func (r *memoryOrderRepository) Update(ctx context.Context, order *model.Order) (*model.Order, error) {
	if order.ID <= 0 {
		return nil, ErrInvalidOrderID
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	existingOrder, exists := r.orders[order.ID]
	if !exists {
		return nil, ErrOrderNotFound
	}

	// 更新字段
	if order.Amount > 0 {
		existingOrder.Amount = order.Amount
	}
	if order.Status != "" {
		existingOrder.Status = order.Status
	}
	if order.Description != "" {
		existingOrder.Description = order.Description
	}
	
	existingOrder.UpdatedAt = time.Now()

	return r.copyOrder(existingOrder), nil
}

func (r *memoryOrderRepository) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrInvalidOrderID
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.orders[id]; !exists {
		return ErrOrderNotFound
	}

	delete(r.orders, id)
	return nil
}

func (r *memoryOrderRepository) List(ctx context.Context, req *model.ListOrdersRequest) (*model.ListOrdersResponse, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filteredOrders []*model.Order
	for _, order := range r.orders {
		// 用户ID过滤
		if req.UserID > 0 && order.UserID != req.UserID {
			continue
		}
		
		// 状态过滤
		if req.Status != "" && order.Status != req.Status {
			continue
		}
		
		filteredOrders = append(filteredOrders, r.copyOrder(order))
	}

	// 分页处理
	total := int32(len(filteredOrders))
	start := (req.Page - 1) * req.PageSize
	end := start + req.PageSize

	if start >= total {
		return &model.ListOrdersResponse{
			Orders:   []*model.Order{},
			Total:    total,
			Page:     req.Page,
			PageSize: req.PageSize,
		}, nil
	}

	if end > total {
		end = total
	}

	return &model.ListOrdersResponse{
		Orders:   filteredOrders[start:end],
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}

func (r *memoryOrderRepository) GetByUserID(ctx context.Context, userID int64, page, pageSize int32) (*model.ListOrdersResponse, error) {
	req := &model.ListOrdersRequest{
		Page:     page,
		PageSize: pageSize,
		UserID:   userID,
	}
	return r.List(ctx, req)
}

// copyOrder 创建订单副本，避免外部修改
func (r *memoryOrderRepository) copyOrder(order *model.Order) *model.Order {
	// 复制订单项
	var items []*model.OrderItem
	for _, item := range order.Items {
		items = append(items, &model.OrderItem{
			ID:          item.ID,
			OrderID:     item.OrderID,
			ProductID:   item.ProductID,
			ProductName: item.ProductName,
			Price:       item.Price,
			Quantity:    item.Quantity,
		})
	}
	
	return &model.Order{
		ID:          order.ID,
		UserID:      order.UserID,
		OrderNo:     order.OrderNo,
		Amount:      order.Amount,
		Status:      order.Status,
		Description: order.Description,
		CreatedAt:   order.CreatedAt,
		UpdatedAt:   order.UpdatedAt,
		Items:       items,
	}
}