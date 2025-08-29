package server

import (
	"context"
	"fmt"
	"math"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rpc-framework/core/internal/interceptor"
	"github.com/rpc-framework/core/pkg/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// Server gRPC服务器
type Server struct {
	mu       sync.RWMutex
	opts     *Options
	server   *grpc.Server
	address  string
	services map[string]interface{}

	// 高并发优化组件
	rateLimiter    *RateLimiter
	connectionPool *ServerConnectionPool
	metrics        *ServerMetrics
	memoryPool     *MemoryPool
	asyncProcessor *ServerAsyncProcessor
	healthChecker  *HealthChecker
}

// Options 服务器选项
type Options struct {
	// 基础配置
	Address        string
	MaxRecvMsgSize int
	MaxSendMsgSize int
	Tracer         *trace.Tracer

	// 高并发优化配置
	MaxConcurrentRequests int           // 最大并发请求数
	RequestTimeout        time.Duration // 请求超时时间
	MaxConnections        int           // 最大连接数
	ConnectionTimeout     time.Duration // 连接超时
	KeepAliveTime         time.Duration // Keep-Alive 时间
	KeepAliveTimeout      time.Duration // Keep-Alive 超时
	RateLimit             int           // 每秒请求限制
	EnableMetrics         bool          // 是否启用指标收集

	// 内存池配置
	EnableMemoryPool  bool // 是否启用内存池
	MemoryPoolSize    int  // 内存池大小
	MemoryPoolMaxSize int  // 内存池最大大小

	// 异步处理配置
	EnableAsync      bool // 是否启用异步处理
	AsyncWorkerCount int  // 异步工作协程数
	AsyncQueueSize   int  // 异步队列大小

	// 健康检查配置
	EnableHealthCheck   bool          // 是否启用健康检查
	HealthCheckInterval time.Duration // 健康检查间隔
	HealthCheckTimeout  time.Duration // 健康检查超时
}

// RateLimiter 令牌桶限流器 - 优化算法
type RateLimiter struct {
	mu         sync.Mutex
	limit      int       // 每秒令牌数
	tokens     float64   // 当前令牌数
	lastRefill time.Time // 上次补充时间
	refillRate float64   // 补充速率（令牌/秒）
	bucketSize int       // 桶容量
}

// ServerConnectionPool 服务器连接池
type ServerConnectionPool struct {
	mu       sync.RWMutex
	conns    map[string]net.Conn
	maxConns int
	timeout  time.Duration
	stats    map[string]*ConnectionStats
}

// ConnectionStats 连接统计
type ConnectionStats struct {
	Address      string
	Connections  int64
	LastUsed     time.Time
	ResponseTime time.Duration
	AccessCount  int64
}

// ServerMetrics 服务器指标 - 优化为原子操作
type ServerMetrics struct {
	// 使用原子操作替代锁，提升并发性能
	requestCount      int64 // atomic
	errorCount        int64 // atomic
	activeConnections int64 // atomic
	// 非热点数据仍使用锁保护
	mu           sync.RWMutex
	responseTime time.Duration
	throughput   float64
	latency      time.Duration
}

// MemoryPool 内存池 - 使用sync.Pool优化
type MemoryPool struct {
	bufferPool  *sync.Pool // 使用sync.Pool提高性能
	size        int
	maxSize     int
	allocated   int64        // 使用原子操作计数
	bufferSizes []int        // 支持多种缓冲区大小
	statsMu     sync.RWMutex // 统计信息锁
	hits        int64        // 命中次数
	misses      int64        // 未命中次数
}

// ServerAsyncProcessor 服务器异步处理器
type ServerAsyncProcessor struct {
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

// HealthChecker 健康检查器
type HealthChecker struct {
	mu       sync.RWMutex
	checks   map[string]HealthCheck
	interval time.Duration
	timeout  time.Duration
	running  bool
	stopChan chan struct{}
}

// HealthCheck 健康检查函数
type HealthCheck func(ctx context.Context) error

// New 创建新的服务器
func New(opts *Options) *Server {
	if opts == nil {
		opts = DefaultOptions()
	}

	// 创建拦截器链
	var unaryInterceptors []grpc.UnaryServerInterceptor
	var streamInterceptors []grpc.StreamServerInterceptor

	// 添加追踪拦截器
	if opts.Tracer != nil {
		unaryInterceptors = append(unaryInterceptors, interceptor.TraceServerInterceptor(opts.Tracer))
		streamInterceptors = append(streamInterceptors, interceptor.TraceStreamServerInterceptor(opts.Tracer))
	}

	// 添加限流拦截器 - 优化的令牌桶限流器
	rateLimiter := &RateLimiter{
		limit:      opts.RateLimit,
		tokens:     float64(opts.RateLimit),
		lastRefill: time.Now(),
		refillRate: float64(opts.RateLimit),
		bucketSize: opts.RateLimit,
	}
	unaryInterceptors = append(unaryInterceptors, rateLimiter.UnaryInterceptor())
	streamInterceptors = append(streamInterceptors, rateLimiter.StreamInterceptor())

	// 添加超时拦截器
	unaryInterceptors = append(unaryInterceptors, timeoutUnaryInterceptor(opts.RequestTimeout))
	streamInterceptors = append(streamInterceptors, timeoutStreamInterceptor(opts.RequestTimeout))

	// 添加指标拦截器
	var metrics *ServerMetrics
	if opts.EnableMetrics {
		metrics = &ServerMetrics{}
		unaryInterceptors = append(unaryInterceptors, metrics.UnaryInterceptor())
		streamInterceptors = append(streamInterceptors, metrics.StreamInterceptor())
	}

	// 创建 gRPC 服务器
	server := grpc.NewServer(
		grpc.MaxRecvMsgSize(opts.MaxRecvMsgSize),
		grpc.MaxSendMsgSize(opts.MaxSendMsgSize),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle: opts.KeepAliveTime,
			MaxConnectionAge:  opts.KeepAliveTimeout,
			Time:              opts.KeepAliveTime,
			Timeout:           opts.KeepAliveTimeout,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             time.Second * 30,
			PermitWithoutStream: true,
		}),
		grpc.UnaryInterceptor(chainUnaryServer(unaryInterceptors...)),
		grpc.StreamInterceptor(chainStreamServer(streamInterceptors...)),
	)

	// 创建连接池
	connPool := &ServerConnectionPool{
		conns:    make(map[string]net.Conn),
		maxConns: opts.MaxConnections,
		timeout:  opts.ConnectionTimeout,
		stats:    make(map[string]*ConnectionStats),
	}

	// 创建内存池 - 使用sync.Pool优化
	var memoryPool *MemoryPool
	if opts.EnableMemoryPool {
		memoryPool = NewMemoryPool(opts.MemoryPoolSize, opts.MemoryPoolMaxSize)
	}

	// 创建异步处理器
	var asyncProcessor *ServerAsyncProcessor
	if opts.EnableAsync {
		asyncProcessor = &ServerAsyncProcessor{
			workers:  opts.AsyncWorkerCount,
			queue:    make(chan AsyncTask, opts.AsyncQueueSize),
			handlers: make(map[string]AsyncHandler),
			running:  false,
		}
		asyncProcessor.Start()
	}

	// 创建健康检查器
	var healthChecker *HealthChecker
	if opts.EnableHealthCheck {
		healthChecker = &HealthChecker{
			checks:   make(map[string]HealthCheck),
			interval: opts.HealthCheckInterval,
			timeout:  opts.HealthCheckTimeout,
			stopChan: make(chan struct{}),
		}
		healthChecker.Start()
	}

	return &Server{
		opts:           opts,
		server:         server,
		address:        opts.Address,
		services:       make(map[string]interface{}),
		rateLimiter:    rateLimiter,
		connectionPool: connPool,
		metrics:        metrics,
		memoryPool:     memoryPool,
		asyncProcessor: asyncProcessor,
		healthChecker:  healthChecker,
	}
}

