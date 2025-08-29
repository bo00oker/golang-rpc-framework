package middleware

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// LoggingMiddleware 日志中间件
func LoggingMiddleware() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		
		// 调用下一个处理器
		resp, err := handler(ctx, req)
		
		duration := time.Since(start)
		
		// 记录日志
		if err != nil {
			// 这里可以集成具体的日志库
			// logger.Errorf("gRPC call failed: method=%s, duration=%v, error=%v", info.FullMethod, duration, err)
		} else {
			// logger.Infof("gRPC call success: method=%s, duration=%v", info.FullMethod, duration)
		}
		
		_ = duration // 避免未使用变量警告
		
		return resp, err
	}
}

// RecoveryMiddleware 恢复中间件，防止panic导致服务崩溃
func RecoveryMiddleware() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				// logger.Errorf("gRPC panic recovered: method=%s, panic=%v", info.FullMethod, r)
				err = status.Errorf(codes.Internal, "internal server error")
			}
		}()
		
		return handler(ctx, req)
	}
}

// TimeoutMiddleware 超时中间件
func TimeoutMiddleware(timeout time.Duration) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		
		return handler(ctx, req)
	}
}

// RateLimitMiddleware 限流中间件
func RateLimitMiddleware(maxRequests int, window time.Duration) grpc.UnaryServerInterceptor {
	// 简单的内存限流实现，生产环境建议使用Redis等
	requestCounts := make(map[string]int)
	lastReset := time.Now()
	
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		now := time.Now()
		
		// 重置计数器
		if now.Sub(lastReset) > window {
			requestCounts = make(map[string]int)
			lastReset = now
		}
		
		// 检查限流
		method := info.FullMethod
		if requestCounts[method] >= maxRequests {
			return nil, status.Errorf(codes.ResourceExhausted, "rate limit exceeded")
		}
		
		requestCounts[method]++
		return handler(ctx, req)
	}
}

// AuthMiddleware 认证中间件
func AuthMiddleware(authFunc func(ctx context.Context) error) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// 执行认证
		if err := authFunc(ctx); err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "authentication failed: %v", err)
		}
		
		return handler(ctx, req)
	}
}

// MetricsMiddleware 指标收集中间件
func MetricsMiddleware() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		
		resp, err := handler(ctx, req)
		
		duration := time.Since(start)
		
		// 收集指标
		// metrics.RecordRPCDuration(info.FullMethod, duration)
		// metrics.RecordRPCCount(info.FullMethod, err == nil)
		
		_ = duration // 避免未使用变量警告
		return resp, err
	}
}

// ChainUnaryInterceptors 链式组合多个拦截器
func ChainUnaryInterceptors(interceptors ...grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if len(interceptors) == 0 {
			return handler(ctx, req)
		}
		
		// 递归构建拦截器链
		var chainHandler grpc.UnaryHandler
		chainHandler = func(currentCtx context.Context, currentReq interface{}) (interface{}, error) {
			return interceptors[0](currentCtx, currentReq, info, func(nextCtx context.Context, nextReq interface{}) (interface{}, error) {
				if len(interceptors) == 1 {
					return handler(nextCtx, nextReq)
				}
				return ChainUnaryInterceptors(interceptors[1:]...)(nextCtx, nextReq, info, handler)
			})
		}
		
		return chainHandler(ctx, req)
	}
}