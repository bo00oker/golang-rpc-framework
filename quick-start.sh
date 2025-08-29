#!/bin/bash

# 快速启动脚本 - 仅启动核心服务
# Usage: ./quick-start.sh

set -e

echo "🚀 启动 RPC 框架核心服务..."

# 启动基础设施
echo "📦 启动基础设施服务..."
docker-compose up -d mysql redis nacos jaeger

echo "⏳ 等待基础设施服务启动完成..."
sleep 30

# 启动应用服务
echo "🔥 启动应用服务..."
docker-compose up -d user-service order-service gateway

echo "⏳ 等待应用服务启动完成..."
sleep 20

# 显示状态
echo "✅ 服务启动完成！"
echo ""
echo "🌐 服务访问地址："
echo "  - API网关: http://localhost:8080"
echo "  - Nacos控制台: http://localhost:8848/nacos (nacos/nacos)"
echo "  - Jaeger UI: http://localhost:16686"
echo ""
echo "📊 查看服务状态："
docker-compose ps

echo ""
echo "📝 查看日志："
echo "  docker-compose logs -f [service_name]"
echo ""
echo "🛑 停止服务："
echo "  docker-compose down"