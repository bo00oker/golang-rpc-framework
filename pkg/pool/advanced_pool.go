package pool

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc"
)

// AdvancedConnectionPool 高级连接池
type AdvancedConnectionPool struct {
	mu             sync.RWMutex
	pools          map[string]*ServicePool // 按服务分组的连接池
	globalConfig   *PoolConfig
	healthChecker  HealthChecker
	loadBalancer   LoadBalancer
	circuitBreaker CircuitBreaker
	metrics        *PoolMetrics
	cleanupTicker  *time.Ticker
	stopChan       chan struct{}
}

// ServicePool 服务连接池
type ServicePool struct {
	serviceName string
	connections []*PooledConnection
	roundRobin  int64
	mu          sync.RWMutex
	config      *ServicePoolConfig
	lastUsed    time.Time
}

// PooledConnection 池化连接
type PooledConnection struct {
	conn      *grpc.ClientConn
	target    string
	createdAt time.Time
	lastUsed  time.Time
	useCount  int64
	isHealthy bool
	mu        sync.RWMutex
}

// PoolConfig 连接池配置
type PoolConfig struct {
	DefaultMaxConnections int           `yaml:"default_max_connections"`
	DefaultMaxIdle        int           `yaml:"default_max_idle"`
	DefaultIdleTimeout    time.Duration `yaml:"default_idle_timeout"`
	DefaultConnTimeout    time.Duration `yaml:"default_conn_timeout"`
	HealthCheckInterval   time.Duration `yaml:"health_check_interval"`
	CleanupInterval       time.Duration `yaml:"cleanup_interval"`
	EnableCircuitBreaker  bool          `yaml:"enable_circuit_breaker"`
	EnableLoadBalancing   bool          `yaml:"enable_load_balancing"`
	EnableMetrics         bool          `yaml:"enable_metrics"`
}

// ServicePoolConfig 服务池配置
type ServicePoolConfig struct {
	ServiceName     string        `yaml:"service_name"`
	MaxConnections  int           `yaml:"max_connections"`
	MaxIdle         int           `yaml:"max_idle"`
	IdleTimeout     time.Duration `yaml:"idle_timeout"`
	ConnTimeout     time.Duration `yaml:"conn_timeout"`
	Targets         []string      `yaml:"targets"`
	LoadBalanceType string        `yaml:"load_balance_type"`
}

// PoolMetrics 连接池指标
type PoolMetrics struct {
	TotalConnections  int64     `json:"total_connections"`
	ActiveConnections int64     `json:"active_connections"`
	IdleConnections   int64     `json:"idle_connections"`
	FailedConnections int64     `json:"failed_connections"`
	TotalRequests     int64     `json:"total_requests"`
	FailedRequests    int64     `json:"failed_requests"`
	AvgResponseTime   float64   `json:"avg_response_time"`
	LastCleanupTime   time.Time `json:"last_cleanup_time"`
	PoolUtilization   float64   `json:"pool_utilization"`
}

// HealthChecker 健康检查器
type HealthChecker interface {
	CheckConnection(ctx context.Context, conn *grpc.ClientConn) error
}

// LoadBalancer 负载均衡器
type LoadBalancer interface {
	Select(targets []string, connections []*PooledConnection) string
}

// CircuitBreaker 熔断器
type CircuitBreaker interface {
	Allow(serviceName string) bool
	RecordSuccess(serviceName string)
	RecordFailure(serviceName string)
}

// NewAdvancedConnectionPool 创建高级连接池
func NewAdvancedConnectionPool(config *PoolConfig) *AdvancedConnectionPool {
	pool := &AdvancedConnectionPool{
		pools:        make(map[string]*ServicePool),
		globalConfig: config,
		metrics:      &PoolMetrics{},
		stopChan:     make(chan struct{}),
	}

	// 启动清理协程
	if config.CleanupInterval > 0 {
		pool.cleanupTicker = time.NewTicker(config.CleanupInterval)
		go pool.cleanup()
	}

	// 启动健康检查协程
	if config.HealthCheckInterval > 0 {
		go pool.healthCheck()
	}

	return pool
}

