package api

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Version API版本
type Version struct {
	Major int
	Minor int
	Patch int
}

// String 返回版本字符串
func (v Version) String() string {
	return fmt.Sprintf("v%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// IsCompatible 检查版本兼容性
func (v Version) IsCompatible(other Version) bool {
	// 主版本必须相同
	if v.Major != other.Major {
		return false
	}

	// 次版本向后兼容
	if v.Minor < other.Minor {
		return false
	}

	return true
}

// ParseVersion 解析版本字符串
func ParseVersion(versionStr string) (Version, error) {
	versionStr = strings.TrimPrefix(versionStr, "v")
	parts := strings.Split(versionStr, ".")

	if len(parts) != 3 {
		return Version{}, fmt.Errorf("invalid version format: %s", versionStr)
	}

	var major, minor, patch int
	if _, err := fmt.Sscanf(versionStr, "%d.%d.%d", &major, &minor, &patch); err != nil {
		return Version{}, fmt.Errorf("failed to parse version: %w", err)
	}

	return Version{Major: major, Minor: minor, Patch: patch}, nil
}

// VersionManager 版本管理器
type VersionManager struct {
	currentVersion     Version
	supportedVersions  []Version
	deprecatedVersions map[string]bool
}

// NewVersionManager 创建版本管理器
func NewVersionManager(currentVersion Version) *VersionManager {
	return &VersionManager{
		currentVersion:     currentVersion,
		supportedVersions:  []Version{currentVersion},
		deprecatedVersions: make(map[string]bool),
	}
}

// AddSupportedVersion 添加支持的版本
func (vm *VersionManager) AddSupportedVersion(version Version) {
	vm.supportedVersions = append(vm.supportedVersions, version)
}

// DeprecateVersion 废弃版本
func (vm *VersionManager) DeprecateVersion(version Version) {
	vm.deprecatedVersions[version.String()] = true
}

// IsVersionSupported 检查版本是否支持
func (vm *VersionManager) IsVersionSupported(version Version) bool {
	for _, supported := range vm.supportedVersions {
		if supported == version {
			return true
		}
	}
	return false
}

// IsVersionDeprecated 检查版本是否已废弃
func (vm *VersionManager) IsVersionDeprecated(version Version) bool {
	return vm.deprecatedVersions[version.String()]
}

// GetCurrentVersion 获取当前版本
func (vm *VersionManager) GetCurrentVersion() Version {
	return vm.currentVersion
}

// GetSupportedVersions 获取支持的版本列表
func (vm *VersionManager) GetSupportedVersions() []Version {
	return vm.supportedVersions
}

// VersionInterceptor 版本拦截器
type VersionInterceptor struct {
	versionManager *VersionManager
}

// NewVersionInterceptor 创建版本拦截器
func NewVersionInterceptor(versionManager *VersionManager) *VersionInterceptor {
	return &VersionInterceptor{
		versionManager: versionManager,
	}
}

// UnaryInterceptor 一元RPC版本拦截器
func (vi *VersionInterceptor) UnaryInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	// 获取客户端版本
	clientVersion, err := vi.getClientVersion(ctx)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid version: %v", err)
	}

	// 检查版本是否支持
	if !vi.versionManager.IsVersionSupported(clientVersion) {
		return nil, status.Errorf(codes.Unimplemented,
			"version %s is not supported", clientVersion.String())
	}

	// 检查版本是否已废弃
	if vi.versionManager.IsVersionDeprecated(clientVersion) {
		// 添加警告头
		grpc.SendHeader(ctx, metadata.Pairs(
			"x-deprecated-version", clientVersion.String(),
			"x-current-version", vi.versionManager.GetCurrentVersion().String(),
		))
	}

	// 添加版本信息到context
	ctx = context.WithValue(ctx, "api_version", clientVersion)

	return handler(ctx, req)
}

// getClientVersion 从context获取客户端版本
func (vi *VersionInterceptor) getClientVersion(ctx context.Context) (Version, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		// 如果没有版本信息，使用当前版本
		return vi.versionManager.GetCurrentVersion(), nil
	}

	versions := md.Get("x-api-version")
	if len(versions) == 0 {
		// 如果没有版本信息，使用当前版本
		return vi.versionManager.GetCurrentVersion(), nil
	}

	return ParseVersion(versions[0])
}

