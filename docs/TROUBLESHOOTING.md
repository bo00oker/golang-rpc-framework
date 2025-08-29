# 故障排除指南

## 概述

本指南帮助您快速诊断和解决RPC微服务框架中常见的问题，包括服务启动、网络连接、性能问题等。

## 常见问题分类

### 🚀 服务启动问题

#### 1. 服务无法启动

**症状**: 服务启动后立即退出或报错

**可能原因**:
- 端口被占用
- 配置文件错误
- 依赖服务未启动
- 环境变量配置错误

**排查步骤**:

```bash
# 1. 检查端口占用
lsof -i :8080          # Gateway端口
lsof -i :50051         # User Service端口
lsof -i :50052         # Order Service端口

# 2. 检查配置文件
cat configs/app.yaml | grep -E "(port|address|server_addr)"

# 3. 检查服务状态
./scripts/start_with_gateway.sh status

# 4. 查看服务日志
tail -f logs/gateway.log
tail -f logs/user-service.log
tail -f logs/order-service.log
```

**解决方法**:

```bash
# 杀死占用端口的进程
kill -9 $(lsof -t -i:8080)

# 修改配置文件端口
vim configs/app.yaml

# 重新启动服务
./scripts/start_with_gateway.sh restart
```

#### 2. Nacos连接失败

**症状**: 服务启动但无法注册到Nacos

**错误日志**:
```
Failed to register service: connection refused
Failed to discover service: nacos client not ready
```

**排查步骤**:

```bash
# 1. 检查Nacos服务状态
docker ps | grep nacos
curl http://localhost:8848/nacos/v1/console/health

# 2. 检查网络连接
telnet localhost 8848
ping localhost

# 3. 检查Nacos配置
grep -A 5 "nacos:" configs/app.yaml
```

**解决方法**:

```bash
# 启动Nacos服务
docker run --name nacos-standalone \
  -e MODE=standalone \
  -p 8848:8848 \
  -p 9848:9848 \
  nacos/nacos-server:v2.2.3

# 等待Nacos完全启动（约30秒）
sleep 30

# 验证Nacos可访问
curl http://localhost:8848/nacos/

# 重启服务
./scripts/start_with_gateway.sh restart
```

### 🌐 网络连接问题

#### 1. Gateway无法连接后端服务

**症状**: API请求返回503 Service Unavailable

**错误日志**:
```
Failed to dial service user.UserService: no instances found
RPC client connection failed: context deadline exceeded
```

**排查步骤**:

```bash
# 1. 检查服务注册状态
curl http://localhost:8848/nacos/v1/ns/instance/list?serviceName=user.UserService

# 2. 检查Gateway日志
grep "service discovery" logs/gateway.log
grep "Failed to dial" logs/gateway.log

# 3. 测试gRPC连接
grpcurl -plaintext localhost:50051 list
grpcurl -plaintext localhost:50052 list
```

**解决方法**:

```bash
# 1. 确保后端服务正常运行
ps aux | grep -E "(user-service|order-service)"

# 2. 检查服务注册
# 访问 http://localhost:8848/nacos 查看服务列表

# 3. 重启服务（按顺序）
./scripts/start_with_gateway.sh stop
./scripts/start_with_gateway.sh start

# 4. 验证服务健康
curl http://localhost:8080/health
```

#### 2. 服务间通信超时

**症状**: 请求处理缓慢或超时

**错误日志**:
```
context deadline exceeded
RPC timeout after 30s
```

**排查步骤**:

```bash
# 1. 检查超时配置
grep -A 5 "timeout" configs/app.yaml

# 2. 监控连接状态
netstat -an | grep :50051
netstat -an | grep :50052

# 3. 检查系统资源
top -p $(pgrep gateway)
free -h
```

**解决方法**:

```yaml
# 调整配置文件
client:
  timeout: 60s          # 增加超时时间
  conn_timeout: 10s     # 连接超时
  keep_alive: 30s       # 保持连接
```

### 🔐 认证授权问题

#### 1. JWT Token验证失败

**症状**: API返回401 Unauthorized

**错误日志**:
```
Invalid token: signature is invalid
Token expired
Missing authorization header
```

**排查步骤**:

```bash
# 1. 检查Token格式
curl -v http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIs..."

# 2. 验证JWT配置
grep -A 3 "security:" configs/app.yaml

# 3. 测试登录接口
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"password"}'
```

