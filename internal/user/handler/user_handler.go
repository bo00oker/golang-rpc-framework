package handler
package handler

import (
	"context"
	"errors"

	"github.com/rpc-framework/core/internal/user/model"
	"github.com/rpc-framework/core/internal/user/service"
	"github.com/rpc-framework/core/pkg/logger"
	"github.com/rpc-framework/core/pkg/trace"
	"github.com/rpc-framework/core/proto/user"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UserHandler 用户gRPC处理器
type UserHandler struct {
	user.UnimplementedUserServiceServer
	userService service.UserService
	tracer      *trace.Tracer
	logger      logger.Logger
}

// NewUserHandler 创建用户处理器
func NewUserHandler(userService service.UserService, tracer *trace.Tracer) *UserHandler {
	return &UserHandler{
		userService: userService,
		tracer:      tracer,
		logger:      logger.GetGlobalLogger(),
	}
}

// CreateUser 创建用户
func (h *UserHandler) CreateUser(ctx context.Context, req *user.CreateUserRequest) (*user.CreateUserResponse, error) {
	ctx, span := h.tracer.StartSpan(ctx, "UserHandler.CreateUser")
	defer span.End()

	// 转换请求参数
	createReq := &model.CreateUserRequest{
		Username: req.Username,
		Email:    req.Email,
		Phone:    req.Phone,
		Age:      req.Age,
		Address:  req.Address,
	}

	// 调用服务层
	createdUser, err := h.userService.CreateUser(ctx, createReq)
	if err != nil {
		h.logger.Errorf("Failed to create user: %v", err)
		
		if errors.Is(err, service.ErrInvalidRequest) {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid request: %v", err)
		}
		if errors.Is(err, service.ErrUserExists) {
			return &user.CreateUserResponse{
				Message: "用户名已存在",
			}, nil
		}
		
		return nil, status.Errorf(codes.Internal, "Internal error: %v", err)
	}

	// 转换响应
	return &user.CreateUserResponse{
		User:    h.convertUserToProto(createdUser),
		Message: "用户创建成功",
	}, nil
}

// GetUser 查询用户
func (h *UserHandler) GetUser(ctx context.Context, req *user.GetUserRequest) (*user.GetUserResponse, error) {
	ctx, span := h.tracer.StartSpan(ctx, "UserHandler.GetUser")
	defer span.End()

	// 调用服务层
	foundUser, err := h.userService.GetUser(ctx, req.UserId)
	if err != nil {
		h.logger.Errorf("Failed to get user: %v", err)
		
		if errors.Is(err, service.ErrInvalidRequest) {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid request: %v", err)
		}
		if errors.Is(err, service.ErrUserNotFound) {
			return &user.GetUserResponse{
				Message: "用户不存在",
			}, nil
		}
		
		return nil, status.Errorf(codes.Internal, "Internal error: %v", err)
	}

	// 转换响应
	return &user.GetUserResponse{
		User:    h.convertUserToProto(foundUser),
		Message: "查询成功",
	}, nil
}

// UpdateUser 更新用户
func (h *UserHandler) UpdateUser(ctx context.Context, req *user.UpdateUserRequest) (*user.UpdateUserResponse, error) {
	ctx, span := h.tracer.StartSpan(ctx, "UserHandler.UpdateUser")
	defer span.End()

	// 转换请求参数
	updateReq := &model.UpdateUserRequest{
		ID:       req.UserId,
		Username: req.Username,
		Email:    req.Email,
		Phone:    req.Phone,
		Age:      req.Age,
		Address:  req.Address,
	}

	// 调用服务层
	updatedUser, err := h.userService.UpdateUser(ctx, updateReq)
	if err != nil {
		h.logger.Errorf("Failed to update user: %v", err)
		
		if errors.Is(err, service.ErrInvalidRequest) {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid request: %v", err)
		}
		if errors.Is(err, service.ErrUserNotFound) {
			return &user.UpdateUserResponse{
				Message: "用户不存在",
			}, nil
		}
		
		return nil, status.Errorf(codes.Internal, "Internal error: %v", err)
	}

	// 转换响应
	return &user.UpdateUserResponse{
		User:    h.convertUserToProto(updatedUser),
		Message: "用户更新成功",
	}, nil
}

// DeleteUser 删除用户
func (h *UserHandler) DeleteUser(ctx context.Context, req *user.DeleteUserRequest) (*user.DeleteUserResponse, error) {
	ctx, span := h.tracer.StartSpan(ctx, "UserHandler.DeleteUser")
	defer span.End()

	// 调用服务层
	err := h.userService.DeleteUser(ctx, req.UserId)
	if err != nil {
		h.logger.Errorf("Failed to delete user: %v", err)
		
		if errors.Is(err, service.ErrInvalidRequest) {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid request: %v", err)
		}
		if errors.Is(err, service.ErrUserNotFound) {
			return &user.DeleteUserResponse{
				Success: false,
				Message: "用户不存在",
			}, nil
		}
		
		return nil, status.Errorf(codes.Internal, "Internal error: %v", err)
	}

	// 返回响应
	return &user.DeleteUserResponse{
		Success: true,
		Message: "用户删除成功",
	}, nil
}

// ListUsers 查询用户列表
func (h *UserHandler) ListUsers(ctx context.Context, req *user.ListUsersRequest) (*user.ListUsersResponse, error) {
	ctx, span := h.tracer.StartSpan(ctx, "UserHandler.ListUsers")
	defer span.End()

	// 转换请求参数
	listReq := &model.ListUsersRequest{
		Page:     req.Page,
		PageSize: req.PageSize,
		Keyword:  req.Keyword,
	}

	// 设置默认值
	if listReq.Page <= 0 {
		listReq.Page = 1
	}
	if listReq.PageSize <= 0 {
		listReq.PageSize = 10
	}

	// 调用服务层
	response, err := h.userService.ListUsers(ctx, listReq)
	if err != nil {
		h.logger.Errorf("Failed to list users: %v", err)
		
		if errors.Is(err, service.ErrInvalidRequest) {
			return nil, status.Errorf(codes.InvalidArgument, "Invalid request: %v", err)
		}
		
		return nil, status.Errorf(codes.Internal, "Internal error: %v", err)
	}

	// 转换响应
	protoUsers := make([]*user.User, len(response.Users))
	for i, u := range response.Users {
		protoUsers[i] = h.convertUserToProto(u)
	}

	return &user.ListUsersResponse{
		Users:    protoUsers,
		Total:    response.Total,
		Page:     response.Page,
		PageSize: response.PageSize,
		Message:  "查询成功",
	}, nil
}

// convertUserToProto 将领域模型转换为protobuf格式
func (h *UserHandler) convertUserToProto(u *model.User) *user.User {
	return &user.User{
		Id:        u.ID,
		Username:  u.Username,
		Email:     u.Email,
		Phone:     u.Phone,
		Age:       u.Age,
		Address:   u.Address,
		CreatedAt: u.CreatedAt.Unix(),
		UpdatedAt: u.UpdatedAt.Unix(),
	}
}