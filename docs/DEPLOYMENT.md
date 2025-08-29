# 部署指南

## 概述

本指南详细介绍了RPC微服务框架的各种部署方式，包括本地开发环境、Docker容器化部署、Kubernetes集群部署等。

## 环境要求

### 基础环境
- **Go**: 1.21+
- **Docker**: 20.0+
- **Docker Compose**: 2.0+（可选）
- **Kubernetes**: 1.20+（集群部署）

### 依赖服务
- **Nacos**: 2.x（服务注册发现）
- **Jaeger**: latest（分布式追踪，可选）
- **Prometheus**: latest（监控指标，可选）
- **Grafana**: latest（监控面板，可选）

## 本地开发部署

### 1. 环境准备

#### 克隆项目
```bash
git clone <repository-url>
cd rpc2
```

#### 安装依赖
```bash
go mod tidy
```

### 2. 启动依赖服务

#### 启动Nacos（服务注册中心）
```bash
# 使用Docker启动Nacos
docker run --name nacos-standalone \
  -e MODE=standalone \
  -p 8848:8848 \
  -p 9848:9848 \
  nacos/nacos-server:v2.2.3
```

#### 启动Jaeger（可选，用于分布式追踪）
```bash
# 使用Docker启动Jaeger
docker run -d --name jaeger \
  -p 16686:16686 \
  -p 14268:14268 \
  jaegertracing/all-in-one:latest
```

### 3. 配置文件

确保`configs/app.yaml`配置正确：

```yaml
# Nacos配置
nacos:
  server_addr: "localhost:8848"
  namespace: "public"
  group: "DEFAULT_GROUP"
  timeout: 10s
  username: "nacos"
  password: "nacos"

# Gateway配置
gateway:
  port: 8080
  enable_cors: true
  enable_auth: true
  enable_metrics: true
  enable_rate_limit: true
  rate_limit: 1000

# 追踪配置
trace:
  service_name: "rpc-framework"
  jaeger_endpoint: "http://localhost:14268/api/traces"
  sample_rate: 1.0
  enable_console: true
```

### 4. 启动服务

#### 方式一：使用脚本一键启动
```bash
# 启动所有服务
./scripts/start_with_gateway.sh start

# 查看服务状态
./scripts/start_with_gateway.sh status

# 停止所有服务
./scripts/start_with_gateway.sh stop
```

#### 方式二：手动启动各服务
```bash
# 1. 构建所有服务
go build -o bin/user-service ./cmd/user-service
go build -o bin/order-service ./cmd/order-service
go build -o bin/gateway ./cmd/gateway

# 2. 启动用户服务
./bin/user-service &

# 3. 启动订单服务
./bin/order-service &

# 4. 启动API网关
./bin/gateway &
```

### 5. 验证部署

#### 健康检查
```bash
# 检查Gateway健康状态
curl http://localhost:8080/health

# 检查Nacos控制台
# 访问 http://localhost:8848/nacos
# 用户名: nacos, 密码: nacos
```

#### API测试
```bash
# 运行API测试脚本
./scripts/test_gateway.sh

# 或手动测试
curl -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{"username":"test","email":"test@example.com","phone":"13800138000","age":25}'
```

## Docker容器化部署

### 1. 创建Dockerfile

#### Gateway Dockerfile
```dockerfile
# 多阶段构建
FROM golang:1.21-alpine AS builder

# 设置工作目录
WORKDIR /app

# 复制依赖文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建Gateway
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o gateway ./cmd/gateway

# 运行时镜像
FROM alpine:latest

# 安装CA证书
RUN apk --no-cache add ca-certificates

# 创建工作目录
WORKDIR /root/

# 复制二进制文件和配置
COPY --from=builder /app/gateway .
COPY --from=builder /app/configs ./configs

# 暴露端口
EXPOSE 8080

# 启动命令
CMD ["./gateway"]
```

#### 用户服务Dockerfile
```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o user-service ./cmd/user-service

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/user-service .
COPY --from=builder /app/configs ./configs
EXPOSE 50051
CMD ["./user-service"]
```