// DefaultOptions 返回默认服务器配置
func DefaultOptions() *Options {
	return &Options{
		// 基础配置
		Address:        ":50051",
		MaxRecvMsgSize: 4 * 1024 * 1024,
		MaxSendMsgSize: 4 * 1024 * 1024,

		// 高并发配置 - 优化后的默认值
		MaxConcurrentRequests: 10000, // 提升10倍
		RequestTimeout:        30 * time.Second,
		MaxConnections:        5000,             // 提升5倍
		ConnectionTimeout:     3 * time.Second,  // 降低连接超时
		KeepAliveTime:         60 * time.Second, // 增加保活时间
		KeepAliveTimeout:      10 * time.Second, // 增加保活超时
		RateLimit:             5000,             // 提升5倍
		EnableMetrics:         true,

		// 内存池配置 - 优化后的默认值
		EnableMemoryPool:  true,
		MemoryPoolSize:    10000,  // 提升10倍
		MemoryPoolMaxSize: 100000, // 提升10倍

		// 异步处理配置 - 优化后的默认值
		EnableAsync:      true,
		AsyncWorkerCount: 32,    // 增加到32个工作协程
		AsyncQueueSize:   50000, // 提升50倍

		// 健康检查配置
		EnableHealthCheck:   true,
		HealthCheckInterval: 30 * time.Second,
		HealthCheckTimeout:  5 * time.Second,
	}
}

