package repository
package repository

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/rpc-framework/core/internal/user/model"
)

var (
	ErrUserNotFound     = errors.New("user not found")
	ErrUserExists       = errors.New("user already exists")
	ErrInvalidUserID    = errors.New("invalid user ID")
)

// UserRepository 用户仓储接口
type UserRepository interface {
	Create(ctx context.Context, user *model.User) (*model.User, error)
	GetByID(ctx context.Context, id int64) (*model.User, error)
	GetByUsername(ctx context.Context, username string) (*model.User, error)
	Update(ctx context.Context, user *model.User) (*model.User, error)
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, req *model.ListUsersRequest) (*model.ListUsersResponse, error)
}

// memoryUserRepository 内存用户仓储实现
type memoryUserRepository struct {
	mu     sync.RWMutex
	users  map[int64]*model.User
	nextID int64
}

// NewMemoryUserRepository 创建内存用户仓储
func NewMemoryUserRepository() UserRepository {
	return &memoryUserRepository{
		users:  make(map[int64]*model.User),
		nextID: 1,
	}
}

func (r *memoryUserRepository) Create(ctx context.Context, user *model.User) (*model.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 检查用户名是否已存在
	for _, u := range r.users {
		if u.Username == user.Username {
			return nil, ErrUserExists
		}
	}

	now := time.Now()
	newUser := &model.User{
		ID:        r.nextID,
		Username:  user.Username,
		Email:     user.Email,
		Phone:     user.Phone,
		Age:       user.Age,
		Address:   user.Address,
		CreatedAt: now,
		UpdatedAt: now,
	}

	r.users[newUser.ID] = newUser
	r.nextID++

	return newUser, nil
}

func (r *memoryUserRepository) GetByID(ctx context.Context, id int64) (*model.User, error) {
	if id <= 0 {
		return nil, ErrInvalidUserID
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	user, exists := r.users[id]
	if !exists {
		return nil, ErrUserNotFound
	}

	// 返回用户副本，避免外部修改
	return &model.User{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		Phone:     user.Phone,
		Age:       user.Age,
		Address:   user.Address,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}, nil
}

func (r *memoryUserRepository) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	if username == "" {
		return nil, errors.New("username cannot be empty")
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, user := range r.users {
		if user.Username == username {
			return &model.User{
				ID:        user.ID,
				Username:  user.Username,
				Email:     user.Email,
				Phone:     user.Phone,
				Age:       user.Age,
				Address:   user.Address,
				CreatedAt: user.CreatedAt,
				UpdatedAt: user.UpdatedAt,
			}, nil
		}
	}

	return nil, ErrUserNotFound
}

func (r *memoryUserRepository) Update(ctx context.Context, user *model.User) (*model.User, error) {
	if user.ID <= 0 {
		return nil, ErrInvalidUserID
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	existingUser, exists := r.users[user.ID]
	if !exists {
		return nil, ErrUserNotFound
	}

	// 更新字段
	if user.Username != "" {
		existingUser.Username = user.Username
	}
	if user.Email != "" {
		existingUser.Email = user.Email
	}
	if user.Phone != "" {
		existingUser.Phone = user.Phone
	}
	if user.Age > 0 {
		existingUser.Age = user.Age
	}
	if user.Address != "" {
		existingUser.Address = user.Address
	}
	existingUser.UpdatedAt = time.Now()

	return &model.User{
		ID:        existingUser.ID,
		Username:  existingUser.Username,
		Email:     existingUser.Email,
		Phone:     existingUser.Phone,
		Age:       existingUser.Age,
		Address:   existingUser.Address,
		CreatedAt: existingUser.CreatedAt,
		UpdatedAt: existingUser.UpdatedAt,
	}, nil
}

func (r *memoryUserRepository) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrInvalidUserID
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.users[id]; !exists {
		return ErrUserNotFound
	}

	delete(r.users, id)
	return nil
}

func (r *memoryUserRepository) List(ctx context.Context, req *model.ListUsersRequest) (*model.ListUsersResponse, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var filteredUsers []*model.User
	for _, user := range r.users {
		// 关键字过滤
		if req.Keyword != "" {
			keyword := strings.ToLower(req.Keyword)
			if !strings.Contains(strings.ToLower(user.Username), keyword) &&
				!strings.Contains(strings.ToLower(user.Email), keyword) {
				continue
			}
		}
		
		// 创建用户副本
		filteredUsers = append(filteredUsers, &model.User{
			ID:        user.ID,
			Username:  user.Username,
			Email:     user.Email,
			Phone:     user.Phone,
			Age:       user.Age,
			Address:   user.Address,
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
		})
	}

	// 分页处理
	total := int32(len(filteredUsers))
	start := (req.Page - 1) * req.PageSize
	end := start + req.PageSize

	if start >= total {
		return &model.ListUsersResponse{
			Users:    []*model.User{},
			Total:    total,
			Page:     req.Page,
			PageSize: req.PageSize,
		}, nil
	}

	if end > total {
		end = total
	}

	return &model.ListUsersResponse{
		Users:    filteredUsers[start:end],
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}, nil
}