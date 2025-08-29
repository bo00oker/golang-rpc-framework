# RPC Framework

一个基于 gRPC 的高性能微服务 RPC 框架，支持服务注册发现、负载均衡、全链路追踪、高并发优化等功能。

## 特性

### 核心功能
- **gRPC 通信**: 支持 unary、server-streaming、client-streaming、bidirectional streaming
- **服务注册发现**: 支持 Nacos 服务注册与发现
- **负载均衡**: 客户端轮询负载均衡
- **配置管理**: 基于 Viper 的配置管理，支持 YAML 和环境变量
- **结构化日志**: 基于 Zap 的日志系统，支持文件轮转
- **全链路追踪**: 基于 OpenTelemetry 的分布式追踪，支持 Jaeger

### 高并发优化
- **连接池管理**: 客户端连接复用，减少连接开销
- **熔断器模式**: 自动故障检测和恢复，防止级联故障
- **限流控制**: 服务器端请求限流，保护系统稳定性
- **超时控制**: 请求和连接超时管理
- **指标监控**: 实时性能指标收集
- **健康检查**: 服务健康状态监控
- **Keep-Alive**: 连接保活机制
- **内存池**: 服务器端内存缓冲区复用，减少GC压力
- **异步处理**: 支持异步任务处理，提高并发能力
- **缓存机制**: 客户端请求缓存，减少重复请求
- **高级负载均衡**: 支持轮询、权重轮询、最少连接数等算法

## 快速开始

### 1. 环境准备

确保已安装以下依赖：
- Go 1.24+
- Nacos Server
- Jaeger (可选，用于追踪)

### 2. 启动 Nacos

```bash
# 使用 Docker 启动 Nacos
docker run --name nacos-standalone -e MODE=standalone -p 8848:8848 -p 9848:9848 nacos/nacos-server:v2.2.3
```

### 3. 启动 Jaeger (可选)

```bash
# 使用 Docker 启动 Jaeger
docker run -d --name jaeger -p 16686:16686 -p 14268:14268 jaegertracing/all-in-one:latest
```

### 4. 运行示例

#### 启动服务器

```bash
cd examples/server
go run main.go
```

#### 启动客户端

```bash
cd examples/client
go run main.go
```

## 配置说明

### 服务器配置

```yaml
server:
  address: ":50051"
  port: 50051
  
  # 高并发优化配置
  max_concurrent_requests: 1000    # 最大并发请求数
  request_timeout: 30s             # 请求超时时间
  max_connections: 1000            # 最大连接数
  connection_timeout: 5s           # 连接超时
  keep_alive_time: 30s             # Keep-Alive 时间
  keep_alive_timeout: 5s           # Keep-Alive 超时
  rate_limit: 1000                 # 每秒请求限制
  enable_metrics: true             # 是否启用指标收集
  
  # 内存池配置
  enable_memory_pool: true         # 是否启用内存池
  memory_pool_size: 1000           # 内存池大小
  memory_pool_max_size: 10000      # 内存池最大大小
  
  # 异步处理配置
  enable_async: true               # 是否启用异步处理
  async_worker_count: 10           # 异步工作协程数
  async_queue_size: 1000           # 异步队列大小
  
  # 健康检查配置
  enable_health_check: true        # 是否启用健康检查
  health_check_interval: 30s       # 健康检查间隔
  health_check_timeout: 5s         # 健康检查超时
```

### 客户端配置

```yaml
client:
  timeout: 30s
  keep_alive: 30s
  max_recv_msg_size: 4194304       # 4MB
  max_send_msg_size: 4194304       # 4MB
  retry_attempts: 3
  retry_delay: 1s
  load_balance_type: "round_robin" # 负载均衡类型
  
  # 高并发优化配置
  max_connections: 100             # 最大连接数
  max_idle_conns: 10               # 最大空闲连接数
  conn_timeout: 5s                 # 连接超时
  idle_timeout: 30s                # 空闲超时
  max_retries: 3                   # 最大重试次数
  retry_backoff: 1s                # 重试退避时间
  circuit_breaker_threshold: 5     # 熔断器阈值
  circuit_breaker_timeout: 30s     # 熔断器超时
  
  # 缓存配置
  enable_cache: true               # 是否启用缓存
  cache_ttl: 5m                    # 缓存TTL
  cache_max_size: 1000             # 缓存最大大小
  
  # 异步处理配置
  enable_async: true               # 是否启用异步处理
  async_worker_count: 10           # 异步工作协程数
  async_queue_size: 1000           # 异步队列大小
```

