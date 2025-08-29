package main

import (
	"context"
	"time"

	"github.com/rpc-framework/core/pkg/logger"
	"github.com/rpc-framework/core/pkg/trace"
)

func main() {
	// 创建配置
	logConfig := &logger.Config{
		Level:         "info",
		Format:        "json",
		Output:        "stdout",
		ShowCaller:    true, // 显示代码行数
		EnableTraceID: true, // 启用trace_id
	}

	// 创建日志实例
	log, err := logger.NewLogger(logConfig)
	if err != nil {
		panic(err)
	}

	// 创建trace实例
	tracer, err := trace.NewTracer(trace.DefaultConfig())
	if err != nil {
		panic(err)
	}
	defer tracer.Shutdown(context.Background())

	// 创建context和span
	ctx := context.Background()
	ctx, span := tracer.StartSpan(ctx, "test-operation")
	defer span.End()

	// 测试基本日志功能（不带trace_id）
	log.Info("这是一条普通的信息日志")
	log.Infof("这是一条格式化的信息日志，当前时间：%s", time.Now().Format("2006-01-02 15:04:05"))

	// 测试错误日志
	log.Error("这是一条错误日志")
	log.Errorf("这是一条格式化的错误日志，错误代码：%d", 404)

	// 测试带context的日志功能（自动添加trace_id）
	log.InfoCtx(ctx, "这是一条带context的信息日志，会自动添加trace_id")
	log.InfofCtx(ctx, "这是一条带context的格式化日志，用户ID：%d", 12345)

	// 测试带字段的日志
	logWithFields := log.WithFields(map[string]interface{}{
		"user_id": 12345,
		"action":  "login",
		"ip":      "192.168.1.100",
	})
	logWithFields.InfoCtx(ctx, "用户登录成功")

	// 测试手动添加trace_id
	traceID := trace.TraceIDFromContext(ctx)
	logWithTrace := log.WithTraceID(traceID)
	logWithTrace.Info("这是一条手动添加trace_id的日志")

	// 模拟在函数中使用
	testFunction(ctx, log)
}

func testFunction(ctx context.Context, log logger.Logger) {
	// 在函数中使用带context的日志，会显示正确的代码行数
	log.InfoCtx(ctx, "在testFunction中记录日志")
	log.WarnCtx(ctx, "这是一条警告日志")

	// 嵌套函数调用
	nestedFunction(ctx, log)
}

func nestedFunction(ctx context.Context, log logger.Logger) {
	log.ErrorCtx(ctx, "嵌套函数中的错误日志")
	log.DebugCtx(ctx, "嵌套函数中的调试日志（可能不会显示，取决于日志级别）")
}