// RegisterService 注册服务
func (s *Server) RegisterService(desc *grpc.ServiceDesc, impl interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.server.RegisterService(desc, impl)
	s.services[desc.ServiceName] = impl
}

// RegisterAsyncHandler 注册异步处理器
func (s *Server) RegisterAsyncHandler(taskType string, handler AsyncHandler) {
	if s.asyncProcessor != nil {
		s.asyncProcessor.RegisterHandler(taskType, handler)
	}
}

// SubmitAsyncTask 提交异步任务
func (s *Server) SubmitAsyncTask(ctx context.Context, taskType string, data interface{}) error {
	if s.asyncProcessor != nil {
		return s.asyncProcessor.SubmitTask(ctx, taskType, data)
	}
	return fmt.Errorf("async processor not enabled")
}

// RegisterHealthCheck 注册健康检查
func (s *Server) RegisterHealthCheck(name string, check HealthCheck) {
	if s.healthChecker != nil {
		s.healthChecker.RegisterCheck(name, check)
	}
}

// GetBuffer 从内存池获取缓冲区
func (s *Server) GetBuffer() []byte {
	if s.memoryPool != nil {
		return s.memoryPool.Get()
	}
	return make([]byte, 4096)
}

// PutBuffer 将缓冲区放回内存池
func (s *Server) PutBuffer(buf []byte) {
	if s.memoryPool != nil {
		s.memoryPool.Put(buf)
	}
}

// Start 启动服务器
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 启用反射服务
	reflection.Register(s.server)

	// 创建监听器
	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.address, err)
	}

	// 启动服务器
	go func() {
		if err := s.server.Serve(listener); err != nil {
			fmt.Printf("Server failed to serve: %v\n", err)
		}
	}()

	return nil
}

// Stop 停止服务器
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.server != nil {
		s.server.GracefulStop()
	}

	// 关闭连接池
	if s.connectionPool != nil {
		s.connectionPool.Close()
	}

	// 停止异步处理器
	if s.asyncProcessor != nil {
		s.asyncProcessor.Stop()
	}

	// 停止健康检查器
	if s.healthChecker != nil {
		s.healthChecker.Stop()
	}

	return nil
}

// GetMetrics 获取服务器指标
func (s *Server) GetMetrics() *ServerMetrics {
	return s.metrics
}

// GetMemoryPoolStats 获取内存池统计信息
func (s *Server) GetMemoryPoolStats() map[string]interface{} {
	if s.memoryPool != nil {
		return s.memoryPool.GetStats()
	}
	return nil
}

