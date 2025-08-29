#!/bin/bash

# Gateway API 测试脚本

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Gateway地址
GATEWAY_URL="http://localhost:8080"
TOKEN=""

# 显示测试结果
show_result() {
    local description="$1"
    local status_code="$2"
    local response="$3"
    
    echo -e "${BLUE}测试: $description${NC}"
    
    if [ "$status_code" -eq 200 ] || [ "$status_code" -eq 201 ]; then
        echo -e "${GREEN}✓ 成功 (状态码: $status_code)${NC}"
    else
        echo -e "${RED}✗ 失败 (状态码: $status_code)${NC}"
    fi
    
    echo "响应: $response"
    echo ""
}

# 健康检查
test_health() {
    echo -e "${BLUE}=== 健康检查 ===${NC}"
    
    local response=$(curl -s -w "%{http_code}" -o /tmp/response "$GATEWAY_URL/health")
    local status_code="${response: -3}"
    local body=$(cat /tmp/response)
    
    show_result "健康检查" "$status_code" "$body"
}

# 用户登录
test_login() {
    echo -e "${BLUE}=== 用户登录 ===${NC}"
    
    local response=$(curl -s -w "%{http_code}" -o /tmp/response \
        -X POST "$GATEWAY_URL/api/v1/auth/login" \
        -H "Content-Type: application/json" \
        -d '{
            "username": "admin",
            "password": "password123"
        }')
    
    local status_code="${response: -3}"
    local body=$(cat /tmp/response)
    
    # 提取token
    TOKEN=$(echo "$body" | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)
    
    show_result "用户登录" "$status_code" "$body"
}

# 创建用户
test_create_user() {
    echo -e "${BLUE}=== 创建用户 ===${NC}"
    
    local response=$(curl -s -w "%{http_code}" -o /tmp/response \
        -X POST "$GATEWAY_URL/api/v1/users" \
        -H "Content-Type: application/json" \
        -d '{
            "username": "testuser123",
            "email": "test@example.com",
            "phone": "13800138000",
            "age": 25,
            "address": "测试地址"
        }')
    
    local status_code="${response: -3}"
    local body=$(cat /tmp/response)
    
    show_result "创建用户" "$status_code" "$body"
}

# 查询用户列表
test_list_users() {
    echo -e "${BLUE}=== 查询用户列表 ===${NC}"
    
    local response=$(curl -s -w "%{http_code}" -o /tmp/response \
        "$GATEWAY_URL/api/v1/users?page=1&page_size=10")
    
    local status_code="${response: -3}"
    local body=$(cat /tmp/response)
    
    show_result "查询用户列表" "$status_code" "$body"
}

# 获取用户详情
test_get_user() {
    echo -e "${BLUE}=== 获取用户详情 ===${NC}"
    
    local response=$(curl -s -w "%{http_code}" -o /tmp/response \
        "$GATEWAY_URL/api/v1/users/1")
    
    local status_code="${response: -3}"
    local body=$(cat /tmp/response)
    
    show_result "获取用户详情" "$status_code" "$body"
}

# 创建订单
test_create_order() {
    echo -e "${BLUE}=== 创建订单 ===${NC}"
    
    local response=$(curl -s -w "%{http_code}" -o /tmp/response \
        -X POST "$GATEWAY_URL/api/v1/orders" \
        -H "Content-Type: application/json" \
        -d '{
            "user_id": 1,
            "amount": 99.99,
            "description": "测试订单",
            "items": [
                {
                    "product_id": 1,
                    "product_name": "测试商品",
                    "price": 99.99,
                    "quantity": 1
                }
            ]
        }')
    
    local status_code="${response: -3}"
    local body=$(cat /tmp/response)
    
    show_result "创建订单" "$status_code" "$body"
}

# 查询订单列表
test_list_orders() {
    echo -e "${BLUE}=== 查询订单列表 ===${NC}"
    
    local response=$(curl -s -w "%{http_code}" -o /tmp/response \
        "$GATEWAY_URL/api/v1/orders?page=1&page_size=10")
    
    local status_code="${response: -3}"
    local body=$(cat /tmp/response)
    
    show_result "查询订单列表" "$status_code" "$body"
}

