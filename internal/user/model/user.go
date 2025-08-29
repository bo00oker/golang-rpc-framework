package model

import (
	"time"
)

// User 用户领域模型
type User struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone"`
	Age       int32     `json:"age"`
	Address   string    `json:"address"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateUserRequest 创建用户请求
type CreateUserRequest struct {
	Username string `json:"username" validate:"required,min=3,max=20"`
	Email    string `json:"email" validate:"required,email"`
	Phone    string `json:"phone" validate:"required"`
	Age      int32  `json:"age" validate:"min=0,max=120"`
	Address  string `json:"address"`
}

// UpdateUserRequest 更新用户请求
type UpdateUserRequest struct {
	ID       int64  `json:"id" validate:"required"`
	Username string `json:"username,omitempty" validate:"omitempty,min=3,max=20"`
	Email    string `json:"email,omitempty" validate:"omitempty,email"`
	Phone    string `json:"phone,omitempty"`
	Age      int32  `json:"age,omitempty" validate:"omitempty,min=0,max=120"`
	Address  string `json:"address,omitempty"`
}

// ListUsersRequest 查询用户列表请求
type ListUsersRequest struct {
	Page     int32  `json:"page" validate:"min=1"`
	PageSize int32  `json:"page_size" validate:"min=1,max=100"`
	Keyword  string `json:"keyword,omitempty"`
}

// ListUsersResponse 查询用户列表响应
type ListUsersResponse struct {
	Users    []*User `json:"users"`
	Total    int32   `json:"total"`
	Page     int32   `json:"page"`
	PageSize int32   `json:"page_size"`
}
