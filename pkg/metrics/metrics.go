package metrics

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rpc-framework/core/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MetricsConfig 指标配置
type MetricsConfig struct {
	Enable    bool   `yaml:"enable" mapstructure:"enable"`
	Port      int    `yaml:"port" mapstructure:"port"`
	Path      string `yaml:"path" mapstructure:"path"`
	Namespace string `yaml:"namespace" mapstructure:"namespace"`
	Subsystem string `yaml:"subsystem" mapstructure:"subsystem"`
}

// Metrics 指标收集器
type Metrics struct {
	config *MetricsConfig
	logger logger.Logger

	// gRPC请求指标
	requestsTotal    *prometheus.CounterVec
	requestDuration  *prometheus.HistogramVec
	requestsInFlight *prometheus.GaugeVec

	// 系统指标
	connectionsActive prometheus.Gauge
	memoryUsage       prometheus.Gauge
	cpuUsage          prometheus.Gauge

	// 业务指标
	usersTotal  prometheus.Gauge
	ordersTotal prometheus.Gauge
	cacheHits   *prometheus.CounterVec
	cacheSize   prometheus.Gauge

	// 数据库指标
	dbConnections   *prometheus.GaugeVec
	dbQueries       *prometheus.CounterVec
	dbQueryDuration *prometheus.HistogramVec

	registry *prometheus.Registry
}

// NewMetrics 创建指标收集器
func NewMetrics(config *MetricsConfig) *Metrics {
	if config == nil {
		config = &MetricsConfig{
			Enable:    true,
			Port:      9090,
			Path:      "/metrics",
			Namespace: "rpc_framework",
			Subsystem: "server",
		}
	}

	registry := prometheus.NewRegistry()

	m := &Metrics{
		config:   config,
		logger:   logger.GetGlobalLogger(),
		registry: registry,
	}

	m.initMetrics()
	m.registerMetrics()

	return m
}

// initMetrics 初始化指标
func (m *Metrics) initMetrics() {
	// gRPC请求指标
	m.requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "grpc_requests_total",
			Help:      "Total number of gRPC requests",
		},
		[]string{"method", "code"},
	)

	m.requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "grpc_request_duration_seconds",
			Help:      "Duration of gRPC requests in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method"},
	)

	m.requestsInFlight = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "grpc_requests_in_flight",
			Help:      "Number of gRPC requests currently in flight",
		},
		[]string{"method"},
	)

	// 系统指标
	m.connectionsActive = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "connections_active",
			Help:      "Number of active connections",
		},
	)

	m.memoryUsage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "memory_usage_bytes",
			Help:      "Memory usage in bytes",
		},
	)

	m.cpuUsage = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "cpu_usage_percent",
			Help:      "CPU usage percentage",
		},
	)

	// 业务指标
	m.usersTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: m.config.Namespace,
			Subsystem: "business",
			Name:      "users_total",
			Help:      "Total number of users",
		},
	)

	m.ordersTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: m.config.Namespace,
			Subsystem: "business",
			Name:      "orders_total",
			Help:      "Total number of orders",
		},
	)

	m.cacheHits = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: "cache",
			Name:      "cache_operations_total",
			Help:      "Total number of cache operations",
		},
		[]string{"operation", "result"},
	)

	m.cacheSize = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: m.config.Namespace,
			Subsystem: "cache",
			Name:      "cache_size_bytes",
			Help:      "Cache size in bytes",
		},
	)

	// 数据库指标
	m.dbConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: m.config.Namespace,
			Subsystem: "database",
			Name:      "connections",
			Help:      "Number of database connections",
		},
		[]string{"state"}, // active, idle, open
	)

	m.dbQueries = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: "database",
			Name:      "queries_total",
			Help:      "Total number of database queries",
		},
		[]string{"operation", "table"},
	)

	m.dbQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: m.config.Namespace,
			Subsystem: "database",
			Name:      "query_duration_seconds",
			Help:      "Duration of database queries in seconds",
			Buckets:   []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0, 5.0},
		},
		[]string{"operation", "table"},
	)
}

// registerMetrics 注册指标
func (m *Metrics) registerMetrics() {
	m.registry.MustRegister(
		m.requestsTotal,
		m.requestDuration,
		m.requestsInFlight,
		m.connectionsActive,
		m.memoryUsage,
		m.cpuUsage,
		m.usersTotal,
		m.ordersTotal,
		m.cacheHits,
		m.cacheSize,
		m.dbConnections,
		m.dbQueries,
		m.dbQueryDuration,
	)
}

// Start 启动指标服务器
func (m *Metrics) Start() error {
	if !m.config.Enable {
		m.logger.Info("Metrics disabled")
		return nil
	}

	mux := http.NewServeMux()
	mux.Handle(m.config.Path, promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{}))

	server := &http.Server{
		Addr:    ":" + strconv.Itoa(m.config.Port),
		Handler: mux,
	}

	m.logger.Infof("Starting metrics server on port %d", m.config.Port)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			m.logger.Errorf("Metrics server failed: %v", err)
		}
	}()

	return nil
}

// RecordGRPCRequest 记录gRPC请求指标
func (m *Metrics) RecordGRPCRequest(method string, code codes.Code, duration time.Duration) {
	if !m.config.Enable {
		return
	}

	m.requestsTotal.WithLabelValues(method, code.String()).Inc()
	m.requestDuration.WithLabelValues(method).Observe(duration.Seconds())
}

// IncGRPCInFlight 增加进行中的请求
func (m *Metrics) IncGRPCInFlight(method string) {
	if !m.config.Enable {
		return
	}
	m.requestsInFlight.WithLabelValues(method).Inc()
}

