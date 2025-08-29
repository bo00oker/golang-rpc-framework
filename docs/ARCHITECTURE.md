# 系统架构文档

## 概述

本文档详细描述了RPC微服务框架的整体架构设计，包括系统组件、交互模式、技术选型等。

## 架构概览

### 系统架构图

```
                    ┌─────────────────────────────────┐
                    │        Client Applications      │
                    │    (Web, Mobile, Third-party)   │
                    └──────────────┬──────────────────┘
                                   │ HTTP/HTTPS
                                   ▼
    ┌─────────────────────────────────────────────────────────────┐
    │                  API Gateway Layer                          │
    │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐           │
    │  │HTTP Router  │ │Middleware   │ │Auth & RBAC  │           │
    │  │- RESTful    │ │- CORS       │ │- JWT Token  │           │
    │  │- Load Bal   │ │- Logging    │ │- Permissions│           │
    │  │- Rate Limit │ │- Metrics    │ │- Validation │           │
    │  └─────────────┘ └─────────────┘ └─────────────┘           │
    │                                                             │
    │  ┌─────────────────────────────────────────────────────────┐ │
    │  │              RPC Client Manager                        │ │
    │  │  ┌──────────┐ ┌──────────┐ ┌─────────────────────────┐ │ │
    │  │  │ Service  │ │ Load     │ │ Connection Pool         │ │ │
    │  │  │Discovery │ │Balancer  │ │ - gRPC Connections      │ │ │
    │  │  │ (Nacos)  │ │(Round    │ │ - Health Checks         │ │ │
    │  │  │          │ │ Robin)   │ │ - Circuit Breaker       │ │ │
    │  │  └──────────┘ └──────────┘ └─────────────────────────┘ │ │
    │  └─────────────────────────────────────────────────────────┘ │
    └──────────────────────┬──────────────────────────────────────┘
                           │ gRPC
                           ▼
    ┌─────────────────────────────────────────────────────────────┐
    │                 Microservices Layer                         │
    │  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐     │
    │  │User Service │    │Order Service│    │Product Svc  │     │
    │  │Port: 50051  │    │Port: 50052  │    │Port: 50053  │     │
    │  │             │    │             │    │             │     │
    │  │Handler      │    │Handler      │    │Handler      │     │
    │  │Service      │    │Service      │    │Service      │     │
    │  │Repository   │    │Repository   │    │Repository   │     │
    │  └─────────────┘    └─────────────┘    └─────────────┘     │
    └─────────────────────────────────────────────────────────────┘
                           │
                           ▼
    ┌─────────────────────────────────────────────────────────────┐
    │                 Infrastructure Layer                        │
    │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐          │
    │  │Service Reg  │ │Monitoring   │ │Configuration│          │
    │  │  - Nacos    │ │- Prometheus │ │- YAML Files │          │
    │  │  - Etcd     │ │- Grafana    │ │- Env Vars   │          │
    │  │             │ │- Jaeger     │ │- Hot Reload │          │
    │  └─────────────┘ └─────────────┘ └─────────────┘          │
    │                                                             │
    │  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐          │
    │  │Database     │ │Cache        │ │Message Queue│          │
    │  │- MySQL      │ │- Redis      │ │- NATS       │          │
    │  │- PostgreSQL │ │- Memory     │ │- RabbitMQ   │          │
    │  └─────────────┘ └─────────────┘ └─────────────┘          │
    └─────────────────────────────────────────────────────────────┘
```

## 核心组件

### 1. API Gateway

**职责**: 作为系统的统一入口点，处理所有外部请求

**核心功能**:
- HTTP到gRPC协议转换
- 请求路由和负载均衡
- 认证和授权
- 限流和熔断
- 监控和日志

**技术实现**:
```go
type Gateway struct {
    server      *http.Server          // HTTP服务器
    router      *mux.Router          // 路由器
    clientMgr   *ClientManager       // RPC客户端管理器
    authService *security.AuthService // 认证服务
    metrics     *metrics.Metrics     // 监控指标
}
```

