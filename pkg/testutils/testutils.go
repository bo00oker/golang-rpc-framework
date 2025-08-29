package testutils

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/rpc-framework/core/pkg/logger"
	"github.com/rpc-framework/core/pkg/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// TestServer 测试服务器
type TestServer struct {
	server   *grpc.Server
	listener *bufconn.Listener
	conn     *grpc.ClientConn
	address  string
}

// NewTestServer 创建测试服务器
func NewTestServer(t *testing.T) *TestServer {
	// 创建缓冲连接
	listener := bufconn.Listen(1024 * 1024)

	// 创建gRPC服务器
	grpcServer := grpc.NewServer()

	// 获取可用端口
	address := getFreePort(t)

	ts := &TestServer{
		server:   grpcServer,
		listener: listener,
		address:  address,
	}

	// 启动服务器
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Logf("Test server failed: %v", err)
		}
	}()

	// 创建客户端连接
	conn, err := grpc.DialContext(
		context.Background(),
		"bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	ts.conn = conn

	return ts
}

// GetServer 获取gRPC服务器
func (ts *TestServer) GetServer() *grpc.Server {
	return ts.server
}

// GetConn 获取客户端连接
func (ts *TestServer) GetConn() *grpc.ClientConn {
	return ts.conn
}

// GetAddress 获取服务器地址
func (ts *TestServer) GetAddress() string {
	return ts.address
}

// Close 关闭测试服务器
func (ts *TestServer) Close() {
	if ts.conn != nil {
		ts.conn.Close()
	}
	if ts.server != nil {
		ts.server.Stop()
	}
	if ts.listener != nil {
		ts.listener.Close()
	}
}

// getFreePort 获取可用端口
func getFreePort(t *testing.T) string {
	listener, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	return fmt.Sprintf("localhost:%d", port)
}

// MockRegistry 模拟注册中心
type MockRegistry struct {
	services map[string][]registry.ServiceInstance
}

// NewMockRegistry 创建模拟注册中心
func NewMockRegistry() *MockRegistry {
	return &MockRegistry{
		services: make(map[string][]registry.ServiceInstance),
	}
}

// Register 注册服务
func (m *MockRegistry) Register(ctx context.Context, instance *registry.ServiceInstance) error {
	if m.services[instance.ServiceName] == nil {
		m.services[instance.ServiceName] = make([]registry.ServiceInstance, 0)
	}
	m.services[instance.ServiceName] = append(m.services[instance.ServiceName], *instance)
	return nil
}

// Deregister 注销服务
func (m *MockRegistry) Deregister(ctx context.Context, instance *registry.ServiceInstance) error {
	services := m.services[instance.ServiceName]
	for i, svc := range services {
		if svc.ID == instance.ID {
			m.services[instance.ServiceName] = append(services[:i], services[i+1:]...)
			break
		}
	}
	return nil
}

// Discover 发现服务
func (m *MockRegistry) Discover(ctx context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	services := m.services[serviceName]
	result := make([]*registry.ServiceInstance, len(services))
	for i, svc := range services {
		result[i] = &svc
	}
	return result, nil
}

// Watch 监听服务变化
func (m *MockRegistry) Watch(ctx context.Context, serviceName string) (<-chan []*registry.ServiceInstance, error) {
	ch := make(chan []*registry.ServiceInstance, 1)

	// 发送初始服务列表
	go func() {
		instances, _ := m.Discover(ctx, serviceName)
		select {
		case ch <- instances:
		case <-ctx.Done():
			return
		}
	}()

	return ch, nil
}

// Close 关闭注册中心
func (m *MockRegistry) Close() error {
	return nil
}

// TestCase 测试用例
type TestCase struct {
	Name        string
	Setup       func(t *testing.T) interface{}
	Run         func(t *testing.T, data interface{})
	Cleanup     func(t *testing.T, data interface{})
	ExpectError bool
}

// RunTestCases 运行测试用例
func RunTestCases(t *testing.T, cases []TestCase) {
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			var data interface{}

			// 设置
			if tc.Setup != nil {
				data = tc.Setup(t)
			}

			// 清理
			if tc.Cleanup != nil {
				defer tc.Cleanup(t, data)
			}

			// 执行
			if tc.ExpectError {
				assert.Panics(t, func() {
					tc.Run(t, data)
				})
			} else {
				tc.Run(t, data)
			}
		})
	}
}

// BenchmarkRunner 性能测试运行器
type BenchmarkRunner struct {
	name        string
	setupFunc   func(b *testing.B) interface{}
	runFunc     func(b *testing.B, data interface{})
	cleanupFunc func(b *testing.B, data interface{})
}

// NewBenchmarkRunner 创建性能测试运行器
func NewBenchmarkRunner(name string) *BenchmarkRunner {
	return &BenchmarkRunner{
		name: name,
	}
}

// Setup 设置函数
func (br *BenchmarkRunner) Setup(fn func(b *testing.B) interface{}) *BenchmarkRunner {
	br.setupFunc = fn
	return br
}

// Run 运行函数
func (br *BenchmarkRunner) Run(fn func(b *testing.B, data interface{})) *BenchmarkRunner {
	br.runFunc = fn
	return br
}

// Cleanup 清理函数
func (br *BenchmarkRunner) Cleanup(fn func(b *testing.B, data interface{})) *BenchmarkRunner {
	br.cleanupFunc = fn
	return br
}

