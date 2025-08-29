package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rpc-framework/core/internal/user/model"
	"github.com/rpc-framework/core/internal/user/repository"
	"github.com/rpc-framework/core/internal/user/service"
	"github.com/rpc-framework/core/pkg/client"
	"github.com/rpc-framework/core/pkg/server"
	"github.com/rpc-framework/core/pkg/trace"
)

func main() {
	fmt.Println("🚀 开始高并发性能测试...")

	// 测试各个优化组件的性能
	testMemoryPool()
	testShardedCache()
	testAtomicMetrics()
	testRateLimiter()
	testConcurrentUserService()

	fmt.Println("✅ 所有高并发性能测试完成！")
}

// 测试内存池性能
func testMemoryPool() {
	fmt.Println("\n📊 测试内存池性能（sync.Pool优化）...")

	// 创建服务器选项（带内存池）
	serverOpts := &server.Options{
		EnableMemoryPool:  true,
		MemoryPoolSize:    10000,
		MemoryPoolMaxSize: 100000,
	}

	// 创建服务器（带内存池）
	srv := server.New(serverOpts)

	start := time.Now()
	concurrency := 1000
	operations := 10000

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				buf := srv.GetBuffer()
				// 模拟使用缓冲区
				copy(buf[:100], "test data")
				srv.PutBuffer(buf)
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)

	totalOps := concurrency * operations
	qps := float64(totalOps) / duration.Seconds()

	fmt.Printf("   内存池操作: %d 次\n", totalOps)
	fmt.Printf("   用时: %v\n", duration)
	fmt.Printf("   QPS: %.0f ops/sec\n", qps)
	fmt.Printf("   每次操作平均用时: %v\n", duration/time.Duration(totalOps))
}

// 测试分片缓存性能
func testShardedCache() {
	fmt.Println("\n📊 测试分片缓存性能（16分片，减少锁竞争）...")

	cache := client.NewShardedCache(5*time.Minute, 10000)

	start := time.Now()
	concurrency := 1000
	operations := 1000

	var wg sync.WaitGroup
	var hits, misses int64

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				key := fmt.Sprintf("key-%d-%d", goroutineID, j)
				value := fmt.Sprintf("value-%d-%d", goroutineID, j)

				// 设置
				cache.Set(key, value)

				// 获取
				if result := cache.Get(key); result != nil {
					atomic.AddInt64(&hits, 1)
				} else {
					atomic.AddInt64(&misses, 1)
				}
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	totalOps := concurrency * operations * 2 // Set + Get
	qps := float64(totalOps) / duration.Seconds()

	stats := cache.GetStats()

	fmt.Printf("   缓存操作: %d 次 (Set + Get)\n", totalOps)
	fmt.Printf("   用时: %v\n", duration)
	fmt.Printf("   QPS: %.0f ops/sec\n", qps)
	fmt.Printf("   命中: %d, 未命中: %d\n", hits, misses)
	fmt.Printf("   缓存统计: %+v\n", stats)
}

// 测试原子操作指标性能
func testAtomicMetrics() {
	fmt.Println("\n📊 测试原子操作指标性能（无锁计数器）...")

	var requestCount, errorCount, activeConnections int64

	start := time.Now()
	concurrency := 1000
	operations := 10000

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				// 模拟请求处理
				atomic.AddInt64(&requestCount, 1)
				atomic.AddInt64(&activeConnections, 1)

				// 模拟处理时间
				time.Sleep(time.Microsecond)

				// 模拟错误
				if j%100 == 0 {
					atomic.AddInt64(&errorCount, 1)
				}

				atomic.AddInt64(&activeConnections, -1)
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)

	totalOps := concurrency * operations
	qps := float64(totalOps) / duration.Seconds()

	fmt.Printf("   指标更新操作: %d 次\n", totalOps)
	fmt.Printf("   用时: %v\n", duration)
	fmt.Printf("   QPS: %.0f ops/sec\n", qps)
	fmt.Printf("   最终计数 - 请求: %d, 错误: %d, 活跃连接: %d\n",
		atomic.LoadInt64(&requestCount),
		atomic.LoadInt64(&errorCount),
		atomic.LoadInt64(&activeConnections))
}

// 测试令牌桶限流器性能
func testRateLimiter() {
	fmt.Println("\n📊 测试令牌桶限流器性能（优化算法）...")

	// 这里我们可以测试Gateway的限流器性能
	// 为简化，我们创建一个模拟的令牌桶限流器
	type TokenBucket struct {
		mu     sync.Mutex
		tokens float64
		limit  int
		last   time.Time
	}

	limiter := &TokenBucket{
		tokens: 1000,
		limit:  1000,
		last:   time.Now(),
	}

	allow := func() bool {
		limiter.mu.Lock()
		defer limiter.mu.Unlock()

		now := time.Now()
		elapsed := now.Sub(limiter.last).Seconds()
		limiter.tokens += elapsed * float64(limiter.limit)
		if limiter.tokens > float64(limiter.limit) {
			limiter.tokens = float64(limiter.limit)
		}
		limiter.last = now

		if limiter.tokens >= 1.0 {
			limiter.tokens -= 1.0
			return true
		}
		return false
	}

	start := time.Now()
	concurrency := 100
	operations := 1000

	var wg sync.WaitGroup
	var allowed, denied int64

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				if allow() {
					atomic.AddInt64(&allowed, 1)
				} else {
					atomic.AddInt64(&denied, 1)
				}
				// 模拟请求间隔
				time.Sleep(time.Microsecond * 10)
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)

	totalRequests := concurrency * operations
	qps := float64(totalRequests) / duration.Seconds()

	fmt.Printf("   限流检查: %d 次\n", totalRequests)
	fmt.Printf("   用时: %v\n", duration)
	fmt.Printf("   QPS: %.0f checks/sec\n", qps)
	fmt.Printf("   允许: %d, 拒绝: %d\n", allowed, denied)
	fmt.Printf("   通过率: %.2f%%\n", float64(allowed)/float64(totalRequests)*100)
}

// 测试并发用户服务性能
func testConcurrentUserService() {
	fmt.Println("\n📊 测试并发用户服务性能（完整业务流程）...")

	// 创建服务
	userRepo := repository.NewMemoryUserRepository()
	tracer, _ := trace.NewTracer(trace.DefaultConfig())
	userService := service.NewUserService(userRepo, tracer)

	ctx := context.Background()
	start := time.Now()
	concurrency := 100
	operations := 100

	var wg sync.WaitGroup
	var successCount, errorCount int64

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				req := &model.CreateUserRequest{
					Username: fmt.Sprintf("user-%d-%d", goroutineID, j),
					Email:    fmt.Sprintf("user-%d-%d@example.com", goroutineID, j),
					Phone:    "13800138000",
					Age:      25,
				}

				if _, err := userService.CreateUser(ctx, req); err != nil {
					atomic.AddInt64(&errorCount, 1)
				} else {
					atomic.AddInt64(&successCount, 1)
				}
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	totalOps := concurrency * operations
	qps := float64(successCount) / duration.Seconds()

	fmt.Printf("   并发用户创建: %d 次\n", totalOps)
	fmt.Printf("   用时: %v\n", duration)
	fmt.Printf("   成功: %d, 失败: %d\n", successCount, errorCount)
	fmt.Printf("   成功率: %.2f%%\n", float64(successCount)/float64(totalOps)*100)
	fmt.Printf("   有效QPS: %.0f creates/sec\n", qps)
	fmt.Printf("   每次操作平均用时: %v\n", duration/time.Duration(successCount))
}