### 2. 微服务层

**设计模式**: 分层架构 + DDD（领域驱动设计）

**服务结构**:
```
Service/
├── handler/        # gRPC处理器（接口层）
├── service/        # 业务逻辑层
├── repository/     # 数据访问层
└── model/         # 数据模型
```

**交互模式**:
```
gRPC Request → Handler → Service → Repository → Database
                ↓
           Response ← Format ← Process ← Query Result
```

### 3. 服务注册发现

**架构模式**: 客户端发现模式

**工作流程**:
1. 服务启动时向注册中心注册
2. 客户端从注册中心获取服务列表
3. 客户端直接调用服务实例
4. 定期健康检查和服务更新

**技术选型**:
- **Nacos**: 阿里云开源的服务发现和配置管理平台
- **Etcd**: 分布式键值存储，Kubernetes生态

### 4. 负载均衡

**策略**:
- **轮询（Round Robin）**: 默认策略，请求均匀分发
- **权重轮询**: 根据服务实例权重分发
- **最少连接**: 选择连接数最少的实例

**实现**:
```go
type LoadBalancer interface {
    Next() string                    // 获取下一个服务实例
    UpdateAddresses([]string)        // 更新服务地址列表
    GetHealthyAddresses() []string   // 获取健康的服务实例
}
```

## 设计原则

### 1. 单一职责原则

每个微服务专注于单一的业务领域：
- **User Service**: 用户管理
- **Order Service**: 订单处理  
- **Product Service**: 商品管理

### 2. 接口隔离原则

通过Protocol Buffers定义清晰的服务接口：
```protobuf
service UserService {
  rpc CreateUser(CreateUserRequest) returns (CreateUserResponse);
  rpc GetUser(GetUserRequest) returns (GetUserResponse);
  rpc UpdateUser(UpdateUserRequest) returns (UpdateUserResponse);
}
```

### 3. 依赖倒置原则

通过接口定义依赖关系：
```go
type UserRepository interface {
    Create(ctx context.Context, user *model.User) error
    GetByID(ctx context.Context, id int64) (*model.User, error)
}

type UserService struct {
    repo UserRepository  // 依赖接口而非具体实现
}
```

### 4. 开闭原则

通过中间件机制实现功能扩展：
```go
func (g *Gateway) Use(middleware func(http.Handler) http.Handler) {
    g.middlewares = append(g.middlewares, middleware)
}
```

## 数据流向

### 1. 请求处理流程

```
Client Request
     ↓
[API Gateway]
     ├─ CORS Middleware
     ├─ Logging Middleware  
     ├─ Metrics Middleware
     ├─ Rate Limit Middleware
     ├─ Auth Middleware
     └─ Router Handler
            ↓
[Service Discovery & Load Balancing]
     ├─ Get Service Instances
     ├─ Select Healthy Instance
     └─ Create gRPC Connection
            ↓
[gRPC Service Call]
     ├─ Handler Layer
     ├─ Service Layer
     ├─ Repository Layer
     └─ Database/Cache
            ↓
[Response Processing]
     ├─ gRPC Response
     ├─ HTTP Response Conversion
     ├─ Metrics Recording
     └─ Logging
            ↓
Client Response
```

### 2. 服务注册流程

```
Service Startup
     ↓
[Load Configuration]
     ├─ Service Info (Name, Port, Version)
     ├─ Registry Config (Nacos/Etcd)
     └─ Health Check Settings
     ↓
[Create Registry Client]
     ├─ Connect to Registry
     ├─ Authenticate if Required
     └─ Verify Connection
     ↓
[Register Service]
     ├─ Submit Service Info
     ├─ Set TTL and Metadata
     └─ Start Health Check
     ↓
[Maintain Registration]
     ├─ Periodic Health Reports
     ├─ Handle Registry Events
     └─ Graceful Deregistration
```

### 3. 监控数据流

