# RPC框架示例项目

本项目展示了如何使用RPC框架构建微服务应用，包含Hello服务、用户服务和订单服务。

## 项目结构

```
examples/
├── services/                    # 独立服务目录
│   ├── user/                   # 用户服务
│   │   └── main.go            # 用户服务主程序
│   └── order/                  # 订单服务
│       └── main.go            # 订单服务主程序
├── client/                     # 客户端测试程序
│   ├── main.go                # 客户端主程序
│   ├── user_client.go         # 用户服务客户端
│   ├── order_client.go        # 订单服务客户端
│   ├── multi_service_client.go # 多服务客户端
│   └── simple_test.go         # 简单测试
└── server/                     # 多服务服务器（已废弃，建议使用独立服务）
    ├── main.go                # 多服务服务器主程序
    ├── user_service.go        # 用户服务实现
    └── order_service.go       # 订单服务实现
```

## 服务说明

### 1. Hello服务
- **功能**: 基础的gRPC服务示例
- **接口**: SayHello, SayHelloStream, SayHelloBidirectional
- **用途**: 演示基本的gRPC调用、流式调用和双向流式调用

### 2. 用户服务 (UserService)
- **功能**: 用户管理服务
- **接口**: 
  - CreateUser: 创建用户
  - GetUser: 查询用户
  - ListUsers: 用户列表（支持分页和关键字搜索）
  - UpdateUser: 更新用户
  - DeleteUser: 删除用户
- **特性**: 
  - 用户名唯一性验证
  - 内存数据存储
  - 分页和搜索功能

### 3. 订单服务 (OrderService)
- **功能**: 订单管理服务
- **接口**:
  - CreateOrder: 创建订单
  - GetOrder: 查询订单
  - ListOrders: 订单列表（支持按状态和用户ID过滤）
  - UpdateOrder: 更新订单
  - DeleteOrder: 删除订单
  - GetOrdersByUser: 根据用户ID查询订单
- **特性**:
  - 自动生成订单号
  - 订单状态管理
  - 支持订单项
  - 按用户查询订单

## 运行方式

### 1. 独立服务模式（推荐）

#### 启动用户服务
```bash
cd examples/services/user
go run main.go
```

#### 启动订单服务
```bash
cd examples/services/order
go run main.go
```

#### 测试用户服务
```bash
cd examples/client
go run . user
```

#### 测试订单服务
```bash
cd examples/client
go run . order
```

### 2. 多服务模式（已废弃）

#### 启动多服务服务器
```bash
cd examples/server
go run .
```

#### 测试多服务
```bash
cd examples/client
go run . multi
```

### 3. 测试Hello服务
```bash
cd examples/client
go run . hello
```

## 配置说明

所有服务都使用 `configs/app.yaml` 配置文件，包含以下配置：

### 服务器配置
```yaml
server:
  address: ":50051"
  port: 50051
  max_recv_msg_size: 4194304
  max_send_msg_size: 4194304
  # 高并发优化配置
  max_concurrent_requests: 1000
  request_timeout: 30s
  max_connections: 1000
  connection_timeout: 5s
  keep_alive_time: 30s
  keep_alive_timeout: 5s
  rate_limit: 1000
  enable_metrics: true
  # 内存池配置
  enable_memory_pool: true
  memory_pool_size: 1000
  memory_pool_max_size: 10000
  # 异步处理配置
  enable_async: true
  async_worker_count: 10
  async_queue_size: 1000
  # 健康检查配置
  enable_health_check: true
  health_check_interval: 30s
  health_check_timeout: 5s
```

### 客户端配置
```yaml
client:
  timeout: 30s
  keep_alive: 30s
  max_recv_msg_size: 4194304
  max_send_msg_size: 4194304
  retry_attempts: 3
  retry_delay: 1s
  load_balance_type: "round_robin"
  # 高并发优化配置
  max_connections: 100
  max_idle_conns: 10
  conn_timeout: 5s
  idle_timeout: 30s
  max_retries: 3
  retry_backoff: 1s
  circuit_breaker_threshold: 5
  circuit_breaker_timeout: 30s
  # 缓存配置
  enable_cache: true
  cache_ttl: 5m
  cache_max_size: 1000
  # 异步处理配置
  enable_async: true
  async_worker_count: 5
  async_queue_size: 100
```

