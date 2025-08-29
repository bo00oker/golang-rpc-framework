package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rpc-framework/core/internal/user/model"
	"github.com/rpc-framework/core/internal/user/repository"
	"github.com/rpc-framework/core/pkg/testutils"
	"github.com/rpc-framework/core/pkg/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserService_CreateUser(t *testing.T) {
	// 设置
	userRepo := repository.NewMemoryUserRepository()
	tracer, _ := trace.NewTracer(trace.DefaultConfig())
	userService := NewUserService(userRepo, tracer)

	ctx := context.Background()

	// 测试用例
	tests := []testutils.TestCase{
		{
			Name: "创建用户成功",
			Setup: func(t *testing.T) interface{} {
				return &model.CreateUserRequest{
					Username: "testuser",
					Email:    "test@example.com",
					Phone:    "13800138000",
					Age:      25,
					Address:  "Test Address",
				}
			},
			Run: func(t *testing.T, data interface{}) {
				req := data.(*model.CreateUserRequest)
				user, err := userService.CreateUser(ctx, req)

				require.NoError(t, err)
				assert.NotNil(t, user)
				assert.Equal(t, req.Username, user.Username)
				assert.Equal(t, req.Email, user.Email)
				assert.True(t, user.ID > 0)
				assert.False(t, user.CreatedAt.IsZero())
			},
		},
		{
			Name: "用户名为空",
			Setup: func(t *testing.T) interface{} {
				return &model.CreateUserRequest{
					Username: "",
					Email:    "test@example.com",
					Phone:    "13800138000",
					Age:      25,
				}
			},
			Run: func(t *testing.T, data interface{}) {
				req := data.(*model.CreateUserRequest)
				user, err := userService.CreateUser(ctx, req)

				assert.Error(t, err)
				assert.Nil(t, user)
				assert.Contains(t, err.Error(), "username is required")
			},
			ExpectError: false, // 这里不是panic，而是返回错误
		},
		{
			Name: "邮箱格式无效",
			Setup: func(t *testing.T) interface{} {
				return &model.CreateUserRequest{
					Username: "testuser",
					Email:    "invalid-email",
					Phone:    "13800138000",
					Age:      25,
				}
			},
			Run: func(t *testing.T, data interface{}) {
				req := data.(*model.CreateUserRequest)
				user, err := userService.CreateUser(ctx, req)

				assert.Error(t, err)
				assert.Nil(t, user)
			},
		},
	}

	// 运行测试用例
	testutils.RunTestCases(t, tests)
}

func TestUserService_GetUser(t *testing.T) {
	// 设置
	userRepo := repository.NewMemoryUserRepository()
	tracer, _ := trace.NewTracer(trace.DefaultConfig())
	userService := NewUserService(userRepo, tracer)

	ctx := context.Background()

	// 先创建一个用户
	createReq := &model.CreateUserRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Phone:    "13800138000",
		Age:      25,
		Address:  "Test Address",
	}

	createdUser, err := userService.CreateUser(ctx, createReq)
	require.NoError(t, err)
	require.NotNil(t, createdUser)

	t.Run("获取存在的用户", func(t *testing.T) {
		user, err := userService.GetUser(ctx, createdUser.ID)

		assert.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, createdUser.ID, user.ID)
		assert.Equal(t, createdUser.Username, user.Username)
	})

	t.Run("获取不存在的用户", func(t *testing.T) {
		user, err := userService.GetUser(ctx, 999999)

		assert.Error(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "user not found")
	})

	t.Run("无效的用户ID", func(t *testing.T) {
		user, err := userService.GetUser(ctx, -1)

		assert.Error(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "invalid user ID")
	})
}