```
Service Operation
     ↓
[Metrics Collection]
     ├─ HTTP Request Metrics
     ├─ gRPC Call Metrics
     ├─ System Resource Metrics
     └─ Business Metrics
     ↓
[Metrics Aggregation]
     ├─ Prometheus Format
     ├─ Time Series Data
     └─ Labels and Tags
     ↓
[Export and Storage]
     ├─ Prometheus Server
     ├─ Grafana Dashboard
     └─ Alert Manager
     ↓
[Visualization & Alerting]
```

## 安全架构

### 1. 认证机制

**JWT (JSON Web Token)**:
```
Client Login
     ↓
[Validate Credentials]
     ↓
[Generate JWT Token]
     ├─ Header: {"alg": "HS256", "typ": "JWT"}
     ├─ Payload: {"user_id": 123, "role": "user", "exp": 1629876543}
     └─ Signature: HMACSHA256(base64UrlEncode(header) + "." + base64UrlEncode(payload), secret)
     ↓
[Return Token to Client]
```

**Token验证流程**:
```
API Request with Token
     ↓
[Extract Token from Header]
     ↓
[Validate Token Signature]
     ↓
[Check Token Expiration]
     ↓
[Extract User Information]
     ↓
[Proceed with Request]
```

### 2. 授权机制

**RBAC (Role-Based Access Control)**:
```go
type Permission struct {
    Resource string  // 资源
    Action   string  // 操作
}

type Role struct {
    Name        string
    Permissions []Permission
}

type User struct {
    ID    int64
    Roles []Role
}
```

**权限检查流程**:
```
Request with User Context
     ↓
[Extract User Roles]
     ↓
[Get Required Permission]
     ↓
[Check Role Permissions]
     ↓
[Allow/Deny Request]
```

### 3. 网络安全

**TLS终止**:
- API Gateway处理TLS终止
- 内部服务间通信可使用明文（受信任网络）
- 生产环境建议全链路TLS

**网络隔离**:
```
Internet → [Load Balancer + TLS] → [API Gateway] → [Service Mesh] → [Microservices]
                    ↓                      ↓              ↓
                [WAF Filter]          [Rate Limit]   [Circuit Breaker]
```

## 性能设计

### 1. 连接管理

**连接池策略**:
```go
type ConnectionPool struct {
    maxConnections int           // 最大连接数
    maxIdleConns   int          // 最大空闲连接数
    connTimeout    time.Duration // 连接超时
    idleTimeout    time.Duration // 空闲超时
}
```

**连接复用**:
- HTTP/2多路复用
- gRPC连接复用
- Keep-Alive机制

### 2. 缓存策略

**多级缓存**:
```
Client Request
     ↓
[Gateway Cache] (HTTP Response Cache)
     ↓ (Cache Miss)
[Service Cache] (Business Data Cache)
     ↓ (Cache Miss)
[Database Query]
```

**缓存更新策略**:
- **Write-Through**: 写入时同步更新缓存
- **Write-Behind**: 异步更新缓存
- **Cache-Aside**: 应用层管理缓存

### 3. 异步处理

**异步任务模式**:
```go
type AsyncProcessor struct {
    workers  int                    // 工作协程数
    queue    chan AsyncTask         // 任务队列
    handlers map[string]AsyncHandler // 任务处理器
}
```

**使用场景**:
- 邮件发送
- 消息推送
- 数据同步
- 报表生成

## 可观测性

### 1. 监控指标

**四个黄金信号**:
- **延迟（Latency）**: 请求处理时间
- **流量（Traffic）**: 请求速率
- **错误（Errors）**: 错误率
- **饱和度（Saturation）**: 资源使用率

**指标分类**:
```
业务指标:
├── 用户注册数
├── 订单创建数
└── 商品浏览数

技术指标:
├── HTTP请求数/延迟/错误率
├── gRPC调用数/延迟/错误率
├── 数据库连接数/查询时间
└── 缓存命中率

基础设施指标:
├── CPU使用率
├── 内存使用率
├── 磁盘I/O
└── 网络I/O
```

### 2. 分布式追踪

