# RPC框架项目结构

## 项目概述

这是一个基于gRPC的微服务RPC框架，支持服务注册发现、负载均衡、全链路追踪、高并发优化等特性。

## 目录结构

```
rpc2/
├── bin/                                    # 编译输出目录
│   ├── client
│   └── server
├── cmd/                                    # 命令行工具
├── configs/                                # 配置文件
│   └── app.yaml                           # 主配置文件
├── examples/                               # 示例项目
│   ├── README.md                          # 示例项目说明
│   ├── start_services.sh                  # 服务启动脚本
│   ├── services/                          # 独立服务目录
│   │   ├── user/                          # 用户服务
│   │   │   └── main.go                    # 用户服务主程序
│   │   └── order/                         # 订单服务
│   │       └── main.go                    # 订单服务主程序
│   ├── client/                            # 客户端测试程序
│   │   ├── main.go                        # 客户端主程序
│   │   ├── user_client.go                 # 用户服务客户端
│   │   ├── order_client.go                # 订单服务客户端
│   │   ├── multi_service_client.go        # 多服务客户端
│   │   └── simple_test.go                 # 简单测试
│   └── server/                            # 多服务服务器（已废弃）
│       ├── main.go                        # 多服务服务器主程序
│       ├── user_service.go                # 用户服务实现
│       └── order_service.go               # 订单服务实现
├── github.com/rpc-framework/core/         # 生成的protobuf代码
│   └── proto/
│       ├── hello/                         # Hello服务生成的Go代码
│       │   ├── hello.pb.go
│       │   └── hello_grpc.pb.go
│       ├── user/                          # 用户服务生成的Go代码
│       │   ├── user.pb.go
│       │   └── user_grpc.pb.go
│       └── order/                         # 订单服务生成的Go代码
│           ├── order.pb.go
│           └── order_grpc.pb.go
├── internal/                              # 内部包
│   ├── interceptor/                       # 拦截器
│   ├── middleware/                        # 中间件
│   │   └── middleware.go
│   └── pool/                              # 连接池
├── pkg/                                   # 公共包
│   ├── client/                            # 客户端包
│   │   └── client.go                      # gRPC客户端实现
│   ├── config/                            # 配置包
│   │   └── config.go                      # 配置管理
│   ├── logger/                            # 日志包
│   │   └── logger.go                      # 日志实现
│   ├── mq/                                # 消息队列包
│   │   └── nats.go                        # NATS实现
│   ├── registry/                          # 服务注册发现包
│   │   ├── etcd.go                        # etcd实现
│   │   └── nacos.go                       # Nacos实现
│   ├── server/                            # 服务器包
│   │   └── server.go                      # gRPC服务器实现
│   └── trace/                             # 追踪包
│       └── trace.go                       # 全链路追踪实现
├── proto/                                 # protobuf定义文件
│   ├── hello.proto                        # Hello服务定义
│   ├── user.proto                         # 用户服务定义
│   └── order.proto                        # 订单服务定义
├── go.mod                                 # Go模块文件
├── go.sum                                 # Go依赖校验文件
├── README.md                              # 项目说明
└── PROJECT_STRUCTURE.md                   # 项目结构说明
```

## 核心模块说明

### 1. pkg/client (客户端包)
- **功能**: gRPC客户端封装
- **特性**: 
  - 连接池管理
  - 负载均衡
  - 熔断器
  - 重试机制
  - 缓存支持
  - 异步处理

### 2. pkg/server (服务器包)
- **功能**: gRPC服务器封装
- **特性**:
  - 拦截器支持
  - 限流保护
  - 指标收集
  - 健康检查
  - 内存池
  - 异步处理

### 3. pkg/registry (服务注册发现包)
- **功能**: 服务注册发现抽象
- **实现**:
  - etcd注册中心
  - Nacos注册中心
- **特性**:
  - 服务注册/注销
  - 服务发现
  - 服务监听
  - 健康检查

### 4. pkg/config (配置包)
- **功能**: 配置管理
- **特性**:
  - YAML配置文件支持
  - 环境变量覆盖
  - 配置验证
  - 默认值设置

### 5. pkg/logger (日志包)
- **功能**: 结构化日志
- **特性**:
  - Zap日志库
  - 文件轮转
  - 多级别日志
  - 结构化输出

### 6. pkg/trace (追踪包)
- **功能**: 全链路追踪
- **特性**:
  - OpenTelemetry集成
  - Jaeger导出器
  - Console导出器
  - 自动span创建

### 7. pkg/mq (消息队列包)
- **功能**: 消息队列抽象
- **实现**: NATS消息队列
- **特性**: 发布/订阅模式

## 示例项目结构

### 独立服务模式（推荐）

