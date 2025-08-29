package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/rpc-framework/core/pkg/config"
	"github.com/rpc-framework/core/pkg/logger"
	"github.com/rpc-framework/core/pkg/metrics"
	"github.com/rpc-framework/core/pkg/registry"
	"github.com/rpc-framework/core/pkg/security"
)

// GatewayConfig Gateway配置
type GatewayConfig struct {
	Port            int           `yaml:"port" mapstructure:"port"`
	ReadTimeout     time.Duration `yaml:"read_timeout" mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout" mapstructure:"write_timeout"`
	EnableCORS      bool          `yaml:"enable_cors" mapstructure:"enable_cors"`
	EnableAuth      bool          `yaml:"enable_auth" mapstructure:"enable_auth"`
	EnableMetrics   bool          `yaml:"enable_metrics" mapstructure:"enable_metrics"`
	EnableRateLimit bool          `yaml:"enable_rate_limit" mapstructure:"enable_rate_limit"`
	RateLimit       int           `yaml:"rate_limit" mapstructure:"rate_limit"`
}

// Gateway API网关
type Gateway struct {
	config      *GatewayConfig
	server      *http.Server
	router      *mux.Router
	clientMgr   *ClientManager
	authService *security.AuthService
	metrics     *metrics.Metrics
	logger      logger.Logger
}

// NewGateway 创建Gateway实例
func NewGateway(
	cfg *config.Config,
	registry registry.Registry,
	authService *security.AuthService,
	metrics *metrics.Metrics,
) (*Gateway, error) {
	gatewayConfig := &GatewayConfig{
		Port:            getIntWithDefault(cfg.GetInt("gateway.port"), 8080),
		ReadTimeout:     getDurationWithDefault(cfg.GetDuration("gateway.read_timeout"), 30*time.Second),
		WriteTimeout:    getDurationWithDefault(cfg.GetDuration("gateway.write_timeout"), 30*time.Second),
		EnableCORS:      getBoolWithDefault(cfg.GetBool("gateway.enable_cors"), true),
		EnableAuth:      getBoolWithDefault(cfg.GetBool("gateway.enable_auth"), true),
		EnableMetrics:   getBoolWithDefault(cfg.GetBool("gateway.enable_metrics"), true),
		EnableRateLimit: getBoolWithDefault(cfg.GetBool("gateway.enable_rate_limit"), true),
		RateLimit:       getIntWithDefault(cfg.GetInt("gateway.rate_limit"), 1000),
	}

	// 创建RPC客户端管理器
	clientMgr, err := NewClientManager(cfg, registry)
	if err != nil {
		return nil, fmt.Errorf("failed to create client manager: %w", err)
	}

	gateway := &Gateway{
		config:      gatewayConfig,
		router:      mux.NewRouter(),
		clientMgr:   clientMgr,
		authService: authService,
		metrics:     metrics,
		logger:      logger.GetGlobalLogger(),
	}

	// 设置路由
	gateway.setupRoutes()

	// 设置中间件
	gateway.setupMiddleware()

	// 创建HTTP服务器
	gateway.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", gatewayConfig.Port),
		Handler:      gateway.router,
		ReadTimeout:  gatewayConfig.ReadTimeout,
		WriteTimeout: gatewayConfig.WriteTimeout,
	}

	return gateway, nil
}

// setupRoutes 设置路由
func (g *Gateway) setupRoutes() {
	api := g.router.PathPrefix("/api/v1").Subrouter()

	// 认证相关路由
	auth := api.PathPrefix("/auth").Subrouter()
	auth.HandleFunc("/login", g.handleLogin).Methods("POST")
	auth.HandleFunc("/refresh", g.handleRefreshToken).Methods("POST")
	auth.HandleFunc("/logout", g.handleLogout).Methods("POST")

	// 用户相关路由
	users := api.PathPrefix("/users").Subrouter()
	users.HandleFunc("", g.handleCreateUser).Methods("POST")
	users.HandleFunc("", g.handleListUsers).Methods("GET")
	users.HandleFunc("/{id:[0-9]+}", g.handleGetUser).Methods("GET")
	users.HandleFunc("/{id:[0-9]+}", g.handleUpdateUser).Methods("PUT")
	users.HandleFunc("/{id:[0-9]+}", g.handleDeleteUser).Methods("DELETE")

	// 订单相关路由
	orders := api.PathPrefix("/orders").Subrouter()
	orders.HandleFunc("", g.handleCreateOrder).Methods("POST")
	orders.HandleFunc("", g.handleListOrders).Methods("GET")
	orders.HandleFunc("/{id:[0-9]+}", g.handleGetOrder).Methods("GET")
	orders.HandleFunc("/{id:[0-9]+}", g.handleUpdateOrder).Methods("PUT")
	orders.HandleFunc("/{id:[0-9]+}", g.handleDeleteOrder).Methods("DELETE")
	orders.HandleFunc("/user/{user_id:[0-9]+}", g.handleGetOrdersByUser).Methods("GET")

	// 健康检查
	g.router.HandleFunc("/health", g.handleHealth).Methods("GET")

	// 指标
	if g.config.EnableMetrics {
		g.router.HandleFunc("/metrics", g.handleMetrics).Methods("GET")
	}
}

