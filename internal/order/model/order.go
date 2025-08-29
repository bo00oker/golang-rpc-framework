package model
package model

import (
	"fmt"
	"time"
)

// 订单状态常量
const (
	OrderStatusPending    = "pending"    // 待处理
	OrderStatusProcessing = "processing" // 处理中
	OrderStatusCompleted  = "completed"  // 已完成
	OrderStatusCancelled  = "cancelled"  // 已取消
)

// Order 订单领域模型
type Order struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	OrderNo     string    `json:"order_no"`
	Amount      float64   `json:"amount"`
	Status      string    `json:"status"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Items       []*OrderItem `json:"items,omitempty"`
}

// OrderItem 订单项
type OrderItem struct {
	ID          int64   `json:"id"`
	OrderID     int64   `json:"order_id"`
	ProductID   int64   `json:"product_id"`
	ProductName string  `json:"product_name"`
	Price       float64 `json:"price"`
	Quantity    int32   `json:"quantity"`
}

// CreateOrderRequest 创建订单请求
type CreateOrderRequest struct {
	UserID      int64        `json:"user_id" validate:"required"`
	Amount      float64      `json:"amount" validate:"required,min=0"`
	Description string       `json:"description"`
	Items       []*OrderItem `json:"items" validate:"required,min=1"`
}

// UpdateOrderRequest 更新订单请求
type UpdateOrderRequest struct {
	ID          int64   `json:"id" validate:"required"`
	Amount      float64 `json:"amount,omitempty" validate:"omitempty,min=0"`
	Status      string  `json:"status,omitempty"`
	Description string  `json:"description,omitempty"`
}

// ListOrdersRequest 查询订单列表请求
type ListOrdersRequest struct {
	Page     int32  `json:"page" validate:"min=1"`
	PageSize int32  `json:"page_size" validate:"min=1,max=100"`
	Status   string `json:"status,omitempty"`
	UserID   int64  `json:"user_id,omitempty"`
}

// ListOrdersResponse 查询订单列表响应
type ListOrdersResponse struct {
	Orders   []*Order `json:"orders"`
	Total    int32    `json:"total"`
	Page     int32    `json:"page"`
	PageSize int32    `json:"page_size"`
}

// GenerateOrderNo 生成订单号
func GenerateOrderNo() string {
	return fmt.Sprintf("ORD%d", time.Now().UnixNano())
}

// CalculateAmount 计算订单总金额
func (o *Order) CalculateAmount() {
	var total float64
	for _, item := range o.Items {
		total += item.Price * float64(item.Quantity)
	}
	o.Amount = total
}

// IsValidStatus 检查订单状态是否有效
func IsValidStatus(status string) bool {
	switch status {
	case OrderStatusPending, OrderStatusProcessing, OrderStatusCompleted, OrderStatusCancelled:
		return true
	default:
		return false
	}
}