**解决方法**:

```bash
# 1. 重新登录获取新Token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"password"}' | \
  jq -r '.data.access_token')

# 2. 使用新Token测试
curl http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer $TOKEN"

# 3. 检查Token过期时间
echo $TOKEN | cut -d. -f2 | base64 -d | jq '.exp'
```

#### 2. 权限不足

**症状**: API返回403 Forbidden

**错误日志**:
```
Permission denied: insufficient privileges
User does not have required role
```

**排查步骤**:

```bash
# 1. 检查用户角色
curl http://localhost:8080/api/v1/auth/profile \
  -H "Authorization: Bearer $TOKEN"

# 2. 查看权限配置
grep -A 10 "authRoutes" pkg/gateway/gateway.go
```

**解决方法**:

```bash
# 使用管理员账户登录
ADMIN_TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}' | \
  jq -r '.data.access_token')

# 使用管理员Token访问
curl http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

### 📊 性能问题

#### 1. 响应时间过长

**症状**: API响应超过预期时间

**排查步骤**:

```bash
# 1. 检查Gateway指标
curl http://localhost:8080/metrics | grep -E "(duration|latency)"

# 2. 监控系统资源
htop
iostat -x 1

# 3. 分析日志中的响应时间
grep "duration" logs/gateway.log | tail -20

# 4. 使用wrk进行压力测试
wrk -t12 -c400 -d30s http://localhost:8080/api/v1/users
```

**解决方法**:

```yaml
# 优化配置
client:
  max_connections: 200      # 增加连接池
  max_idle_conns: 20       # 增加空闲连接

gateway:
  rate_limit: 2000         # 提高限流阈值

server:
  max_concurrent_requests: 2000  # 增加并发数
```

#### 2. 内存使用过高

**症状**: 服务内存持续增长

**排查步骤**:

```bash
# 1. 监控内存使用
ps aux | grep -E "(gateway|user-service|order-service)"
pmap -x $(pgrep gateway)

# 2. 查看Go运行时统计
curl http://localhost:8080/debug/pprof/heap > heap.prof
go tool pprof heap.prof

# 3. 检查连接泄漏
lsof -p $(pgrep gateway) | wc -l
```

**解决方法**:

```go
// 添加内存监控
func monitorMemory() {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    log.Printf("Alloc = %d KB, TotalAlloc = %d KB, Sys = %d KB", 
        bToKb(m.Alloc), bToKb(m.TotalAlloc), bToKb(m.Sys))
}

// 定期触发GC
go func() {
    for range time.Tick(5 * time.Minute) {
        runtime.GC()
    }
}()
```

### 🔍 监控诊断问题

#### 1. 监控指标缺失

**症状**: Prometheus指标不更新或缺失

**排查步骤**:

```bash
# 1. 检查metrics端点
curl http://localhost:8080/metrics

# 2. 验证Prometheus配置
cat monitoring/prometheus.yml

# 3. 检查指标服务状态
curl http://localhost:9090/targets
```

**解决方法**:

```yaml
# prometheus.yml
scrape_configs:
- job_name: 'gateway'
  static_configs:
  - targets: ['localhost:8080']
  metrics_path: /metrics
  scrape_interval: 15s
```

#### 2. 分布式追踪丢失

**症状**: Jaeger中看不到追踪链路

**排查步骤**:

```bash
# 1. 检查Jaeger服务
curl http://localhost:16686/api/traces?service=gateway

# 2. 验证追踪配置
grep -A 5 "trace:" configs/app.yaml

# 3. 检查追踪导出
grep "trace" logs/gateway.log
```

**解决方法**:

```yaml
# 确保追踪配置正确
trace:
  service_name: "rpc-framework"
  jaeger_endpoint: "http://localhost:14268/api/traces"
  sample_rate: 1.0
  enable_console: true
```

## 调试工具

### 1. 日志分析

```bash
# 实时查看日志
tail -f logs/gateway.log