// setupMiddleware 设置中间件
func (g *Gateway) setupMiddleware() {
	// CORS中间件
	if g.config.EnableCORS {
		g.router.Use(g.corsMiddleware)
	}

	// 日志中间件
	g.router.Use(g.loggingMiddleware)

	// 指标中间件
	if g.config.EnableMetrics {
		g.router.Use(g.metricsMiddleware)
	}

	// 限流中间件
	if g.config.EnableRateLimit {
		g.router.Use(g.rateLimitMiddleware)
	}

	// 认证中间件（仅对需要认证的路由）
	if g.config.EnableAuth {
		// 设置需要认证的路由
		authRoutes := []string{
			"/api/v1/users/{id:[0-9]+}", // PUT, DELETE
			"/api/v1/orders",            // 所有订单操作
		}

		for _, route := range authRoutes {
			g.router.PathPrefix(route).Subrouter().Use(g.authMiddleware)
		}
	}
}

// Start 启动Gateway
func (g *Gateway) Start() error {
	g.logger.Infof("Starting Gateway on port %d", g.config.Port)

	// 启动RPC客户端管理器
	if err := g.clientMgr.Start(); err != nil {
		return fmt.Errorf("failed to start client manager: %w", err)
	}

	// 启动HTTP服务器
	if err := g.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start gateway server: %w", err)
	}

	return nil
}

// Stop 停止Gateway
func (g *Gateway) Stop(ctx context.Context) error {
	g.logger.Info("Stopping Gateway...")

	// 停止HTTP服务器
	if err := g.server.Shutdown(ctx); err != nil {
		g.logger.Errorf("Failed to shutdown gateway server: %v", err)
		return err
	}

	// 停止RPC客户端管理器
	if err := g.clientMgr.Stop(); err != nil {
		g.logger.Errorf("Failed to stop client manager: %v", err)
		return err
	}

	g.logger.Info("Gateway stopped successfully")
	return nil
}

// Response 统一响应格式
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// sendResponse 发送响应
func (g *Gateway) sendResponse(w http.ResponseWriter, code int, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	response := Response{
		Code:    code,
		Message: message,
		Data:    data,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		g.logger.Errorf("Failed to encode response: %v", err)
	}
}

// sendError 发送错误响应
func (g *Gateway) sendError(w http.ResponseWriter, code int, message string) {
	g.sendResponse(w, code, message, nil)
}

// parseIDFromPath 从路径中解析ID
func (g *Gateway) parseIDFromPath(r *http.Request, paramName string) (int64, error) {
	vars := mux.Vars(r)
	idStr, exists := vars[paramName]
	if !exists {
		return 0, fmt.Errorf("missing %s parameter", paramName)
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s parameter: %s", paramName, idStr)
	}

	return id, nil
}

// getQueryParam 获取查询参数
func (g *Gateway) getQueryParam(r *http.Request, key, defaultValue string) string {
	value := r.URL.Query().Get(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// getQueryParamInt 获取整数查询参数
func (g *Gateway) getQueryParamInt(r *http.Request, key string, defaultValue int) int {
	value := r.URL.Query().Get(key)
	if value == "" {
		return defaultValue
	}

	intValue, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}

	return intValue
}

// getUserFromContext 从context获取用户信息
func (g *Gateway) getUserFromContext(ctx context.Context) (userID int64, username string, role string, ok bool) {
	return security.GetUserFromContext(ctx)
}

// 辅助函数
func getIntWithDefault(value int, defaultValue int) int {
	if value == 0 {
		return defaultValue
	}
	return value
}

func getDurationWithDefault(value time.Duration, defaultValue time.Duration) time.Duration {
	if value == 0 {
		return defaultValue
	}
	return value
}

func getBoolWithDefault(value bool, defaultValue bool) bool {
	// 对于bool类型，false是零值，但可能是有意设置的
	// 这里简化处理，可以考虑使用指针类型来区分零值和未设置
	return value
}
