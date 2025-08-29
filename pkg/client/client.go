package client

import (
	"context"
	"fmt"
	"hash/fnv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rpc-framework/core/internal/interceptor"
	"github.com/rpc-framework/core/pkg/registry"
	"github.com/rpc-framework/core/pkg/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// Client gRPC客户端
type Client struct {
	mu             sync.RWMutex
	opts           *ClientOptions
	loadBalancer   LoadBalancer
	connPool       *ConnectionPool
	circuitBreaker *CircuitBreaker
	cache          *Cache
	asyncProcessor *AsyncProcessor
}

// ClientOptions 客户端选项
type ClientOptions struct {
	// 基础配置
	Timeout         time.Duration
	KeepAlive       time.Duration
	MaxRecvMsgSize  int
	MaxSendMsgSize  int
	RetryAttempts   int
	RetryDelay      time.Duration
	LoadBalanceType string
	Tracer          *trace.Tracer

	// 高并发优化配置
	MaxConnections          int           // 最大连接数
	MaxIdleConns            int           // 最大空闲连接数
	ConnTimeout             time.Duration // 连接超时
	IdleTimeout             time.Duration // 空闲超时
	MaxRetries              int           // 最大重试次数
	RetryBackoff            time.Duration // 重试退避时间
	CircuitBreakerThreshold int           // 熔断器阈值
	CircuitBreakerTimeout   time.Duration // 熔断器超时

	// 缓存配置
	EnableCache  bool          // 是否启用缓存
	CacheTTL     time.Duration // 缓存TTL
	CacheMaxSize int           // 缓存最大大小

	// 异步处理配置
	EnableAsync      bool // 是否启用异步处理
	AsyncWorkerCount int  // 异步工作协程数
	AsyncQueueSize   int  // 异步队列大小
}

// LoadBalancer 负载均衡器接口
type LoadBalancer interface {
	Next() string
	UpdateAddresses(addresses []string)
	GetHealthyAddresses() []string
	GetStats() map[string]interface{}
}

// RoundRobinLoadBalancer 轮询负载均衡器
type RoundRobinLoadBalancer struct {
	mu        sync.RWMutex
	addresses []string
	current   int
	health    map[string]bool
	stats     map[string]int64
}

// WeightedRoundRobinLoadBalancer 权重轮询负载均衡器
type WeightedRoundRobinLoadBalancer struct {
	mu        sync.RWMutex
	addresses []*WeightedAddress
	current   int
	weight    int
}

// WeightedAddress 带权重的地址
type WeightedAddress struct {
	Address string
	Weight  int
}

// LeastConnectionsLoadBalancer 最少连接数负载均衡器
type LeastConnectionsLoadBalancer struct {
	mu        sync.RWMutex
	addresses map[string]*ConnectionStats
}

// ConnectionStats 连接统计
type ConnectionStats struct {
	Address      string
	Connections  int64
	LastUsed     time.Time
	ResponseTime time.Duration
	AccessCount  int64
}

// Cache 分片缓存管理器 - 优化锁竞争
type Cache struct {
	shards  []cacheShard
	mask    uint64
	ttl     time.Duration
	maxSize int
}

// cacheShard 缓存分片
type cacheShard struct {
	mu    sync.RWMutex
	items map[string]*CacheItem
	count int64 // 使用原子操作计数
}

const cacheShardCount = 16 // 16个分片，减少锁竞争

// CacheItem 缓存项 - 优化内存布局
type CacheItem struct {
	Value       interface{}
	ExpireTime  time.Time
	AccessTime  time.Time
	AccessCount int64 // 使用原子操作
}

// AsyncProcessor 异步处理器
type AsyncProcessor struct {
	mu       sync.RWMutex
	workers  int
	queue    chan AsyncTask
	handlers map[string]AsyncHandler
	running  bool
}

// AsyncTask 异步任务
type AsyncTask struct {
	ID      string
	Type    string
	Data    interface{}
	Context context.Context
}

// AsyncHandler 异步处理器
type AsyncHandler func(ctx context.Context, data interface{}) error

// ConnectionPool 连接池
type ConnectionPool struct {
	mu          sync.RWMutex
	conns       map[string]*grpc.ClientConn
	maxConns    int
	maxIdle     int
	timeout     time.Duration
	idleTimeout time.Duration
	stats       map[string]*ConnectionStats
}

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	mu           sync.RWMutex
	failureCount int64
	lastFailure  time.Time
	state        CircuitState
	threshold    int
	timeout      time.Duration
	successCount int64
}

