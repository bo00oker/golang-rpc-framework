#!/bin/bash

# 开发环境启动脚本
# Usage: ./dev-start.sh

set -e

echo "🛠️  启动开发环境..."

# 仅启动必要的基础设施
echo "📦 启动基础设施..."
docker-compose up -d mysql redis nacos

echo "⏳ 等待基础设施启动..."
sleep 20

echo "✅ 开发环境就绪！"
echo ""
echo "📋 开发环境配置："
echo "  - MySQL: localhost:3306 (rpc_user/rpc_pass123)"
echo "  - Redis: localhost:6379"
echo "  - Nacos: localhost:8848 (nacos/nacos)"
echo ""
echo "🚀 手动启动服务："
echo "  go run cmd/user-service/main.go"
echo "  go run cmd/order-service/main.go"
echo "  go run cmd/gateway/main.go"
echo ""
echo "🧪 运行测试："
echo "  go run cmd/client/main.go"