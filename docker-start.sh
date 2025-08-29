#!/bin/bash

# Docker Compose 启动脚本
# Usage: ./docker-start.sh [command] [options]

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 项目名称
PROJECT_NAME="rpc-framework"

# 日志函数
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查依赖
check_dependencies() {
    log_info "检查依赖环境..."
    
    if ! command -v docker &> /dev/null; then
        log_error "Docker 未安装或未在PATH中"
        exit 1
    fi
    
    if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
        log_error "Docker Compose 未安装"
        exit 1
    fi
    
    log_success "依赖检查完成"
}

# 创建必要的目录
create_directories() {
    log_info "创建必要的目录..."
    
    mkdir -p docker/{mysql/{init,conf},redis,nacos/logs,prometheus/data,grafana/{data,dashboards,datasources}}
    mkdir -p logs
    
    # 设置权限（避免权限问题）
    sudo chown -R 1001:1001 docker/grafana/data 2>/dev/null || true
    sudo chown -R 999:999 docker/mysql/data 2>/dev/null || true
    
    log_success "目录创建完成"
}

# 构建镜像
build_images() {
    log_info "构建Docker镜像..."
    
    docker-compose build --no-cache
    
    log_success "镜像构建完成"
}

# 启动基础设施服务
start_infrastructure() {
    log_info "启动基础设施服务..."
    
    # 启动数据库和注册中心
    docker-compose up -d mysql redis nacos jaeger prometheus grafana
    
    log_info "等待基础设施服务启动..."
    sleep 30
    
    # 检查服务状态
    check_service_health "mysql" "mysql:3306"
    check_service_health "redis" "redis:6379"
    check_service_health "nacos" "nacos:8848"
    check_service_health "jaeger" "jaeger:16686"
    
    log_success "基础设施服务启动完成"
}

# 启动应用服务
start_application() {
    log_info "启动应用服务..."
    
    # 启动业务服务
    docker-compose up -d user-service order-service gateway
    
    log_info "等待应用服务启动..."
    sleep 20
    
    # 检查应用服务状态
    check_service_health "user-service" "user-service:50051"
    check_service_health "order-service" "order-service:50052"
    check_service_health "gateway" "gateway:8080"
    
    log_success "应用服务启动完成"
}

# 检查服务健康状态
check_service_health() {
    local service_name=$1
    local endpoint=$2
    local max_attempts=30
    local attempt=1
    
    log_info "检查 $service_name 服务状态..."
    
    while [ $attempt -le $max_attempts ]; do
        if docker-compose ps $service_name | grep -q "Up"; then
            log_success "$service_name 服务运行正常"
            return 0
        fi
        
        log_warning "$service_name 服务尚未就绪，等待中... ($attempt/$max_attempts)"
        sleep 3
        ((attempt++))
    done
    
    log_error "$service_name 服务启动失败"
    return 1
}

# 显示服务状态
show_status() {
    log_info "服务状态："
    docker-compose ps
    
    echo ""
    log_info "服务访问地址："
    echo "  - API网关: http://localhost:8080"
    echo "  - Nacos控制台: http://localhost:8848/nacos (nacos/nacos)"
    echo "  - Jaeger UI: http://localhost:16686"
    echo "  - Grafana: http://localhost:3000 (admin/admin123)"
    echo "  - Prometheus: http://localhost:9091"
    echo "  - 用户服务: grpc://localhost:50051"
    echo "  - 订单服务: grpc://localhost:50052"
}

# 显示日志
show_logs() {
    local service=$1
    if [ -z "$service" ]; then
        docker-compose logs -f
    else
        docker-compose logs -f $service
    fi
}

# 停止服务
stop_services() {
    log_info "停止所有服务..."
    docker-compose down
    log_success "服务已停止"
}

# 清理环境
cleanup() {
    log_info "清理Docker环境..."
    
    # 停止并删除容器
    docker-compose down -v --remove-orphans
    
    # 删除镜像（可选）
    if [ "$1" = "--images" ]; then
        docker-compose down --rmi all
    fi
    
    # 清理数据卷（可选）
    if [ "$1" = "--volumes" ]; then
        docker volume prune -f
    fi
    
    log_success "环境清理完成"
}

# 重启服务
restart_services() {
    log_info "重启所有服务..."
    docker-compose restart
    log_success "服务重启完成"
}

# 运行测试
run_tests() {
    log_info "运行集成测试..."
    
    # 启动测试客户端
    docker-compose --profile test up --build client
    
    log_success "测试完成"
}

# 更新服务
update_services() {
    log_info "更新服务..."
    
    # 重新构建并启动
    docker-compose build --no-cache
    docker-compose up -d --force-recreate
    
    log_success "服务更新完成"
}

# 备份数据
backup_data() {
    local backup_dir="backup/$(date +%Y%m%d_%H%M%S)"
    mkdir -p $backup_dir
    
    log_info "备份数据到 $backup_dir..."
    
    # 备份MySQL数据
    docker-compose exec mysql mysqldump -u root -p$MYSQL_ROOT_PASSWORD --all-databases > $backup_dir/mysql_backup.sql
    
    # 备份Redis数据
    docker-compose exec redis redis-cli BGSAVE
    docker cp $(docker-compose ps -q redis):/data/dump.rdb $backup_dir/redis_backup.rdb
    
    log_success "数据备份完成"
}

# 监控服务
monitor_services() {
    log_info "监控服务状态..."
    
    while true; do
        clear
        show_status
        echo ""
        echo "按 Ctrl+C 退出监控"
        sleep 5
    done
}

# 显示帮助信息
show_help() {
    cat << EOF
Docker Compose 管理脚本

用法: $0 [命令] [选项]

命令:
  start              启动所有服务
  stop               停止所有服务
  restart            重启所有服务
  status             显示服务状态
  logs [service]     显示日志
  build              构建镜像
  test               运行测试
  update             更新服务
  cleanup [options]  清理环境
  backup             备份数据
  monitor            监控服务
  help               显示帮助信息

示例:
  $0 start                    # 启动所有服务
  $0 logs gateway            # 查看网关日志
  $0 cleanup --volumes       # 清理环境包括数据卷
  $0 test                    # 运行测试

清理选项:
  --images              同时删除镜像
  --volumes             同时删除数据卷

EOF
}

# 主函数
main() {
    case "$1" in
        "start")
            check_dependencies
            create_directories
            build_images
            start_infrastructure
            start_application
            show_status
            ;;
        "stop")
            stop_services
            ;;
        "restart")
            restart_services
            ;;
        "status")
            show_status
            ;;
        "logs")
            show_logs $2
            ;;
        "build")
            check_dependencies
            build_images
            ;;
        "test")
            run_tests
            ;;
        "update")
            update_services
            ;;
        "cleanup")
            cleanup $2
            ;;
        "backup")
            backup_data
            ;;
        "monitor")
            monitor_services
            ;;
        "help"|"--help"|"-h")
            show_help
            ;;
        "")
            log_info "启动完整环境..."
            check_dependencies
            create_directories
            build_images
            start_infrastructure
            start_application
            show_status
            ;;
        *)
            log_error "未知命令: $1"
            show_help
            exit 1
            ;;
    esac
}

# 执行主函数
main "$@"