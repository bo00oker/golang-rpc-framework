package main

import (
	"context"
	"fmt"

	"github.com/rpc-framework/core/pkg/client"
	"github.com/rpc-framework/core/pkg/logger"
	"github.com/rpc-framework/core/pkg/registry"
	"github.com/rpc-framework/core/pkg/trace"
	"github.com/rpc-framework/core/proto/user"
	"google.golang.org/grpc"
)

const UserServiceName = "user.UserService"

// UserServiceClient 用户服务客户端封装
type UserServiceClient struct {
	conn   *grpc.ClientConn
	client user.UserServiceClient
	tracer *trace.Tracer
	logger logger.Logger
}

// NewUserServiceClient 创建用户服务客户端
func NewUserServiceClient(cli *client.Client, registry registry.Registry, tracer *trace.Tracer) (*UserServiceClient, error) {
	// 通过服务发现获取连接
	conn, err := cli.DialService(context.Background(), UserServiceName, registry)
	if err != nil {
		return nil, fmt.Errorf("failed to dial user service: %w", err)
	}

	return &UserServiceClient{
		conn:   conn,
		client: user.NewUserServiceClient(conn),
		tracer: tracer,
		logger: logger.GetGlobalLogger(),
	}, nil
}

// Close 关闭客户端连接
func (c *UserServiceClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// CreateTestUser 创建测试用户
func (c *UserServiceClient) CreateTestUser(ctx context.Context, username, email string) (int64, error) {
	ctx, span := c.tracer.StartSpan(ctx, "UserServiceClient.CreateTestUser")
	defer span.End()

	c.logger.Infof("Creating test user: %s", username)

	resp, err := c.client.CreateUser(ctx, &user.CreateUserRequest{
		Username: username,
		Email:    email,
		Phone:    "13800138000",
		Age:      25,
		Address:  "Test Address",
	})

	if err != nil {
		c.logger.Errorf("Failed to create user: %v", err)
		return 0, err
	}

	if resp.User != nil {
		c.logger.Infof("User created successfully: ID=%d, Username=%s", resp.User.Id, resp.User.Username)
		return resp.User.Id, nil
	}

	return 0, fmt.Errorf("user creation failed: %s", resp.Message)
}

// TestUserOperations 测试用户服务的所有操作
func (c *UserServiceClient) TestUserOperations(ctx context.Context) {
	c.logger.Info("Starting user service operations test...")

	// 1. 创建用户
	c.logger.Info("1. Testing CreateUser...")
	createResp, err := c.client.CreateUser(ctx, &user.CreateUserRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Phone:    "13800138000",
		Age:      30,
		Address:  "Test City",
	})
	if err != nil {
		c.logger.Errorf("CreateUser failed: %v", err)
		return
	}
	c.logger.Infof("CreateUser success: %s", createResp.Message)
	if createResp.User == nil {
		c.logger.Error("No user returned from CreateUser")
		return
	}

	userID := createResp.User.Id
	c.logger.Infof("Created user with ID: %d", userID)

	// 2. 查询用户
	c.logger.Info("2. Testing GetUser...")
	getUserResp, err := c.client.GetUser(ctx, &user.GetUserRequest{
		UserId: userID,
	})
	if err != nil {
		c.logger.Errorf("GetUser failed: %v", err)
		return
	}
	c.logger.Infof("GetUser success: %s", getUserResp.Message)
	if getUserResp.User != nil {
		c.logger.Infof("Found user: %s (ID: %d)", getUserResp.User.Username, getUserResp.User.Id)
	}

	// 3. 更新用户
	c.logger.Info("3. Testing UpdateUser...")
	updateResp, err := c.client.UpdateUser(ctx, &user.UpdateUserRequest{
		UserId:   userID,
		Username: "updated_testuser",
		Age:      31,
		Address:  "Updated Test City",
	})
	if err != nil {
		c.logger.Errorf("UpdateUser failed: %v", err)
		return
	}
	c.logger.Infof("UpdateUser success: %s", updateResp.Message)

	// 4. 查询用户列表
	c.logger.Info("4. Testing ListUsers...")
	listResp, err := c.client.ListUsers(ctx, &user.ListUsersRequest{
		Page:     1,
		PageSize: 10,
		Keyword:  "",
	})
	if err != nil {
		c.logger.Errorf("ListUsers failed: %v", err)
		return
	}
	c.logger.Infof("ListUsers success: %s, Total: %d", listResp.Message, listResp.Total)
	for i, u := range listResp.Users {
		c.logger.Infof("User %d: %s (ID: %d)", i+1, u.Username, u.Id)
	}

	// 5. 关键字搜索
	c.logger.Info("5. Testing ListUsers with keyword...")
	searchResp, err := c.client.ListUsers(ctx, &user.ListUsersRequest{
		Page:     1,
		PageSize: 10,
		Keyword:  "updated",
	})
	if err != nil {
		c.logger.Errorf("ListUsers with keyword failed: %v", err)
		return
	}
	c.logger.Infof("Search result: Total: %d", searchResp.Total)

	// 6. 删除用户
	c.logger.Info("6. Testing DeleteUser...")
	deleteResp, err := c.client.DeleteUser(ctx, &user.DeleteUserRequest{
		UserId: userID,
	})
	if err != nil {
		c.logger.Errorf("DeleteUser failed: %v", err)
		return
	}
	c.logger.Infof("DeleteUser success: %s", deleteResp.Message)

	// 7. 验证用户已删除
	c.logger.Info("7. Verifying user deletion...")
	_, err = c.client.GetUser(ctx, &user.GetUserRequest{
		UserId: userID,
	})
	if err != nil {
		c.logger.Infof("User deletion verified (GetUser returned error as expected)")
	} else {
		c.logger.Warn("User still exists after deletion")
	}

	c.logger.Info("User service operations test completed!")
}
