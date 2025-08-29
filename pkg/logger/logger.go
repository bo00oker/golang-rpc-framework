package logger

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/rpc-framework/core/pkg/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger 日志接口
type Logger interface {
	// 基本日志方法
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Warn(args ...interface{})
	Warnf(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})

	// 带context的日志方法（自动添加trace_id）
	DebugCtx(ctx context.Context, args ...interface{})
	DebugfCtx(ctx context.Context, format string, args ...interface{})
	InfoCtx(ctx context.Context, args ...interface{})
	InfofCtx(ctx context.Context, format string, args ...interface{})
	WarnCtx(ctx context.Context, args ...interface{})
	WarnfCtx(ctx context.Context, format string, args ...interface{})
	ErrorCtx(ctx context.Context, args ...interface{})
	ErrorfCtx(ctx context.Context, format string, args ...interface{})
	FatalCtx(ctx context.Context, args ...interface{})
	FatalfCtx(ctx context.Context, format string, args ...interface{})

	// 添加字段
	WithField(key string, value interface{}) Logger
	WithFields(fields map[string]interface{}) Logger
	WithTraceID(traceID string) Logger
}

// ZapLogger zap 实现
type ZapLogger struct {
	base    *zap.Logger
	sugared *zap.SugaredLogger
}

// Config 日志配置
type Config struct {
	Level      string `json:"level"`
	Format     string `json:"format"`
	Output     string `json:"output"`
	FilePath   string `json:"file_path"`
	MaxSize    int    `json:"max_size"`
	MaxBackups int    `json:"max_backups"`
	MaxAge     int    `json:"max_age"`
	Compress   bool   `json:"compress"`
	// 是否显示代码行号
	ShowCaller bool `json:"show_caller"`
	// 是否自动添加trace_id
	EnableTraceID bool `json:"enable_trace_id"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Level:         "info",
		Format:        "json",
		Output:        "stdout",
		MaxSize:       100,
		MaxBackups:    3,
		MaxAge:        28,
		Compress:      true,
		ShowCaller:    true, // 默认显示代码行号
		EnableTraceID: true, // 默认启用trace_id
	}
}

// NewLogger 创建新的日志实例
func NewLogger(config *Config) (Logger, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// 级别
	var lvl zapcore.Level
	switch config.Level {
	case "debug":
		lvl = zapcore.DebugLevel
	case "info":
		lvl = zapcore.InfoLevel
	case "warn", "warning":
		lvl = zapcore.WarnLevel
	case "error":
		lvl = zapcore.ErrorLevel
	case "fatal":
		lvl = zapcore.FatalLevel
	default:
		return nil, fmt.Errorf("invalid log level: %s", config.Level)
	}

	// 编码器
	encCfg := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stack",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     func(t time.Time, pae zapcore.PrimitiveArrayEncoder) { pae.AppendString(t.Format(time.RFC3339)) },
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// 根据配置决定是否显示调用者信息
	if !config.ShowCaller {
		encCfg.CallerKey = zapcore.OmitKey
	}
	var encoder zapcore.Encoder
	switch config.Format {
	case "json":
		encoder = zapcore.NewJSONEncoder(encCfg)
	case "text", "console":
		encoder = zapcore.NewConsoleEncoder(encCfg)
	default:
		return nil, fmt.Errorf("unsupported log format: %s", config.Format)
	}

	ws, err := buildWriteSyncer(config)
	if err != nil {
		return nil, err
	}

	core := zapcore.NewCore(encoder, ws, lvl)

	// 添加调用者信息和跳过指定的调用层次
	options := []zap.Option{}
	if config.ShowCaller {
		options = append(options, zap.AddCaller(), zap.AddCallerSkip(1))
	}

	base := zap.New(core, options...)
	sug := base.Sugar()
	return &ZapLogger{base: base, sugared: sug}, nil
}

func buildWriteSyncer(config *Config) (zapcore.WriteSyncer, error) {
	switch config.Output {
	case "stdout":
		return zapcore.AddSync(os.Stdout), nil
	case "stderr":
		return zapcore.AddSync(os.Stderr), nil
	case "file":
		if config.FilePath == "" {
			return nil, fmt.Errorf("file path is required when output is file")
		}
		dir := filepath.Dir(config.FilePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}
		rotator := &lumberjack.Logger{
			Filename:   config.FilePath,
			MaxSize:    config.MaxSize, // MB
			MaxBackups: config.MaxBackups,
			MaxAge:     config.MaxAge, // days
			Compress:   config.Compress,
		}
		return zapcore.AddSync(rotator), nil
	default:
		return nil, fmt.Errorf("unsupported output type: %s", config.Output)
	}
}

// Debug 调试日志
func (l *ZapLogger) Debug(args ...interface{}) { l.logWithCaller(zapcore.DebugLevel, "", args...) }

// Debugf 格式化调试日志
func (l *ZapLogger) Debugf(format string, args ...interface{}) {
	l.logWithCallerf(zapcore.DebugLevel, format, args...)
}

// Info 信息日志
func (l *ZapLogger) Info(args ...interface{}) { l.logWithCaller(zapcore.InfoLevel, "", args...) }

// Infof 格式化信息日志
func (l *ZapLogger) Infof(format string, args ...interface{}) {
	l.logWithCallerf(zapcore.InfoLevel, format, args...)
}

// Warn 警告日志
func (l *ZapLogger) Warn(args ...interface{}) { l.logWithCaller(zapcore.WarnLevel, "", args...) }

// Warnf 格式化警告日志
func (l *ZapLogger) Warnf(format string, args ...interface{}) {
	l.logWithCallerf(zapcore.WarnLevel, format, args...)
}

// Error 错误日志
func (l *ZapLogger) Error(args ...interface{}) { l.logWithCaller(zapcore.ErrorLevel, "", args...) }

// Errorf 格式化错误日志
func (l *ZapLogger) Errorf(format string, args ...interface{}) {
	l.logWithCallerf(zapcore.ErrorLevel, format, args...)
}

// Fatal 致命错误日志
func (l *ZapLogger) Fatal(args ...interface{}) { l.logWithCaller(zapcore.FatalLevel, "", args...) }

// Fatalf 格式化致命错误日志
func (l *ZapLogger) Fatalf(format string, args ...interface{}) {
	l.logWithCallerf(zapcore.FatalLevel, format, args...)
}

// WithField 添加字段
func (l *ZapLogger) WithField(key string, value interface{}) Logger {
	return &ZapLogger{base: l.base, sugared: l.sugared.With(key, value)}
}

// WithFields 添加多个字段
func (l *ZapLogger) WithFields(fields map[string]interface{}) Logger {
	kv := make([]interface{}, 0, len(fields)*2)
	for k, v := range fields {
		kv = append(kv, k, v)
	}
	return &ZapLogger{base: l.base, sugared: l.sugared.With(kv...)}
}

// WithTraceID 添加trace_id字段
func (l *ZapLogger) WithTraceID(traceID string) Logger {
	if traceID == "" {
		return l
	}
	return &ZapLogger{base: l.base, sugared: l.sugared.With("trace_id", traceID)}
}

// ========== 带Context的日志方法（自动添加trace_id） ==========

// DebugCtx 带context的调试日志
func (l *ZapLogger) DebugCtx(ctx context.Context, args ...interface{}) {
	l.logWithCallerCtx(ctx, zapcore.DebugLevel, "", args...)
}

// DebugfCtx 带context的格式化调试日志
func (l *ZapLogger) DebugfCtx(ctx context.Context, format string, args ...interface{}) {
	l.logWithCallerfCtx(ctx, zapcore.DebugLevel, format, args...)
}

// InfoCtx 带context的信息日志
func (l *ZapLogger) InfoCtx(ctx context.Context, args ...interface{}) {
	l.logWithCallerCtx(ctx, zapcore.InfoLevel, "", args...)
}

// InfofCtx 带context的格式化信息日志
func (l *ZapLogger) InfofCtx(ctx context.Context, format string, args ...interface{}) {
	l.logWithCallerfCtx(ctx, zapcore.InfoLevel, format, args...)
}

// WarnCtx 带context的警告日志
func (l *ZapLogger) WarnCtx(ctx context.Context, args ...interface{}) {
	l.logWithCallerCtx(ctx, zapcore.WarnLevel, "", args...)
}

// WarnfCtx 带context的格式化警告日志
func (l *ZapLogger) WarnfCtx(ctx context.Context, format string, args ...interface{}) {
	l.logWithCallerfCtx(ctx, zapcore.WarnLevel, format, args...)
}

// ErrorCtx 带context的错误日志
func (l *ZapLogger) ErrorCtx(ctx context.Context, args ...interface{}) {
	l.logWithCallerCtx(ctx, zapcore.ErrorLevel, "", args...)
}

// ErrorfCtx 带context的格式化错误日志
func (l *ZapLogger) ErrorfCtx(ctx context.Context, format string, args ...interface{}) {
	l.logWithCallerfCtx(ctx, zapcore.ErrorLevel, format, args...)
}

// FatalCtx 带context的致命错误日志
func (l *ZapLogger) FatalCtx(ctx context.Context, args ...interface{}) {
	l.logWithCallerCtx(ctx, zapcore.FatalLevel, "", args...)
}

// FatalfCtx 带context的格式化致命错误日志
func (l *ZapLogger) FatalfCtx(ctx context.Context, format string, args ...interface{}) {
	l.logWithCallerfCtx(ctx, zapcore.FatalLevel, format, args...)
}

// ========== 私有辅助方法 ==========

// logWithCaller 带调用者信息的日志记录
func (l *ZapLogger) logWithCaller(level zapcore.Level, format string, args ...interface{}) {
	fields := []zap.Field{}

	// 添加调用者信息
	if pc, file, line, ok := runtime.Caller(2); ok {
		fields = append(fields, zap.String("file", formatCaller(file, line)))
		fields = append(fields, zap.String("func", formatFunction(pc)))
	}

	var msg string
	if format != "" {
		msg = fmt.Sprintf(format, args...)
	} else {
		msg = fmt.Sprint(args...)
	}

	l.base.Log(level, msg, fields...)
}

// logWithCallerf 带调用者信息的格式化日志记录
func (l *ZapLogger) logWithCallerf(level zapcore.Level, format string, args ...interface{}) {
	l.logWithCaller(level, format, args...)
}

// logWithCallerCtx 带调用者信息和context的日志记录
func (l *ZapLogger) logWithCallerCtx(ctx context.Context, level zapcore.Level, format string, args ...interface{}) {
	fields := []zap.Field{}

	// 添加trace_id
	if traceID := extractTraceID(ctx); traceID != "" {
		fields = append(fields, zap.String("trace_id", traceID))
	}

	// 添加调用者信息
	if pc, file, line, ok := runtime.Caller(2); ok {
		fields = append(fields, zap.String("file", formatCaller(file, line)))
		fields = append(fields, zap.String("func", formatFunction(pc)))
	}

	var msg string
	if format != "" {
		msg = fmt.Sprintf(format, args...)
	} else {
		msg = fmt.Sprint(args...)
	}

	l.base.Log(level, msg, fields...)
}

// logWithCallerfCtx 带调用者信息和context的格式化日志记录
func (l *ZapLogger) logWithCallerfCtx(ctx context.Context, level zapcore.Level, format string, args ...interface{}) {
	l.logWithCallerCtx(ctx, level, format, args...)
}

// extractTraceID 从上下文中提取trace_id
func extractTraceID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

// formatCaller 格式化调用者信息
func formatCaller(file string, line int) string {
	// 只显示文件名和行号
	if idx := strings.LastIndex(file, "/"); idx >= 0 {
		file = file[idx+1:]
	}
	return fmt.Sprintf("%s:%d", file, line)
}

// formatFunction 格式化函数名
func formatFunction(pc uintptr) string {
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return "unknown"
	}
	name := fn.Name()
	// 只显示函数名部分
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	if idx := strings.Index(name, "."); idx >= 0 {
		return name[idx+1:]
	}
	return name
}

// MultiWriter 多输出写入器
type MultiWriter struct{ writers []io.Writer }

// NewMultiWriter 创建多输出写入器
func NewMultiWriter(writers ...io.Writer) *MultiWriter { return &MultiWriter{writers: writers} }

// Write 写入数据到所有输出
func (mw *MultiWriter) Write(p []byte) (n int, err error) {
	for _, w := range mw.writers {
		n, err = w.Write(p)
		if err != nil {
			return
		}
	}
	return len(p), nil
}

// RotatingFileWriter 轮转文件写入器
type RotatingFileWriter struct {
	mu          sync.Mutex
	filePath    string
	maxSize     int64
	maxBackups  int
	maxAge      int
	compress    bool
	currentFile *os.File
	currentSize int64
}

// NewRotatingFileWriter 创建轮转文件写入器
func NewRotatingFileWriter(filePath string, maxSize, maxBackups, maxAge int, compress bool) (*RotatingFileWriter, error) {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}
	rfw := &RotatingFileWriter{filePath: filePath, maxSize: int64(maxSize) * 1024 * 1024, maxBackups: maxBackups, maxAge: maxAge, compress: compress}
	if err := rfw.openFile(); err != nil {
		return nil, err
	}
	return rfw, nil
}

// Write 写入数据
func (rfw *RotatingFileWriter) Write(p []byte) (n int, err error) {
	rfw.mu.Lock()
	defer rfw.mu.Unlock()
	if rfw.currentSize+int64(len(p)) > rfw.maxSize {
		if err := rfw.rotate(); err != nil {
			return 0, err
		}
	}
	n, err = rfw.currentFile.Write(p)
	if err != nil {
		return n, err
	}
	rfw.currentSize += int64(n)
	return n, nil
}

// Close 关闭文件
func (rfw *RotatingFileWriter) Close() error {
	rfw.mu.Lock()
	defer rfw.mu.Unlock()
	if rfw.currentFile != nil {
		return rfw.currentFile.Close()
	}
	return nil
}

// openFile 打开文件
func (rfw *RotatingFileWriter) openFile() error {
	file, err := os.OpenFile(rfw.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}
	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return err
	}
	rfw.currentFile = file
	rfw.currentSize = stat.Size()
	return nil
}

// rotate 轮转文件
func (rfw *RotatingFileWriter) rotate() error {
	if rfw.currentFile != nil {
		rfw.currentFile.Close()
	}
	timestamp := time.Now().Format("20060102-150405")
	backupPath := fmt.Sprintf("%s.%s", rfw.filePath, timestamp)
	if err := os.Rename(rfw.filePath, backupPath); err != nil {
		return err
	}
	if rfw.compress {
		go rfw.compressFile(backupPath)
	}
	go rfw.cleanupOldFiles()
	return rfw.openFile()
}

func (rfw *RotatingFileWriter) compressFile(filePath string) { _ = filePath }
func (rfw *RotatingFileWriter) cleanupOldFiles()             {}

// GlobalLogger 全局日志实例
var (
	globalLogger Logger
	globalMu     sync.RWMutex
)

// SetGlobalLogger 设置全局日志实例
func SetGlobalLogger(logger Logger) { globalMu.Lock(); defer globalMu.Unlock(); globalLogger = logger }

// GetGlobalLogger 获取全局日志实例
func GetGlobalLogger() Logger {
	globalMu.RLock()
	defer globalMu.RUnlock()
	if globalLogger == nil {
		logger, _ := NewLogger(DefaultConfig())
		return logger
	}
	return globalLogger
}

// 全局日志函数
func Debug(args ...interface{})                       { GetGlobalLogger().Debug(args...) }
func Debugf(format string, args ...interface{})       { GetGlobalLogger().Debugf(format, args...) }
func Info(args ...interface{})                        { GetGlobalLogger().Info(args...) }
func Infof(format string, args ...interface{})        { GetGlobalLogger().Infof(format, args...) }
func Warn(args ...interface{})                        { GetGlobalLogger().Warn(args...) }
func Warnf(format string, args ...interface{})        { GetGlobalLogger().Warnf(format, args...) }
func Error(args ...interface{})                       { GetGlobalLogger().Error(args...) }
func Errorf(format string, args ...interface{})       { GetGlobalLogger().Errorf(format, args...) }
func Fatal(args ...interface{})                       { GetGlobalLogger().Fatal(args...) }
func Fatalf(format string, args ...interface{})       { GetGlobalLogger().Fatalf(format, args...) }
func WithField(key string, value interface{}) Logger  { return GetGlobalLogger().WithField(key, value) }
func WithFields(fields map[string]interface{}) Logger { return GetGlobalLogger().WithFields(fields) }
