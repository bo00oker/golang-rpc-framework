package security

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rpc-framework/core/pkg/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// AuthConfig 认证配置
type AuthConfig struct {
	JWTSecret     string        `yaml:"jwt_secret" mapstructure:"jwt_secret"`
	TokenExpiry   time.Duration `yaml:"token_expiry" mapstructure:"token_expiry"`
	RefreshExpiry time.Duration `yaml:"refresh_expiry" mapstructure:"refresh_expiry"`
	Issuer        string        `yaml:"issuer" mapstructure:"issuer"`
	EnableTLS     bool          `yaml:"enable_tls" mapstructure:"enable_tls"`
	CertFile      string        `yaml:"cert_file" mapstructure:"cert_file"`
	KeyFile       string        `yaml:"key_file" mapstructure:"key_file"`
}

// Claims JWT声明
type Claims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// TokenPair 令牌对
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// AuthService 认证服务
type AuthService struct {
	config     *AuthConfig
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	logger     logger.Logger
}

// NewAuthService 创建认证服务
func NewAuthService(config *AuthConfig) (*AuthService, error) {
	// 生成RSA密钥对
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate RSA key: %w", err)
	}

	return &AuthService{
		config:     config,
		privateKey: privateKey,
		publicKey:  &privateKey.PublicKey,
		logger:     logger.GetGlobalLogger(),
	}, nil
}

// GenerateTokens 生成令牌对
func (a *AuthService) GenerateTokens(userID int64, username, role string) (*TokenPair, error) {
	now := time.Now()

	// 访问令牌声明
	accessClaims := &Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    a.config.Issuer,
			Subject:   fmt.Sprintf("user:%d", userID),
			Audience:  []string{"rpc-service"},
			ExpiresAt: jwt.NewNumericDate(now.Add(a.config.TokenExpiry)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	// 生成访问令牌
	accessToken := jwt.NewWithClaims(jwt.SigningMethodRS256, accessClaims)
	accessTokenString, err := accessToken.SignedString(a.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	// 刷新令牌声明
	refreshClaims := &Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    a.config.Issuer,
			Subject:   fmt.Sprintf("user:%d", userID),
			Audience:  []string{"rpc-service"},
			ExpiresAt: jwt.NewNumericDate(now.Add(a.config.RefreshExpiry)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	// 生成刷新令牌
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodRS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString(a.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
		ExpiresIn:    int64(a.config.TokenExpiry.Seconds()),
	}, nil
}

// ValidateToken 验证令牌
func (a *AuthService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return a.publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// RefreshTokens 刷新令牌
func (a *AuthService) RefreshTokens(refreshToken string) (*TokenPair, error) {
	claims, err := a.ValidateToken(refreshToken)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	// 生成新的令牌对
	return a.GenerateTokens(claims.UserID, claims.Username, claims.Role)
}

// GetPublicKeyPEM 获取公钥PEM格式
func (a *AuthService) GetPublicKeyPEM() (string, error) {
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(a.publicKey)
	if err != nil {
		return "", fmt.Errorf("failed to marshal public key: %w", err)
	}

	pubKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyBytes,
	})

	return string(pubKeyPEM), nil
}

// AuthInterceptor 认证拦截器
type AuthInterceptor struct {
	authService *AuthService
	logger      logger.Logger
	// 白名单方法，不需要认证
	whitelist map[string]bool
}

// NewAuthInterceptor 创建认证拦截器
func NewAuthInterceptor(authService *AuthService) *AuthInterceptor {
	whitelist := map[string]bool{
		"/user.UserService/CreateUser": true, // 注册用户
		"/auth.AuthService/Login":      true, // 登录
		"/auth.AuthService/Refresh":    true, // 刷新令牌
		"/health.HealthService/Check":  true, // 健康检查
	}

	return &AuthInterceptor{
		authService: authService,
		logger:      logger.GetGlobalLogger(),
		whitelist:   whitelist,
	}
}

// UnaryInterceptor 一元RPC认证拦截器
func (i *AuthInterceptor) UnaryInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	// 检查是否在白名单中
	if i.whitelist[info.FullMethod] {
		return handler(ctx, req)
	}

	// 验证认证
	claims, err := i.authenticate(ctx)
	if err != nil {
		i.logger.Warnf("Authentication failed for method %s: %v", info.FullMethod, err)
		return nil, status.Errorf(codes.Unauthenticated, "authentication failed: %v", err)
	}

	// 将用户信息添加到context中
	ctx = context.WithValue(ctx, "user_id", claims.UserID)
	ctx = context.WithValue(ctx, "username", claims.Username)
	ctx = context.WithValue(ctx, "role", claims.Role)

	return handler(ctx, req)
}

// StreamInterceptor 流式RPC认证拦截器
func (i *AuthInterceptor) StreamInterceptor(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	// 检查是否在白名单中
	if i.whitelist[info.FullMethod] {
		return handler(srv, ss)
	}

	// 验证认证
	claims, err := i.authenticate(ss.Context())
	if err != nil {
		i.logger.Warnf("Authentication failed for stream method %s: %v", info.FullMethod, err)
		return status.Errorf(codes.Unauthenticated, "authentication failed: %v", err)
	}

	// 包装ServerStream以添加用户信息
	wrappedStream := &AuthenticatedServerStream{
		ServerStream: ss,
		ctx:          context.WithValue(ss.Context(), "user_id", claims.UserID),
	}

	return handler(srv, wrappedStream)
}

// authenticate 执行认证
func (i *AuthInterceptor) authenticate(ctx context.Context) (*Claims, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, fmt.Errorf("missing metadata")
	}

	authHeaders := md.Get("authorization")
	if len(authHeaders) == 0 {
		return nil, fmt.Errorf("missing authorization header")
	}

	authHeader := authHeaders[0]
	if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
		return nil, fmt.Errorf("invalid authorization header format")
	}

	token := authHeader[7:]
	return i.authService.ValidateToken(token)
}