# 按错误级别过滤
grep "ERROR\|FATAL" logs/*.log

# 按时间范围查看
grep "2024-08-29 10:" logs/gateway.log

# 统计错误频率
grep "ERROR" logs/gateway.log | cut -d' ' -f1-2 | sort | uniq -c
```

### 2. 网络诊断

```bash
# 检查端口监听
netstat -tulpn | grep -E ":(8080|50051|50052|8848)"

# 测试网络连通性
telnet localhost 8080
nc -zv localhost 50051

# 抓包分析
tcpdump -i lo -A -s 0 port 8080

# gRPC调试
grpcurl -plaintext localhost:50051 describe
grpcurl -plaintext localhost:50051 user.UserService/GetUser
```

### 3. 性能分析

```bash
# CPU性能分析
go tool pprof http://localhost:8080/debug/pprof/profile

# 内存分析
go tool pprof http://localhost:8080/debug/pprof/heap

# 协程分析
go tool pprof http://localhost:8080/debug/pprof/goroutine

# 压力测试
hey -n 10000 -c 100 http://localhost:8080/api/v1/users
```

### 4. 数据库诊断

```bash
# 检查连接池状态
curl http://localhost:8080/debug/vars | jq '.database'

# 慢查询分析
grep "slow query" logs/*.log

# 连接数监控
ss -s | grep tcp
```

## 应急处理

### 1. 服务宕机

```bash
# 快速重启服务
./scripts/start_with_gateway.sh restart

# 如果脚本失败，手动重启
pkill -f gateway
pkill -f user-service
pkill -f order-service

# 重新启动
nohup ./bin/gateway > logs/gateway.log 2>&1 &
nohup ./bin/user-service > logs/user-service.log 2>&1 &
nohup ./bin/order-service > logs/order-service.log 2>&1 &
```

### 2. 内存泄漏

```bash
# 紧急重启服务
systemctl restart rpc-gateway

# 生成内存dump
curl http://localhost:8080/debug/pprof/heap > emergency-heap.prof

# 分析内存使用
go tool pprof emergency-heap.prof
```

### 3. 数据库连接池耗尽

```bash
# 检查连接数
mysql -e "SHOW PROCESSLIST;"

# 重启服务释放连接
./scripts/start_with_gateway.sh restart

# 调整配置
vim configs/app.yaml
# 增加 max_idle_conns 和 max_open_conns
```

## 预防措施

### 1. 监控告警

```yaml
# alerting rules
groups:
- name: rpc-framework
  rules:
  - alert: HighErrorRate
    expr: rate(gateway_http_requests_total{status=~"5.."}[5m]) > 0.1
    for: 2m
    labels:
      severity: warning
    annotations:
      summary: "High error rate detected"

  - alert: HighMemoryUsage
    expr: process_resident_memory_bytes / 1024 / 1024 > 1000
    for: 5m
    labels:
      severity: critical
    annotations:
      summary: "High memory usage"
```

### 2. 健康检查

```bash
# 添加到crontab
*/1 * * * * curl -f http://localhost:8080/health || /path/to/restart.sh

# 监控脚本
#!/bin/bash
SERVICES=("gateway:8080" "user-service:50051" "order-service:50052")

for service in "${SERVICES[@]}"; do
    name=${service%:*}
    port=${service#*:}
    
    if ! nc -z localhost $port; then
        echo "$(date): $name is down, restarting..."
        pkill -f $name
        nohup ./bin/$name > logs/$name.log 2>&1 &
    fi
done
```

### 3. 备份策略

```bash
# 配置备份
cp -r configs/ backup/configs-$(date +%Y%m%d)/

# 日志轮转
logrotate -f /etc/logrotate.d/rpc-framework

# 数据备份
mysqldump -u root -p rpc_framework > backup/db-$(date +%Y%m%d).sql
```

## 联系支持

如果问题仍然无法解决，请收集以下信息并联系技术支持：

### 必需信息
1. **错误描述**: 详细的错误现象和复现步骤
2. **环境信息**: 
   ```bash
   go version
   docker --version
   cat /etc/os-release
   ```
3. **服务状态**:
   ```bash
   ./scripts/start_with_gateway.sh status
   ```
4. **日志文件**: 
   ```bash
   tar -czf logs-$(date +%Y%m%d).tar.gz logs/
   ```
5. **配置文件**: 
   ```bash
   cat configs/app.yaml
   ```

### 性能问题
- 系统资源使用情况
- 并发请求数量
- 网络延迟测试结果
- 数据库连接状态

### 安全问题
- 认证配置
- 网络策略
- 防火墙设置
- SSL证书状态

通过本故障排除指南，您应该能够快速定位和解决大部分常见问题。如有其他问题，请参考项目文档或提交Issue。