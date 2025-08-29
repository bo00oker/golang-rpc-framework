package gateway

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// corsMiddleware CORS中间件
func (g *Gateway) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 设置CORS头
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		w.Header().Set("Access-Control-Max-Age", "3600")

		// 处理预检请求
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware 日志中间件
func (g *Gateway) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 创建响应写入器包装器来捕获状态码
		ww := &responseWriter{ResponseWriter: w, statusCode: 200}

		// 执行请求
		next.ServeHTTP(ww, r)

		// 记录日志
		duration := time.Since(start)
		g.logger.Infof("HTTP %s %s - %d - %v - %s",
			r.Method, r.URL.Path, ww.statusCode, duration, r.RemoteAddr)
	})
}

// responseWriter 响应写入器包装器
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// metricsMiddleware 指标中间件
func (g *Gateway) metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 增加进行中的请求计数
		if g.metrics != nil {
			// 这里需要添加HTTP指标方法到metrics包
			// g.metrics.IncHTTPInFlight(r.Method, r.URL.Path)
			// defer g.metrics.DecHTTPInFlight(r.Method, r.URL.Path)
		}

		ww := &responseWriter{ResponseWriter: w, statusCode: 200}
		next.ServeHTTP(ww, r)

		// 记录指标
		duration := time.Since(start)
		if g.metrics != nil {
			// g.metrics.RecordHTTPRequest(r.Method, r.URL.Path, ww.statusCode, duration)
			_ = duration // 避免unused变量错误，实际使用时会记录到metrics
		}
	})
}

// rateLimitMiddleware 限流中间件
func (g *Gateway) rateLimitMiddleware(next http.Handler) http.Handler {
	// 简单的内存限流实现，生产环境建议使用Redis
	limiter := NewRateLimiter(g.config.RateLimit)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := getClientIP(r)

		if !limiter.Allow(clientIP) {
			g.sendError(w, http.StatusTooManyRequests, "Rate limit exceeded")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// authMiddleware 认证中间件
func (g *Gateway) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 获取Authorization头
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			g.sendError(w, http.StatusUnauthorized, "Missing authorization header")
			return
		}

		// 检查Bearer token格式
		if !strings.HasPrefix(authHeader, "Bearer ") {
			g.sendError(w, http.StatusUnauthorized, "Invalid authorization header format")
			return
		}

		// 提取token
		token := authHeader[7:]

		// 验证token
		claims, err := g.authService.ValidateToken(token)
		if err != nil {
			g.sendError(w, http.StatusUnauthorized, "Invalid token")
			return
		}

		// 将用户信息添加到context
		ctx := context.WithValue(r.Context(), "user_id", claims.UserID)
		ctx = context.WithValue(ctx, "username", claims.Username)
		ctx = context.WithValue(ctx, "role", claims.Role)

		// 继续处理请求
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// getClientIP 获取客户端IP
func getClientIP(r *http.Request) string {
	// 检查X-Forwarded-For头
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}

	// 检查X-Real-IP头
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	// 使用RemoteAddr
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}

	return ip
}

// RateLimiter 令牌桶限流器 - 优化版本
type RateLimiter struct {
	mu              sync.Mutex
	clientLimiters  map[string]*ClientLimiter
	limit           int           // 每秒令牌数
	window          time.Duration // 时间窗口
	cleanupInterval time.Duration // 清理间隔
	lastCleanup     time.Time
}

// ClientLimiter 客户端限流器
type ClientLimiter struct {
	tokens     float64   // 当前令牌数
	lastRefill time.Time // 上次补充时间
	refillRate float64   // 补充速率（令牌/秒）
	bucketSize int       // 桶容量
}

// NewRateLimiter 创建令牌桶限流器
func NewRateLimiter(limit int) *RateLimiter {
	return &RateLimiter{
		clientLimiters:  make(map[string]*ClientLimiter),
		limit:           limit,
		window:          time.Minute,
		cleanupInterval: 5 * time.Minute, // 每5分钟清理一次
		lastCleanup:     time.Now(),
	}
}