// AuthenticatedServerStream 带认证信息的服务器流
type AuthenticatedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

// Context 返回带认证信息的context
func (s *AuthenticatedServerStream) Context() context.Context {
	return s.ctx
}

// RBAC 基于角色的访问控制
type RBAC struct {
	permissions map[string][]string // role -> permissions
	logger      logger.Logger
}

// NewRBAC 创建RBAC实例
func NewRBAC() *RBAC {
	permissions := map[string][]string{
		"admin": {
			"user:create", "user:read", "user:update", "user:delete",
			"order:create", "order:read", "order:update", "order:delete",
		},
		"user": {
			"user:read", "user:update", // 只能操作自己的信息
			"order:create", "order:read", // 能创建和查看订单
		},
		"guest": {
			"user:read", // 只能查看基本信息
		},
	}

	return &RBAC{
		permissions: permissions,
		logger:      logger.GetGlobalLogger(),
	}
}

// HasPermission 检查权限
func (r *RBAC) HasPermission(role, permission string) bool {
	if perms, exists := r.permissions[role]; exists {
		for _, perm := range perms {
			if perm == permission {
				return true
			}
		}
	}
	return false
}

// AuthorizeInterceptor 授权拦截器
func (r *RBAC) AuthorizeInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	// 从context获取用户角色
	role, ok := ctx.Value("role").(string)
	if !ok {
		return nil, status.Errorf(codes.PermissionDenied, "missing user role")
	}

	// 根据方法名映射权限
	permission := r.mapMethodToPermission(info.FullMethod)
	if permission == "" {
		// 如果没有映射，允许访问
		return handler(ctx, req)
	}

	// 检查权限
	if !r.HasPermission(role, permission) {
		r.logger.Warnf("Access denied: role %s lacks permission %s for method %s",
			role, permission, info.FullMethod)
		return nil, status.Errorf(codes.PermissionDenied,
			"insufficient permissions: %s", permission)
	}

	return handler(ctx, req)
}

// mapMethodToPermission 映射方法到权限
func (r *RBAC) mapMethodToPermission(method string) string {
	methodPermissions := map[string]string{
		"/user.UserService/CreateUser": "user:create",
		"/user.UserService/GetUser":    "user:read",
		"/user.UserService/UpdateUser": "user:update",
		"/user.UserService/DeleteUser": "user:delete",
		"/user.UserService/ListUsers":  "user:read",

		"/order.OrderService/CreateOrder":     "order:create",
		"/order.OrderService/GetOrder":        "order:read",
		"/order.OrderService/UpdateOrder":     "order:update",
		"/order.OrderService/DeleteOrder":     "order:delete",
		"/order.OrderService/ListOrders":      "order:read",
		"/order.OrderService/GetOrdersByUser": "order:read",
	}

	if perm, exists := methodPermissions[method]; exists {
		return perm
	}

	return ""
}

// GetUserFromContext 从context获取用户信息
func GetUserFromContext(ctx context.Context) (userID int64, username string, role string, ok bool) {
	userID, ok1 := ctx.Value("user_id").(int64)
	username, ok2 := ctx.Value("username").(string)
	role, ok3 := ctx.Value("role").(string)

	return userID, username, role, ok1 && ok2 && ok3
}