#### 订单服务Dockerfile
```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o order-service ./cmd/order-service

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/order-service .
COPY --from=builder /app/configs ./configs
EXPOSE 50052
CMD ["./order-service"]
```

### 2. 构建镜像

```bash
# 构建Gateway镜像
docker build -f Dockerfile.gateway -t rpc-framework/gateway:latest .

# 构建用户服务镜像
docker build -f Dockerfile.user -t rpc-framework/user-service:latest .

# 构建订单服务镜像
docker build -f Dockerfile.order -t rpc-framework/order-service:latest .
```

### 3. Docker Compose部署

创建`docker-compose.yml`：

```yaml
version: '3.8'

services:
  nacos:
    image: nacos/nacos-server:v2.2.3
    container_name: nacos
    environment:
      - MODE=standalone
    ports:
      - "8848:8848"
      - "9848:9848"
    networks:
      - rpc-network

  jaeger:
    image: jaegertracing/all-in-one:latest
    container_name: jaeger
    ports:
      - "16686:16686"
      - "14268:14268"
    networks:
      - rpc-network

  user-service:
    image: rpc-framework/user-service:latest
    container_name: user-service
    depends_on:
      - nacos
    environment:
      - NACOS_SERVER_ADDR=nacos:8848
    ports:
      - "50051:50051"
    networks:
      - rpc-network

  order-service:
    image: rpc-framework/order-service:latest
    container_name: order-service
    depends_on:
      - nacos
    environment:
      - NACOS_SERVER_ADDR=nacos:8848
    ports:
      - "50052:50052"
    networks:
      - rpc-network

  gateway:
    image: rpc-framework/gateway:latest
    container_name: gateway
    depends_on:
      - nacos
      - user-service
      - order-service
    environment:
      - NACOS_SERVER_ADDR=nacos:8848
      - TRACE_JAEGER_ENDPOINT=http://jaeger:14268/api/traces
    ports:
      - "8080:8080"
    networks:
      - rpc-network

  prometheus:
    image: prom/prometheus:latest
    container_name: prometheus
    ports:
      - "9090:9090"
    volumes:
      - ./monitoring/prometheus.yml:/etc/prometheus/prometheus.yml
    networks:
      - rpc-network

  grafana:
    image: grafana/grafana:latest
    container_name: grafana
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
    volumes:
      - grafana-data:/var/lib/grafana
    networks:
      - rpc-network

networks:
  rpc-network:
    driver: bridge

volumes:
  grafana-data:
```

### 4. 启动容器环境

```bash
# 启动所有服务
docker-compose up -d

# 查看服务状态
docker-compose ps

# 查看日志
docker-compose logs -f gateway

# 停止所有服务
docker-compose down
```

## Kubernetes集群部署

### 1. 创建命名空间

```yaml
# namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: rpc-framework
  labels:
    name: rpc-framework
```

### 2. 部署Nacos

```yaml
# nacos-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nacos
  namespace: rpc-framework
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nacos
  template:
    metadata:
      labels:
        app: nacos
    spec:
      containers:
      - name: nacos
        image: nacos/nacos-server:v2.2.3
        ports:
        - containerPort: 8848
        - containerPort: 9848
        env:
        - name: MODE
          value: "standalone"
        livenessProbe:
          httpGet:
            path: /nacos/
            port: 8848
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /nacos/
            port: 8848
          initialDelaySeconds: 5
          periodSeconds: 5

---
apiVersion: v1
kind: Service
metadata:
  name: nacos-service
  namespace: rpc-framework
spec:
  selector:
    app: nacos
  ports:
  - name: http
    port: 8848
    targetPort: 8848
  - name: grpc
    port: 9848
    targetPort: 9848
  type: ClusterIP
```

### 3. 部署微服务

```yaml
# user-service-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: user-service
  namespace: rpc-framework
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
        image: rpc-framework/user-service:latest
        ports:
        - containerPort: 50051
        env:
        - name: NACOS_SERVER_ADDR
          value: "nacos-service:8848"
        livenessProbe:
          tcpSocket:
            port: 50051
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          tcpSocket:
            port: 50051
          initialDelaySeconds: 5
          periodSeconds: 5
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi

---
apiVersion: v1
kind: Service
metadata:
  name: user-service
  namespace: rpc-framework
spec:
  selector:
    app: user-service
  ports:
  - port: 50051
    targetPort: 50051
  type: ClusterIP
```