// CircuitState 熔断器状态
type CircuitState int

const (
	StateClosed CircuitState = iota
	StateOpen
	StateHalfOpen
)

// NewClient 创建新的客户端
func NewClient(opts *ClientOptions) *Client {
	if opts == nil {
		opts = DefaultClientOptions()
	}

	// 创建负载均衡器
	var lb LoadBalancer
	switch opts.LoadBalanceType {
	case "weighted_round_robin":
		lb = &WeightedRoundRobinLoadBalancer{
			addresses: make([]*WeightedAddress, 0),
			current:   0,
			weight:    0,
		}
	case "least_connections":
		lb = &LeastConnectionsLoadBalancer{
			addresses: make(map[string]*ConnectionStats),
		}
	default: // round_robin
		lb = &RoundRobinLoadBalancer{
			addresses: make([]string, 0),
			current:   0,
			health:    make(map[string]bool),
			stats:     make(map[string]int64),
		}
	}

	// 创建连接池
	connPool := &ConnectionPool{
		conns:       make(map[string]*grpc.ClientConn),
		maxConns:    opts.MaxConnections,
		maxIdle:     opts.MaxIdleConns,
		timeout:     opts.ConnTimeout,
		idleTimeout: opts.IdleTimeout,
		stats:       make(map[string]*ConnectionStats),
	}

	// 创建熔断器
	circuitBreaker := &CircuitBreaker{
		threshold: opts.CircuitBreakerThreshold,
		timeout:   opts.CircuitBreakerTimeout,
		state:     StateClosed,
	}

	// 创建分片缓存
	var cache *Cache
	if opts.EnableCache {
		cache = NewShardedCache(opts.CacheTTL, opts.CacheMaxSize)
	}

	// 创建异步处理器
	var asyncProcessor *AsyncProcessor
	if opts.EnableAsync {
		asyncProcessor = &AsyncProcessor{
			workers:  opts.AsyncWorkerCount,
			queue:    make(chan AsyncTask, opts.AsyncQueueSize),
			handlers: make(map[string]AsyncHandler),
			running:  false,
		}
		asyncProcessor.Start()
	}

	return &Client{
		opts:           opts,
		loadBalancer:   lb,
		connPool:       connPool,
		circuitBreaker: circuitBreaker,
		cache:          cache,
		asyncProcessor: asyncProcessor,
	}
}

// DefaultClientOptions 返回默认客户端配置
func DefaultClientOptions() *ClientOptions {
	return &ClientOptions{
		// 基础配置
		Timeout:         30 * time.Second,
		KeepAlive:       30 * time.Second,
		MaxRecvMsgSize:  4 * 1024 * 1024,
		MaxSendMsgSize:  4 * 1024 * 1024,
		RetryAttempts:   3,
		RetryDelay:      time.Second,
		LoadBalanceType: "round_robin",

		// 高并发配置 - 优化后的默认值
		MaxConnections:          500,                    // 提升5倍
		MaxIdleConns:            100,                    // 提升10倍
		ConnTimeout:             3 * time.Second,        // 降低连接超时
		IdleTimeout:             300 * time.Second,      // 增加空闲超时
		MaxRetries:              5,                      // 增加重试次数
		RetryBackoff:            100 * time.Millisecond, // 降低重试间隔
		CircuitBreakerThreshold: 10,                     // 增加熔断阈值
		CircuitBreakerTimeout:   60 * time.Second,       // 增加熔断超时

		// 缓存配置 - 优化后的默认值
		EnableCache:  true,
		CacheTTL:     5 * time.Minute,
		CacheMaxSize: 10000, // 提升10倍

		// 异步处理配置 - 优化后的默认值
		EnableAsync:      true,
		AsyncWorkerCount: 32,    // 增加到32个工作协程
		AsyncQueueSize:   50000, // 提升50倍
	}
}

