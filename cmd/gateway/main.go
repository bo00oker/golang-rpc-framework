package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rpc-framework/core/pkg/config"
	"github.com/rpc-framework/core/pkg/gateway"
	"github.com/rpc-framework/core/pkg/logger"
	"github.com/rpc-framework/core/pkg/metrics"
	"github.com/rpc-framework/core/pkg/registry"
	"github.com/rpc-framework/core/pkg/security"
	"github.com/rpc-framework/core/pkg/trace"
)

func main() {
	// 加载配置
	cfg := config.New()
	if err := cfg.LoadFromFile("./configs/app.yaml"); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 获取日志配置
	logCfg, err := cfg.GetLogConfig()
	if err != nil {
		log.Fatalf("Failed to get log config: %v", err)
	}

	// 初始化日志
	globalLogger, err := logger.NewLogger(&logger.Config{
		Level:      logCfg.Level,
		Format:     logCfg.Format,
		Output:     logCfg.Output,
		FilePath:   logCfg.Filename,
		MaxSize:    logCfg.MaxSize,
		MaxBackups: logCfg.MaxBackups,
		MaxAge:     logCfg.MaxAge,
		Compress:   logCfg.Compress,
	})
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	logger.SetGlobalLogger(globalLogger)
	globalLogger.Info("Starting API Gateway...")

	// 获取追踪配置
	traceCfg, err := cfg.GetTraceConfig()
	if err != nil {
		globalLogger.Errorf("Failed to get trace config: %v", err)
		os.Exit(1)
	}

	// 初始化追踪器
	tracer, err := trace.NewTracer(&trace.Config{
		ServiceName:    traceCfg.ServiceName + "-gateway",
		ServiceVersion: traceCfg.ServiceVersion,
		Environment:    traceCfg.Environment,
		JaegerEndpoint: traceCfg.JaegerEndpoint,
		SampleRate:     traceCfg.SampleRate,
		EnableConsole:  traceCfg.EnableConsole,
	})
	if err != nil {
		globalLogger.Errorf("Failed to initialize tracer: %v", err)
		// 不用退出，可以不使用追踪
		tracer = nil
	}
	if tracer != nil {
		defer tracer.Shutdown(context.Background())
	}

	// 初始化注册中心
	registryClient, err := createRegistry(cfg)
	if err != nil {
		globalLogger.Fatalf("Failed to create registry: %v", err)
	}
	defer registryClient.Close()

	// 初始化认证服务
	authConfig := &security.AuthConfig{
		JWTSecret:     getStringWithDefault(cfg.GetString("security.jwt_secret"), "default-secret"),
		TokenExpiry:   getDurationWithDefault(cfg.GetDuration("security.token_expiry"), time.Hour),
		RefreshExpiry: getDurationWithDefault(cfg.GetDuration("security.refresh_expiry"), 24*time.Hour),
		Issuer:        getStringWithDefault(cfg.GetString("security.issuer"), "api-gateway"),
		EnableTLS:     cfg.GetBool("security.enable_tls"),
		CertFile:      cfg.GetString("security.cert_file"),
		KeyFile:       cfg.GetString("security.key_file"),
	}

	authService, err := security.NewAuthService(authConfig)
	if err != nil {
		globalLogger.Fatalf("Failed to create auth service: %v", err)
	}

	// 初始化指标收集
	metricsConfig := &metrics.MetricsConfig{
		Enable:    getBoolWithDefault(cfg.GetBool("metrics.enable"), true),
		Port:      getIntWithDefault(cfg.GetInt("metrics.port"), 9090),
		Path:      getStringWithDefault(cfg.GetString("metrics.path"), "/metrics"),
		Namespace: getStringWithDefault(cfg.GetString("metrics.namespace"), "gateway"),
		Subsystem: getStringWithDefault(cfg.GetString("metrics.subsystem"), "http"),
	}

	metricsCollector := metrics.NewMetrics(metricsConfig)
	if err := metricsCollector.Start(); err != nil {
		globalLogger.Fatalf("Failed to start metrics server: %v", err)
	}

	// 创建Gateway
	gatewayInstance, err := gateway.NewGateway(cfg, registryClient, authService, metricsCollector)
	if err != nil {
		globalLogger.Fatalf("Failed to create gateway: %v", err)
	}

	// 启动Gateway
	go func() {
		gatewayPort := getIntWithDefault(cfg.GetInt("gateway.port"), 8080)
		globalLogger.Infof("API Gateway listening on port %d", gatewayPort)
		if err := gatewayInstance.Start(); err != nil {
			globalLogger.Fatalf("Failed to start gateway: %v", err)
		}
	}()

	// 等待中断信号
	waitForShutdown(gatewayInstance, globalLogger)
}

// createRegistry 创建注册中心客户端
func createRegistry(cfg *config.Config) (registry.Registry, error) {
	registryType := getStringWithDefault(cfg.GetString("registry.type"), "nacos")

	switch registryType {
	case "nacos":
		nacosCfg, err := cfg.GetNacosConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to get nacos config: %w", err)
		}
		return registry.NewNacosRegistry(&registry.NacosOptions{
			Endpoints:   []string{nacosCfg.ServerAddr},
			NamespaceID: nacosCfg.Namespace,
			Group:       nacosCfg.Group,
			TimeoutMs:   uint64(nacosCfg.Timeout.Milliseconds()),
			Username:    nacosCfg.Username,
			Password:    nacosCfg.Password,
		})
	case "etcd":
		return registry.NewEtcdRegistry(&registry.EtcdOptions{
			Endpoints:   cfg.GetStringSlice("etcd.endpoints"),
			Timeout:     getDurationWithDefault(cfg.GetDuration("etcd.timeout"), 5*time.Second),
			DialTimeout: getDurationWithDefault(cfg.GetDuration("etcd.dial_timeout"), 5*time.Second),
			KeyPrefix:   getStringWithDefault(cfg.GetString("etcd.key_prefix"), "/services"),
		})
	default:
		return nil, fmt.Errorf("unsupported registry type: %s", registryType)
	}
}

// waitForShutdown 等待关闭信号
func waitForShutdown(gateway *gateway.Gateway, logger logger.Logger) {
	// 创建信号通道
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// 等待信号
	sig := <-sigCh
	logger.Infof("Received signal: %v, shutting down...", sig)

	// 创建关闭上下文
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 优雅关闭Gateway
	if err := gateway.Stop(ctx); err != nil {
		logger.Errorf("Failed to stop gateway gracefully: %v", err)
		os.Exit(1)
	}

	logger.Info("Gateway shutdown completed")
}

// 辅助函数
func getStringWithDefault(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

func getIntWithDefault(value, defaultValue int) int {
	if value == 0 {
		return defaultValue
	}
	return value
}

func getDurationWithDefault(value, defaultValue time.Duration) time.Duration {
	if value == 0 {
		return defaultValue
	}
	return value
}

func getBoolWithDefault(value bool, defaultValue bool) bool {
	// 对于bool类型，false是零值，但可能是有意设置的
	// 这里简化处理，可以考虑使用指针类型来区分零值和未设置
	return value
}

func getFloat64WithDefault(value, defaultValue float64) float64 {
	if value == 0 {
		return defaultValue
	}
	return value
}
