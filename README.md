# RPC微服务框架 - 企业级解决方案

[![Go Version](https://img.shields.io/badge/Go-1.21%2B-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Build Status](https://img.shields.io/badge/Build-Passing-brightgreen.svg)](#)

## 🚀 项目概述

这是一个基于Go语言开发的高性能、可扩展的企业级RPC微服务框架。该框架提供了完整的微服务解决方案，包括服务注册发现、负载均衡、API网关、认证授权、分布式追踪、监控指标等核心功能。

### ✨ 核心特性

- **🌐 API Gateway**: 统一入口点，支持HTTP到gRPC协议转换
- **⚖️ 负载均衡**: 多种负载均衡算法（轮询、权重、最少连接）
- **🔍 服务发现**: 支持Nacos和Etcd注册中心
- **🔐 安全认证**: JWT Token认证 + RBAC权限控制
- **📊 监控指标**: Prometheus指标收集和健康检查
- **🔍 分布式追踪**: Jaeger链路追踪支持
- **🚦 限流熔断**: 内置限流器和熔断器
- **📦 连接池**: 高效的gRPC连接池管理
- **🎯 中间件**: 可插拔的中间件架构

## 📋 系统架构

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   前端应用      │────│   API Gateway   │────│   微服务集群    │
│                 │    │                 │    │                 │
│ Web/Mobile/API  │    │ ✓ 路由转发      │    │ ✓ User Service  │
│                 │    │ ✓ 认证授权      │    │ ✓ Order Service │
│                 │    │ ✓ 限流熔断      │    │ ✓ 其他服务      │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                ↓
                       ┌─────────────────┐
                       │   基础设施      │
                       │                 │
                       │ ✓ 服务注册发现  │
                       │ ✓ 配置管理      │
                       │ ✓ 监控告警      │
                       │ ✓ 链路追踪      │
                       └─────────────────┘
```

## 🏗️ 项目结构

```
rpc2/
├── cmd/                    # 主程序入口
│   ├── gateway/            # API网关
│   ├── user-service/       # 用户服务
│   ├── order-service/      # 订单服务
│   └── client/            # 测试客户端
├── pkg/                    # 核心包
│   ├── gateway/           # 网关核心逻辑
│   ├── server/            # 服务器框架
│   ├── client/            # 客户端框架
│   ├── registry/          # 服务注册发现
│   ├── config/            # 配置管理
│   ├── logger/            # 日志框架
│   ├── metrics/           # 监控指标
│   ├── security/          # 安全认证
│   └── trace/             # 分布式追踪
├── internal/               # 业务逻辑
│   ├── user/              # 用户服务实现
│   └── order/             # 订单服务实现
├── proto/                  # Protocol Buffers定义
├── configs/                # 配置文件
├── scripts/                # 部署脚本
└── docs/                   # 文档
```

## 🚀 快速开始

### 环境准备

- Go 1.21+
- Nacos 2.x (或 Etcd 3.x)
- Docker (可选)

### 安装部署

```bash
# 克隆项目
git clone <repository-url>
cd rpc2

# 安装依赖
go mod tidy

# 构建所有服务
make build
```

### 启动服务

#### 方式一：使用脚本启动

```bash
# 启动所有服务（包括Gateway）
./scripts/start_with_gateway.sh start

# 查看服务状态
./scripts/start_with_gateway.sh status

# 停止所有服务
./scripts/start_with_gateway.sh stop
```

#### 方式二：手动启动

```bash
# 1. 启动用户服务
./bin/user-service

# 2. 启动订单服务
./bin/order-service

# 3. 启动API网关
./bin/gateway
```

### 测试API

```bash
# 使用Gateway测试脚本
./scripts/test_gateway.sh

# 或手动测试API
curl -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{"username":"test","email":"test@example.com","phone":"13800138000","age":25}'
```

## 🌐 API Gateway 功能

### HTTP API 接口

#### 认证相关
- `POST /api/v1/auth/login` - 用户登录
- `POST /api/v1/auth/refresh` - 刷新Token
- `POST /api/v1/auth/logout` - 用户登出

#### 用户管理
- `POST /api/v1/users` - 创建用户
- `GET /api/v1/users` - 获取用户列表
- `GET /api/v1/users/{id}` - 获取用户详情
- `PUT /api/v1/users/{id}` - 更新用户信息
- `DELETE /api/v1/users/{id}` - 删除用户

#### 订单管理
- `POST /api/v1/orders` - 创建订单
- `GET /api/v1/orders` - 获取订单列表
- `GET /api/v1/orders/{id}` - 获取订单详情
- `PUT /api/v1/orders/{id}` - 更新订单
- `DELETE /api/v1/orders/{id}` - 删除订单
- `GET /api/v1/orders/user/{user_id}` - 获取用户订单

#### 系统接口
- `GET /health` - 健康检查
- `GET /metrics` - 监控指标

### 中间件功能

- **CORS支持**: 跨域请求处理
- **请求日志**: 详细的请求/响应日志
- **限流控制**: 基于IP的请求限流
- **认证授权**: JWT Token验证
- **指标收集**: Prometheus指标收集

### 网关配置

```yaml
gateway:
  port: 8080
  read_timeout: 30s
  write_timeout: 30s
  enable_cors: true
  enable_auth: true
  enable_metrics: true
  enable_rate_limit: true
  rate_limit: 1000
```

## 🔧 配置说明

### 完整配置示例

```yaml
# 服务器配置
server:
  address: ":50051"
  port: 50051
  max_recv_msg_size: 4194304
  max_send_msg_size: 4194304
  max_concurrent_requests: 1000
  request_timeout: 30s

# 客户端配置
client:
  timeout: 30s
  keep_alive: 30s
  max_connections: 100
  max_idle_conns: 10
  conn_timeout: 5s
  load_balance_type: "round_robin"

# API网关配置
gateway:
  port: 8080
  read_timeout: 30s
  write_timeout: 30s
  enable_cors: true
  enable_auth: true
  enable_metrics: true
  enable_rate_limit: true
  rate_limit: 1000

# 注册中心配置
registry:
  type: "nacos"

nacos:
  server_addr: "localhost:8848"
  namespace: "public"
  group: "DEFAULT_GROUP"
  timeout: 10s
  username: "nacos"
  password: "nacos"

# 安全配置
security:
  jwt_secret: "your-secret-key"
  token_expiry: 1h
  refresh_expiry: 24h
  issuer: "api-gateway"

# 监控配置
metrics:
  enable: true
  port: 9090
  path: "/metrics"
  namespace: "gateway"
  subsystem: "http"

# 日志配置
log:
  level: "info"
  format: "json"
  output: "stdout"
  filename: "logs/app.log"

# 追踪配置
trace:
  service_name: "rpc-framework"
  service_version: "1.0.0"
  environment: "development"
  jaeger_endpoint: "http://localhost:14268/api/traces"
  sample_rate: 1.0
  enable_console: true
```

## 📊 监控与观测

### 监控指标

- **系统指标**: CPU、内存、连接数
- **业务指标**: 请求数、错误率、响应时间
- **网关指标**: 路由转发、认证成功率
- **服务指标**: gRPC调用统计

### 健康检查

```bash
# Gateway健康检查
curl http://localhost:8080/health

# 服务健康检查
curl http://localhost:9090/health
```

### Prometheus指标

```bash
# Gateway指标
curl http://localhost:8080/metrics

# 服务指标
curl http://localhost:9090/metrics
```

## 🔐 安全特性

### JWT认证

```bash
# 登录获取Token
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"password"}'

# 使用Token访问受保护资源
curl -X GET http://localhost:8080/api/v1/users/1 \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

### 权限控制

支持基于角色的访问控制(RBAC)：
- `admin`: 所有权限
- `user`: 用户和订单的基本操作
- `guest`: 只读权限

## 🔄 负载均衡

### 支持的算法

1. **轮询 (round_robin)**: 简单轮询分发
2. **权重轮询 (weighted_round_robin)**: 基于权重分发
3. **最少连接 (least_connections)**: 选择连接数最少的服务

### 配置示例

```yaml
client:
  load_balance_type: "round_robin"
  
# 权重配置（在服务注册时设置）
metadata:
  weight: "100"
```

## 🛠️ 开发指南

### 添加新服务

1. 定义Proto文件
2. 生成Go代码
3. 实现服务逻辑
4. 注册到Gateway路由

### 添加中间件

```go
// 自定义中间件
func customMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 前置处理
        next.ServeHTTP(w, r)
        // 后置处理
    })
}

// 注册中间件
gateway.Use(customMiddleware)
```

### 扩展认证

```go
// 自定义认证逻辑
func customAuthHandler(claims *security.Claims) error {
    // 自定义认证逻辑
    return nil
}
```

## 📈 性能优化

### 连接池优化

```yaml
client:
  max_connections: 100      # 根据并发量调整
  max_idle_conns: 10        # 减少连接创建开销
  conn_timeout: 5s          # 连接超时
  idle_timeout: 30s         # 空闲超时
```

### 限流配置

```yaml
gateway:
  enable_rate_limit: true
  rate_limit: 1000          # 每分钟请求限制
```

### 缓存策略

```yaml
client:
  enable_cache: true
  cache_ttl: 5m             # 缓存TTL
  cache_max_size: 1000      # 缓存大小
```

## 🚀 部署建议

### Docker部署

```dockerfile
# Dockerfile示例
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o gateway ./cmd/gateway

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/gateway .
COPY --from=builder /app/configs ./configs
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
        image: your-registry/api-gateway:latest
        ports:
        - containerPort: 8080
        env:
        - name: NACOS_SERVER_ADDR
          value: "nacos.default.svc.cluster.local:8848"
```

## 🧪 测试

### 单元测试

```bash
# 运行所有测试
go test ./...

# 运行特定包测试
go test ./pkg/gateway/...

# 生成测试覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### 集成测试

```bash
# 启动测试环境
docker-compose -f docker-compose.test.yml up -d

# 运行集成测试
go test -tags=integration ./tests/...
```

### 压力测试

```bash
# 使用hey进行压力测试
hey -n 10000 -c 100 http://localhost:8080/api/v1/users

# 使用wrk进行压力测试
wrk -t12 -c400 -d30s http://localhost:8080/api/v1/users
```

## 📚 相关文档

- [API文档](docs/API.md)
- [架构设计](docs/ARCHITECTURE.md)
- [部署指南](docs/DEPLOYMENT.md)
- [开发指南](docs/DEVELOPMENT.md)
- [故障排除](docs/TROUBLESHOOTING.md)

## 🤝 贡献指南

1. Fork 项目
2. 创建功能分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 打开 Pull Request

## 📄 许可证

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。

## 🙏 致谢

感谢所有为这个项目做出贡献的开发者！

---

**如果这个项目对你有帮助，请给一个 ⭐️！**