// DialService 按服务名拨号（自动发现+负载均衡）
func (c *Client) DialService(ctx context.Context, serviceName string, reg registry.Registry) (*grpc.ClientConn, error) {
	// 检查缓存
	if c.cache != nil {
		if cached := c.cache.Get(serviceName); cached != nil {
			if conn, ok := cached.(*grpc.ClientConn); ok {
				return conn, nil
			}
		}
	}

	// 检查熔断器状态
	if !c.circuitBreaker.IsAllowed() {
		return nil, fmt.Errorf("circuit breaker is open")
	}

	// 发现服务实例
	instances, err := reg.Discover(ctx, serviceName)
	if err != nil {
		c.circuitBreaker.RecordFailure()
		return nil, fmt.Errorf("failed to discover service %s: %w", serviceName, err)
	}

	if len(instances) == 0 {
		c.circuitBreaker.RecordFailure()
		return nil, fmt.Errorf("no instances found for service %s", serviceName)
	}

	// 转换为地址列表
	addresses := make([]string, 0, len(instances))
	for _, ins := range instances {
		addresses = append(addresses, fmt.Sprintf("%s:%d", ins.Address, ins.Port))
	}

	// 使用负载均衡器连接
	conn, err := c.connPool.GetConnection(addresses[0], c.opts)
	if err != nil {
		c.circuitBreaker.RecordFailure()
		return nil, err
	}

	// 缓存连接
	if c.cache != nil {
		c.cache.Set(serviceName, conn)
	}

	// 启动服务变更监听
	go c.watchServiceChanges(ctx, serviceName, reg)

	return conn, nil
}

// watchServiceChanges 监听服务变更
func (c *Client) watchServiceChanges(ctx context.Context, serviceName string, reg registry.Registry) {
	ch, err := reg.Watch(ctx, serviceName)
	if err != nil {
		return
	}

	for instances := range ch {
		addresses := make([]string, 0, len(instances))
		for _, ins := range instances {
			addresses = append(addresses, fmt.Sprintf("%s:%d", ins.Address, ins.Port))
		}
		c.loadBalancer.UpdateAddresses(addresses)
	}
}

// RegisterAsyncHandler 注册异步处理器
func (c *Client) RegisterAsyncHandler(taskType string, handler AsyncHandler) {
	if c.asyncProcessor != nil {
		c.asyncProcessor.RegisterHandler(taskType, handler)
	}
}

// SubmitAsyncTask 提交异步任务
func (c *Client) SubmitAsyncTask(ctx context.Context, taskType string, data interface{}) error {
	if c.asyncProcessor != nil {
		return c.asyncProcessor.SubmitTask(ctx, taskType, data)
	}
	return fmt.Errorf("async processor not enabled")
}

// Close 关闭所有连接
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var lastErr error

	// 关闭连接池中的连接
	if err := c.connPool.Close(); err != nil {
		lastErr = err
	}

	// 停止异步处理器
	if c.asyncProcessor != nil {
		c.asyncProcessor.Stop()
	}

	return lastErr
}

// GetConnection 从连接池获取连接
func (cp *ConnectionPool) GetConnection(address string, opts *ClientOptions) (*grpc.ClientConn, error) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	// 检查是否已有连接
	if conn, exists := cp.conns[address]; exists {
		// 更新统计信息
		if stats, ok := cp.stats[address]; ok {
			stats.LastUsed = time.Now()
			stats.AccessCount++
		}
		return conn, nil
	}

	// 检查连接数限制
	if len(cp.conns) >= cp.maxConns {
		return nil, fmt.Errorf("connection pool is full")
	}

	// 创建新连接
	conn, err := cp.createConnection(address, opts)
	if err != nil {
		return nil, err
	}

	cp.conns[address] = conn

	// 初始化统计信息
	cp.stats[address] = &ConnectionStats{
		Address:     address,
		Connections: 1,
		LastUsed:    time.Now(),
	}

	return conn, nil
}

