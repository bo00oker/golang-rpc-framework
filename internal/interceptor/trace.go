package interceptor

import (
	"context"
	"fmt"

	"github.com/rpc-framework/core/pkg/trace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// TraceServerInterceptor 服务端追踪拦截器
func TraceServerInterceptor(tracer *trace.Tracer) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// 提取追踪信息
		ctx = tracer.ExtractTraceInfo(ctx)

		// 开始 span
		spanCtx, span := tracer.StartSpan(ctx, fmt.Sprintf("%s/Unary", info.FullMethod),
			oteltrace.WithAttributes(
				attribute.String("grpc.method", info.FullMethod),
				attribute.String("grpc.type", "unary"),
			),
		)
		defer span.End()

		// 记录请求信息
		span.SetAttributes(
			attribute.String("request.type", fmt.Sprintf("%T", req)),
		)

		// 调用处理器
		resp, err := handler(spanCtx, req)

		// 记录响应信息
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			span.SetAttributes(
				attribute.String("error.message", err.Error()),
			)
			if st, ok := status.FromError(err); ok {
				span.SetAttributes(
					attribute.Int("grpc.status_code", int(st.Code())),
				)
			}
		} else {
			span.SetStatus(codes.Ok, "")
			span.SetAttributes(
				attribute.String("response.type", fmt.Sprintf("%T", resp)),
			)
		}

		return resp, err
	}
}

// TraceClientInterceptor 客户端追踪拦截器
func TraceClientInterceptor(tracer *trace.Tracer) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		// 注入追踪信息
		ctx = tracer.InjectTraceInfo(ctx)

		// 开始 span
		spanCtx, span := tracer.StartSpan(ctx, fmt.Sprintf("%s/Unary", method),
			oteltrace.WithAttributes(
				attribute.String("grpc.method", method),
				attribute.String("grpc.target", cc.Target()),
				attribute.String("grpc.type", "unary"),
			),
		)
		defer span.End()

		// 记录请求信息
		span.SetAttributes(
			attribute.String("request.type", fmt.Sprintf("%T", req)),
		)

		// 调用远程方法
		err := invoker(spanCtx, method, req, reply, cc, opts...)

		// 记录响应信息
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			span.SetAttributes(
				attribute.String("error.message", err.Error()),
			)
			if st, ok := status.FromError(err); ok {
				span.SetAttributes(
					attribute.Int("grpc.status_code", int(st.Code())),
				)
			}
		} else {
			span.SetStatus(codes.Ok, "")
			span.SetAttributes(
				attribute.String("response.type", fmt.Sprintf("%T", reply)),
			)
		}

		return err
	}
}

// TraceStreamServerInterceptor 服务端流式追踪拦截器
func TraceStreamServerInterceptor(tracer *trace.Tracer) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// 提取追踪信息
		ctx := tracer.ExtractTraceInfo(ss.Context())

		// 开始 span
		spanCtx, span := tracer.StartSpan(ctx, fmt.Sprintf("%s/Stream", info.FullMethod),
			oteltrace.WithAttributes(
				attribute.String("grpc.method", info.FullMethod),
				attribute.String("grpc.type", "stream"),
				attribute.Bool("grpc.is_client_stream", info.IsClientStream),
				attribute.Bool("grpc.is_server_stream", info.IsServerStream),
			),
		)
		defer span.End()

		// 包装 ServerStream
		wrappedStream := &traceServerStream{
			ServerStream: ss,
			ctx:          spanCtx,
		}

		// 调用处理器
		err := handler(srv, wrappedStream)

		// 记录错误信息
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			span.SetAttributes(
				attribute.String("error.message", err.Error()),
			)
			if st, ok := status.FromError(err); ok {
				span.SetAttributes(
					attribute.Int("grpc.status_code", int(st.Code())),
				)
			}
		} else {
			span.SetStatus(codes.Ok, "")
		}

		return err
	}
}

// TraceStreamClientInterceptor 客户端流式追踪拦截器
func TraceStreamClientInterceptor(tracer *trace.Tracer) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		// 注入追踪信息
		ctx = tracer.InjectTraceInfo(ctx)

		// 开始 span
		spanCtx, span := tracer.StartSpan(ctx, fmt.Sprintf("%s/Stream", method),
			oteltrace.WithAttributes(
				attribute.String("grpc.method", method),
				attribute.String("grpc.target", cc.Target()),
				attribute.String("grpc.type", "stream"),
				attribute.Bool("grpc.is_client_stream", desc.ClientStreams),
				attribute.Bool("grpc.is_server_stream", desc.ServerStreams),
			),
		)
		defer span.End()

		// 创建流
		clientStream, err := streamer(spanCtx, desc, cc, method, opts...)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			span.SetAttributes(
				attribute.String("error.message", err.Error()),
			)
			return nil, err
		}

		// 包装 ClientStream
		wrappedStream := &traceClientStream{
			ClientStream: clientStream,
			span:         span,
		}

		return wrappedStream, nil
	}
}

// traceServerStream 包装的 ServerStream
type traceServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

// Context 返回追踪上下文
func (tss *traceServerStream) Context() context.Context {
	return tss.ctx
}

// traceClientStream 包装的 ClientStream
type traceClientStream struct {
	grpc.ClientStream
	span oteltrace.Span
}

// SendMsg 发送消息时记录追踪信息
func (tcs *traceClientStream) SendMsg(m interface{}) error {
	err := tcs.ClientStream.SendMsg(m)
	if err != nil {
		tcs.span.SetStatus(codes.Error, err.Error())
		tcs.span.SetAttributes(
			attribute.String("error.message", err.Error()),
		)
	}
	return err
}

// RecvMsg 接收消息时记录追踪信息
func (tcs *traceClientStream) RecvMsg(m interface{}) error {
	err := tcs.ClientStream.RecvMsg(m)
	if err != nil {
		tcs.span.SetStatus(codes.Error, err.Error())
		tcs.span.SetAttributes(
			attribute.String("error.message", err.Error()),
		)
	}
	return err
}
