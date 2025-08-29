# 开发指南

## 概述

本指南介绍如何在RPC微服务框架中进行开发，包括添加新服务、扩展功能、编写测试等。

## 开发环境搭建

### 环境要求
- **Go**: 1.21+
- **IDE**: GoLand、VS Code + Go插件
- **Docker**: 用于启动依赖服务

### 项目初始化
```bash
git clone <repository-url>
cd rpc2
go mod tidy

# 启动Nacos
docker run --name nacos-standalone -e MODE=standalone -p 8848:8848 nacos/nacos-server:v2.2.3
```

## 项目结构

```
rpc2/
├── cmd/                    # 主程序入口
├── pkg/                    # 核心库代码  
├── internal/               # 内部业务逻辑
│   ├── user/              # 用户服务实现
│   │   ├── handler/       # gRPC处理器
│   │   ├── service/       # 业务逻辑层
│   │   └── repository/    # 数据访问层
│   └── order/             # 订单服务实现
├── proto/                  # Protocol Buffers定义
├── configs/                # 配置文件
└── docs/                   # 文档
```

## 添加新微服务

### 1. 定义Protocol Buffers

创建`proto/product/product.proto`：

```protobuf
syntax = "proto3";
package product;
option go_package = "github.com/rpc-framework/core/proto/product";

message Product {
  int64 id = 1;
  string name = 2;
  double price = 3;
  int32 stock = 4;
}

message CreateProductRequest {
  string name = 1;
  double price = 2;
  int32 stock = 3;
}

message CreateProductResponse {
  int64 id = 1;
  Product product = 2;
}

service ProductService {
  rpc CreateProduct(CreateProductRequest) returns (CreateProductResponse);
  rpc GetProduct(GetProductRequest) returns (GetProductResponse);
}
```

### 2. 生成Go代码
```bash
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       proto/product/product.proto
```

### 3. 实现服务层次

#### Repository层 (`internal/product/repository/`)
```go
type ProductRepository interface {
    Create(ctx context.Context, product *model.Product) (*model.Product, error)
    GetByID(ctx context.Context, id int64) (*model.Product, error)
}

type MemoryProductRepository struct {
    products map[int64]*model.Product
    nextID   int64
    mu       sync.RWMutex
}
```

#### Service层 (`internal/product/service/`)
```go
type ProductService struct {
    repo   repository.ProductRepository
    tracer *trace.Tracer
}

func (s *ProductService) CreateProduct(ctx context.Context, req *CreateProductRequest) (*model.Product, error) {
    // 业务逻辑实现
}
```

#### Handler层 (`internal/product/handler/`)
```go
type ProductHandler struct {
    product.UnimplementedProductServiceServer
    service *service.ProductService
}

func (h *ProductHandler) CreateProduct(ctx context.Context, req *product.CreateProductRequest) (*product.CreateProductResponse, error) {
    // gRPC处理器实现
}
```

### 4. 创建服务主程序

创建`cmd/product-service/main.go`：

```go
func main() {
    // 加载配置
    cfg := config.New()
    cfg.LoadFromFile("./configs/app.yaml")
    
    // 初始化组件
    tracer, _ := trace.NewTracer(&trace.Config{...})
    srv := server.New(&server.Options{...})
    
    // 创建服务实例
    repo := repository.NewMemoryProductRepository()
    svc := service.NewProductService(repo, tracer)
    handler := handler.NewProductHandler(svc, tracer)
    
    // 注册服务
    srv.RegisterService(&product.ProductService_ServiceDesc, handler)
    
    // 启动服务
    srv.Start()
}
```

### 5. 在Gateway中集成

修改`pkg/gateway/client_manager.go`：

```go
// 添加到服务列表
services := []string{
    "user.UserService",
    "order.OrderService", 
    "product.ProductService", // 新增
}

// 添加客户端获取方法
func (cm *ClientManager) GetProductClient() (product.ProductServiceClient, error) {
    client, err := cm.getServiceClient("product.ProductService")
    if err != nil {
        return nil, err
    }
    return client.(product.ProductServiceClient), nil
}
```

修改`pkg/gateway/handlers.go`添加路由：

```go
// 在setupRoutes中添加
products := api.PathPrefix("/products").Subrouter()
products.HandleFunc("", g.handleCreateProduct).Methods("POST")
products.HandleFunc("/{id:[0-9]+}", g.handleGetProduct).Methods("GET")
```

## 扩展中间件

### 创建自定义中间件

```go
// pkg/middleware/custom.go
func RequestIDMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        requestID := generateRequestID()
        w.Header().Set("X-Request-ID", requestID)
        ctx := context.WithValue(r.Context(), "request_id", requestID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

func CacheMiddleware(ttl time.Duration) func(http.Handler) http.Handler {
    cache := make(map[string]CacheItem)
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // 缓存逻辑实现
        })
    }
}
```

### 在Gateway中使用

```go
func (g *Gateway) setupMiddleware() {
    g.router.Use(middleware.RequestIDMiddleware)
    
    // 对特定路径使用缓存
    cacheRouter := g.router.PathPrefix("/api/v1/products").Subrouter()
    cacheRouter.Use(middleware.CacheMiddleware(5 * time.Minute))
}
```

## 编写测试

### 单元测试

