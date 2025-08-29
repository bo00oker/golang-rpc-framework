# API Gateway 架构设计

## 🏗️ 整体架构

API Gateway作为微服务架构的统一入口，提供了以下核心功能：

```
┌─────────────────────────────────────────────────────────────┐
│                    Client Applications                      │
│             (Web, Mobile, Third-party APIs)                │
└─────────────────────┬───────────────────────────────────────┘
                      │ HTTP/HTTPS
                      ▼
┌─────────────────────────────────────────────────────────────┐
│                  API Gateway (Port: 8080)                   │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐           │
│  │ HTTP Server │ │ Middleware  │ │ Router      │           │
│  │             │ │ - Auth      │ │ - /api/v1   │           │
│  │ - CORS      │ │ - Logging   │ │ - /health   │           │
│  │ - Rate Lmt  │ │ - Metrics   │ │ - /metrics  │           │
│  └─────────────┘ └─────────────┘ └─────────────┘           │
│                                                             │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │              RPC Client Manager                        │ │
│  │  ┌──────────┐ ┌──────────┐ ┌─────────────────────────┐ │ │
│  │  │ Service  │ │ Load     │ │ Connection Pool         │ │ │
│  │  │Discovery │ │Balancing │ │ - Health Check          │ │ │
│  │  │ (Nacos)  │ │(Round    │ │ - Auto Reconnect        │ │ │
│  │  │          │ │ Robin)   │ │ - Circuit Breaker       │ │ │
│  │  └──────────┘ └──────────┘ └─────────────────────────┘ │ │
│  └─────────────────────────────────────────────────────────┘ │
└─────────────────────┬───────────────────────────────────────┘
                      │ gRPC
                      ▼
┌─────────────────────────────────────────────────────────────┐
│                Backend Microservices                        │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐     │
│  │User Service │    │Order Service│    │  ... More   │     │
│  │Port: 50051  │    │Port: 50052  │    │  Services   │     │
│  │             │    │             │    │             │     │
│  │- CRUD Users │    │- CRUD Orders│    │- Inventory  │     │
│  │- Profile Mgmt│   │- Order Items│    │- Payment    │     │
│  │- Auth       │    │- Status Mgmt│    │- Notification│     │
│  └─────────────┘    └─────────────┘    └─────────────┘     │
└─────────────────────────────────────────────────────────────┘
```

## 🔧 核心组件

### 1. HTTP Server
- **框架**: Gorilla Mux
- **功能**: 处理HTTP请求，支持RESTful API
- **特性**: 
  - 支持路径参数和查询参数
  - 自动内容协商
  - 优雅关闭

### 2. 中间件系统
```go
// 中间件执行顺序
Request → CORS → Logging → Metrics → RateLimit → Auth → Handler
```

#### CORS中间件
- 跨域资源共享支持
- 支持预检请求(OPTIONS)
- 可配置允许的域名、方法、头部

#### 认证中间件
- JWT Token验证
- Bearer Token格式
- 用户信息注入Context

#### 日志中间件
- 结构化日志记录
- 请求响应时间统计
- 错误链路追踪

#### 指标中间件
- Prometheus指标收集
- HTTP请求计数
- 响应时间分布
- 错误率统计

#### 限流中间件
- 基于IP的限流
- 滑动窗口算法
- 可配置限流阈值

### 3. 路由系统

#### RESTful API设计
```
/api/v1/auth/
├── POST /login          # 用户登录
├── POST /refresh        # 刷新Token
└── POST /logout         # 用户登出

/api/v1/users/
├── POST /               # 创建用户
├── GET /                # 查询用户列表
├── GET /{id}            # 获取用户详情
├── PUT /{id}            # 更新用户信息
└── DELETE /{id}         # 删除用户

/api/v1/orders/
├── POST /               # 创建订单
├── GET /                # 查询订单列表
├── GET /{id}            # 获取订单详情
├── PUT /{id}            # 更新订单信息
├── DELETE /{id}         # 删除订单
└── GET /user/{user_id}  # 根据用户查询订单

系统接口:
├── GET /health          # 健康检查
└── GET /metrics         # Prometheus指标
```

### 4. RPC客户端管理器

#### 服务发现
- 支持Nacos和Etcd注册中心
- 自动服务实例发现
- 实时监听服务变化
- 故障实例自动剔除

#### 负载均衡
- 轮询(Round Robin)算法
- 服务实例健康检查
- 故障转移机制
- 连接复用

#### 连接池管理
```go
type ServiceClient struct {
    ServiceName string           // 服务名称
    Connections []*grpc.ClientConn  // gRPC连接池
    Clients     []interface{}    // 客户端实例池
    CurrentIdx  int             // 当前轮询索引
    healthy     bool            // 健康状态
}
```

## 🔐 安全特性

### JWT认证
- RSA 2048位密钥对
- 访问令牌(1小时过期)
- 刷新令牌(24小时过期)
- 自动令牌刷新机制

### RBAC权限控制
```go
// 权限模型
admin: 所有权限
user:  用户和订单的基本操作
guest: 只读权限

// 权限映射
"user:create" "user:read" "user:update" "user:delete"
"order:create" "order:read" "order:update" "order:delete"
```