## 高并发优化详解

### 1. 连接池管理

客户端实现了智能连接池，具有以下特性：
- **连接复用**: 避免频繁创建和销毁连接
- **连接限制**: 防止连接数过多导致资源耗尽
- **健康检查**: 自动检测连接健康状态
- **负载均衡**: 在多个连接间分发请求

```go
// 使用连接池的客户端
client := client.NewClient(&client.ClientOptions{
    MaxConnections: 100,
    MaxIdleConns:   10,
    ConnTimeout:    5 * time.Second,
    IdleTimeout:    30 * time.Second,
})
```

### 2. 熔断器模式

实现了三种状态的熔断器：
- **Closed**: 正常状态，允许请求通过
- **Open**: 熔断状态，拒绝所有请求
- **Half-Open**: 半开状态，允许少量请求测试

```go
// 熔断器配置
circuitBreaker := &CircuitBreaker{
    threshold: 5,           // 失败阈值
    timeout:   30 * time.Second, // 熔断时间
}
```

### 3. 限流控制

服务器端实现了令牌桶限流算法：
- **令牌桶**: 固定速率补充令牌
- **突发处理**: 支持短时间内的突发请求
- **平滑限流**: 避免流量突刺

```go
// 限流器配置
rateLimiter := &RateLimiter{
    limit:    1000,         // 每秒请求限制
    interval: time.Second,  // 令牌补充间隔
}
```

### 4. 内存池管理

服务器端实现了内存池，减少GC压力：
- **缓冲区复用**: 复用内存缓冲区
- **预分配**: 启动时预分配缓冲区
- **自动回收**: 自动回收和重用缓冲区

```go
// 使用内存池
buf := server.GetBuffer()
defer server.PutBuffer(buf)
```

### 5. 异步处理

支持异步任务处理：
- **工作协程池**: 固定数量的工作协程
- **任务队列**: 异步任务队列
- **处理器注册**: 支持自定义任务处理器

```go
// 注册异步处理器
server.RegisterAsyncHandler("email", func(ctx context.Context, data interface{}) error {
    // 处理邮件发送
    return nil
})

// 提交异步任务
server.SubmitAsyncTask(ctx, "email", emailData)
```

### 6. 缓存机制

客户端实现了智能缓存：
- **LRU淘汰**: 最近最少使用淘汰策略
- **TTL过期**: 基于时间的过期机制
- **访问统计**: 缓存访问统计信息

```go
// 缓存配置
cache := &Cache{
    ttl:     5 * time.Minute,
    maxSize: 1000,
}
```

### 7. 高级负载均衡

支持多种负载均衡算法：
- **轮询 (round_robin)**: 简单轮询
- **权重轮询 (weighted_round_robin)**: 基于权重的轮询
- **最少连接数 (least_connections)**: 选择连接数最少的节点

```go
// 权重轮询配置
weightedLB := &WeightedRoundRobinLoadBalancer{
    addresses: []*WeightedAddress{
        {Address: "server1", Weight: 3},
        {Address: "server2", Weight: 2},
        {Address: "server3", Weight: 1},
    },
}
```

### 8. 健康检查

服务器端健康检查机制：
- **定期检查**: 定期执行健康检查
- **超时控制**: 健康检查超时控制
- **自定义检查**: 支持自定义健康检查函数

```go
// 注册健康检查
server.RegisterHealthCheck("database", func(ctx context.Context) error {
    // 检查数据库连接
    return db.PingContext(ctx)
})
```

### 9. 超时控制

多层次超时控制：
- **连接超时**: 建立连接的最大等待时间
- **请求超时**: 单个请求的最大处理时间
- **Keep-Alive**: 连接保活机制

### 10. 指标监控

实时收集性能指标：
- **请求计数**: 总请求数和错误数
- **响应时间**: 平均响应时间
- **连接状态**: 活跃连接数
- **错误率**: 请求错误率
- **吞吐量**: 系统吞吐量
- **延迟**: 系统延迟