// createConnection 创建新连接
func (cp *ConnectionPool) createConnection(address string, opts *ClientOptions) (*grpc.ClientConn, error) {
	// 创建拦截器链
	var unaryInterceptors []grpc.UnaryClientInterceptor
	var streamInterceptors []grpc.StreamClientInterceptor

	// 添加追踪拦截器
	if opts.Tracer != nil {
		unaryInterceptors = append(unaryInterceptors, interceptor.TraceClientInterceptor(opts.Tracer))
		streamInterceptors = append(streamInterceptors, interceptor.TraceStreamClientInterceptor(opts.Tracer))
	}

	// 创建拨号选项
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(opts.MaxRecvMsgSize),
			grpc.MaxCallSendMsgSize(opts.MaxSendMsgSize),
		),
		grpc.WithBlock(),
		grpc.WithTimeout(cp.timeout),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                opts.KeepAlive,
			Timeout:             opts.Timeout,
			PermitWithoutStream: true,
		}),
		grpc.WithUnaryInterceptor(chainUnaryClient(unaryInterceptors...)),
		grpc.WithStreamInterceptor(chainStreamClient(streamInterceptors...)),
	}

	// 创建新连接
	conn, err := grpc.Dial(address, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", address, err)
	}

	return conn, nil
}

// Close 关闭连接池
func (cp *ConnectionPool) Close() error {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	var lastErr error
	for address, conn := range cp.conns {
		if err := conn.Close(); err != nil {
			lastErr = err
		}
		delete(cp.conns, address)
		delete(cp.stats, address)
	}

	return lastErr
}

// GetStats 获取连接池统计信息
func (cp *ConnectionPool) GetStats() map[string]interface{} {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	stats := make(map[string]interface{})
	for addr, stat := range cp.stats {
		stats[addr] = map[string]interface{}{
			"connections":   stat.Connections,
			"last_used":     stat.LastUsed,
			"response_time": stat.ResponseTime,
		}
	}

	return stats
}

// IsAllowed 检查熔断器是否允许请求
func (cb *CircuitBreaker) IsAllowed() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(cb.lastFailure) > cb.timeout {
			cb.state = StateHalfOpen
			return true
		}
		return false
	case StateHalfOpen:
		return true
	default:
		return false
	}
}

// RecordFailure 记录失败
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount++
	cb.lastFailure = time.Now()

	if cb.state == StateClosed && cb.failureCount >= int64(cb.threshold) {
		cb.state = StateOpen
	} else if cb.state == StateHalfOpen {
		cb.state = StateOpen
	}
}

// RecordSuccess 记录成功
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.successCount++
	if cb.state == StateHalfOpen {
		cb.state = StateClosed
		cb.failureCount = 0
	}
}

// GetStats 获取熔断器统计信息
func (cb *CircuitBreaker) GetStats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return map[string]interface{}{
		"state":         cb.state,
		"failure_count": cb.failureCount,
		"success_count": cb.successCount,
		"last_failure":  cb.lastFailure,
	}
}

// Next 获取下一个地址（轮询）
func (lb *RoundRobinLoadBalancer) Next() string {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if len(lb.addresses) == 0 {
		return ""
	}

	// 查找健康的下一个地址
	start := lb.current
	for i := 0; i < len(lb.addresses); i++ {
		idx := (start + i) % len(lb.addresses)
		addr := lb.addresses[idx]
		if lb.health[addr] {
			lb.current = (idx + 1) % len(lb.addresses)
			lb.stats[addr]++
			return addr
		}
	}

	return ""
}

