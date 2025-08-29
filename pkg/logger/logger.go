package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger 日志接口
type Logger interface {
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
	WithField(key string, value interface{}) Logger
	WithFields(fields map[string]interface{}) Logger
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
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Level:      "info",
		Format:     "json",
		Output:     "stdout",
		MaxSize:    100,
		MaxBackups: 3,
		MaxAge:     28,
		Compress:   true,
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
	base := zap.New(core, zap.AddCaller())
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
func (l *ZapLogger) Debug(args ...interface{}) { l.sugared.Debug(args...) }

// Debugf 格式化调试日志
func (l *ZapLogger) Debugf(format string, args ...interface{}) { l.sugared.Debugf(format, args...) }

// Info 信息日志
func (l *ZapLogger) Info(args ...interface{}) { l.sugared.Info(args...) }

// Infof 格式化信息日志
func (l *ZapLogger) Infof(format string, args ...interface{}) { l.sugared.Infof(format, args...) }

// Warn 警告日志
func (l *ZapLogger) Warn(args ...interface{}) { l.sugared.Warn(args...) }

// Warnf 格式化警告日志
func (l *ZapLogger) Warnf(format string, args ...interface{}) { l.sugared.Warnf(format, args...) }

// Error 错误日志
func (l *ZapLogger) Error(args ...interface{}) { l.sugared.Error(args...) }

// Errorf 格式化错误日志
func (l *ZapLogger) Errorf(format string, args ...interface{}) { l.sugared.Errorf(format, args...) }

// Fatal 致命错误日志
func (l *ZapLogger) Fatal(args ...interface{}) { l.sugared.Fatal(args...) }

// Fatalf 格式化致命错误日志
func (l *ZapLogger) Fatalf(format string, args ...interface{}) { l.sugared.Fatalf(format, args...) }

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
