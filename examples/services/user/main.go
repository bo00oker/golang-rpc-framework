package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/rpc-framework/core/pkg/config"
	"github.com/rpc-framework/core/pkg/logger"
	"github.com/rpc-framework/core/pkg/registry"
	"github.com/rpc-framework/core/pkg/server"
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

func main() {
	// 加载配置
	cfg := config.New()
	if err := cfg.LoadFromFile("./configs/app.yaml"); err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 获取服务器配置
	serverCfg, err := cfg.GetServerConfig()
	if err != nil {
		fmt.Printf("Failed to get server config: %v\n", err)
		os.Exit(1)
	}

	// 获取日志配置
	logCfg, err := cfg.GetLogConfig()
	if err != nil {
		fmt.Printf("Failed to get log config: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	log, err := logger.NewLogger(&logger.Config{
		Level:      logCfg.Level,
		Format:     logCfg.Format,
		Output:     logCfg.Output,
		FilePath:   logCfg.Filename,
		MaxSize:    logCfg.MaxSize,
		MaxBackups: logCfg.MaxBackups,
		MaxAge:     logCfg.MaxAge,
		Compress:   logCfg.Compress,
	})
	if err != nil {
		fmt.Printf("Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	logger.SetGlobalLogger(log)

	// 获取追踪配置
	traceCfg, err := cfg.GetTraceConfig()
	if err != nil {
		log.Errorf("Failed to get trace config: %v", err)
		os.Exit(1)
	}

	// 初始化追踪器
	tracer, err := trace.NewTracer(&trace.Config{
		ServiceName:    traceCfg.ServiceName + "-user",
		ServiceVersion: traceCfg.ServiceVersion,
		Environment:    traceCfg.Environment,
		JaegerEndpoint: traceCfg.JaegerEndpoint,
		SampleRate:     traceCfg.SampleRate,
		EnableConsole:  traceCfg.EnableConsole,
	})
	if err != nil {
		log.Errorf("Failed to create tracer: %v", err)
		os.Exit(1)
	}
	defer tracer.Shutdown(context.Background())

	// 获取Nacos配置
	nacosCfg, err := cfg.GetNacosConfig()
	if err != nil {
		log.Errorf("Failed to get nacos config: %v", err)
		os.Exit(1)
	}

	// 创建服务器
	srv := server.New(&server.Options{
		Address:        serverCfg.Address,
		MaxRecvMsgSize: serverCfg.MaxRecvMsgSize,
		MaxSendMsgSize: serverCfg.MaxSendMsgSize,
		Tracer:         tracer,
	})

	// 注册用户服务
	userService := NewUserService(tracer)
	srv.RegisterService(&user.UserService_ServiceDesc, userService)

	// 启动服务器
	if err := srv.Start(); err != nil {
		log.Errorf("Failed to start server: %v", err)
		os.Exit(1)
	}

	log.Infof("User service started on %s", serverCfg.Address)

	// 创建Nacos注册中心
	nacosReg, err := registry.NewNacosRegistry(&registry.NacosOptions{
		Endpoints:   []string{nacosCfg.ServerAddr},
		NamespaceID: nacosCfg.Namespace,
		Group:       nacosCfg.Group,
		TimeoutMs:   uint64(nacosCfg.Timeout.Milliseconds()),
		Username:    nacosCfg.Username,
		Password:    nacosCfg.Password,
	})
	if err != nil {
		log.Errorf("Failed to create nacos registry: %v", err)
		os.Exit(1)
	}
	defer nacosReg.Close()

	// 注册用户服务到Nacos
	serviceID := fmt.Sprintf("%s-user-%s", traceCfg.ServiceName, serverCfg.Address)
	serviceInfo := &registry.ServiceInfo{
		ID:      serviceID,
		Name:    "user.UserService",
		Version: traceCfg.ServiceVersion,
		Address: "127.0.0.1",
		Port:    serverCfg.Port,
		Metadata: map[string]string{
			"environment": traceCfg.Environment,
			"description": "用户服务",
		},
		TTL: 30,
	}

	if err := nacosReg.Register(context.Background(), serviceInfo); err != nil {
		log.Errorf("Failed to register user service: %v", err)
		os.Exit(1)
	}

	log.Infof("User service registered with ID: %s", serviceID)

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutting down user service...")

	// 注销服务
	if err := nacosReg.Deregister(context.Background(), serviceID); err != nil {
		log.Errorf("Failed to deregister user service: %v", err)
	}

	// 停止服务器
	if err := srv.Stop(); err != nil {
		log.Errorf("Failed to stop server: %v", err)
	}

	log.Info("User service stopped")
}
