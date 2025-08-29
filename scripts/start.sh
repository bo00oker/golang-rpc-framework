#!/bin/bash

# RPC框架服务启动脚本

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印带颜色的消息
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查Go环境
check_go() {
    if ! command -v go &> /dev/null; then
        print_error "Go is not installed or not in PATH"
        exit 1
    fi
    print_info "Go version: $(go version)"
}

# 检查依赖服务
check_dependencies() {
    print_info "Checking dependencies..."
    
    # 检查Nacos
    if ! curl -s http://localhost:8848/nacos/ > /dev/null; then
        print_warning "Nacos is not running on localhost:8848"
        print_info "To start Nacos with Docker:"
        echo "  docker run --name nacos-standalone -e MODE=standalone -p 8848:8848 -p 9848:9848 nacos/nacos-server:v2.2.3"
    else
        print_success "Nacos is running"
    fi
    
    # 检查Jaeger（可选）
    if ! curl -s http://localhost:16686/ > /dev/null; then
        print_warning "Jaeger is not running on localhost:16686 (optional)"
        print_info "To start Jaeger with Docker:"
        echo "  docker run -d --name jaeger -p 16686:16686 -p 14268:14268 jaegertracing/all-in-one:latest"
    else
        print_success "Jaeger is running"
    fi
}

# 构建所有服务
build_services() {
    print_info "Building services..."
    
    # 创建bin目录
    mkdir -p bin
    
    # 构建用户服务
    print_info "Building user service..."
    go build -o bin/user-service ./cmd/user-service/
    print_success "User service built successfully"
    
    # 构建订单服务
    print_info "Building order service..."
    go build -o bin/order-service ./cmd/order-service/
    print_success "Order service built successfully"
    
    # 构建客户端
    print_info "Building client..."
    go build -o bin/client ./cmd/client/
    print_success "Client built successfully"
}

# 启动用户服务
start_user_service() {
    print_info "Starting user service..."
    cd cmd/user-service
    go run main.go &
    USER_SERVICE_PID=$!
    cd ../..
    print_success "User service started (PID: $USER_SERVICE_PID)"
    sleep 2
}

# 启动订单服务
start_order_service() {
    print_info "Starting order service..."
    cd cmd/order-service
    go run main.go &
    ORDER_SERVICE_PID=$!
    cd ../..
    print_success "Order service started (PID: $ORDER_SERVICE_PID)"
    sleep 2
}

# 运行客户端测试
run_client_tests() {
    print_info "Running client tests..."
    cd cmd/client
    go run *.go all
    cd ../..
    print_success "Client tests completed"
}

# 停止所有服务
stop_services() {
    print_info "Stopping services..."
    if [[ ! -z "$USER_SERVICE_PID" ]]; then
        kill $USER_SERVICE_PID 2>/dev/null || true
        print_info "User service stopped"
    fi
    if [[ ! -z "$ORDER_SERVICE_PID" ]]; then
        kill $ORDER_SERVICE_PID 2>/dev/null || true
        print_info "Order service stopped"
    fi
}

# 清理资源
cleanup() {
    print_info "Cleaning up..."
    stop_services
    print_success "Cleanup completed"
}

# 信号处理
trap cleanup EXIT

# 显示帮助信息
show_help() {
    echo "RPC框架服务管理脚本"
    echo ""
    echo "用法: $0 [命令]"
    echo ""
    echo "命令:"
    echo "  check      - 检查环境和依赖"
    echo "  build      - 构建所有服务"
    echo "  start      - 启动所有服务"
    echo "  user       - 仅启动用户服务"
    echo "  order      - 仅启动订单服务"
    echo "  test       - 运行客户端测试"
    echo "  dev        - 开发模式（启动服务并运行测试）"
    echo "  stop       - 停止所有服务"
    echo "  clean      - 清理构建文件"
    echo "  help       - 显示此帮助信息"
    echo ""
    echo "示例:"
    echo "  $0 dev      # 启动开发模式"
    echo "  $0 check    # 检查环境"
    echo "  $0 build    # 构建服务"
}

# 主逻辑
main() {
    case "${1:-dev}" in
        "check")
            check_go
            check_dependencies
            ;;
        "build")
            check_go
            build_services
            ;;
        "start")
            check_go
            check_dependencies
            start_user_service
            start_order_service
            print_success "All services started"
            print_info "Press Ctrl+C to stop all services"
            wait
            ;;
        "user")
            check_go
            check_dependencies
            start_user_service
            print_info "Press Ctrl+C to stop user service"
            wait
            ;;
        "order")
            check_go
            check_dependencies
            start_order_service
            print_info "Press Ctrl+C to stop order service"
            wait
            ;;
        "test")
            check_go
            run_client_tests
            ;;
        "dev")
            print_info "Starting development mode..."
            check_go
            check_dependencies
            start_user_service
            start_order_service
            sleep 3
            run_client_tests
            print_success "Development mode completed"
            print_info "Services are still running. Press Ctrl+C to stop."
            wait
            ;;
        "stop")
            stop_services
            ;;
        "clean")
            print_info "Cleaning build files..."
            rm -rf bin/
            print_success "Build files cleaned"
            ;;
        "help"|"-h"|"--help")
            show_help
            ;;
        *)
            print_error "Unknown command: $1"
            show_help
            exit 1
            ;;
    esac
}

# 运行主函数
main "$@"