// GetHealthyAddresses 获取健康地址列表
func (lb *RoundRobinLoadBalancer) GetHealthyAddresses() []string {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	var healthy []string
	for _, addr := range lb.addresses {
		if lb.health[addr] {
			healthy = append(healthy, addr)
		}
	}
	return healthy
}

// UpdateAddresses 更新地址列表
func (lb *RoundRobinLoadBalancer) UpdateAddresses(addresses []string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.addresses = make([]string, len(addresses))
	copy(lb.addresses, addresses)

	// 重置健康状态
	for _, addr := range addresses {
		lb.health[addr] = true
	}

	if lb.current >= len(lb.addresses) {
		lb.current = 0
	}
}

// GetStats 获取负载均衡器统计信息
func (lb *RoundRobinLoadBalancer) GetStats() map[string]interface{} {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	return map[string]interface{}{
		"addresses": lb.addresses,
		"current":   lb.current,
		"health":    lb.health,
		"stats":     lb.stats,
	}
}

// Next 获取下一个地址（权重轮询）
func (wlb *WeightedRoundRobinLoadBalancer) Next() string {
	wlb.mu.Lock()
	defer wlb.mu.Unlock()

	if len(wlb.addresses) == 0 {
		return ""
	}

	for {
		wlb.current = (wlb.current + 1) % len(wlb.addresses)
		addr := wlb.addresses[wlb.current]

		if wlb.current == 0 {
			wlb.weight = wlb.weight - 1
			if wlb.weight <= 0 {
				wlb.weight = wlb.getMaxWeight()
			}
		}

		if addr.Weight >= wlb.weight {
			return addr.Address
		}
	}
}

// getMaxWeight 获取最大权重
func (wlb *WeightedRoundRobinLoadBalancer) getMaxWeight() int {
	maxWeight := 0
	for _, addr := range wlb.addresses {
		if addr.Weight > maxWeight {
			maxWeight = addr.Weight
		}
	}
	return maxWeight
}

// UpdateAddresses 更新地址列表（权重轮询）
func (wlb *WeightedRoundRobinLoadBalancer) UpdateAddresses(addresses []string) {
	wlb.mu.Lock()
	defer wlb.mu.Unlock()

	wlb.addresses = make([]*WeightedAddress, len(addresses))
	for i, addr := range addresses {
		wlb.addresses[i] = &WeightedAddress{
			Address: addr,
			Weight:  1, // 默认权重为1
		}
	}
	wlb.weight = wlb.getMaxWeight()
}

// GetHealthyAddresses 获取健康地址列表
func (wlb *WeightedRoundRobinLoadBalancer) GetHealthyAddresses() []string {
	wlb.mu.RLock()
	defer wlb.mu.RUnlock()

	addresses := make([]string, len(wlb.addresses))
	for i, addr := range wlb.addresses {
		addresses[i] = addr.Address
	}
	return addresses
}

// GetStats 获取统计信息
func (wlb *WeightedRoundRobinLoadBalancer) GetStats() map[string]interface{} {
	wlb.mu.RLock()
	defer wlb.mu.RUnlock()

	addresses := make([]map[string]interface{}, len(wlb.addresses))
	for i, addr := range wlb.addresses {
		addresses[i] = map[string]interface{}{
			"address": addr.Address,
			"weight":  addr.Weight,
		}
	}

	return map[string]interface{}{
		"addresses": addresses,
		"current":   wlb.current,
		"weight":    wlb.weight,
	}
}

// Next 获取下一个地址（最少连接数）
func (lclb *LeastConnectionsLoadBalancer) Next() string {
	lclb.mu.Lock()
	defer lclb.mu.Unlock()

	if len(lclb.addresses) == 0 {
		return ""
	}

	var minConnections int64 = 1<<63 - 1
	var selectedAddr string

	for addr, stats := range lclb.addresses {
		if stats.Connections < minConnections {
			minConnections = stats.Connections
			selectedAddr = addr
		}
	}

	if selectedAddr != "" {
		lclb.addresses[selectedAddr].Connections++
		lclb.addresses[selectedAddr].LastUsed = time.Now()
	}

	return selectedAddr
}

