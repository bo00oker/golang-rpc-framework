package trace

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/metadata"
)

// Config 追踪配置
type Config struct {
	ServiceName    string `mapstructure:"service_name"`
	ServiceVersion string `mapstructure:"service_version"`
	Environment    string `mapstructure:"environment"`

	// Jaeger 配置
	JaegerEndpoint string `mapstructure:"jaeger_endpoint"`

	// 采样配置
	SampleRate float64 `mapstructure:"sample_rate"`

	// 是否启用控制台输出
	EnableConsole bool `mapstructure:"enable_console"`
}

// Tracer 追踪器
type Tracer struct {
	tracer trace.Tracer
	config *Config
	tp     *sdktrace.TracerProvider
}

// NewTracer 创建追踪器
func NewTracer(config *Config) (*Tracer, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// 验证配置
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// 创建资源
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceName(config.ServiceName),
			semconv.ServiceVersion(config.ServiceVersion),
			attribute.String("environment", config.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// 创建导出器
	exporters, err := createExporters(config)
	if err != nil {
		return nil, err
	}

	// 创建 TracerProvider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporters[0]),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(config.SampleRate)),
	)

	// 设置全局 TracerProvider
	otel.SetTracerProvider(tp)

	// 设置全局传播器
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// 创建追踪器
	tracer := tp.Tracer(config.ServiceName)

	return &Tracer{
		tracer: tracer,
		config: config,
		tp:     tp,
	}, nil
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		ServiceName:    "rpc-framework",
		ServiceVersion: "v1.0.0",
		Environment:    "development",
		SampleRate:     1.0,
		EnableConsole:  true,
	}
}

// validateConfig 验证配置
func validateConfig(config *Config) error {
	if config.ServiceName == "" {
		return fmt.Errorf("service name is required")
	}
	if config.SampleRate < 0 || config.SampleRate > 1 {
		return fmt.Errorf("sample rate must be between 0 and 1")
	}
	return nil
}

// createExporters 创建导出器
func createExporters(config *Config) ([]sdktrace.SpanExporter, error) {
	var exporters []sdktrace.SpanExporter

	// 控制台导出器
	if config.EnableConsole {
		consoleExporter, err := stdouttrace.New(
			stdouttrace.WithPrettyPrint(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create console exporter: %w", err)
		}
		exporters = append(exporters, consoleExporter)
	}

	// Jaeger 导出器
	if config.JaegerEndpoint != "" {
		jaegerExporter, err := jaeger.New(
			jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(config.JaegerEndpoint)),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create jaeger exporter: %w", err)
		}
		exporters = append(exporters, jaegerExporter)
	}

	if len(exporters) == 0 {
		return nil, fmt.Errorf("no exporters configured")
	}

	return exporters, nil
}

// StartSpan 开始一个新的 span
func (t *Tracer) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, name, opts...)
}

// ExtractTraceInfo 从 gRPC metadata 中提取追踪信息
func (t *Tracer) ExtractTraceInfo(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}

	// 提取追踪上下文
	ctx = otel.GetTextMapPropagator().Extract(ctx, MetadataCarrier(md))
	return ctx
}

// InjectTraceInfo 将追踪信息注入到 gRPC metadata 中
func (t *Tracer) InjectTraceInfo(ctx context.Context) context.Context {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	}

	// 注入追踪上下文
	otel.GetTextMapPropagator().Inject(ctx, MetadataCarrier(md))
	return metadata.NewOutgoingContext(ctx, md)
}

// Shutdown 关闭追踪器
func (t *Tracer) Shutdown(ctx context.Context) error {
	if t.tp != nil {
		return t.tp.Shutdown(ctx)
	}
	return nil
}

// MetadataCarrier 实现 propagation.TextMapCarrier 接口
type MetadataCarrier metadata.MD

// Get 获取值
func (mc MetadataCarrier) Get(key string) string {
	values := metadata.MD(mc).Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

// Set 设置值
func (mc MetadataCarrier) Set(key, value string) {
	metadata.MD(mc).Set(key, value)
}

// Keys 获取所有键
func (mc MetadataCarrier) Keys() []string {
	keys := make([]string, 0, len(mc))
	for k := range mc {
		keys = append(keys, k)
	}
	return keys
}

// SpanFromContext 从上下文中获取 span
func SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// TraceIDFromContext 从上下文中获取 trace ID
func TraceIDFromContext(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

// SpanIDFromContext 从上下文中获取 span ID
func SpanIDFromContext(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return span.SpanContext().SpanID().String()
	}
	return ""
}