func TestUserService_UpdateUser(t *testing.T) {
	// 设置
	userRepo := repository.NewMemoryUserRepository()
	tracer, _ := trace.NewTracer(trace.DefaultConfig())
	userService := NewUserService(userRepo, tracer)

	ctx := context.Background()

	// 先创建一个用户
	createReq := &model.CreateUserRequest{
		Username: "testuser",
		Email:    "test@example.com",
		Phone:    "13800138000",
		Age:      25,
		Address:  "Test Address",
	}

	createdUser, err := userService.CreateUser(ctx, createReq)
	require.NoError(t, err)

	t.Run("更新用户成功", func(t *testing.T) {
		updateReq := &model.UpdateUserRequest{
			ID:       createdUser.ID,
			Username: "updateduser",
			Email:    "updated@example.com",
			Age:      30,
		}

		updatedUser, err := userService.UpdateUser(ctx, updateReq)

		assert.NoError(t, err)
		assert.NotNil(t, updatedUser)
		assert.Equal(t, updateReq.Username, updatedUser.Username)
		assert.Equal(t, updateReq.Email, updatedUser.Email)
		assert.Equal(t, updateReq.Age, updatedUser.Age)
		assert.True(t, updatedUser.UpdatedAt.After(createdUser.UpdatedAt))
	})

	t.Run("更新不存在的用户", func(t *testing.T) {
		updateReq := &model.UpdateUserRequest{
			ID:       999999,
			Username: "updateduser",
		}

		user, err := userService.UpdateUser(ctx, updateReq)

		assert.Error(t, err)
		assert.Nil(t, user)
	})
}

func TestUserService_ListUsers(t *testing.T) {
	// 设置
	userRepo := repository.NewMemoryUserRepository()
	tracer, _ := trace.NewTracer(trace.DefaultConfig())
	userService := NewUserService(userRepo, tracer)

	ctx := context.Background()

	// 创建多个测试用户
	users := []struct {
		username string
		email    string
	}{
		{"user1", "user1@example.com"},
		{"user2", "user2@example.com"},
		{"testuser", "test@example.com"},
	}

	for _, u := range users {
		createReq := &model.CreateUserRequest{
			Username: u.username,
			Email:    u.email,
			Phone:    "13800138000",
			Age:      25,
		}
		_, err := userService.CreateUser(ctx, createReq)
		require.NoError(t, err)
	}

	t.Run("查询所有用户", func(t *testing.T) {
		req := &model.ListUsersRequest{
			Page:     1,
			PageSize: 10,
		}

		response, err := userService.ListUsers(ctx, req)

		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, int32(3), response.Total)
		assert.Len(t, response.Users, 3)
	})

	t.Run("关键字搜索", func(t *testing.T) {
		req := &model.ListUsersRequest{
			Page:     1,
			PageSize: 10,
			Keyword:  "test",
		}

		response, err := userService.ListUsers(ctx, req)

		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, int32(1), response.Total)
		assert.Len(t, response.Users, 1)
		assert.Equal(t, "testuser", response.Users[0].Username)
	})

	t.Run("分页查询", func(t *testing.T) {
		req := &model.ListUsersRequest{
			Page:     1,
			PageSize: 2,
		}

		response, err := userService.ListUsers(ctx, req)

		assert.NoError(t, err)
		assert.NotNil(t, response)
		assert.Equal(t, int32(3), response.Total)
		assert.Len(t, response.Users, 2)
	})
}

func BenchmarkUserService_CreateUser(b *testing.B) {
	runner := testutils.NewBenchmarkRunner("CreateUser").
		Setup(func(b *testing.B) interface{} {
			userRepo := repository.NewMemoryUserRepository()
			tracer, _ := trace.NewTracer(trace.DefaultConfig())
			userService := NewUserService(userRepo, tracer)

			return map[string]interface{}{
				"service": userService,
				"ctx":     context.Background(),
			}
		}).
		Run(func(b *testing.B, data interface{}) {
			d := data.(map[string]interface{})
			userService := d["service"].(UserService)
			ctx := d["ctx"].(context.Context)

			req := &model.CreateUserRequest{
				Username: "benchuser",
				Email:    "bench@example.com",
				Phone:    "13800138000",
				Age:      25,
			}

			_, err := userService.CreateUser(ctx, req)
			if err != nil {
				b.Fatalf("CreateUser failed: %v", err)
			}
		})

	runner.Execute(b)
}