# 根据用户查询订单
test_orders_by_user() {
    echo -e "${BLUE}=== 根据用户查询订单 ===${NC}"
    
    local response=$(curl -s -w "%{http_code}" -o /tmp/response \
        "$GATEWAY_URL/api/v1/orders/user/1?page=1&page_size=10")
    
    local status_code="${response: -3}"
    local body=$(cat /tmp/response)
    
    show_result "根据用户查询订单" "$status_code" "$body"
}

# 测试认证保护的接口
test_protected_api() {
    echo -e "${BLUE}=== 测试认证保护的接口 ===${NC}"
    
    if [ -z "$TOKEN" ]; then
        echo -e "${YELLOW}跳过认证测试，因为没有有效的token${NC}"
        return
    fi
    
    local response=$(curl -s -w "%{http_code}" -o /tmp/response \
        -X PUT "$GATEWAY_URL/api/v1/users/1" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $TOKEN" \
        -d '{
            "username": "updateduser",
            "age": 30
        }')
    
    local status_code="${response: -3}"
    local body=$(cat /tmp/response)
    
    show_result "更新用户信息（需要认证）" "$status_code" "$body"
}

# 运行所有测试
run_all_tests() {
    echo -e "${BLUE}开始 Gateway API 测试${NC}"
    echo -e "${BLUE}====================================${NC}"
    echo ""
    
    # 检查Gateway是否运行
    if ! curl -s "$GATEWAY_URL/health" > /dev/null; then
        echo -e "${RED}Gateway 未运行，请先启动 Gateway${NC}"
        echo "运行: ./scripts/start_with_gateway.sh start gateway"
        exit 1
    fi
    
    test_health
    test_login
    test_create_user
    test_list_users
    test_get_user
    test_create_order
    test_list_orders
    test_orders_by_user
    test_protected_api
    
    echo -e "${GREEN}所有测试完成！${NC}"
    
    # 清理临时文件
    rm -f /tmp/response
}

# 显示API文档
show_api_docs() {
    echo -e "${BLUE}Gateway API 文档${NC}"
    echo -e "${BLUE}===================${NC}"
    echo ""
    echo -e "${YELLOW}认证相关:${NC}"
    echo "  POST /api/v1/auth/login      - 用户登录"
    echo "  POST /api/v1/auth/refresh    - 刷新令牌"
    echo "  POST /api/v1/auth/logout     - 用户登出"
    echo ""
    echo -e "${YELLOW}用户管理:${NC}"
    echo "  POST /api/v1/users           - 创建用户"
    echo "  GET  /api/v1/users           - 查询用户列表"
    echo "  GET  /api/v1/users/{id}      - 获取用户详情"
    echo "  PUT  /api/v1/users/{id}      - 更新用户信息 [需要认证]"
    echo "  DELETE /api/v1/users/{id}    - 删除用户 [需要认证]"
    echo ""
    echo -e "${YELLOW}订单管理:${NC}"
    echo "  POST /api/v1/orders          - 创建订单 [需要认证]"
    echo "  GET  /api/v1/orders          - 查询订单列表 [需要认证]"
    echo "  GET  /api/v1/orders/{id}     - 获取订单详情 [需要认证]"
    echo "  PUT  /api/v1/orders/{id}     - 更新订单信息 [需要认证]"
    echo "  DELETE /api/v1/orders/{id}   - 删除订单 [需要认证]"
    echo "  GET  /api/v1/orders/user/{user_id} - 根据用户查询订单 [需要认证]"
    echo ""
    echo -e "${YELLOW}系统接口:${NC}"
    echo "  GET  /health                 - 健康检查"
    echo "  GET  /metrics                - Prometheus指标"
    echo ""
    echo -e "${BLUE}Gateway地址: $GATEWAY_URL${NC}"
}

# 主函数
main() {
    local command=${1:-"test"}
    
    case $command in
        "test")
            run_all_tests
            ;;
        "docs")
            show_api_docs
            ;;
        "health")
            test_health
            ;;
        "login")
            test_login
            ;;
        "help" | "--help" | "-h")
            echo "Usage: $0 [test|docs|health|login|help]"
            echo "  test   - 运行所有API测试"
            echo "  docs   - 显示API文档"
            echo "  health - 健康检查"
            echo "  login  - 测试登录"
            echo "  help   - 显示帮助信息"
            ;;
        *)
            echo -e "${RED}未知命令: $command${NC}"
            echo "运行 '$0 help' 查看帮助信息"
            exit 1
            ;;
    esac
}

# 执行主函数
main "$@"