**追踪架构**:
```
Request → [Gateway] → [Service A] → [Service B] → [Database]
   ↓          ↓           ↓           ↓            ↓
[Span 1]  [Span 2]   [Span 3]   [Span 4]    [Span 5]
   ↓          ↓           ↓           ↓            ↓
[Jaeger Collector] ← ← ← ← ← ← ← ← ← ← ← ← ← ← ← ←
   ↓
[Jaeger Query] → [Jaeger UI]
```

**追踪信息**:
- **Trace ID**: 整个请求链路的唯一标识
- **Span ID**: 单个操作的唯一标识
- **操作名称**: 具体的操作描述
- **开始/结束时间**: 操作的时间范围
- **标签和日志**: 附加的元数据信息

### 3. 日志管理

**日志级别**:
```
FATAL → 系统无法继续运行
ERROR → 错误但系统可继续运行
WARN  → 警告信息
INFO  → 一般信息
DEBUG → 调试信息
```

**结构化日志**:
```json
{
  "timestamp": "2024-08-29T10:00:00Z",
  "level": "INFO",
  "service": "gateway",
  "trace_id": "abc123def456",
  "message": "User created successfully",
  "fields": {
    "user_id": 123,
    "duration": "50ms",
    "method": "POST",
    "path": "/api/v1/users"
  }
}
```

## 部署架构

### 1. 容器化部署

**Docker镜像构建**:
```dockerfile
# 多阶段构建
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o service ./cmd/service

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/service .
CMD ["./service"]
```

### 2. Kubernetes部署

**资源对象**:
```yaml
# Deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gateway
spec:
  replicas: 3
  selector:
    matchLabels:
      app: gateway
  template:
    spec:
      containers:
      - name: gateway
        image: rpc-framework/gateway:latest
        resources:
          requests:
            cpu: 200m
            memory: 256Mi
          limits:
            cpu: 1000m
            memory: 1Gi

# Service
apiVersion: v1
kind: Service
metadata:
  name: gateway-service
spec:
  selector:
    app: gateway
  ports:
  - port: 80
    targetPort: 8080
  type: LoadBalancer

# ConfigMap
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
data:
  app.yaml: |
    server:
      address: ":8080"
    nacos:
      server_addr: "nacos-service:8848"
```

### 3. 高可用部署

**多实例部署**:
- Gateway: 3个实例（负载均衡）
- User Service: 2个实例（数据一致性）
- Order Service: 2个实例（业务隔离）

**故障转移**:
- 健康检查自动剔除故障实例
- 服务网格提供流量管理
- 数据库主从配置

## 扩展性设计

### 1. 水平扩展

**无状态设计**:
- 服务实例无状态
- 会话信息存储在缓存中
- 数据库连接池管理

**自动扩缩容**:
```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: gateway-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: gateway
  minReplicas: 3
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

### 2. 功能扩展

**插件机制**:
```go
type Plugin interface {
    Name() string
    Init(config map[string]interface{}) error
    Execute(ctx context.Context, req interface{}) (interface{}, error)
}

type PluginManager struct {
    plugins map[string]Plugin
}
```

**API版本管理**:
```go
// v1 API
router.PathPrefix("/api/v1").Subrouter()

// v2 API (新功能)
router.PathPrefix("/api/v2").Subrouter()

// 版本兼容性检查
func (g *Gateway) checkAPIVersion(version string) bool {
    return supportedVersions[version]
}
```

## 技术债务管理

### 1. 代码质量

**静态分析**:
- golangci-lint代码检查
- SonarQube质量扫描
- 单元测试覆盖率监控

**重构策略**:
- 定期代码审查
- 技术债务记录和跟踪
- 渐进式重构

### 2. 性能优化

**性能监控**:
- 定期性能基准测试
- 关键路径性能分析
- 资源使用趋势分析

**优化策略**:
- 数据库查询优化
- 缓存策略调整
- 算法和数据结构优化

本架构文档描述了RPC微服务框架的完整设计思路和实现方案，为系统的维护、扩展和优化提供了全面的技术指导。