// Execute 执行性能测试
func (br *BenchmarkRunner) Execute(b *testing.B) {
	var data interface{}

	// 设置
	if br.setupFunc != nil {
		data = br.setupFunc(b)
	}

	// 清理
	if br.cleanupFunc != nil {
		defer br.cleanupFunc(b, data)
	}

	// 重置计时器
	b.ResetTimer()

	// 运行测试
	for i := 0; i < b.N; i++ {
		br.runFunc(b, data)
	}
}

// LoadTestConfig 负载测试配置
type LoadTestConfig struct {
	Duration       time.Duration
	Concurrency    int
	RequestsPerSec int
	RampUpTime     time.Duration
}

// LoadTestResult 负载测试结果
type LoadTestResult struct {
	TotalRequests   int64
	SuccessRequests int64
	FailedRequests  int64
	AverageLatency  time.Duration
	MinLatency      time.Duration
	MaxLatency      time.Duration
	P50Latency      time.Duration
	P90Latency      time.Duration
	P99Latency      time.Duration
	Throughput      float64
	ErrorRate       float64
}

// LoadTester 负载测试器
type LoadTester struct {
	config *LoadTestConfig
	logger logger.Logger
}

// NewLoadTester 创建负载测试器
func NewLoadTester(config *LoadTestConfig) *LoadTester {
	return &LoadTester{
		config: config,
		logger: logger.GetGlobalLogger(),
	}
}

// RunLoadTest 运行负载测试
func (lt *LoadTester) RunLoadTest(
	ctx context.Context,
	requestFunc func(ctx context.Context) error,
) *LoadTestResult {
	result := &LoadTestResult{
		MinLatency: time.Hour, // 初始化为很大的值
	}

	// 这里应该实现完整的负载测试逻辑
	// 包括并发控制、请求速率控制、延迟统计等
	// 为了简化，这里只是一个框架

	lt.logger.Infof("Starting load test with %d concurrent users for %v",
		lt.config.Concurrency, lt.config.Duration)

	// 实际实现会更复杂
	result.TotalRequests = int64(lt.config.Duration.Seconds()) * int64(lt.config.RequestsPerSec)
	result.SuccessRequests = result.TotalRequests
	result.Throughput = float64(result.SuccessRequests) / lt.config.Duration.Seconds()

	return result
}

// AssertEventually 最终一致性断言
func AssertEventually(t *testing.T, condition func() bool, timeout time.Duration, interval time.Duration, msgAndArgs ...interface{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	timeoutCh := time.After(timeout)

	for {
		select {
		case <-timeoutCh:
			assert.Fail(t, "Condition was not met within timeout", msgAndArgs...)
			return
		case <-ticker.C:
			if condition() {
				return
			}
		}
	}
}

// MockLogger 模拟日志器
type MockLogger struct {
	logs []LogEntry
}

// LogEntry 日志条目
type LogEntry struct {
	Level   string
	Message string
	Time    time.Time
}

// NewMockLogger 创建模拟日志器
func NewMockLogger() *MockLogger {
	return &MockLogger{
		logs: make([]LogEntry, 0),
	}
}

// Info 信息日志
func (m *MockLogger) Info(args ...interface{}) {
	m.logs = append(m.logs, LogEntry{
		Level:   "INFO",
		Message: fmt.Sprint(args...),
		Time:    time.Now(),
	})
}

// Infof 格式化信息日志
func (m *MockLogger) Infof(format string, args ...interface{}) {
	m.logs = append(m.logs, LogEntry{
		Level:   "INFO",
		Message: fmt.Sprintf(format, args...),
		Time:    time.Now(),
	})
}

// Error 错误日志
func (m *MockLogger) Error(args ...interface{}) {
	m.logs = append(m.logs, LogEntry{
		Level:   "ERROR",
		Message: fmt.Sprint(args...),
		Time:    time.Now(),
	})
}

// Errorf 格式化错误日志
func (m *MockLogger) Errorf(format string, args ...interface{}) {
	m.logs = append(m.logs, LogEntry{
		Level:   "ERROR",
		Message: fmt.Sprintf(format, args...),
		Time:    time.Now(),
	})
}

// Warn 警告日志
func (m *MockLogger) Warn(args ...interface{}) {
	m.logs = append(m.logs, LogEntry{
		Level:   "WARN",
		Message: fmt.Sprint(args...),
		Time:    time.Now(),
	})
}

// Warnf 格式化警告日志
func (m *MockLogger) Warnf(format string, args ...interface{}) {
	m.logs = append(m.logs, LogEntry{
		Level:   "WARN",
		Message: fmt.Sprintf(format, args...),
		Time:    time.Now(),
	})
}

// Debug 调试日志
func (m *MockLogger) Debug(args ...interface{}) {
	m.logs = append(m.logs, LogEntry{
		Level:   "DEBUG",
		Message: fmt.Sprint(args...),
		Time:    time.Now(),
	})
}

// Debugf 格式化调试日志
func (m *MockLogger) Debugf(format string, args ...interface{}) {
	m.logs = append(m.logs, LogEntry{
		Level:   "DEBUG",
		Message: fmt.Sprintf(format, args...),
		Time:    time.Now(),
	})
}

// GetLogs 获取日志
func (m *MockLogger) GetLogs() []LogEntry {
	return m.logs
}

// HasLog 检查是否包含特定日志
func (m *MockLogger) HasLog(level, message string) bool {
	for _, log := range m.logs {
		if log.Level == level && log.Message == message {
			return true
		}
	}
	return false
}

// Clear 清除日志
func (m *MockLogger) Clear() {
	m.logs = m.logs[:0]
}
