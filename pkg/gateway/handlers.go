package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/rpc-framework/core/proto/order"
	"github.com/rpc-framework/core/proto/user"
)

// === 认证相关处理器 ===

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// handleLogin 处理登录请求
func (g *Gateway) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// 验证用户名和密码（这里简化处理，实际应该验证数据库）
	if req.Username == "" || req.Password == "" {
		g.sendError(w, http.StatusBadRequest, "Username and password are required")
		return
	}

	// 生成JWT令牌（简化处理，实际应该验证用户凭据）
	userID := int64(1) // 模拟用户ID
	role := "user"     // 模拟用户角色

	tokenPair, err := g.authService.GenerateTokens(userID, req.Username, role)
	if err != nil {
		g.sendError(w, http.StatusInternalServerError, "Failed to generate tokens")
		return
	}

	g.sendResponse(w, http.StatusOK, "Login successful", tokenPair)
}

// RefreshTokenRequest 刷新令牌请求
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// handleRefreshToken 处理刷新令牌请求
func (g *Gateway) handleRefreshToken(w http.ResponseWriter, r *http.Request) {
	var req RefreshTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	tokenPair, err := g.authService.RefreshTokens(req.RefreshToken)
	if err != nil {
		g.sendError(w, http.StatusUnauthorized, "Invalid refresh token")
		return
	}

	g.sendResponse(w, http.StatusOK, "Token refreshed", tokenPair)
}

// handleLogout 处理登出请求
func (g *Gateway) handleLogout(w http.ResponseWriter, r *http.Request) {
	// 在实际实现中，应该将token加入黑名单
	g.sendResponse(w, http.StatusOK, "Logout successful", nil)
}

// === 用户相关处理器 ===

// CreateUserRequest 创建用户请求
type CreateUserRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Age      int32  `json:"age"`
	Address  string `json:"address"`
}

