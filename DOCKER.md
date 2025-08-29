# Docker Compose 文件使用指南

## 📖 概述

本项目提供了完整的Docker容器化解决方案，包括：
- 🐳 多阶段构建的Dockerfile
- 🐳 各服务专用的Dockerfile  
- 🐳 完整的docker-compose配置
- 🛠️ 便捷的启动脚本

## 🏗️ 架构组成

### 基础设施服务
- **MySQL 8.0**: 主数据库，端口3306
- **Redis 7**: 缓存服务，端口6379  
- **Nacos 2.2.3**: 服务注册中心，端口8848
- **Jaeger**: 分布式追踪，UI端口16686
- **Prometheus**: 监控数据收集，端口9091
- **Grafana**: 监控可视化，端口3000

### 应用服务
- **user-service**: 用户服务，端口50051
- **order-service**: 订单服务，端口50052  
- **gateway**: API网关，端口8080

## 🚀 快速启动

### 方式1: 使用启动脚本（推荐）

```bash
# 完整启动所有服务
./docker-start.sh start

# 快速启动核心服务
./quick-start.sh

# 开发环境（仅基础设施）
./dev-start.sh
```

### 方式2: 直接使用docker-compose

```bash
# 启动所有服务
docker-compose up -d

# 启动特定服务
docker-compose up -d mysql redis nacos
docker-compose up -d user-service order-service gateway

# 查看服务状态
docker-compose ps

# 查看日志
docker-compose logs -f gateway
```

## 🔧 配置说明

### 环境变量
项目使用`.env`文件管理环境变量：

```bash
# 数据库配置
MYSQL_ROOT_PASSWORD=root123456
MYSQL_DATABASE=rpc_framework
MYSQL_USER=rpc_user
MYSQL_PASSWORD=rpc_pass123

# Nacos认证
NACOS_AUTH_IDENTITY_VALUE=nacos

# Grafana密码
GRAFANA_PASSWORD=admin123
```

### 配置文件
- `configs/app.yaml`: 本地开发配置
- `docker/configs/app-docker.yaml`: Docker环境专用配置

主要差异：
- 服务地址使用容器名称（如`mysql:3306`）
- 网络配置适配Docker网络
- 资源限制优化

## 🌐 服务访问

| 服务 | 地址 | 认证信息 |
|------|------|----------|
| API网关 | http://localhost:8080 | - |
| Nacos控制台 | http://localhost:8848/nacos | nacos/nacos |
| Jaeger UI | http://localhost:16686 | - |
| Grafana | http://localhost:3000 | admin/admin123 |
| Prometheus | http://localhost:9091 | - |
| MySQL | localhost:3306 | rpc_user/rpc_pass123 |
| Redis | localhost:6379 | 无密码 |

## 📊 监控和观测

### 健康检查
所有服务都配置了健康检查：
```bash
# 查看健康状态
docker-compose ps
docker inspect $(docker-compose ps -q gateway) | grep Health
```

### 日志管理
```bash
# 查看所有服务日志
docker-compose logs -f

# 查看特定服务日志
docker-compose logs -f user-service

# 实时监控
./docker-start.sh monitor
```

### 指标监控
- Prometheus采集各服务指标
- Grafana提供可视化面板
- 自定义指标端点：`/metrics`

## 🔄 开发工作流

### 1. 开发环境
```bash
# 启动基础设施
./dev-start.sh

# 本地开发服务
go run cmd/user-service/main.go
go run cmd/order-service/main.go  
go run cmd/gateway/main.go
```

### 2. 测试环境
```bash
# 启动完整环境
./docker-start.sh start

# 运行集成测试
./docker-start.sh test
```

### 3. 更新部署
```bash
# 更新代码后重新构建
./docker-start.sh build
./docker-start.sh update
```

## 🛠️ 管理命令

### 启动脚本功能
```bash
./docker-start.sh start      # 启动所有服务
./docker-start.sh stop       # 停止所有服务
./docker-start.sh restart    # 重启服务
./docker-start.sh status     # 查看状态
./docker-start.sh logs       # 查看日志
./docker-start.sh build      # 构建镜像
./docker-start.sh test       # 运行测试
./docker-start.sh cleanup    # 清理环境
./docker-start.sh backup     # 备份数据
./docker-start.sh monitor    # 监控状态
```

### 数据管理
```bash
# 备份数据
./docker-start.sh backup

# 清理环境（保留镜像）
./docker-start.sh cleanup

# 完全清理（包括镜像和数据卷）
./docker-start.sh cleanup --images --volumes
```

## 🐛 故障排除

### 常见问题

1. **端口冲突**
   ```bash
   # 检查端口占用
   lsof -i :8080
   # 修改docker-compose.yml中的端口映射
   ```

2. **权限问题**
   ```bash
   # 修复目录权限
   sudo chown -R $(id -u):$(id -g) docker/grafana/data
   sudo chown -R 999:999 docker/mysql/data
   ```

3. **服务启动失败**
   ```bash
   # 查看详细日志
   docker-compose logs service-name
   
   # 检查健康状态
   docker-compose ps
   ```

4. **网络问题**
   ```bash
   # 重建网络
   docker-compose down
   docker network prune
   docker-compose up -d
   ```

### 调试模式
```bash
# 前台启动查看日志
docker-compose up

# 进入容器调试
docker-compose exec user-service sh

# 查看容器资源使用
docker stats
```

## 📈 性能优化

### 资源配置
- MySQL: 256MB内存池
- Redis: 256MB最大内存
- 应用服务: 根据负载调整

### 扩展部署
```bash
# 水平扩展服务
docker-compose up -d --scale user-service=3
docker-compose up -d --scale order-service=2
```

## 🔒 安全配置

### 生产环境建议
1. 修改默认密码
2. 启用TLS/SSL
3. 配置防火墙规则
4. 定期更新镜像
5. 使用密钥管理系统

### 网络隔离
项目使用自定义网络`rpc-network`，确保服务间通信的安全性。

## 📚 扩展阅读

- [Docker官方文档](https://docs.docker.com/)
- [Docker Compose指南](https://docs.docker.com/compose/)
- [Go应用容器化最佳实践](https://docs.docker.com/language/golang/)

---

有问题请查看日志或提交issue！ 🎉