### 白名单机制
```go
// 无需认证的接口
"/api/v1/users"               // POST - 用户注册
"/api/v1/auth/login"          // 登录
"/api/v1/auth/refresh"        // 刷新令牌
"/health"                     // 健康检查
```

## 📊 监控可观测性

### Prometheus指标
```
# HTTP请求指标
gateway_http_requests_total{method, path, status}
gateway_http_request_duration_seconds{method, path}
gateway_http_requests_in_flight{method, path}

# RPC客户端指标
gateway_rpc_requests_total{service, method, status}
gateway_rpc_request_duration_seconds{service, method}
gateway_rpc_connections_active{service}

# 业务指标
gateway_rate_limit_hits_total{source_ip}
gateway_auth_failures_total{reason}
```

### 健康检查
```json
{
  "status": "healthy",
  "checks": {
    "gateway": "healthy",
    "rpc_clients": "healthy",
    "user_service": "healthy",
    "order_service": "healthy"
  }
}
```

### 分布式追踪
- OpenTelemetry集成
- Jaeger链路追踪
- 跨服务调用链
- 性能瓶颈分析

## 🚀 部署架构

### 单机部署
```bash
# 启动所有服务
./scripts/start_with_gateway.sh start

# 启动顺序
1. User Service (Port: 50051)
2. Order Service (Port: 50052)  
3. Gateway (Port: 8080)
```

### 容器化部署
```dockerfile
# Gateway Dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o gateway ./cmd/gateway

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/gateway .
COPY ./configs ./configs
CMD ["./gateway"]
```

### Kubernetes部署
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api-gateway
spec:
  replicas: 3
  selector:
    matchLabels:
      app: api-gateway
  template:
    metadata:
      labels:
        app: api-gateway
    spec:
      containers:
      - name: gateway
        image: rpc-framework/gateway:latest
        ports:
        - containerPort: 8080
        env:
        - name: NACOS_SERVER_ADDR
          value: "nacos-service:8848"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
```

## 🔄 API请求流程

### 典型请求流程
```
1. 客户端发送HTTP请求到Gateway
   ↓
2. CORS中间件处理跨域
   ↓
3. 日志中间件记录请求
   ↓
4. 指标中间件统计
   ↓
5. 限流中间件检查频率
   ↓
6. 认证中间件验证Token(如需要)
   ↓
7. 路由器匹配处理器
   ↓
8. RPC客户端管理器选择后端服务
   ↓
9. 负载均衡选择服务实例
   ↓
10. 发送gRPC请求到后端服务
    ↓
11. 后端服务处理业务逻辑
    ↓
12. 返回gRPC响应
    ↓
13. Gateway转换为HTTP响应
    ↓
14. 返回给客户端
```

### 错误处理流程
```
gRPC错误 → HTTP状态码映射 → 统一错误格式
├── INVALID_ARGUMENT → 400 Bad Request
├── UNAUTHENTICATED  → 401 Unauthorized  
├── PERMISSION_DENIED → 403 Forbidden
├── NOT_FOUND        → 404 Not Found
├── ALREADY_EXISTS   → 409 Conflict
├── INTERNAL         → 500 Internal Server Error
└── UNAVAILABLE      → 503 Service Unavailable
```

## 📈 性能优化

### 连接复用
- gRPC连接池管理
- HTTP Keep-Alive
- 连接健康检查
- 自动重连机制

### 缓存策略
- 服务发现结果缓存
- JWT Token本地验证
- 静态资源缓存
- 响应内容缓存

### 并发控制
- Goroutine池管理
- 请求并发限制
- 资源竞争避免
- 优雅关闭

## 🔧 配置管理

### Gateway配置
```yaml
gateway:
  port: 8080                    # 监听端口
  read_timeout: 30s             # 读取超时
  write_timeout: 30s            # 写入超时
  enable_cors: true             # 启用CORS
  enable_auth: true             # 启用认证
  enable_metrics: true          # 启用指标
  enable_rate_limit: true       # 启用限流
  rate_limit: 1000              # 每分钟请求限制

security:
  jwt_secret: "your-secret"     # JWT密钥
  token_expiry: 1h              # Token过期时间
  refresh_expiry: 24h           # 刷新Token过期时间

registry:
  type: "nacos"                 # 注册中心类型
```

## 🧪 测试策略

### API测试
```bash
# 运行Gateway API测试
./scripts/test_gateway.sh test

# 测试覆盖
- 健康检查
- 用户认证
- CRUD操作
- 错误处理
- 权限控制
```

### 性能测试
- 并发用户测试
- 压力测试
- 内存泄漏检测
- 响应时间分析

### 集成测试
- 端到端测试
- 服务间通信测试
- 故障恢复测试
- 数据一致性测试

## 🎯 未来扩展

### 高级特性
- [ ] API版本管理
- [ ] 请求/响应转换
- [ ] WebSocket支持
- [ ] GraphQL集成
- [ ] 服务网格集成(Istio)

### 运维特性
- [ ] 蓝绿部署
- [ ] 金丝雀发布
- [ ] 自动扩缩容
- [ ] 智能路由
- [ ] A/B测试

### 安全增强
- [ ] OAuth2集成
- [ ] API密钥管理
- [ ] WAF防护
- [ ] DDoS防护
- [ ] 数据加密

这个Gateway架构提供了完整的微服务入口解决方案，具备生产级的可靠性、安全性和可观测性。