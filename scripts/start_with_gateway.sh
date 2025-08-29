#!/bin/bash

# RPC Framework 服务管理脚本
# 支持启动单个服务、所有服务或Gateway

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 默认配置
CONFIG_FILE="./configs/app.yaml"
LOG_DIR="./logs"
PID_DIR="./pids"

# 创建目录
mkdir -p "$LOG_DIR" "$PID_DIR"

# 显示帮助信息
show_help() {
    echo -e "${BLUE}RPC Framework 服务管理脚本${NC}"
    echo -e "${BLUE}===========================================${NC}"
    echo "Usage: $0 [COMMAND] [SERVICE]"
    echo ""
    echo "Commands:"
    echo "  start [service]    - 启动服务 (默认启动所有服务)"
    echo "  stop [service]     - 停止服务 (默认停止所有服务)"
    echo "  restart [service]  - 重启服务"
    echo "  status             - 查看服务状态"
    echo "  logs [service]     - 查看服务日志"
    echo "  help              - 显示帮助信息"
    echo ""
    echo "Services:"
    echo "  gateway           - API Gateway (端口: 8080)"
    echo "  user              - 用户服务 (端口: 50051)"
    echo "  order             - 订单服务 (端口: 50052)"
    echo "  all               - 所有服务 (默认)"
    echo ""
    echo "Examples:"
    echo "  $0 start           # 启动所有服务"
    echo "  $0 start gateway   # 只启动Gateway"
    echo "  $0 start user      # 只启动用户服务"
    echo "  $0 stop            # 停止所有服务"
    echo "  $0 status          # 查看所有服务状态"
    echo "  $0 logs gateway    # 查看Gateway日志"
}

# 检查服务是否运行
is_running() {
    local service=$1
    local pid_file="$PID_DIR/$service.pid"
    
    if [ -f "$pid_file" ]; then
        local pid=$(cat "$pid_file")
        if ps -p "$pid" > /dev/null 2>&1; then
            return 0
        else
            rm -f "$pid_file"
            return 1
        fi
    fi
    return 1
}

# 启动单个服务
start_service() {
    local service=$1
    local pid_file="$PID_DIR/$service.pid"
    local log_file="$LOG_DIR/$service.log"
    
    if is_running "$service"; then
        echo -e "${YELLOW}$service 已经在运行中${NC}"
        return 0
    fi
    
    echo -e "${BLUE}正在启动 $service...${NC}"
    
    case $service in
        "gateway")
            nohup go run ./cmd/gateway/main.go > "$log_file" 2>&1 &
            local pid=$!
            echo $pid > "$pid_file"
            echo -e "${GREEN}API Gateway 已启动 (PID: $pid, 端口: 8080)${NC}"
            echo -e "${BLUE}Gateway API地址: http://localhost:8080${NC}"
            echo -e "${BLUE}健康检查: http://localhost:8080/health${NC}"
            ;;
        "user")
            nohup go run ./cmd/user-service/main.go > "$log_file" 2>&1 &
            local pid=$!
            echo $pid > "$pid_file"
            echo -e "${GREEN}用户服务已启动 (PID: $pid, 端口: 50051)${NC}"
            ;;
        "order")
            nohup go run ./cmd/order-service/main.go > "$log_file" 2>&1 &
            local pid=$!
            echo $pid > "$pid_file"
            echo -e "${GREEN}订单服务已启动 (PID: $pid, 端口: 50052)${NC}"
            ;;
        *)
            echo -e "${RED}未知服务: $service${NC}"
            return 1
            ;;
    esac
    
    # 等待服务启动
    sleep 2
    
    if is_running "$service"; then
        echo -e "${GREEN}$service 启动成功${NC}"
        return 0
    else
        echo -e "${RED}$service 启动失败，请查看日志: $log_file${NC}"
        return 1
    fi
}

# 停止单个服务
stop_service() {
    local service=$1
    local pid_file="$PID_DIR/$service.pid"
    
    if ! is_running "$service"; then
        echo -e "${YELLOW}$service 未在运行${NC}"
        return 0
    fi
    
    echo -e "${BLUE}正在停止 $service...${NC}"
    
    local pid=$(cat "$pid_file")
    kill "$pid"
    
    # 等待进程结束
    local count=0
    while ps -p "$pid" > /dev/null 2>&1; do
        sleep 1
        count=$((count + 1))
        if [ $count -gt 10 ]; then
            echo -e "${YELLOW}强制终止 $service${NC}"
            kill -9 "$pid"
            break
        fi
    done
    
    rm -f "$pid_file"
    echo -e "${GREEN}$service 已停止${NC}"
}