```go
// internal/product/service/product_service_test.go
func TestProductService_CreateProduct(t *testing.T) {
    mockRepo := new(MockProductRepository)
    service := NewProductService(mockRepo, nil)
    
    req := &CreateProductRequest{
        Name:  "Test Product",
        Price: 99.99,
        Stock: 10,
    }
    
    expected := &model.Product{ID: 1, Name: req.Name}
    mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*model.Product")).Return(expected, nil)
    
    result, err := service.CreateProduct(context.Background(), req)
    
    assert.NoError(t, err)
    assert.Equal(t, expected.ID, result.ID)
    mockRepo.AssertExpectations(t)
}
```

### 集成测试

```go
// tests/integration/product_test.go
func TestProductServiceIntegration(t *testing.T) {
    testServer := testutils.NewTestServer(t)
    defer testServer.Close()
    
    // 注册服务
    productHandler := setupProductHandler(t)
    product.RegisterProductServiceServer(testServer.GetServer(), productHandler)
    
    // 创建客户端并测试
    client := product.NewProductServiceClient(testServer.GetConn())
    resp, err := client.CreateProduct(context.Background(), &product.CreateProductRequest{
        Name: "Integration Test",
        Price: 199.99,
    })
    
    assert.NoError(t, err)
    assert.Greater(t, resp.Id, int64(0))
}
```

### 性能测试

```go
func BenchmarkProductService_CreateProduct(b *testing.B) {
    runner := testutils.NewBenchmarkRunner("CreateProduct").
        Setup(func(b *testing.B) interface{} {
            return &product.CreateProductRequest{Name: "Benchmark", Price: 99.99}
        }).
        Run(func(b *testing.B, data interface{}) {
            req := data.(*product.CreateProductRequest)
            _, err := client.CreateProduct(context.Background(), req)
            if err != nil {
                b.Fatalf("CreateProduct failed: %v", err)
            }
        })
    
    runner.Execute(b)
}
```

## 配置管理

### 添加新配置

```go
// pkg/config/config.go
type ProductConfig struct {
    CacheSize int           `mapstructure:"cache_size"`
    CacheTTL  time.Duration `mapstructure:"cache_ttl"`
}

type AppConfig struct {
    Server  ServerConfig  `mapstructure:"server"`
    Product ProductConfig `mapstructure:"product"` // 新增
}

func (c *Config) GetProductConfig() (*ProductConfig, error) {
    config, err := c.GetAppConfig()
    if err != nil {
        return nil, err
    }
    return &config.Product, nil
}
```

### 更新配置文件

```yaml
# configs/app.yaml
product:
  cache_size: 1000
  cache_ttl: 5m
```

### 环境变量支持

```go
func (c *Config) loadFromEnv() {
    if val := os.Getenv("PRODUCT_CACHE_SIZE"); val != "" {
        if size, err := strconv.Atoi(val); err == nil {
            c.viper.Set("product.cache_size", size)
        }
    }
}
```

## 数据库集成

### MySQL Repository实现

```go
type MySQLProductRepository struct {
    db *sql.DB
}

func (r *MySQLProductRepository) Create(ctx context.Context, product *model.Product) (*model.Product, error) {
    query := `INSERT INTO products (name, price, stock, created_at) VALUES (?, ?, ?, ?)`
    result, err := r.db.ExecContext(ctx, query, product.Name, product.Price, product.Stock, time.Now())
    if err != nil {
        return nil, err
    }
    
    id, _ := result.LastInsertId()
    product.ID = id
    return product, nil
}
```

## 部署和运维

### Docker化

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o product-service ./cmd/product-service

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/product-service .
EXPOSE 50053
CMD ["./product-service"]
```

### 添加到启动脚本

```bash
# scripts/start_with_gateway.sh
start_product_service() {
    echo "Starting Product Service..."
    nohup ./bin/product-service > logs/product-service.log 2>&1 &
    echo $! > pids/product-service.pid
    sleep 2
    check_service_health "localhost:50053" "Product Service"
}
```

## 开发最佳实践

### 1. 代码规范
- 使用gofmt格式化代码
- 遵循Go命名约定
- 添加适当的注释和文档

### 2. 错误处理
- 使用具体的错误类型
- 提供有意义的错误消息
- 适当的错误包装和传播

### 3. 日志记录
- 使用结构化日志
- 记录关键操作和错误
- 避免记录敏感信息

### 4. 性能优化
- 使用连接池
- 实现适当的缓存策略
- 避免不必要的内存分配

### 5. 安全考虑
- 验证输入参数
- 实现适当的认证和授权
- 使用HTTPS传输

## 调试技巧

### 1. 本地调试
```bash
# 启动单个服务进行调试
dlv debug ./cmd/product-service

# 远程调试
dlv exec ./bin/product-service --headless --listen=:2345 --api-version=2
```

### 2. 日志调试
```go
logger.Debugf("Processing request: %+v", req)
logger.Infof("Service operation completed: duration=%v", duration)
```

### 3. 分布式追踪
- 查看Jaeger UI分析请求链路
- 添加自定义Span记录关键操作
- 使用Tag和Log记录重要信息

通过本开发指南，您可以快速上手RPC微服务框架的开发，添加新功能，扩展现有服务，并确保代码质量和性能。