// DecGRPCInFlight 减少进行中的请求
func (m *Metrics) DecGRPCInFlight(method string) {
	if !m.config.Enable {
		return
	}
	m.requestsInFlight.WithLabelValues(method).Dec()
}

// SetActiveConnections 设置活跃连接数
func (m *Metrics) SetActiveConnections(count float64) {
	if !m.config.Enable {
		return
	}
	m.connectionsActive.Set(count)
}

// SetMemoryUsage 设置内存使用量
func (m *Metrics) SetMemoryUsage(bytes float64) {
	if !m.config.Enable {
		return
	}
	m.memoryUsage.Set(bytes)
}

// SetCPUUsage 设置CPU使用率
func (m *Metrics) SetCPUUsage(percent float64) {
	if !m.config.Enable {
		return
	}
	m.cpuUsage.Set(percent)
}

// SetUsersTotal 设置用户总数
func (m *Metrics) SetUsersTotal(count float64) {
	if !m.config.Enable {
		return
	}
	m.usersTotal.Set(count)
}

// SetOrdersTotal 设置订单总数
func (m *Metrics) SetOrdersTotal(count float64) {
	if !m.config.Enable {
		return
	}
	m.ordersTotal.Set(count)
}

// RecordCacheOperation 记录缓存操作
func (m *Metrics) RecordCacheOperation(operation, result string) {
	if !m.config.Enable {
		return
	}
	m.cacheHits.WithLabelValues(operation, result).Inc()
}

// SetCacheSize 设置缓存大小
func (m *Metrics) SetCacheSize(bytes float64) {
	if !m.config.Enable {
		return
	}
	m.cacheSize.Set(bytes)
}

// SetDBConnections 设置数据库连接数
func (m *Metrics) SetDBConnections(state string, count float64) {
	if !m.config.Enable {
		return
	}
	m.dbConnections.WithLabelValues(state).Set(count)
}

// RecordDBQuery 记录数据库查询
func (m *Metrics) RecordDBQuery(operation, table string, duration time.Duration) {
	if !m.config.Enable {
		return
	}
	m.dbQueries.WithLabelValues(operation, table).Inc()
	m.dbQueryDuration.WithLabelValues(operation, table).Observe(duration.Seconds())
}

// MetricsInterceptor 指标拦截器
type MetricsInterceptor struct {
	metrics *Metrics
}

// NewMetricsInterceptor 创建指标拦截器
func NewMetricsInterceptor(metrics *Metrics) *MetricsInterceptor {
	return &MetricsInterceptor{
		metrics: metrics,
	}
}

// UnaryInterceptor 一元RPC指标拦截器
func (i *MetricsInterceptor) UnaryInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	start := time.Now()
	method := info.FullMethod

	// 增加进行中的请求
	i.metrics.IncGRPCInFlight(method)
	defer i.metrics.DecGRPCInFlight(method)

	// 执行请求
	resp, err := handler(ctx, req)

	// 记录指标
	duration := time.Since(start)
	code := status.Code(err)
	i.metrics.RecordGRPCRequest(method, code, duration)

	return resp, err
}

// StreamInterceptor 流式RPC指标拦截器
func (i *MetricsInterceptor) StreamInterceptor(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	start := time.Now()
	method := info.FullMethod

	// 增加进行中的请求
	i.metrics.IncGRPCInFlight(method)
	defer i.metrics.DecGRPCInFlight(method)

	// 执行请求
	err := handler(srv, ss)

	// 记录指标
	duration := time.Since(start)
	code := status.Code(err)
	i.metrics.RecordGRPCRequest(method, code, duration)

	return err
}

// HealthChecker 健康检查器
type HealthChecker struct {
	checks map[string]HealthCheckFunc
	logger logger.Logger
}

// HealthCheckFunc 健康检查函数
type HealthCheckFunc func(ctx context.Context) error

// HealthStatus 健康状态
type HealthStatus struct {
	Status  string            `json:"status"`
	Checks  map[string]string `json:"checks"`
	Details map[string]string `json:"details,omitempty"`
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		checks: make(map[string]HealthCheckFunc),
		logger: logger.GetGlobalLogger(),
	}
}

// RegisterCheck 注册健康检查
func (h *HealthChecker) RegisterCheck(name string, checkFunc HealthCheckFunc) {
	h.checks[name] = checkFunc
}

// Check 执行健康检查
func (h *HealthChecker) Check(ctx context.Context) *HealthStatus {
	status := &HealthStatus{
		Status:  "healthy",
		Checks:  make(map[string]string),
		Details: make(map[string]string),
	}

	for name, checkFunc := range h.checks {
		if err := checkFunc(ctx); err != nil {
			status.Status = "unhealthy"
			status.Checks[name] = "failed"
			status.Details[name] = err.Error()
			h.logger.Warnf("Health check failed for %s: %v", name, err)
		} else {
			status.Checks[name] = "passed"
		}
	}

	return status
}

// StartHealthServer 启动健康检查服务器
func (h *HealthChecker) StartHealthServer(port int) {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		status := h.Check(ctx)

		w.Header().Set("Content-Type", "application/json")
		if status.Status == "healthy" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		// 这里应该使用JSON编码，简化处理
		fmt.Fprintf(w, `{"status":"%s"}`, status.Status)
	})

	server := &http.Server{
		Addr:    ":" + strconv.Itoa(port),
		Handler: mux,
	}

	h.logger.Infof("Starting health server on port %d", port)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			h.logger.Errorf("Health server failed: %v", err)
		}
	}()
}