// UnaryInterceptor 限流器 unary 拦截器
func (rl *RateLimiter) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if !rl.Allow() {
			return nil, status.Error(8, "rate limit exceeded")
		}
		return handler(ctx, req)
	}
}

// StreamInterceptor 限流器 stream 拦截器
func (rl *RateLimiter) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if !rl.Allow() {
			return status.Error(8, "rate limit exceeded")
		}
		return handler(srv, ss)
	}
}

// Allow 检查是否允许请求 - 优化的令牌桶算法
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastRefill).Seconds()

	// 计算需要补充的令牌数
	tokensToAdd := elapsed * rl.refillRate
	if tokensToAdd > 0 {
		rl.tokens = math.Min(float64(rl.bucketSize), rl.tokens+tokensToAdd)
		rl.lastRefill = now
	}

	// 检查是否有可用令牌
	if rl.tokens >= 1.0 {
		rl.tokens -= 1.0
		return true
	}

	return false
}

// UnaryInterceptor 指标收集 unary 拦截器 - 优化锁使用
func (sm *ServerMetrics) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()

		// 使用原子操作增加计数器，无需锁
		atomic.AddInt64(&sm.requestCount, 1)
		atomic.AddInt64(&sm.activeConnections, 1)

		defer func() {
			// 减少活跃连接数
			atomic.AddInt64(&sm.activeConnections, -1)

			// 只有非热点数据才使用锁
			sm.mu.Lock()
			sm.responseTime = time.Since(start)
			sm.mu.Unlock()
		}()

		resp, err := handler(ctx, req)

		if err != nil {
			// 使用原子操作增加错误计数，无需锁
			atomic.AddInt64(&sm.errorCount, 1)
		}

		return resp, err
	}
}

// StreamInterceptor 指标收集 stream 拦截器 - 优化锁使用
func (sm *ServerMetrics) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()

		// 使用原子操作增加计数器，无需锁
		atomic.AddInt64(&sm.requestCount, 1)
		atomic.AddInt64(&sm.activeConnections, 1)

		defer func() {
			// 减少活跃连接数
			atomic.AddInt64(&sm.activeConnections, -1)

			// 只有非热点数据才使用锁
			sm.mu.Lock()
			sm.responseTime = time.Since(start)
			sm.mu.Unlock()
		}()

		err := handler(srv, ss)

		if err != nil {
			// 使用原子操作增加错误计数，无需锁
			atomic.AddInt64(&sm.errorCount, 1)
		}

		return err
	}
}

// GetStats 获取统计信息
func (sm *ServerMetrics) GetStats() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	requestCount := atomic.LoadInt64(&sm.requestCount)
	errorCount := atomic.LoadInt64(&sm.errorCount)
	activeConnections := atomic.LoadInt64(&sm.activeConnections)

	var errorRate float64
	if requestCount > 0 {
		errorRate = float64(errorCount) / float64(requestCount)
	}

	return map[string]interface{}{
		"request_count":      requestCount,
		"error_count":        errorCount,
		"active_connections": activeConnections,
		"avg_response_time":  sm.responseTime,
		"error_rate":         errorRate,
		"throughput":         sm.throughput,
		"latency":            sm.latency,
	}
}

// Close 关闭连接池
func (scp *ServerConnectionPool) Close() error {
	scp.mu.Lock()
	defer scp.mu.Unlock()

	var lastErr error
	for _, conn := range scp.conns {
		if err := conn.Close(); err != nil {
			lastErr = err
		}
	}
	scp.conns = make(map[string]net.Conn)
	scp.stats = make(map[string]*ConnectionStats)

	return lastErr
}

// GetStats 获取连接池统计信息
func (scp *ServerConnectionPool) GetStats() map[string]interface{} {
	scp.mu.RLock()
	defer scp.mu.RUnlock()

	stats := make(map[string]interface{})
	for addr, stat := range scp.stats {
		stats[addr] = map[string]interface{}{
			"connections":   stat.Connections,
			"last_used":     stat.LastUsed,
			"response_time": stat.ResponseTime,
			"access_count":  stat.AccessCount,
		}
	}

	return stats
}