```yaml
# order-service-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: order-service
  namespace: rpc-framework
spec:
  replicas: 3
  selector:
    matchLabels:
      app: order-service
  template:
    metadata:
      labels:
        app: order-service
    spec:
      containers:
      - name: order-service
        image: rpc-framework/order-service:latest
        ports:
        - containerPort: 50052
        env:
        - name: NACOS_SERVER_ADDR
          value: "nacos-service:8848"
        livenessProbe:
          tcpSocket:
            port: 50052
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          tcpSocket:
            port: 50052
          initialDelaySeconds: 5
          periodSeconds: 5
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi

---
apiVersion: v1
kind: Service
metadata:
  name: order-service
  namespace: rpc-framework
spec:
  selector:
    app: order-service
  ports:
  - port: 50052
    targetPort: 50052
  type: ClusterIP
```

### 4. 部署API Gateway

```yaml
# gateway-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: api-gateway
  namespace: rpc-framework
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
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
        resources:
          requests:
            cpu: 200m
            memory: 256Mi
          limits:
            cpu: 1000m
            memory: 1Gi

---
apiVersion: v1
kind: Service
metadata:
  name: gateway-service
  namespace: rpc-framework
spec:
  selector:
    app: api-gateway
  ports:
  - port: 80
    targetPort: 8080
  type: LoadBalancer

---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: gateway-ingress
  namespace: rpc-framework
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
spec:
  rules:
  - host: api.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: gateway-service
            port:
              number: 80
```

### 5. 部署ConfigMap

```yaml
# configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
  namespace: rpc-framework
data:
  app.yaml: |
    server:
      address: ":50051"
      max_recv_msg_size: 4194304
      max_send_msg_size: 4194304
    
    client:
      timeout: 30s
      keep_alive: 30s
      load_balance_type: "round_robin"
    
    gateway:
      port: 8080
      enable_cors: true
      enable_auth: true
      enable_metrics: true
    
    nacos:
      server_addr: "nacos-service:8848"
      namespace: "public"
      group: "DEFAULT_GROUP"
    
    log:
      level: "info"
      format: "json"
      output: "stdout"
```

### 6. 部署顺序

```bash
# 1. 创建命名空间
kubectl apply -f namespace.yaml

# 2. 部署配置
kubectl apply -f configmap.yaml

# 3. 部署Nacos
kubectl apply -f nacos-deployment.yaml

# 4. 等待Nacos就绪
kubectl wait --for=condition=ready pod -l app=nacos -n rpc-framework --timeout=120s

# 5. 部署微服务
kubectl apply -f user-service-deployment.yaml
kubectl apply -f order-service-deployment.yaml

# 6. 等待服务就绪
kubectl wait --for=condition=ready pod -l app=user-service -n rpc-framework --timeout=120s
kubectl wait --for=condition=ready pod -l app=order-service -n rpc-framework --timeout=120s

# 7. 部署Gateway
kubectl apply -f gateway-deployment.yaml
```

## 监控部署

### 1. Prometheus配置

```yaml
# prometheus-config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: prometheus-config
  namespace: rpc-framework
data:
  prometheus.yml: |
    global:
      scrape_interval: 15s
    
    scrape_configs:
    - job_name: 'gateway'
      static_configs:
      - targets: ['gateway-service:80']
      metrics_path: /metrics
    
    - job_name: 'kubernetes-pods'
      kubernetes_sd_configs:
      - role: pod
        namespaces:
          names:
          - rpc-framework
```

### 2. 部署Prometheus