### Nacos配置
```yaml
nacos:
  server_addr: "127.0.0.1:8848"
  namespace: "public"
  group: "DEFAULT_GROUP"
  timeout: 5s
  username: ""
  password: ""
```

### 日志配置
```yaml
log:
  level: "info"
  format: "json"
  output: "file"
  filename: "logs/app.log"
  max_size: 100
  max_backups: 3
  max_age: 28
  compress: true
```

### 追踪配置
```yaml
trace:
  service_name: "rpc-framework"
  service_version: "1.0.0"
  environment: "development"
  jaeger_endpoint: "http://localhost:14268/api/traces"
  sample_rate: 1.0
  enable_console: true
```

## 环境变量支持

支持通过环境变量覆盖配置文件中的设置：

```bash
# 服务器配置
export SERVER_ADDRESS=":50052"
export SERVER_MAX_CONCURRENT_REQUESTS="2000"

# 客户端配置
export CLIENT_TIMEOUT="60s"
export CLIENT_LOAD_BALANCE_TYPE="least_connections"

# Nacos配置
export NACOS_SERVER_ADDR="127.0.0.1:8848"
export NACOS_NAMESPACE="public"

# 日志配置
export LOG_LEVEL="debug"
export LOG_OUTPUT="stdout"

# 追踪配置
export TRACE_SERVICE_NAME="my-service"
export TRACE_ENVIRONMENT="production"
```

## 服务发现

所有服务都会自动注册到Nacos服务注册中心：

- **用户服务**: `user.UserService`
- **订单服务**: `order.OrderService`
- **Hello服务**: `hello.HelloService`

客户端通过服务名称自动发现和连接服务，支持负载均衡和故障转移。

## 监控和追踪

### 1. 日志
- 结构化日志输出
- 支持文件轮转
- 可配置日志级别

### 2. 追踪
- 集成OpenTelemetry
- 支持Jaeger和Console输出
- 全链路追踪

### 3. 指标
- 请求计数
- 响应时间
- 错误率
- 连接数

## 高并发特性

### 1. 连接池
- 客户端连接复用
- 连接健康检查
- 自动重连

### 2. 熔断器
- 故障检测
- 自动熔断和恢复
- 防止级联故障

### 3. 限流
- 服务端限流保护
- 客户端限流控制
- 可配置限流策略

### 4. 缓存
- 客户端缓存
- LRU淘汰策略
- TTL过期机制

### 5. 异步处理
- 异步任务处理
- 工作协程池
- 任务队列

## 部署建议

### 1. 容器化部署
```dockerfile
# 用户服务
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o user-service examples/services/user/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/user-service .
CMD ["./user-service"]
```

### 2. Kubernetes部署
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: user-service
spec:
  replicas: 3
  selector:
    matchLabels:
      app: user-service
  template:
    metadata:
      labels:
        app: user-service
    spec:
      containers:
      - name: user-service
        image: user-service:latest
        ports:
        - containerPort: 50051
        env:
        - name: NACOS_SERVER_ADDR
          value: "nacos-service:8848"
```

### 3. 服务网格集成
- 支持Istio服务网格
- 自动注入Sidecar
- 流量管理和安全策略

## 故障排除

### 1. 服务无法启动
- 检查配置文件路径
- 验证Nacos连接
- 查看日志输出

### 2. 客户端连接失败
- 确认服务已注册到Nacos
- 检查网络连通性
- 验证服务名称

### 3. 性能问题
- 调整连接池大小
- 优化缓存配置
- 监控资源使用

## 扩展开发

### 1. 添加新服务
1. 创建protobuf定义
2. 生成Go代码
3. 实现服务逻辑
4. 创建独立服务目录
5. 编写客户端测试

### 2. 集成数据库
1. 添加数据库配置
2. 实现数据访问层
3. 替换内存存储
4. 添加数据迁移

### 3. 添加认证授权
1. 实现JWT认证
2. 添加权限控制
3. 集成OAuth2
4. 配置安全策略

## 贡献指南

1. Fork项目
2. 创建功能分支
3. 提交代码
4. 创建Pull Request

## 许可证

MIT License

