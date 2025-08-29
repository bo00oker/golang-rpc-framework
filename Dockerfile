# 多阶段构建 Dockerfile
# 阶段1：构建阶段
FROM golang:1.24-alpine AS builder

# 安装必要的包
RUN apk add --no-cache git ca-certificates tzdata

# 设置工作目录
WORKDIR /app

# 复制 go mod 文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用程序（默认构建所有服务）
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/user-service cmd/user-service/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/order-service cmd/order-service/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/gateway cmd/gateway/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/client cmd/client/main.go

# 阶段2：运行阶段
FROM alpine:latest AS runtime

# 安装必要的运行时依赖
RUN apk --no-cache add ca-certificates tzdata

# 设置时区
RUN ln -snf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && echo Asia/Shanghai > /etc/timezone

# 创建非root用户
RUN addgroup -g 1001 -S appgroup && \
    adduser -u 1001 -S appuser -G appgroup

# 设置工作目录
WORKDIR /app

# 创建必要的目录
RUN mkdir -p logs configs certs && \
    chown -R appuser:appgroup /app

# 从构建阶段复制二进制文件
COPY --from=builder --chown=appuser:appgroup /app/bin/ ./bin/
COPY --from=builder --chown=appuser:appgroup /app/configs/ ./configs/
COPY --from=builder --chown=appuser:appgroup /app/api/ ./api/

# 切换到非root用户
USER appuser

# 默认服务（可在docker-compose中覆盖）
ARG SERVICE=user-service
ENV SERVICE_NAME=${SERVICE}

# 暴露端口（根据配置文件）
EXPOSE 50051 50052 8080 9090

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD pgrep -f "${SERVICE_NAME}" || exit 1

# 启动脚本
CMD ["sh", "-c", "./bin/${SERVICE_NAME}"]