// Start 启动异步处理器
func (sap *ServerAsyncProcessor) Start() {
	sap.mu.Lock()
	defer sap.mu.Unlock()

	if sap.running {
		return
	}

	sap.running = true
	for i := 0; i < sap.workers; i++ {
		go sap.worker()
	}
}

// Stop 停止异步处理器
func (sap *ServerAsyncProcessor) Stop() {
	sap.mu.Lock()
	defer sap.mu.Unlock()

	if !sap.running {
		return
	}

	sap.running = false
	close(sap.queue)
}

// worker 工作协程
func (sap *ServerAsyncProcessor) worker() {
	for task := range sap.queue {
		if handler, exists := sap.handlers[task.Type]; exists {
			if err := handler(task.Context, task.Data); err != nil {
				fmt.Printf("Async task failed: %v\n", err)
			}
		}
	}
}

// RegisterHandler 注册处理器
func (sap *ServerAsyncProcessor) RegisterHandler(taskType string, handler AsyncHandler) {
	sap.mu.Lock()
	defer sap.mu.Unlock()
	sap.handlers[taskType] = handler
}

// SubmitTask 提交任务
func (sap *ServerAsyncProcessor) SubmitTask(ctx context.Context, taskType string, data interface{}) error {
	sap.mu.RLock()
	defer sap.mu.RUnlock()

	if !sap.running {
		return fmt.Errorf("async processor not running")
	}

	select {
	case sap.queue <- AsyncTask{
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

// Start 启动健康检查器
func (hc *HealthChecker) Start() {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if hc.running {
		return
	}

	hc.running = true
	go hc.run()
}

// Stop 停止健康检查器
func (hc *HealthChecker) Stop() {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if !hc.running {
		return
	}

	hc.running = false
	close(hc.stopChan)
}

// run 运行健康检查
func (hc *HealthChecker) run() {
	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hc.runChecks()
		case <-hc.stopChan:
			return
		}
	}
}

// runChecks 执行健康检查
func (hc *HealthChecker) runChecks() {
	hc.mu.RLock()
	checks := make(map[string]HealthCheck)
	for name, check := range hc.checks {
		checks[name] = check
	}
	hc.mu.RUnlock()

	for name, check := range checks {
		go func(name string, check HealthCheck) {
			ctx, cancel := context.WithTimeout(context.Background(), hc.timeout)
			defer cancel()

			if err := check(ctx); err != nil {
				fmt.Printf("Health check %s failed: %v\n", name, err)
			}
		}(name, check)
	}
}

// RegisterCheck 注册健康检查
func (hc *HealthChecker) RegisterCheck(name string, check HealthCheck) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	hc.checks[name] = check
}

// timeoutUnaryInterceptor 超时控制 unary 拦截器
func timeoutUnaryInterceptor(timeout time.Duration) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}
		return handler(ctx, req)
	}
}

// timeoutStreamInterceptor 超时控制 stream 拦截器
func timeoutStreamInterceptor(timeout time.Duration) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if timeout > 0 {
			ctx, cancel := context.WithTimeout(ss.Context(), timeout)
			defer cancel()

			// 创建带超时的 stream wrapper
			wrappedStream := &timeoutServerStream{
				ServerStream: ss,
				ctx:          ctx,
			}
			return handler(srv, wrappedStream)
		}
		return handler(srv, ss)
	}
}

// timeoutServerStream 带超时的 stream wrapper
type timeoutServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (tss *timeoutServerStream) Context() context.Context {
	return tss.ctx
}

