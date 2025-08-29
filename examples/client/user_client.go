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

// UserClient 用户服务客户端
type UserClient struct {
	cli        *client.Client
	conn       *grpc.ClientConn
	userClient user.UserServiceClient
}

// NewUserClient 创建用户服务客户端
func NewUserClient(cli *client.Client, nacosReg registry.Registry, tracer *trace.Tracer) (*UserClient, error) {
	ctx := context.Background()

	// 连接用户服务
	conn, err := cli.DialService(ctx, "user.UserService", nacosReg)
	if err != nil {
		return nil, fmt.Errorf("failed to dial user service: %w", err)
	}

	return &UserClient{
		cli:        cli,
		conn:       conn,
		userClient: user.NewUserServiceClient(conn),
	}, nil
}

// Close 关闭连接
func (u *UserClient) Close() {
	if u.conn != nil {
		u.conn.Close()
	}
}

// TestUserService 测试用户服务
func (u *UserClient) TestUserService(ctx context.Context) {
	log := logger.GetGlobalLogger()
	log.Info("=== Testing User Service ===")

	// 创建用户
	createResp, err := u.userClient.CreateUser(ctx, &user.CreateUserRequest{
		Username: "testuser1",
		Email:    "test1@example.com",
		Phone:    "13800138001",
		Age:      25,
		Address:  "北京市朝阳区",
	})
	if err != nil {
		log.Errorf("Create user failed: %v", err)
		return
	}
	log.Infof("User created: ID=%d, Username=%s", createResp.User.Id, createResp.User.Username)

	userID := createResp.User.Id

	// 创建第二个用户
	createResp2, err := u.userClient.CreateUser(ctx, &user.CreateUserRequest{
		Username: "testuser2",
		Email:    "test2@example.com",
		Phone:    "13800138002",
		Age:      30,
		Address:  "上海市浦东新区",
	})
	if err != nil {
		log.Errorf("Create second user failed: %v", err)
		return
	}
	log.Infof("Second user created: ID=%d, Username=%s", createResp2.User.Id, createResp2.User.Username)

	// 查询用户
	getResp, err := u.userClient.GetUser(ctx, &user.GetUserRequest{
		UserId: userID,
	})
	if err != nil {
		log.Errorf("Get user failed: %v", err)
		return
	}
	log.Infof("User retrieved: %s", getResp.Message)
	if getResp.User != nil {
		log.Infof("User details: ID=%d, Username=%s, Email=%s",
			getResp.User.Id, getResp.User.Username, getResp.User.Email)
	}

	// 查询用户列表
	listResp, err := u.userClient.ListUsers(ctx, &user.ListUsersRequest{
		Page:     1,
		PageSize: 10,
		Keyword:  "test",
	})
	if err != nil {
		log.Errorf("List users failed: %v", err)
		return
	}
	log.Infof("Users listed: %s, Total=%d", listResp.Message, listResp.Total)
	for _, u := range listResp.Users {
		log.Infof("  - ID=%d, Username=%s, Email=%s", u.Id, u.Username, u.Email)
	}

	// 更新用户
	updateResp, err := u.userClient.UpdateUser(ctx, &user.UpdateUserRequest{
		UserId:  userID,
		Age:     26,
		Address: "北京市海淀区",
	})
	if err != nil {
		log.Errorf("Update user failed: %v", err)
		return
	}
	log.Infof("User updated: %s", updateResp.Message)
	if updateResp.User != nil {
		log.Infof("Updated user: Age=%d, Address=%s", updateResp.User.Age, updateResp.User.Address)
	}

	log.Info("✅ User service test completed!")
}