// GetConnection 获取连接
func (acp *AdvancedConnectionPool) GetConnection(ctx context.Context, serviceName string) (*grpc.ClientConn, error) {
	// 熔断器检查
	if acp.circuitBreaker != nil && !acp.circuitBreaker.Allow(serviceName) {
		return nil, errors.New("circuit breaker is open")
	}

	acp.mu.RLock()
	servicePool, exists := acp.pools[serviceName]
	acp.mu.RUnlock()

	if !exists {
		return nil, errors.New("service pool not found")
	}

	conn, err := servicePool.getConnection(ctx)
	if err != nil {
		atomic.AddInt64(&acp.metrics.FailedRequests, 1)
		if acp.circuitBreaker != nil {
			acp.circuitBreaker.RecordFailure(serviceName)
		}
		return nil, err
	}

	atomic.AddInt64(&acp.metrics.TotalRequests, 1)
	if acp.circuitBreaker != nil {
		acp.circuitBreaker.RecordSuccess(serviceName)
	}

	return conn, nil
}

// CreateServicePool 创建服务连接池
func (acp *AdvancedConnectionPool) CreateServicePool(config *ServicePoolConfig) error {
	acp.mu.Lock()
	defer acp.mu.Unlock()

	servicePool := &ServicePool{
		serviceName: config.ServiceName,
		connections: make([]*PooledConnection, 0, config.MaxConnections),
		config:      config,
		lastUsed:    time.Now(),
	}

	// 预创建连接
	for i := 0; i < config.MaxIdle && i < len(config.Targets); i++ {
		target := config.Targets[i%len(config.Targets)]
		if conn, err := servicePool.createConnection(target); err == nil {
			servicePool.connections = append(servicePool.connections, conn)
			atomic.AddInt64(&acp.metrics.TotalConnections, 1)
		}
	}

	acp.pools[config.ServiceName] = servicePool
	return nil
}

// getConnection 从服务池获取连接
func (sp *ServicePool) getConnection(ctx context.Context) (*grpc.ClientConn, error) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	sp.lastUsed = time.Now()

	// 查找可用连接
	for _, pooledConn := range sp.connections {
		if pooledConn.isHealthy && time.Since(pooledConn.lastUsed) < sp.config.IdleTimeout {
			pooledConn.lastUsed = time.Now()
			atomic.AddInt64(&pooledConn.useCount, 1)
			return pooledConn.conn, nil
		}
	}

	// 如果没有可用连接，创建新连接
	if len(sp.connections) < sp.config.MaxConnections {
		target := sp.selectTarget()
		if pooledConn, err := sp.createConnection(target); err == nil {
			sp.connections = append(sp.connections, pooledConn)
			pooledConn.lastUsed = time.Now()
			atomic.AddInt64(&pooledConn.useCount, 1)
			return pooledConn.conn, nil
		}
	}

	return nil, errors.New("no available connections")
}

// createConnection 创建新连接
func (sp *ServicePool) createConnection(target string) (*PooledConnection, error) {
	ctx, cancel := context.WithTimeout(context.Background(), sp.config.ConnTimeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, target, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	return &PooledConnection{
		conn:      conn,
		target:    target,
		createdAt: time.Now(),
		lastUsed:  time.Now(),
		isHealthy: true,
	}, nil
}

// selectTarget 选择目标地址
func (sp *ServicePool) selectTarget() string {
	if len(sp.config.Targets) == 0 {
		return ""
	}

	// 简单轮询
	index := atomic.AddInt64(&sp.roundRobin, 1) % int64(len(sp.config.Targets))
	return sp.config.Targets[index]
}

// cleanup 清理过期连接
func (acp *AdvancedConnectionPool) cleanup() {
	ticker := time.NewTicker(acp.globalConfig.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			acp.performCleanup()
		case <-acp.stopChan:
			return
		}
	}
}