// UpdateAddresses 更新地址列表（最少连接数）
func (lclb *LeastConnectionsLoadBalancer) UpdateAddresses(addresses []string) {
	lclb.mu.Lock()
	defer lclb.mu.Unlock()

	// 保留现有统计信息
	existingStats := make(map[string]*ConnectionStats)
	for addr, stats := range lclb.addresses {
		existingStats[addr] = stats
	}

	lclb.addresses = make(map[string]*ConnectionStats)
	for _, addr := range addresses {
		if stats, exists := existingStats[addr]; exists {
			lclb.addresses[addr] = stats
		} else {
			lclb.addresses[addr] = &ConnectionStats{
				Address:     addr,
				Connections: 0,
				LastUsed:    time.Now(),
			}
		}
	}
}

// GetHealthyAddresses 获取健康地址列表
func (lclb *LeastConnectionsLoadBalancer) GetHealthyAddresses() []string {
	lclb.mu.RLock()
	defer lclb.mu.RUnlock()

	addresses := make([]string, 0, len(lclb.addresses))
	for addr := range lclb.addresses {
		addresses = append(addresses, addr)
	}
	return addresses
}

// GetStats 获取统计信息
func (lclb *LeastConnectionsLoadBalancer) GetStats() map[string]interface{} {
	lclb.mu.RLock()
	defer lclb.mu.RUnlock()

	stats := make(map[string]interface{})
	for addr, stat := range lclb.addresses {
		stats[addr] = map[string]interface{}{
			"connections":   stat.Connections,
			"last_used":     stat.LastUsed,
			"response_time": stat.ResponseTime,
		}
	}

	return stats
}

// Start 启动异步处理器
func (ap *AsyncProcessor) Start() {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	if ap.running {
		return
	}

	ap.running = true
	for i := 0; i < ap.workers; i++ {
		go ap.worker()
	}
}

// Stop 停止异步处理器
func (ap *AsyncProcessor) Stop() {
	ap.mu.Lock()
	defer ap.mu.Unlock()

	if !ap.running {
		return
	}

	ap.running = false
	close(ap.queue)
}

// worker 工作协程
func (ap *AsyncProcessor) worker() {
	for task := range ap.queue {
		if handler, exists := ap.handlers[task.Type]; exists {
			if err := handler(task.Context, task.Data); err != nil {
				fmt.Printf("Async task failed: %v\n", err)
			}
		}
	}
}

// RegisterHandler 注册处理器
func (ap *AsyncProcessor) RegisterHandler(taskType string, handler AsyncHandler) {
	ap.mu.Lock()
	defer ap.mu.Unlock()
	ap.handlers[taskType] = handler
}

// SubmitTask 提交任务
func (ap *AsyncProcessor) SubmitTask(ctx context.Context, taskType string, data interface{}) error {
	ap.mu.RLock()
	defer ap.mu.RUnlock()

	if !ap.running {
		return fmt.Errorf("async processor not running")
	}

	select {
	case ap.queue <- AsyncTask{
		ID:      fmt.Sprintf("%d", time.Now().UnixNano()),
		Type:    taskType,
		Data:    data,
		Context: ctx,
	}:
		return nil
	default:
		return fmt.Errorf("async queue is full")
	}
}

// chainUnaryClient 链式 unary 客户端拦截器
func chainUnaryClient(interceptors ...grpc.UnaryClientInterceptor) grpc.UnaryClientInterceptor {
	if len(interceptors) == 0 {
		return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			return invoker(ctx, method, req, reply, cc, opts...)
		}
	}

	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		var chain grpc.UnaryInvoker
		for i := len(interceptors) - 1; i >= 0; i-- {
			interceptor := interceptors[i]
			next := chain
			chain = func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
				return interceptor(ctx, method, req, reply, cc, next, opts...)
			}
		}
		return chain(ctx, method, req, reply, cc, opts...)
	}
}

