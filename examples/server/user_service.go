package main

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/rpc-framework/core/pkg/logger"
	"github.com/rpc-framework/core/pkg/trace"
	"github.com/rpc-framework/core/proto/user"
)

// UserServiceImpl 用户服务实现
type UserServiceImpl struct {
	user.UnimplementedUserServiceServer
	tracer *trace.Tracer
	users  map[int64]*user.User
	mu     sync.RWMutex
	nextID int64
}

// NewUserService 创建用户服务实例
func NewUserService(tracer *trace.Tracer) *UserServiceImpl {
	return &UserServiceImpl{
		tracer: tracer,
		users:  make(map[int64]*user.User),
		nextID: 1,
	}
}

// CreateUser 创建用户
func (s *UserServiceImpl) CreateUser(ctx context.Context, req *user.CreateUserRequest) (*user.CreateUserResponse, error) {
	ctx, span := s.tracer.StartSpan(ctx, "CreateUser")
	defer span.End()

	log := logger.GetGlobalLogger()
	log.Infof("Creating user: %s", req.Username)

	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查用户名是否已存在
	for _, u := range s.users {
		if u.Username == req.Username {
			return &user.CreateUserResponse{
				Message: "用户名已存在",
			}, nil
		}
	}

	now := time.Now().Unix()
	newUser := &user.User{
		Id:        s.nextID,
		Username:  req.Username,
		Email:     req.Email,
		Phone:     req.Phone,
		Age:       req.Age,
		Address:   req.Address,
		CreatedAt: now,
		UpdatedAt: now,
	}

	s.users[newUser.Id] = newUser
	s.nextID++

	log.Infof("User created successfully: ID=%d, Username=%s", newUser.Id, newUser.Username)

	return &user.CreateUserResponse{
		User:    newUser,
		Message: "用户创建成功",
	}, nil
}

// GetUser 查询用户
func (s *UserServiceImpl) GetUser(ctx context.Context, req *user.GetUserRequest) (*user.GetUserResponse, error) {
	ctx, span := s.tracer.StartSpan(ctx, "GetUser")
	defer span.End()

	log := logger.GetGlobalLogger()
	log.Infof("Getting user: ID=%d", req.UserId)

	s.mu.RLock()
	defer s.mu.RUnlock()

	u, exists := s.users[req.UserId]
	if !exists {
		return &user.GetUserResponse{
			Message: "用户不存在",
		}, nil
	}

	return &user.GetUserResponse{
		User:    u,
		Message: "查询成功",
	}, nil
}

// ListUsers 查询用户列表
func (s *UserServiceImpl) ListUsers(ctx context.Context, req *user.ListUsersRequest) (*user.ListUsersResponse, error) {
	ctx, span := s.tracer.StartSpan(ctx, "ListUsers")
	defer span.End()

	log := logger.GetGlobalLogger()
	log.Infof("Listing users: page=%d, page_size=%d, keyword=%s", req.Page, req.PageSize, req.Keyword)

	s.mu.RLock()
	defer s.mu.RUnlock()

	var filteredUsers []*user.User
	for _, u := range s.users {
		// 如果有关键字，进行过滤
		if req.Keyword != "" {
			if !contains(u.Username, req.Keyword) && !contains(u.Email, req.Keyword) {
				continue
			}
		}
		filteredUsers = append(filteredUsers, u)
	}

	// 分页处理
	total := int32(len(filteredUsers))
	start := (req.Page - 1) * req.PageSize
	end := start + req.PageSize

	if start >= total {
		return &user.ListUsersResponse{
			Users:    []*user.User{},
			Total:    total,
			Page:     req.Page,
			PageSize: req.PageSize,
			Message:  "查询成功",
		}, nil
	}

	if end > total {
		end = total
	}

	return &user.ListUsersResponse{
		Users:    filteredUsers[start:end],
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Message:  "查询成功",
	}, nil
}

// UpdateUser 更新用户
func (s *UserServiceImpl) UpdateUser(ctx context.Context, req *user.UpdateUserRequest) (*user.UpdateUserResponse, error) {
	ctx, span := s.tracer.StartSpan(ctx, "UpdateUser")
	defer span.End()

	log := logger.GetGlobalLogger()
	log.Infof("Updating user: ID=%d", req.UserId)

	s.mu.Lock()
	defer s.mu.Unlock()

	u, exists := s.users[req.UserId]
	if !exists {
		return &user.UpdateUserResponse{
			Message: "用户不存在",
		}, nil
	}

	// 更新字段
	if req.Username != "" {
		u.Username = req.Username
	}
	if req.Email != "" {
		u.Email = req.Email
	}
	if req.Phone != "" {
		u.Phone = req.Phone
	}
	if req.Age > 0 {
		u.Age = req.Age
	}
	if req.Address != "" {
		u.Address = req.Address
	}
	u.UpdatedAt = time.Now().Unix()

	log.Infof("User updated successfully: ID=%d", req.UserId)

	return &user.UpdateUserResponse{
		User:    u,
		Message: "用户更新成功",
	}, nil
}

// DeleteUser 删除用户
func (s *UserServiceImpl) DeleteUser(ctx context.Context, req *user.DeleteUserRequest) (*user.DeleteUserResponse, error) {
	ctx, span := s.tracer.StartSpan(ctx, "DeleteUser")
	defer span.End()

	log := logger.GetGlobalLogger()
	log.Infof("Deleting user: ID=%d", req.UserId)

	s.mu.Lock()
	defer s.mu.Unlock()

	_, exists := s.users[req.UserId]
	if !exists {
		return &user.DeleteUserResponse{
			Success: false,
			Message: "用户不存在",
		}, nil
	}

	delete(s.users, req.UserId)

	log.Infof("User deleted successfully: ID=%d", req.UserId)

	return &user.DeleteUserResponse{
		Success: true,
		Message: "用户删除成功",
	}, nil
}

// contains 检查字符串是否包含子字符串
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