// Allow 检查是否允许请求 - 优化的令牌桶算法
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// 定期清理过期的客户端限流器
	if now.Sub(rl.lastCleanup) > rl.cleanupInterval {
		rl.cleanup(now)
		rl.lastCleanup = now
	}

	// 获取或创建客户端限流器
	clientLimiter, exists := rl.clientLimiters[key]
	if !exists {
		clientLimiter = &ClientLimiter{
			tokens:     float64(rl.limit),
			lastRefill: now,
			refillRate: float64(rl.limit) / 60.0, // 每秒补充速率
			bucketSize: rl.limit,
		}
		rl.clientLimiters[key] = clientLimiter
	}

	// 计算需要补充的令牌数
	elapsed := now.Sub(clientLimiter.lastRefill).Seconds()
	tokensToAdd := elapsed * clientLimiter.refillRate
	if tokensToAdd > 0 {
		if clientLimiter.tokens+tokensToAdd > float64(clientLimiter.bucketSize) {
			clientLimiter.tokens = float64(clientLimiter.bucketSize)
		} else {
			clientLimiter.tokens += tokensToAdd
		}
		clientLimiter.lastRefill = now
	}

	// 检查是否有可用令牌
	if clientLimiter.tokens >= 1.0 {
		clientLimiter.tokens -= 1.0
		return true
	}

	return false
}

// cleanup 清理过期的客户端限流器
func (rl *RateLimiter) cleanup(now time.Time) {
	cutoff := now.Add(-2 * rl.window) // 清理2个窗口以外的数据

	for key, limiter := range rl.clientLimiters {
		if limiter.lastRefill.Before(cutoff) {
			delete(rl.clientLimiters, key)
		}
	}
}

// handleHealth 健康检查处理器
func (g *Gateway) handleHealth(w http.ResponseWriter, r *http.Request) {
	// 检查后端服务健康状态
	health := map[string]string{
		"gateway": "healthy",
	}

	// 检查RPC连接
	if g.clientMgr != nil {
		if g.clientMgr.IsHealthy() {
			health["rpc_clients"] = "healthy"
		} else {
			health["rpc_clients"] = "unhealthy"
		}
	}

	g.sendResponse(w, http.StatusOK, "Health check passed", health)
}

// handleMetrics 指标处理器
func (g *Gateway) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if g.metrics == nil {
		g.sendError(w, http.StatusNotImplemented, "Metrics not enabled")
		return
	}

	// 这里应该返回Prometheus格式的指标
	// 简化实现，返回基本信息
	stats := map[string]interface{}{
		"active_connections": g.clientMgr.GetActiveConnections(),
		"total_requests":     "metrics implementation needed",
	}

	g.sendResponse(w, http.StatusOK, "Metrics", stats)
}

// RequestValidator 请求验证器
type RequestValidator struct{}

// ValidateCreateUserRequest 验证创建用户请求
func (rv *RequestValidator) ValidateCreateUserRequest(req map[string]interface{}) error {
	// 验证必需字段
	requiredFields := []string{"username", "email", "phone"}
	for _, field := range requiredFields {
		if _, exists := req[field]; !exists {
			return fmt.Errorf("missing required field: %s", field)
		}
	}

	// 验证字段格式
	if username, ok := req["username"].(string); ok {
		if len(username) < 3 || len(username) > 20 {
			return fmt.Errorf("username must be between 3 and 20 characters")
		}
	} else {
		return fmt.Errorf("username must be a string")
	}

	// 验证邮箱格式（简单验证）
	if email, ok := req["email"].(string); ok {
		if !strings.Contains(email, "@") {
			return fmt.Errorf("invalid email format")
		}
	} else {
		return fmt.Errorf("email must be a string")
	}

	// 验证年龄
	if age, exists := req["age"]; exists {
		if ageFloat, ok := age.(float64); ok {
			if ageFloat < 0 || ageFloat > 120 {
				return fmt.Errorf("age must be between 0 and 120")
			}
		} else {
			return fmt.Errorf("age must be a number")
		}
	}

	return nil
}

// ValidateCreateOrderRequest 验证创建订单请求
func (rv *RequestValidator) ValidateCreateOrderRequest(req map[string]interface{}) error {
	// 验证必需字段
	requiredFields := []string{"user_id", "amount"}
	for _, field := range requiredFields {
		if _, exists := req[field]; !exists {
			return fmt.Errorf("missing required field: %s", field)
		}
	}

	// 验证用户ID
	if userID, ok := req["user_id"].(float64); ok {
		if userID <= 0 {
			return fmt.Errorf("user_id must be positive")
		}
	} else {
		return fmt.Errorf("user_id must be a number")
	}

	// 验证金额
	if amount, ok := req["amount"].(float64); ok {
		if amount <= 0 {
			return fmt.Errorf("amount must be positive")
		}
	} else {
		return fmt.Errorf("amount must be a number")
	}

	return nil
}