// performCleanup 执行清理
func (acp *AdvancedConnectionPool) performCleanup() {
	acp.mu.Lock()
	defer acp.mu.Unlock()

	now := time.Now()

	for serviceName, servicePool := range acp.pools {
		servicePool.mu.Lock()

		// 清理过期连接
		activeConnections := make([]*PooledConnection, 0, len(servicePool.connections))
		for _, conn := range servicePool.connections {
			if now.Sub(conn.lastUsed) > servicePool.config.IdleTimeout {
				conn.conn.Close()
				atomic.AddInt64(&acp.metrics.TotalConnections, -1)
			} else {
				activeConnections = append(activeConnections, conn)
			}
		}
		servicePool.connections = activeConnections

		// 如果服务池长时间未使用，则删除
		if now.Sub(servicePool.lastUsed) > acp.globalConfig.DefaultIdleTimeout*2 {
			for _, conn := range servicePool.connections {
				conn.conn.Close()
				atomic.AddInt64(&acp.metrics.TotalConnections, -1)
			}
			delete(acp.pools, serviceName)
		}

		servicePool.mu.Unlock()
	}

	acp.metrics.LastCleanupTime = now
	acp.updateMetrics()
}

// healthCheck 健康检查
func (acp *AdvancedConnectionPool) healthCheck() {
	if acp.healthChecker == nil {
		return
	}

	ticker := time.NewTicker(acp.globalConfig.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			acp.performHealthCheck()
		case <-acp.stopChan:
			return
		}
	}
}

// performHealthCheck 执行健康检查
func (acp *AdvancedConnectionPool) performHealthCheck() {
	acp.mu.RLock()
	pools := make([]*ServicePool, 0, len(acp.pools))
	for _, pool := range acp.pools {
		pools = append(pools, pool)
	}
	acp.mu.RUnlock()

	for _, servicePool := range pools {
		servicePool.mu.Lock()
		for _, conn := range servicePool.connections {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := acp.healthChecker.CheckConnection(ctx, conn.conn)
			cancel()

			conn.mu.Lock()
			conn.isHealthy = (err == nil)
			conn.mu.Unlock()
		}
		servicePool.mu.Unlock()
	}
}

// updateMetrics 更新指标
func (acp *AdvancedConnectionPool) updateMetrics() {
	var totalConns, activeConns, idleConns int64

	acp.mu.RLock()
	for _, servicePool := range acp.pools {
		servicePool.mu.RLock()
		for _, conn := range servicePool.connections {
			totalConns++
			if time.Since(conn.lastUsed) < time.Minute {
				activeConns++
			} else {
				idleConns++
			}
		}
		servicePool.mu.RUnlock()
	}
	acp.mu.RUnlock()

	acp.metrics.TotalConnections = totalConns
	acp.metrics.ActiveConnections = activeConns
	acp.metrics.IdleConnections = idleConns

	if totalConns > 0 {
		acp.metrics.PoolUtilization = float64(activeConns) / float64(totalConns) * 100
	}
}

// GetMetrics 获取指标
func (acp *AdvancedConnectionPool) GetMetrics() *PoolMetrics {
	acp.updateMetrics()
	return acp.metrics
}

// Close 关闭连接池
func (acp *AdvancedConnectionPool) Close() {
	close(acp.stopChan)

	if acp.cleanupTicker != nil {
		acp.cleanupTicker.Stop()
	}

	acp.mu.Lock()
	defer acp.mu.Unlock()

	for _, servicePool := range acp.pools {
		servicePool.mu.Lock()
		for _, conn := range servicePool.connections {
			conn.conn.Close()
		}
		servicePool.mu.Unlock()
	}
}