// chainUnaryServer 链式 unary 服务器拦截器
func chainUnaryServer(interceptors ...grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	if len(interceptors) == 0 {
		return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
			return handler(ctx, req)
		}
	}

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		var chain grpc.UnaryHandler
		for i := len(interceptors) - 1; i >= 0; i-- {
			interceptor := interceptors[i]
			next := chain
			chain = func(ctx context.Context, req interface{}) (interface{}, error) {
				return interceptor(ctx, req, info, next)
			}
		}
		return chain(ctx, req)
	}
}

// chainStreamServer 链式 stream 服务器拦截器
func chainStreamServer(interceptors ...grpc.StreamServerInterceptor) grpc.StreamServerInterceptor {
	if len(interceptors) == 0 {
		return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
			return handler(srv, ss)
		}
	}

	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		var chain grpc.StreamHandler
		for i := len(interceptors) - 1; i >= 0; i-- {
			interceptor := interceptors[i]
			next := chain
			chain = func(srv interface{}, ss grpc.ServerStream) error {
				return interceptor(srv, ss, info, next)
			}
		}
		return chain(srv, ss)
	}
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// NewMemoryPool 创建新的内存池 - 使用sync.Pool优化
func NewMemoryPool(size, maxSize int) *MemoryPool {
	mp := &MemoryPool{
		size:        size,
		maxSize:     maxSize,
		bufferSizes: []int{1024, 4096, 16384, 65536}, // 1KB, 4KB, 16KB, 64KB
	}

	// 初始化sync.Pool
	mp.bufferPool = &sync.Pool{
		New: func() interface{} {
			// 默认创建4KB缓冲区
			return make([]byte, 4096)
		},
	}

	return mp
}

// Get 从内存池获取缓冲区 - 优化版本
func (mp *MemoryPool) Get() []byte {
	if mp == nil || mp.bufferPool == nil {
		return make([]byte, 4096)
	}

	// 从sync.Pool获取缓冲区
	buf := mp.bufferPool.Get().([]byte)

	// 更新统计信息
	atomic.AddInt64(&mp.allocated, 1)
	atomic.AddInt64(&mp.hits, 1)

	return buf
}

// GetWithSize 获取指定大小的缓冲区
func (mp *MemoryPool) GetWithSize(size int) []byte {
	if mp == nil {
		return make([]byte, size)
	}

	// 找到合适的缓冲区大小
	bufferSize := mp.findBestSize(size)
	if bufferSize == 4096 {
		// 使用池中的标准大小
		return mp.Get()
	}

	// 对于非标准大小，直接分配
	atomic.AddInt64(&mp.misses, 1)
	return make([]byte, bufferSize)
}

// Put 将缓冲区放回内存池 - 优化版本
func (mp *MemoryPool) Put(buf []byte) {
	if mp == nil || mp.bufferPool == nil || len(buf) != 4096 {
		// 只回收4KB的标准缓冲区
		return
	}

	// 重置缓冲区（可选，取决于使用场景）
	for i := range buf {
		buf[i] = 0
	}

	// 放回sync.Pool
	mp.bufferPool.Put(buf)
	atomic.AddInt64(&mp.allocated, -1)
}

// findBestSize 找到最适合的缓冲区大小
func (mp *MemoryPool) findBestSize(size int) int {
	for _, bufferSize := range mp.bufferSizes {
		if size <= bufferSize {
			return bufferSize
		}
	}
	// 如果超过最大预定义大小，返回请求的大小
	return size
}

// GetStats 获取内存池统计信息 - 优化版本
func (mp *MemoryPool) GetStats() map[string]interface{} {
	if mp == nil {
		return nil
	}

	allocated := atomic.LoadInt64(&mp.allocated)
	hits := atomic.LoadInt64(&mp.hits)
	misses := atomic.LoadInt64(&mp.misses)
	total := hits + misses

	var hitRate float64
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}

	return map[string]interface{}{
		"pool_size":    mp.size,
		"max_size":     mp.maxSize,
		"allocated":    allocated,
		"hits":         hits,
		"misses":       misses,
		"hit_rate":     hitRate,
		"buffer_sizes": mp.bufferSizes,
	}
}