// VersionedService 版本化服务接口
type VersionedService interface {
	GetSupportedVersions() []Version
	HandleRequest(ctx context.Context, version Version, req interface{}) (interface{}, error)
}

// ServiceRegistry 服务注册表
type ServiceRegistry struct {
	services map[string]map[Version]VersionedService
}

// NewServiceRegistry 创建服务注册表
func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{
		services: make(map[string]map[Version]VersionedService),
	}
}

// RegisterService 注册版本化服务
func (sr *ServiceRegistry) RegisterService(serviceName string, version Version, service VersionedService) {
	if sr.services[serviceName] == nil {
		sr.services[serviceName] = make(map[Version]VersionedService)
	}
	sr.services[serviceName][version] = service
}

// GetService 获取指定版本的服务
func (sr *ServiceRegistry) GetService(serviceName string, version Version) (VersionedService, error) {
	serviceVersions, exists := sr.services[serviceName]
	if !exists {
		return nil, fmt.Errorf("service %s not found", serviceName)
	}

	service, exists := serviceVersions[version]
	if !exists {
		return nil, fmt.Errorf("service %s version %s not found", serviceName, version.String())
	}

	return service, nil
}

// GetLatestService 获取服务的最新版本
func (sr *ServiceRegistry) GetLatestService(serviceName string) (VersionedService, Version, error) {
	serviceVersions, exists := sr.services[serviceName]
	if !exists {
		return nil, Version{}, fmt.Errorf("service %s not found", serviceName)
	}

	var latestVersion Version
	var latestService VersionedService

	for version, service := range serviceVersions {
		if version.Major > latestVersion.Major ||
			(version.Major == latestVersion.Major && version.Minor > latestVersion.Minor) ||
			(version.Major == latestVersion.Major && version.Minor == latestVersion.Minor && version.Patch > latestVersion.Patch) {
			latestVersion = version
			latestService = service
		}
	}

	return latestService, latestVersion, nil
}

// VersionRouter 版本路由器
type VersionRouter struct {
	registry       *ServiceRegistry
	versionManager *VersionManager
}

// NewVersionRouter 创建版本路由器
func NewVersionRouter(registry *ServiceRegistry, versionManager *VersionManager) *VersionRouter {
	return &VersionRouter{
		registry:       registry,
		versionManager: versionManager,
	}
}

// RouteRequest 路由请求到对应版本的服务
func (vr *VersionRouter) RouteRequest(ctx context.Context, serviceName string, req interface{}) (interface{}, error) {
	// 从context获取API版本
	version, ok := ctx.Value("api_version").(Version)
	if !ok {
		version = vr.versionManager.GetCurrentVersion()
	}

	// 获取服务
	service, err := vr.registry.GetService(serviceName, version)
	if err != nil {
		return nil, fmt.Errorf("failed to get service: %w", err)
	}

	// 处理请求
	return service.HandleRequest(ctx, version, req)
}

// CompatibilityLayer 兼容性层
type CompatibilityLayer struct {
	router *VersionRouter
}

// NewCompatibilityLayer 创建兼容性层
func NewCompatibilityLayer(router *VersionRouter) *CompatibilityLayer {
	return &CompatibilityLayer{
		router: router,
	}
}

// TransformRequest 转换请求以支持旧版本
func (cl *CompatibilityLayer) TransformRequest(ctx context.Context, version Version, req interface{}) (interface{}, error) {
	// 根据版本转换请求格式
	// 这里可以实现具体的转换逻辑
	return req, nil
}

// TransformResponse 转换响应以支持旧版本
func (cl *CompatibilityLayer) TransformResponse(ctx context.Context, version Version, resp interface{}) (interface{}, error) {
	// 根据版本转换响应格式
	// 这里可以实现具体的转换逻辑
	return resp, nil
}

// GetVersionFromContext 从context获取API版本
func GetVersionFromContext(ctx context.Context) (Version, bool) {
	version, ok := ctx.Value("api_version").(Version)
	return version, ok
}

// SetVersionInContext 在context中设置API版本
func SetVersionInContext(ctx context.Context, version Version) context.Context {
	return context.WithValue(ctx, "api_version", version)
}

// VersionMiddleware 版本中间件
func VersionMiddleware(versionManager *VersionManager) grpc.UnaryServerInterceptor {
	interceptor := NewVersionInterceptor(versionManager)
	return interceptor.UnaryInterceptor
}