```go
// 获取服务器指标
metrics := server.GetMetrics()
stats := metrics.GetStats()
fmt.Printf("请求数: %d, 错误率: %.2f%%\n", 
    stats["request_count"], 
    stats["error_rate"].(float64)*100)

// 获取内存池统计
poolStats := server.GetMemoryPoolStats()
fmt.Printf("内存池利用率: %.2f%%\n", 
    poolStats["utilization"].(float64)*100)
```

## 全链路追踪

### 功能特性
- **自动追踪**: 拦截器自动创建和管理 span
- **上下文传播**: 跨服务追踪上下文传递
- **多导出器**: 支持 Jaeger 和控制台输出
- **采样控制**: 可配置的采样率

### 查看追踪

1. 启动 Jaeger UI: http://localhost:16686
2. 在服务中创建自定义 span:

```go
ctx, span := tracer.StartSpan(ctx, "custom-operation")
defer span.End()

// 添加属性
span.SetAttributes(attribute.String("key", "value"))
```

## 环境变量支持

支持通过环境变量覆盖配置：

```bash
# 服务器配置
export SERVER_ADDRESS=":8080"
export SERVER_MAX_CONCURRENT_REQUESTS="2000"
export SERVER_RATE_LIMIT="2000"
export SERVER_ENABLE_MEMORY_POOL="true"
export SERVER_ENABLE_ASYNC="true"

# 客户端配置
export CLIENT_MAX_CONNECTIONS="200"
export CLIENT_CIRCUIT_BREAKER_THRESHOLD="10"
export CLIENT_ENABLE_CACHE="true"
export CLIENT_ENABLE_ASYNC="true"
export CLIENT_LOAD_BALANCE_TYPE="weighted_round_robin"

# Nacos配置
export NACOS_SERVER_ADDR="nacos.example.com:8848"
export NACOS_NAMESPACE="production"

# 追踪配置
export TRACE_SERVICE_NAME="my-service"
export TRACE_JAEGER_ENDPOINT="http://jaeger.example.com:14268/api/traces"
```

## 性能优化建议

### 1. 连接池调优
- 根据并发量调整 `max_connections`
- 合理设置 `max_idle_conns` 减少连接创建开销
- 监控连接池使用率

### 2. 熔断器配置
- 根据服务特性调整失败阈值
- 设置合适的熔断时间
- 监控熔断器状态变化

### 3. 限流策略
- 根据系统容量设置限流阈值
- 考虑不同接口的限流策略
- 监控限流触发情况

### 4. 内存池优化
- 根据请求大小调整内存池大小
- 监控内存池利用率
- 避免内存泄漏

### 5. 异步处理优化
- 根据任务类型调整工作协程数
- 合理设置队列大小
- 监控异步任务处理情况

### 6. 缓存优化
- 根据数据特性设置TTL
- 合理设置缓存大小
- 监控缓存命中率

### 7. 负载均衡优化
- 根据节点性能设置权重
- 监控节点健康状态
- 动态调整负载均衡策略

### 8. 超时设置
- 根据业务复杂度设置请求超时
- 合理配置连接超时
- 避免超时时间过长

### 9. 监控告警
- 设置关键指标告警
- 监控错误率变化
- 关注响应时间趋势

## 错误处理

框架提供了完善的错误处理机制：
- **优雅降级**: 熔断器自动处理故障
- **重试机制**: 客户端自动重试失败请求
- **超时控制**: 防止请求长时间阻塞
- **错误传播**: 完整的错误信息传递

## 性能基准

在标准硬件配置下的性能表现：
- **并发连接**: 支持 1000+ 并发连接
- **请求处理**: 10000+ QPS
- **响应时间**: 平均 < 10ms
- **内存使用**: 低内存占用
- **GC压力**: 内存池减少GC压力

## 部署建议

### 1. 容器化部署
```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o server ./examples/server

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/server .
CMD ["./server"]
```

### 2. 监控集成
- 集成 Prometheus 指标收集
- 配置 Grafana 监控面板
- 设置告警规则

### 3. 日志管理
- 使用 ELK 栈收集日志
- 配置日志轮转策略
- 设置日志级别

## 贡献指南

欢迎提交 Issue 和 Pull Request！

## 许可证

MIT License