```
examples/services/
├── user/                    # 用户服务
│   └── main.go             # 独立运行的用户服务
└── order/                   # 订单服务
    └── main.go             # 独立运行的订单服务
```

**优势**:
- 服务独立部署
- 独立扩展
- 技术栈隔离
- 故障隔离

### 多服务模式（已废弃）

```
examples/server/
├── main.go                 # 多服务服务器
├── user_service.go         # 用户服务实现
└── order_service.go        # 订单服务实现
```

**劣势**:
- 服务耦合
- 难以独立扩展
- 单点故障风险

## 服务定义

### 1. Hello服务
```protobuf
service HelloService {
  rpc SayHello(HelloRequest) returns (HelloResponse);
  rpc SayHelloStream(HelloRequest) returns (stream HelloResponse);
  rpc SayHelloBidirectional(stream HelloRequest) returns (stream HelloResponse);
}
```

### 2. 用户服务
```protobuf
service UserService {
  rpc CreateUser(CreateUserRequest) returns (CreateUserResponse);
  rpc GetUser(GetUserRequest) returns (GetUserResponse);
  rpc ListUsers(ListUsersRequest) returns (ListUsersResponse);
  rpc UpdateUser(UpdateUserRequest) returns (UpdateUserResponse);
  rpc DeleteUser(DeleteUserRequest) returns (DeleteUserResponse);
}
```

### 3. 订单服务
```protobuf
service OrderService {
  rpc CreateOrder(CreateOrderRequest) returns (CreateOrderResponse);
  rpc GetOrder(GetOrderRequest) returns (GetOrderResponse);
  rpc ListOrders(ListOrdersRequest) returns (ListOrdersResponse);
  rpc UpdateOrder(UpdateOrderRequest) returns (UpdateOrderResponse);
  rpc DeleteOrder(DeleteOrderRequest) returns (DeleteOrderResponse);
  rpc GetOrdersByUser(GetOrdersByUserRequest) returns (GetOrdersByUserResponse);
}
```

## 配置文件结构

### app.yaml
```yaml
server:
  # 服务器配置
  address: ":50051"
  port: 50051
  
client:
  # 客户端配置
  timeout: 30s
  keep_alive: 30s
  
nacos:
  # Nacos配置
  server_addr: "127.0.0.1:8848"
  namespace: "public"
  
log:
  # 日志配置
  level: "info"
  format: "json"
  
trace:
  # 追踪配置
  service_name: "rpc-framework"
  jaeger_endpoint: "http://localhost:14268/api/traces"
```

## 高并发特性

### 1. 连接池
- 客户端连接复用
- 连接健康检查
- 自动重连机制

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

## 监控和可观测性

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

## 部署架构

### 1. 独立服务部署
```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│  User       │    │  Order      │    │  Hello      │
│  Service    │    │  Service    │    │  Service    │
│  :50051     │    │  :50052     │    │  :50053     │
└─────────────┘    └─────────────┘    └─────────────┘
       │                   │                   │
       └───────────────────┼───────────────────┘
                           │
                    ┌─────────────┐
                    │   Nacos     │
                    │  Registry   │
                    │  :8848      │
                    └─────────────┘
```

### 2. 客户端架构
```
┌─────────────┐
│   Client    │
│  Application│
└─────────────┘
       │
       ├─── User Service Client
       ├─── Order Service Client
       └─── Hello Service Client
```

## 开发流程

### 1. 添加新服务
1. 创建protobuf定义文件
2. 生成Go代码
3. 实现服务逻辑
4. 创建独立服务目录
5. 编写客户端测试

### 2. 服务开发
1. 定义服务接口
2. 实现业务逻辑
3. 添加配置管理
4. 集成监控和日志
5. 编写单元测试

### 3. 客户端开发
1. 创建客户端实例
2. 配置连接参数
3. 实现业务调用
4. 添加错误处理
5. 集成重试机制

## 最佳实践

### 1. 服务设计
- 单一职责原则
- 接口设计清晰
- 错误处理完善
- 版本管理策略

### 2. 配置管理
- 环境变量覆盖
- 配置验证
- 默认值设置
- 敏感信息保护

### 3. 监控告警
- 关键指标监控
- 异常告警
- 性能分析
- 容量规划

### 4. 安全考虑
- 服务间认证
- 数据加密
- 访问控制
- 审计日志

## 扩展方向

### 1. 数据存储
- 数据库集成
- 缓存层
- 分布式存储
- 数据迁移

### 2. 消息队列
- 异步处理
- 事件驱动
- 消息持久化
- 死信队列

### 3. 服务网格
- Istio集成
- 流量管理
- 安全策略
- 可观测性

### 4. 容器化
- Docker镜像
- Kubernetes部署
- 服务编排
- 自动扩缩容

