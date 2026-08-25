package interceptor

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// LoggingOption 自定义 Logging 拦截器行为。
type LoggingOption func(*loggingConfig)

type loggingConfig struct {
	successLevel slog.Level
}

// WithSuccessLevel 覆盖成功请求的日志级别（默认 Debug）。
func WithSuccessLevel(level slog.Level) LoggingOption {
	return func(cfg *loggingConfig) { cfg.successLevel = level }
}

// ParseLogLevel 解析日志级别字符串（debug|info|warn|error），非法值回落 def 并记录告警。
func ParseLogLevel(raw string, def slog.Level, logger *slog.Logger) slog.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	case "":
		return def
	default:
		if logger != nil {
			logger.Warn("invalid grpc access log level; using default", "value", raw, "default", def.String())
		}
		return def
	}
}

// Logging 返回记录 gRPC 请求的拦截器。
func Logging(logger *slog.Logger, opts ...LoggingOption) grpc.UnaryServerInterceptor {
	cfg := loggingConfig{successLevel: slog.LevelDebug}
	for _, opt := range opts {
		opt(&cfg)
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()

		resp, err := handler(ctx, req)

		duration := time.Since(start)
		code := status.Code(err)

		level := cfg.successLevel
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
func StreamLogging(logger *slog.Logger, opts ...LoggingOption) grpc.StreamServerInterceptor {
	cfg := loggingConfig{successLevel: slog.LevelDebug}
	for _, opt := range opts {
		opt(&cfg)
	}
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()

		err := handler(srv, ss)

		duration := time.Since(start)
		code := status.Code(err)

		level := cfg.successLevel
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