// chainStreamClient 链式 stream 客户端拦截器
func chainStreamClient(interceptors ...grpc.StreamClientInterceptor) grpc.StreamClientInterceptor {
	if len(interceptors) == 0 {
		return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
			return streamer(ctx, desc, cc, method, opts...)
		}
	}

	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		var chain grpc.Streamer
		for i := len(interceptors) - 1; i >= 0; i-- {
			interceptor := interceptors[i]
			next := chain
			chain = func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
				return interceptor(ctx, desc, cc, method, next, opts...)
			}
		}
		return chain(ctx, desc, cc, method, opts...)
	}
}

// NewShardedCache 创建分片缓存
func NewShardedCache(ttl time.Duration, maxSize int) *Cache {
	cache := &Cache{
		shards:  make([]cacheShard, cacheShardCount),
		mask:    cacheShardCount - 1,
		ttl:     ttl,
		maxSize: maxSize,
	}

	// 初始化每个分片
	for i := range cache.shards {
		cache.shards[i].items = make(map[string]*CacheItem)
	}

	return cache
}

// getShard 获取key对应的分片
func (c *Cache) getShard(key string) *cacheShard {
	h := fnv.New64a()
	h.Write([]byte(key))
	return &c.shards[h.Sum64()&c.mask]
}

// Set 设置缓存项 - 优化后无全局锁
func (c *Cache) Set(key string, value interface{}) {
	if c == nil {
		return
	}

	shard := c.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	now := time.Now()
	item := &CacheItem{
		Value:       value,
		ExpireTime:  now.Add(c.ttl),
		AccessTime:  now,
		AccessCount: 1,
	}

	shard.items[key] = item
	atomic.AddInt64(&shard.count, 1)

	// 简单的LRU清理机制
	if len(shard.items) > c.maxSize/cacheShardCount {
		c.evictOldest(shard)
	}
}

// Get 获取缓存项 - 优化后无全局锁
func (c *Cache) Get(key string) interface{} {
	if c == nil {
		return nil
	}

	shard := c.getShard(key)
	shard.mu.RLock()
	item, exists := shard.items[key]
	shard.mu.RUnlock()

	if !exists {
		return nil
	}

	// 检查过期
	if time.Now().After(item.ExpireTime) {
		shard.mu.Lock()
		delete(shard.items, key)
		atomic.AddInt64(&shard.count, -1)
		shard.mu.Unlock()
		return nil
	}

	// 更新访问统计 - 使用原子操作
	atomic.AddInt64(&item.AccessCount, 1)
	item.AccessTime = time.Now()

	return item.Value
}

// Delete 删除缓存项
func (c *Cache) Delete(key string) {
	if c == nil {
		return
	}

	shard := c.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	if _, exists := shard.items[key]; exists {
		delete(shard.items, key)
		atomic.AddInt64(&shard.count, -1)
	}
}

// evictOldest 清理最旧的缓存项
func (c *Cache) evictOldest(shard *cacheShard) {
	if len(shard.items) == 0 {
		return
	}

	var oldestKey string
	var oldestTime time.Time = time.Now()

	for key, item := range shard.items {
		if item.AccessTime.Before(oldestTime) {
			oldestTime = item.AccessTime
			oldestKey = key
		}
	}

	if oldestKey != "" {
		delete(shard.items, oldestKey)
		atomic.AddInt64(&shard.count, -1)
	}
}

// GetStats 获取缓存统计信息
func (c *Cache) GetStats() map[string]interface{} {
	if c == nil {
		return nil
	}

	var totalItems int64
	for i := range c.shards {
		totalItems += atomic.LoadInt64(&c.shards[i].count)
	}

	return map[string]interface{}{
		"total_items": totalItems,
		"shard_count": cacheShardCount,
		"max_size":    c.maxSize,
		"ttl_seconds": c.ttl.Seconds(),
		"utilization": float64(totalItems) / float64(c.maxSize),
	}
}