func TestUserService_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试")
	}

	// 集成测试：测试整个用户服务流程
	userRepo := repository.NewMemoryUserRepository()
	tracer, _ := trace.NewTracer(trace.DefaultConfig())
	userService := NewUserService(userRepo, tracer)

	ctx := context.Background()

	// 创建用户
	createReq := &model.CreateUserRequest{
		Username: "integrationuser",
		Email:    "integration@example.com",
		Phone:    "13800138000",
		Age:      25,
		Address:  "Integration Test Address",
	}

	user, err := userService.CreateUser(ctx, createReq)
	require.NoError(t, err)
	require.NotNil(t, user)

	// 获取用户
	retrievedUser, err := userService.GetUser(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, user.Username, retrievedUser.Username)

	// 更新用户
	updateReq := &model.UpdateUserRequest{
		ID:      user.ID,
		Age:     30,
		Address: "Updated Address",
	}

	updatedUser, err := userService.UpdateUser(ctx, updateReq)
	require.NoError(t, err)
	assert.Equal(t, int32(30), updatedUser.Age)
	assert.Equal(t, "Updated Address", updatedUser.Address)

	// 列出用户
	listReq := &model.ListUsersRequest{
		Page:     1,
		PageSize: 10,
	}

	listResponse, err := userService.ListUsers(ctx, listReq)
	require.NoError(t, err)
	assert.True(t, listResponse.Total >= 1)

	// 删除用户
	err = userService.DeleteUser(ctx, user.ID)
	require.NoError(t, err)

	// 确认用户已删除
	_, err = userService.GetUser(ctx, user.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "user not found")
}

func TestUserService_Concurrency(t *testing.T) {
	// 并发测试
	userRepo := repository.NewMemoryUserRepository()
	tracer, _ := trace.NewTracer(trace.DefaultConfig())
	userService := NewUserService(userRepo, tracer)

	ctx := context.Background()
	concurrency := 50
	results := make(chan error, concurrency)

	// 并发创建用户
	for i := 0; i < concurrency; i++ {
		go func(i int) {
			req := &model.CreateUserRequest{
				Username: fmt.Sprintf("user%d", i),
				Email:    fmt.Sprintf("user%d@example.com", i),
				Phone:    "13800138000",
				Age:      25,
			}

			_, err := userService.CreateUser(ctx, req)
			results <- err
		}(i)
	}

	// 收集结果
	for i := 0; i < concurrency; i++ {
		err := <-results
		assert.NoError(t, err)
	}

	// 验证所有用户都创建成功
	listReq := &model.ListUsersRequest{
		Page:     1,
		PageSize: 100,
	}

	listResponse, err := userService.ListUsers(ctx, listReq)
	require.NoError(t, err)
	assert.Equal(t, int32(concurrency), listResponse.Total)
}

func TestUserService_EventualConsistency(t *testing.T) {
	// 最终一致性测试
	userRepo := repository.NewMemoryUserRepository()
	tracer, _ := trace.NewTracer(trace.DefaultConfig())
	userService := NewUserService(userRepo, tracer)

	ctx := context.Background()

	// 创建用户
	createReq := &model.CreateUserRequest{
		Username: "eventualuser",
		Email:    "eventual@example.com",
		Phone:    "13800138000",
		Age:      25,
	}

	user, err := userService.CreateUser(ctx, createReq)
	require.NoError(t, err)

	// 使用AssertEventually测试最终一致性
	testutils.AssertEventually(t, func() bool {
		retrievedUser, err := userService.GetUser(ctx, user.ID)
		return err == nil && retrievedUser.Username == createReq.Username
	}, time.Second*5, time.Millisecond*100, "用户应该最终可见")
}