```yaml
# prometheus-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: prometheus
  namespace: rpc-framework
spec:
  replicas: 1
  selector:
    matchLabels:
      app: prometheus
  template:
    metadata:
      labels:
        app: prometheus
    spec:
      containers:
      - name: prometheus
        image: prom/prometheus:latest
        ports:
        - containerPort: 9090
        volumeMounts:
        - name: config-volume
          mountPath: /etc/prometheus
        args:
        - '--config.file=/etc/prometheus/prometheus.yml'
        - '--storage.tsdb.path=/prometheus'
      volumes:
      - name: config-volume
        configMap:
          name: prometheus-config

---
apiVersion: v1
kind: Service
metadata:
  name: prometheus-service
  namespace: rpc-framework
spec:
  selector:
    app: prometheus
  ports:
  - port: 9090
    targetPort: 9090
  type: LoadBalancer
```

## 故障排除

### 1. 常见问题

#### 服务无法启动
```bash
# 检查Pod状态
kubectl get pods -n rpc-framework

# 查看Pod详情
kubectl describe pod <pod-name> -n rpc-framework

# 查看日志
kubectl logs <pod-name> -n rpc-framework
```

#### 服务注册失败
```bash
# 检查Nacos服务状态
kubectl port-forward svc/nacos-service 8848:8848 -n rpc-framework
# 访问 http://localhost:8848/nacos

# 检查网络连接
kubectl exec -it <pod-name> -n rpc-framework -- nslookup nacos-service
```

#### Gateway无法连接后端服务
```bash
# 检查服务发现
kubectl logs -l app=api-gateway -n rpc-framework | grep "service discovery"

# 检查网络策略
kubectl get networkpolicies -n rpc-framework
```

### 2. 性能调优

#### 资源配置
```yaml
resources:
  requests:
    cpu: 200m
    memory: 256Mi
  limits:
    cpu: 1000m
    memory: 1Gi
```

#### HPA自动扩缩容
```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: gateway-hpa
  namespace: rpc-framework
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: api-gateway
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

## 安全配置

### 1. RBAC权限

```yaml
# rbac.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: rpc-framework-sa
  namespace: rpc-framework

---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: rpc-framework-role
  namespace: rpc-framework
rules:
- apiGroups: [""]
  resources: ["pods", "services"]
  verbs: ["get", "list", "watch"]

---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: rpc-framework-binding
  namespace: rpc-framework
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: rpc-framework-role
subjects:
- kind: ServiceAccount
  name: rpc-framework-sa
  namespace: rpc-framework
```

### 2. NetworkPolicy

```yaml
# network-policy.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: rpc-framework-netpol
  namespace: rpc-framework
spec:
  podSelector:
    matchLabels:
      app: api-gateway
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector: {}
    ports:
    - protocol: TCP
      port: 8080
  egress:
  - to:
    - podSelector:
        matchLabels:
          app: user-service
    ports:
    - protocol: TCP
      port: 50051
  - to:
    - podSelector:
        matchLabels:
          app: order-service
    ports:
    - protocol: TCP
      port: 50052
```

## 备份与恢复

### 1. 配置备份

```bash
# 备份Kubernetes配置
kubectl get all -n rpc-framework -o yaml > rpc-framework-backup.yaml

# 备份ConfigMap
kubectl get configmap app-config -n rpc-framework -o yaml > app-config-backup.yaml
```

### 2. 数据恢复

```bash
# 恢复配置
kubectl apply -f rpc-framework-backup.yaml

# 重启服务
kubectl rollout restart deployment/api-gateway -n rpc-framework
kubectl rollout restart deployment/user-service -n rpc-framework
kubectl rollout restart deployment/order-service -n rpc-framework
```

## 维护操作

### 1. 滚动更新

```bash
# 更新Gateway镜像
kubectl set image deployment/api-gateway gateway=rpc-framework/gateway:v2.0.0 -n rpc-framework

# 查看更新状态
kubectl rollout status deployment/api-gateway -n rpc-framework

# 回滚更新
kubectl rollout undo deployment/api-gateway -n rpc-framework
```

### 2. 日志收集

```bash
# 收集所有Pod日志
kubectl logs -l app=api-gateway -n rpc-framework --since=1h > gateway-logs.txt
kubectl logs -l app=user-service -n rpc-framework --since=1h > user-service-logs.txt
kubectl logs -l app=order-service -n rpc-framework --since=1h > order-service-logs.txt
```

这个部署指南涵盖了从本地开发到生产环境的完整部署流程，确保您可以在各种环境中成功部署RPC微服务框架。