# 查看服务状态
show_status() {
    echo -e "${BLUE}RPC Framework 服务状态${NC}"
    echo -e "${BLUE}================================${NC}"
    
    local services=("gateway" "user" "order")
    
    for service in "${services[@]}"; do
        if is_running "$service"; then
            local pid=$(cat "$PID_DIR/$service.pid")
            local port
            case $service in
                "gateway") port="8080" ;;
                "user") port="50051" ;;
                "order") port="50052" ;;
            esac
            echo -e "${GREEN}✓ $service 正在运行 (PID: $pid, 端口: $port)${NC}"
        else
            echo -e "${RED}✗ $service 未运行${NC}"
        fi
    done
    
    echo ""
    echo -e "${BLUE}服务地址:${NC}"
    if is_running "gateway"; then
        echo -e "  Gateway API: ${GREEN}http://localhost:8080${NC}"
        echo -e "  健康检查: ${GREEN}http://localhost:8080/health${NC}"
        echo -e "  用户API: ${GREEN}http://localhost:8080/api/v1/users${NC}"
        echo -e "  订单API: ${GREEN}http://localhost:8080/api/v1/orders${NC}"
    fi
    
    echo ""
    echo -e "${BLUE}监控地址:${NC}"
    echo -e "  Prometheus指标: ${GREEN}http://localhost:9090/metrics${NC}"
    echo -e "  Jaeger追踪: ${GREEN}http://localhost:16686${NC}"
}

# 查看服务日志
show_logs() {
    local service=${1:-"all"}
    
    if [ "$service" = "all" ]; then
        echo -e "${BLUE}所有服务日志:${NC}"
        for log_file in "$LOG_DIR"/*.log; do
            if [ -f "$log_file" ]; then
                echo -e "${YELLOW}=== $(basename "$log_file") ===${NC}"
                tail -n 20 "$log_file"
                echo ""
            fi
        done
    else
        local log_file="$LOG_DIR/$service.log"
        if [ -f "$log_file" ]; then
            echo -e "${BLUE}$service 服务日志:${NC}"
            tail -f "$log_file"
        else
            echo -e "${RED}日志文件不存在: $log_file${NC}"
        fi
    fi
}

# 启动所有服务
start_all() {
    echo -e "${BLUE}正在启动所有服务...${NC}"
    echo ""
    
    # 首先启动后端服务
    start_service "user"
    sleep 1
    start_service "order"
    sleep 1
    
    # 最后启动Gateway
    start_service "gateway"
    
    echo ""
    echo -e "${GREEN}所有服务启动完成！${NC}"
    echo ""
    show_status
}

# 停止所有服务
stop_all() {
    echo -e "${BLUE}正在停止所有服务...${NC}"
    echo ""
    
    # 先停止Gateway
    stop_service "gateway"
    sleep 1
    
    # 再停止后端服务
    stop_service "user"
    stop_service "order"
    
    echo ""
    echo -e "${GREEN}所有服务已停止${NC}"
}

# 检查环境依赖
check_dependencies() {
    echo -e "${BLUE}正在检查环境依赖...${NC}"
    
    # 检查Go环境
    if ! command -v go &> /dev/null; then
        echo -e "${RED}Go 未安装，请先安装 Go 1.24+${NC}"
        exit 1
    fi
    
    local go_version=$(go version | awk '{print $3}' | sed 's/go//')
    echo -e "${GREEN}Go 版本: $go_version${NC}"
    
    # 检查配置文件
    if [ ! -f "$CONFIG_FILE" ]; then
        echo -e "${RED}配置文件不存在: $CONFIG_FILE${NC}"
        exit 1
    fi
    
    echo -e "${GREEN}环境检查通过${NC}"
    echo ""
}

# 主函数
main() {
    local command=${1:-"help"}
    local service=${2:-"all"}
    
    case $command in
        "start")
            check_dependencies
            if [ "$service" = "all" ]; then
                start_all
            else
                start_service "$service"
            fi
            ;;
        "stop")
            if [ "$service" = "all" ]; then
                stop_all
            else
                stop_service "$service"
            fi
            ;;
        "restart")
            if [ "$service" = "all" ]; then
                stop_all
                sleep 2
                check_dependencies
                start_all
            else
                stop_service "$service"
                sleep 1
                start_service "$service"
            fi
            ;;
        "status")
            show_status
            ;;
        "logs")
            show_logs "$service"
            ;;
        "help" | "--help" | "-h")
            show_help
            ;;
        *)
            echo -e "${RED}未知命令: $command${NC}"
            echo ""
            show_help
            exit 1
            ;;
    esac
}

# 执行主函数
main "$@"