// handleCreateUser 处理创建用户请求
func (g *Gateway) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// 验证请求
	validator := &RequestValidator{}
	reqMap := map[string]interface{}{
		"username": req.Username,
		"email":    req.Email,
		"phone":    req.Phone,
		"age":      float64(req.Age),
		"address":  req.Address,
	}

	if err := validator.ValidateCreateUserRequest(reqMap); err != nil {
		g.sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	// 获取用户服务客户端
	userClient, err := g.clientMgr.GetUserClient()
	if err != nil {
		g.sendError(w, http.StatusServiceUnavailable, "User service unavailable")
		return
	}

	// 调用gRPC服务
	grpcReq := &user.CreateUserRequest{
		Username: req.Username,
		Email:    req.Email,
		Phone:    req.Phone,
		Age:      req.Age,
		Address:  req.Address,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := userClient.CreateUser(ctx, grpcReq)
	if err != nil {
		g.logger.Errorf("Failed to create user: %v", err)
		g.sendError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	g.sendResponse(w, http.StatusCreated, resp.Message, resp.User)
}

// handleGetUser 处理获取用户请求
func (g *Gateway) handleGetUser(w http.ResponseWriter, r *http.Request) {
	// 从路径中获取用户ID
	userID, err := g.parseIDFromPath(r, "id")
	if err != nil {
		g.sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	// 获取用户服务客户端
	userClient, err := g.clientMgr.GetUserClient()
	if err != nil {
		g.sendError(w, http.StatusServiceUnavailable, "User service unavailable")
		return
	}

	// 调用gRPC服务
	grpcReq := &user.GetUserRequest{
		UserId: userID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := userClient.GetUser(ctx, grpcReq)
	if err != nil {
		g.logger.Errorf("Failed to get user: %v", err)
		g.sendError(w, http.StatusNotFound, "User not found")
		return
	}

	g.sendResponse(w, http.StatusOK, resp.Message, resp.User)
}

// UpdateUserRequest 更新用户请求
type UpdateUserRequest struct {
	Username string `json:"username,omitempty"`
	Email    string `json:"email,omitempty"`
	Phone    string `json:"phone,omitempty"`
	Age      int32  `json:"age,omitempty"`
	Address  string `json:"address,omitempty"`
}

// handleUpdateUser 处理更新用户请求
func (g *Gateway) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	// 从路径中获取用户ID
	userID, err := g.parseIDFromPath(r, "id")
	if err != nil {
		g.sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// 获取用户服务客户端
	userClient, err := g.clientMgr.GetUserClient()
	if err != nil {
		g.sendError(w, http.StatusServiceUnavailable, "User service unavailable")
		return
	}

	// 调用gRPC服务
	grpcReq := &user.UpdateUserRequest{
		UserId:   userID,
		Username: req.Username,
		Email:    req.Email,
		Phone:    req.Phone,
		Age:      req.Age,
		Address:  req.Address,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := userClient.UpdateUser(ctx, grpcReq)
	if err != nil {
		g.logger.Errorf("Failed to update user: %v", err)
		g.sendError(w, http.StatusInternalServerError, "Failed to update user")
		return
	}

	g.sendResponse(w, http.StatusOK, resp.Message, resp.User)
}

// handleDeleteUser 处理删除用户请求
func (g *Gateway) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	// 从路径中获取用户ID
	userID, err := g.parseIDFromPath(r, "id")
	if err != nil {
		g.sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	// 获取用户服务客户端
	userClient, err := g.clientMgr.GetUserClient()
	if err != nil {
		g.sendError(w, http.StatusServiceUnavailable, "User service unavailable")
		return
	}

	// 调用gRPC服务
	grpcReq := &user.DeleteUserRequest{
		UserId: userID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := userClient.DeleteUser(ctx, grpcReq)
	if err != nil {
		g.logger.Errorf("Failed to delete user: %v", err)
		g.sendError(w, http.StatusInternalServerError, "Failed to delete user")
		return
	}

	g.sendResponse(w, http.StatusOK, resp.Message, map[string]bool{"success": resp.Success})
}

// handleListUsers 处理查询用户列表请求
func (g *Gateway) handleListUsers(w http.ResponseWriter, r *http.Request) {
	// 获取查询参数
	page := int32(g.getQueryParamInt(r, "page", 1))
	pageSize := int32(g.getQueryParamInt(r, "page_size", 10))
	keyword := g.getQueryParam(r, "keyword", "")

	// 获取用户服务客户端
	userClient, err := g.clientMgr.GetUserClient()
	if err != nil {
		g.sendError(w, http.StatusServiceUnavailable, "User service unavailable")
		return
	}

	// 调用gRPC服务
	grpcReq := &user.ListUsersRequest{
		Page:     page,
		PageSize: pageSize,
		Keyword:  keyword,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := userClient.ListUsers(ctx, grpcReq)
	if err != nil {
		g.logger.Errorf("Failed to list users: %v", err)
		g.sendError(w, http.StatusInternalServerError, "Failed to list users")
		return
	}

	result := map[string]interface{}{
		"users":     resp.Users,
		"total":     resp.Total,
		"page":      resp.Page,
		"page_size": resp.PageSize,
	}

	g.sendResponse(w, http.StatusOK, resp.Message, result)
}

// === 订单相关处理器 ===

// CreateOrderRequest 创建订单请求
type CreateOrderRequest struct {
	UserID      int64                    `json:"user_id"`
	Amount      float64                  `json:"amount"`
	Description string                   `json:"description"`
	Items       []CreateOrderItemRequest `json:"items,omitempty"`
}

// CreateOrderItemRequest 创建订单项请求
type CreateOrderItemRequest struct {
	ProductID   int64   `json:"product_id"`
	ProductName string  `json:"product_name"`
	Price       float64 `json:"price"`
	Quantity    int32   `json:"quantity"`
}

// handleCreateOrder 处理创建订单请求
func (g *Gateway) handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// 验证请求
	validator := &RequestValidator{}
	reqMap := map[string]interface{}{
		"user_id": float64(req.UserID),
		"amount":  req.Amount,
	}

	if err := validator.ValidateCreateOrderRequest(reqMap); err != nil {
		g.sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	// 获取订单服务客户端
	orderClient, err := g.clientMgr.GetOrderClient()
	if err != nil {
		g.sendError(w, http.StatusServiceUnavailable, "Order service unavailable")
		return
	}

	// 转换订单项
	var items []*order.OrderItem
	for _, item := range req.Items {
		items = append(items, &order.OrderItem{
			ProductId:   item.ProductID,
			ProductName: item.ProductName,
			Price:       item.Price,
			Quantity:    item.Quantity,
		})
	}

	// 调用gRPC服务
	grpcReq := &order.CreateOrderRequest{
		UserId:      req.UserID,
		Amount:      req.Amount,
		Description: req.Description,
		Items:       items,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := orderClient.CreateOrder(ctx, grpcReq)
	if err != nil {
		g.logger.Errorf("Failed to create order: %v", err)
		g.sendError(w, http.StatusInternalServerError, "Failed to create order")
		return
	}

	g.sendResponse(w, http.StatusCreated, resp.Message, resp.Order)
}

// handleGetOrder 处理获取订单请求
func (g *Gateway) handleGetOrder(w http.ResponseWriter, r *http.Request) {
	// 从路径中获取订单ID
	orderID, err := g.parseIDFromPath(r, "id")
	if err != nil {
		g.sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	// 获取订单服务客户端
	orderClient, err := g.clientMgr.GetOrderClient()
	if err != nil {
		g.sendError(w, http.StatusServiceUnavailable, "Order service unavailable")
		return
	}

	// 调用gRPC服务
	grpcReq := &order.GetOrderRequest{
		OrderId: orderID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := orderClient.GetOrder(ctx, grpcReq)
	if err != nil {
		g.logger.Errorf("Failed to get order: %v", err)
		g.sendError(w, http.StatusNotFound, "Order not found")
		return
	}

	g.sendResponse(w, http.StatusOK, resp.Message, resp.Order)
}

// UpdateOrderRequest 更新订单请求
type UpdateOrderRequest struct {
	Amount      float64 `json:"amount,omitempty"`
	Status      string  `json:"status,omitempty"`
	Description string  `json:"description,omitempty"`
}

// handleUpdateOrder 处理更新订单请求
func (g *Gateway) handleUpdateOrder(w http.ResponseWriter, r *http.Request) {
	// 从路径中获取订单ID
	orderID, err := g.parseIDFromPath(r, "id")
	if err != nil {
		g.sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req UpdateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.sendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// 获取订单服务客户端
	orderClient, err := g.clientMgr.GetOrderClient()
	if err != nil {
		g.sendError(w, http.StatusServiceUnavailable, "Order service unavailable")
		return
	}

	// 调用gRPC服务
	grpcReq := &order.UpdateOrderRequest{
		OrderId:     orderID,
		Amount:      req.Amount,
		Status:      req.Status,
		Description: req.Description,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := orderClient.UpdateOrder(ctx, grpcReq)
	if err != nil {
		g.logger.Errorf("Failed to update order: %v", err)
		g.sendError(w, http.StatusInternalServerError, "Failed to update order")
		return
	}

	g.sendResponse(w, http.StatusOK, resp.Message, resp.Order)
}

// handleDeleteOrder 处理删除订单请求
func (g *Gateway) handleDeleteOrder(w http.ResponseWriter, r *http.Request) {
	// 从路径中获取订单ID
	orderID, err := g.parseIDFromPath(r, "id")
	if err != nil {
		g.sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	// 获取订单服务客户端
	orderClient, err := g.clientMgr.GetOrderClient()
	if err != nil {
		g.sendError(w, http.StatusServiceUnavailable, "Order service unavailable")
		return
	}

	// 调用gRPC服务
	grpcReq := &order.DeleteOrderRequest{
		OrderId: orderID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := orderClient.DeleteOrder(ctx, grpcReq)
	if err != nil {
		g.logger.Errorf("Failed to delete order: %v", err)
		g.sendError(w, http.StatusInternalServerError, "Failed to delete order")
		return
	}

	g.sendResponse(w, http.StatusOK, resp.Message, map[string]bool{"success": resp.Success})
}

// handleListOrders 处理查询订单列表请求
func (g *Gateway) handleListOrders(w http.ResponseWriter, r *http.Request) {
	// 获取查询参数
	page := int32(g.getQueryParamInt(r, "page", 1))
	pageSize := int32(g.getQueryParamInt(r, "page_size", 10))
	status := g.getQueryParam(r, "status", "")
	userIDStr := g.getQueryParam(r, "user_id", "0")

	userID, _ := strconv.ParseInt(userIDStr, 10, 64)

	// 获取订单服务客户端
	orderClient, err := g.clientMgr.GetOrderClient()
	if err != nil {
		g.sendError(w, http.StatusServiceUnavailable, "Order service unavailable")
		return
	}

	// 调用gRPC服务
	grpcReq := &order.ListOrdersRequest{
		Page:     page,
		PageSize: pageSize,
		Status:   status,
		UserId:   userID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := orderClient.ListOrders(ctx, grpcReq)
	if err != nil {
		g.logger.Errorf("Failed to list orders: %v", err)
		g.sendError(w, http.StatusInternalServerError, "Failed to list orders")
		return
	}

	result := map[string]interface{}{
		"orders":    resp.Orders,
		"total":     resp.Total,
		"page":      resp.Page,
		"page_size": resp.PageSize,
	}

	g.sendResponse(w, http.StatusOK, resp.Message, result)
}

// handleGetOrdersByUser 处理根据用户ID查询订单请求
func (g *Gateway) handleGetOrdersByUser(w http.ResponseWriter, r *http.Request) {
	// 从路径中获取用户ID
	userID, err := g.parseIDFromPath(r, "user_id")
	if err != nil {
		g.sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	// 获取查询参数
	page := int32(g.getQueryParamInt(r, "page", 1))
	pageSize := int32(g.getQueryParamInt(r, "page_size", 10))

	// 获取订单服务客户端
	orderClient, err := g.clientMgr.GetOrderClient()
	if err != nil {
		g.sendError(w, http.StatusServiceUnavailable, "Order service unavailable")
		return
	}

	// 调用gRPC服务
	grpcReq := &order.GetOrdersByUserRequest{
		UserId:   userID,
		Page:     page,
		PageSize: pageSize,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := orderClient.GetOrdersByUser(ctx, grpcReq)
	if err != nil {
		g.logger.Errorf("Failed to get orders by user: %v", err)
		g.sendError(w, http.StatusInternalServerError, "Failed to get orders")
		return
	}

	result := map[string]interface{}{
		"orders":    resp.Orders,
		"total":     resp.Total,
		"page":      resp.Page,
		"page_size": resp.PageSize,
	}

	g.sendResponse(w, http.StatusOK, resp.Message, result)
}
