#!/bin/bash

# RPC框架服务启动脚本

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

# 检查依赖
check_dependencies() {
    print_info "检查依赖..."
    
    if ! command -v go &> /dev/null; then
        print_error "Go未安装，请先安装Go"
        exit 1
    fi
    
    if ! command -v protoc &> /dev/null; then
        print_warning "protoc未安装，protobuf文件可能无法重新生成"
    fi
    
    print_success "依赖检查完成"
}

# 生成protobuf文件
generate_proto() {
    print_info "生成protobuf文件..."
    
    if command -v protoc &> /dev/null; then
        export PATH=$PATH:$(go env GOPATH)/bin
        
        # 创建目录
        mkdir -p github.com/rpc-framework/core/proto/user
        mkdir -p github.com/rpc-framework/core/proto/order
        
        # 生成用户服务protobuf
        protoc --go_out=. --go_opt=paths=source_relative \
               --go-grpc_out=. --go-grpc_opt=paths=source_relative \
               proto/user.proto
        
        # 生成订单服务protobuf
        protoc --go_out=. --go_opt=paths=source_relative \
               --go-grpc_out=. --go-grpc_opt=paths=source_relative \
               proto/order.proto
        
        # 移动文件到正确位置
        mv proto/user.pb.go github.com/rpc-framework/core/proto/user/
        mv proto/user_grpc.pb.go github.com/rpc-framework/core/proto/user/
        mv proto/order.pb.go github.com/rpc-framework/core/proto/order/
        mv proto/order_grpc.pb.go github.com/rpc-framework/core/proto/order/
        
        print_success "protobuf文件生成完成"
    else
        print_warning "跳过protobuf生成，protoc未安装"
    fi
}

# 编译项目
build_project() {
    print_info "编译项目..."
    
    if go build ./...; then
        print_success "项目编译成功"
    else
        print_error "项目编译失败"
        exit 1
    fi
}

# 启动Nacos（如果未运行）
start_nacos() {
    print_info "检查Nacos服务..."
    
    if ! curl -s http://localhost:8848/nacos/v1/console/health/readiness &> /dev/null; then
        print_warning "Nacos未运行，请先启动Nacos服务"
        print_info "可以使用Docker启动Nacos:"
        echo "docker run --name nacos-standalone -e MODE=standalone -p 8848:8848 -p 9848:9848 -d nacos/nacos-server:v2.2.3"
        echo ""
        read -p "是否继续启动服务？(y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    else
        print_success "Nacos服务运行正常"
    fi
}

# 启动用户服务
start_user_service() {
    print_info "启动用户服务..."
    
    cd examples/services/user
    go run main.go &
    USER_PID=$!
    cd ../../..
    
    print_success "用户服务已启动 (PID: $USER_PID)"
    echo $USER_PID > /tmp/user_service.pid
}

# 启动订单服务
start_order_service() {
    print_info "启动订单服务..."
    
    cd examples/services/order
    go run main.go &
    ORDER_PID=$!
    cd ../../..
    
    print_success "订单服务已启动 (PID: $ORDER_PID)"
    echo $ORDER_PID > /tmp/order_service.pid
}

# 启动Hello服务（多服务模式）
start_hello_service() {
    print_info "启动Hello服务（多服务模式）..."
    
    cd examples/server
    go run main.go &
    HELLO_PID=$!
    cd ../..
    
    print_success "Hello服务已启动 (PID: $HELLO_PID)"
    echo $HELLO_PID > /tmp/hello_service.pid
}

# 等待服务启动
wait_for_services() {
    print_info "等待服务启动..."
    sleep 5
    
    # 检查服务是否启动成功
    if pgrep -f "user.UserService" > /dev/null; then
        print_success "用户服务启动成功"
    else
        print_error "用户服务启动失败"
    fi
    
    if pgrep -f "order.OrderService" > /dev/null; then
        print_success "订单服务启动成功"
    else
        print_error "订单服务启动失败"
    fi
}

# 运行测试
run_tests() {
    print_info "运行测试..."
    
    cd examples/client
    
    print_info "测试用户服务..."
    go run . user
    
    sleep 2
    
    print_info "测试订单服务..."
    go run . order
    
    sleep 2
    
    print_info "测试多服务..."
    go run . multi
    
    cd ../..
    
    print_success "测试完成"
}

# 停止服务
stop_services() {
    print_info "停止服务..."
    
    # 停止用户服务
    if [ -f /tmp/user_service.pid ]; then
        USER_PID=$(cat /tmp/user_service.pid)
        if kill $USER_PID 2>/dev/null; then
            print_success "用户服务已停止"
        fi
        rm -f /tmp/user_service.pid
    fi
    
    # 停止订单服务
    if [ -f /tmp/order_service.pid ]; then
        ORDER_PID=$(cat /tmp/order_service.pid)
        if kill $ORDER_PID 2>/dev/null; then
            print_success "订单服务已停止"
        fi
        rm -f /tmp/order_service.pid
    fi
    
    # 停止Hello服务
    if [ -f /tmp/hello_service.pid ]; then
        HELLO_PID=$(cat /tmp/hello_service.pid)
        if kill $HELLO_PID 2>/dev/null; then
            print_success "Hello服务已停止"
        fi
        rm -f /tmp/hello_service.pid
    fi
}

# 清理
cleanup() {
    print_info "清理临时文件..."
    rm -f /tmp/*_service.pid
}

# 显示帮助
show_help() {
    echo "RPC框架服务启动脚本"
    echo ""
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  start       启动所有服务"
    echo "  stop        停止所有服务"
    echo "  restart     重启所有服务"
    echo "  test        运行测试"
    echo "  build       编译项目"
    echo "  proto       生成protobuf文件"
    echo "  help        显示此帮助信息"
    echo ""
    echo "示例:"
    echo "  $0 start    启动所有服务"
    echo "  $0 test     运行测试"
    echo "  $0 stop     停止所有服务"
}

# 主函数
main() {
    case "${1:-start}" in
        start)
            check_dependencies
            generate_proto
            build_project
            start_nacos
            start_user_service
            start_order_service
            wait_for_services
            print_success "所有服务启动完成！"
            print_info "使用 '$0 test' 运行测试"
            print_info "使用 '$0 stop' 停止服务"
            ;;
        stop)
            stop_services
            cleanup
            print_success "所有服务已停止"
            ;;
        restart)
            stop_services
            cleanup
            sleep 2
            $0 start
            ;;
        test)
            run_tests
            ;;
        build)
            check_dependencies
            generate_proto
            build_project
            ;;
        proto)
            generate_proto
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            print_error "未知选项: $1"
            show_help
            exit 1
            ;;
    esac
}

# 设置信号处理
trap 'print_info "收到中断信号，正在停止服务..."; stop_services; cleanup; exit 0' INT TERM

# 运行主函数
main "$@"

