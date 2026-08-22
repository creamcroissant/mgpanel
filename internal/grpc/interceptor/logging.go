package interceptor

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// Logging 返回记录 gRPC 请求的拦截器。
func Logging(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()

		resp, err := handler(ctx, req)

		duration := time.Since(start)
		code := status.Code(err)

		level := slog.LevelDebug
		msg := "gRPC request"
		if code != 0 {
			level = slog.LevelWarn
			msg = "gRPC request error"
		}

		logger.LogAttrs(ctx, level, msg,
			slog.String("method", info.FullMethod),
			slog.Duration("duration", duration),
			slog.String("code", code.String()),
		)

		return resp, err
	}
}

// StreamLogging 返回记录 gRPC 流的拦截器。
func StreamLogging(logger *slog.Logger) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()

		err := handler(srv, ss)

		duration := time.Since(start)
		code := status.Code(err)

		level := slog.LevelDebug
		msg := "gRPC stream"
		if code != 0 {
			level = slog.LevelWarn
			msg = "gRPC stream error"
		}

		logger.LogAttrs(ss.Context(), level, msg,
			slog.String("method", info.FullMethod),
			slog.Duration("duration", duration),
			slog.String("code", code.String()),
		)

		return err
	}
}
