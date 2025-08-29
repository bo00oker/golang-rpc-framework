package service
package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/rpc-framework/core/internal/user/model"
	"github.com/rpc-framework/core/internal/user/repository"
	"github.com/rpc-framework/core/pkg/logger"
	"github.com/rpc-framework/core/pkg/trace"
)

var (
	ErrInvalidRequest = errors.New("invalid request")
	ErrUserExists     = errors.New("user already exists")
	ErrUserNotFound   = errors.New("user not found")
)

// UserService 用户服务接口
type UserService interface {
	CreateUser(ctx context.Context, req *model.CreateUserRequest) (*model.User, error)
	GetUser(ctx context.Context, id int64) (*model.User, error)
	UpdateUser(ctx context.Context, req *model.UpdateUserRequest) (*model.User, error)
	DeleteUser(ctx context.Context, id int64) error
	ListUsers(ctx context.Context, req *model.ListUsersRequest) (*model.ListUsersResponse, error)
}

// userService 用户服务实现
type userService struct {
	userRepo repository.UserRepository
	tracer   *trace.Tracer
	logger   logger.Logger
}

// NewUserService 创建用户服务实例
func NewUserService(userRepo repository.UserRepository, tracer *trace.Tracer) UserService {
	return &userService{
		userRepo: userRepo,
		tracer:   tracer,
		logger:   logger.GetGlobalLogger(),
	}
}

func (s *userService) CreateUser(ctx context.Context, req *model.CreateUserRequest) (*model.User, error) {
	ctx, span := s.tracer.StartSpan(ctx, "UserService.CreateUser")
	defer span.End()

	s.logger.Infof("Creating user: %s", req.Username)

	// 验证请求参数
	if err := s.validateCreateUserRequest(req); err != nil {
		s.logger.Errorf("Invalid create user request: %v", err)
		return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}

	// 检查用户名是否已存在
	if _, err := s.userRepo.GetByUsername(ctx, req.Username); err == nil {
		s.logger.Warnf("User already exists: %s", req.Username)
		return nil, ErrUserExists
	}

	// 创建用户
	user := &model.User{
		Username: req.Username,
		Email:    req.Email,
		Phone:    req.Phone,
		Age:      req.Age,
		Address:  req.Address,
	}

	createdUser, err := s.userRepo.Create(ctx, user)
	if err != nil {
		s.logger.Errorf("Failed to create user: %v", err)
		return nil, err
	}

	s.logger.Infof("User created successfully: ID=%d, Username=%s", createdUser.ID, createdUser.Username)
	return createdUser, nil
}

func (s *userService) GetUser(ctx context.Context, id int64) (*model.User, error) {
	ctx, span := s.tracer.StartSpan(ctx, "UserService.GetUser")
	defer span.End()

	s.logger.Infof("Getting user: ID=%d", id)

	if id <= 0 {
		return nil, fmt.Errorf("%w: invalid user ID", ErrInvalidRequest)
	}

	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		s.logger.Errorf("Failed to get user: %v", err)
		return nil, err
	}

	s.logger.Infof("User retrieved successfully: ID=%d", id)
	return user, nil
}

func (s *userService) UpdateUser(ctx context.Context, req *model.UpdateUserRequest) (*model.User, error) {
	ctx, span := s.tracer.StartSpan(ctx, "UserService.UpdateUser")
	defer span.End()

	s.logger.Infof("Updating user: ID=%d", req.ID)

	// 验证请求参数
	if err := s.validateUpdateUserRequest(req); err != nil {
		s.logger.Errorf("Invalid update user request: %v", err)
		return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}

	// 检查用户是否存在
	_, err := s.userRepo.GetByID(ctx, req.ID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	// 更新用户
	user := &model.User{
		ID:       req.ID,
		Username: req.Username,
		Email:    req.Email,
		Phone:    req.Phone,
		Age:      req.Age,
		Address:  req.Address,
	}

	updatedUser, err := s.userRepo.Update(ctx, user)
	if err != nil {
		s.logger.Errorf("Failed to update user: %v", err)
		return nil, err
	}

	s.logger.Infof("User updated successfully: ID=%d", req.ID)
	return updatedUser, nil
}

func (s *userService) DeleteUser(ctx context.Context, id int64) error {
	ctx, span := s.tracer.StartSpan(ctx, "UserService.DeleteUser")
	defer span.End()

	s.logger.Infof("Deleting user: ID=%d", id)

	if id <= 0 {
		return fmt.Errorf("%w: invalid user ID", ErrInvalidRequest)
	}

	err := s.userRepo.Delete(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return ErrUserNotFound
		}
		s.logger.Errorf("Failed to delete user: %v", err)
		return err
	}

	s.logger.Infof("User deleted successfully: ID=%d", id)
	return nil
}

func (s *userService) ListUsers(ctx context.Context, req *model.ListUsersRequest) (*model.ListUsersResponse, error) {
	ctx, span := s.tracer.StartSpan(ctx, "UserService.ListUsers")
	defer span.End()

	s.logger.Infof("Listing users: page=%d, page_size=%d, keyword=%s", req.Page, req.PageSize, req.Keyword)

	// 验证请求参数
	if err := s.validateListUsersRequest(req); err != nil {
		s.logger.Errorf("Invalid list users request: %v", err)
		return nil, fmt.Errorf("%w: %v", ErrInvalidRequest, err)
	}

	response, err := s.userRepo.List(ctx, req)
	if err != nil {
		s.logger.Errorf("Failed to list users: %v", err)
		return nil, err
	}

	s.logger.Infof("Users listed successfully: total=%d", response.Total)
	return response, nil
}

// 验证创建用户请求
func (s *userService) validateCreateUserRequest(req *model.CreateUserRequest) error {
	if req.Username == "" {
		return errors.New("username is required")
	}
	if len(req.Username) < 3 || len(req.Username) > 20 {
		return errors.New("username must be between 3 and 20 characters")
	}
	if req.Email == "" {
		return errors.New("email is required")
	}
	if req.Phone == "" {
		return errors.New("phone is required")
	}
	if req.Age < 0 || req.Age > 120 {
		return errors.New("age must be between 0 and 120")
	}
	return nil
}

// 验证更新用户请求
func (s *userService) validateUpdateUserRequest(req *model.UpdateUserRequest) error {
	if req.ID <= 0 {
		return errors.New("user ID is required")
	}
	if req.Username != "" && (len(req.Username) < 3 || len(req.Username) > 20) {
		return errors.New("username must be between 3 and 20 characters")
	}
	if req.Age < 0 || req.Age > 120 {
		return errors.New("age must be between 0 and 120")
	}
	return nil
}

// 验证查询用户列表请求
func (s *userService) validateListUsersRequest(req *model.ListUsersRequest) error {
	if req.Page <= 0 {
		return errors.New("page must be greater than 0")
	}
	if req.PageSize <= 0 || req.PageSize > 100 {
		return errors.New("page_size must be between 1 and 100")
	}
	return nil
}