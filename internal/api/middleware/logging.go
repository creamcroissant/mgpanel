// 文件路径: internal/api/middleware/logging.go
// 模块说明: 增强日志中间件，支持请求追踪 ID 和慢请求日志
package middleware

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	chiMiddleware "github.com/go-chi/chi/v5/middleware"
)

// LoggingConfig 日志中间件配置
type LoggingConfig struct {
	Logger         *slog.Logger
	SlowThreshold  time.Duration // 慢请求阈值，超过此时间会记录为 WARN
	SkipPaths      []string      // 跳过日志的路径（如健康检查）
	LogRequestBody bool          // 是否记录请求体（仅开发模式）
}

// DefaultLoggingConfig 默认配置
func DefaultLoggingConfig() LoggingConfig {
	return LoggingConfig{
		Logger:         slog.Default(),
		SlowThreshold:  500 * time.Millisecond,
		SkipPaths:      []string{"/health", "/healthz", "/_internal/ready"},
		LogRequestBody: false,
	}
}

// sensitiveQueryParams 记录日志时需遮盖的敏感查询参数
var sensitiveQueryParams = map[string]bool{
	"token":        true,
	"access_token": true,
	"api_key":      true,
	"secret":       true,
}

// StructuredLogger 结构化日志中间件
func StructuredLogger(config LoggingConfig) func(http.Handler) http.Handler {
	if config.Logger == nil {
		config.Logger = slog.Default()
	}
	if config.SlowThreshold == 0 {
		config.SlowThreshold = 500 * time.Millisecond
	}

	skipPathMap := make(map[string]bool)
	for _, p := range config.SkipPaths {
		skipPathMap[p] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 跳过特定路径
			if skipPathMap[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()

			// 获取或生成请求 ID
			requestID := chiMiddleware.GetReqID(r.Context())
			if requestID == "" {
				requestID = "unknown"
			}

			// 包装 ResponseWriter 以捕获状态码
			ww := chiMiddleware.NewWrapResponseWriter(w, r.ProtoMajor)

			// 在响应头中设置请求 ID
			ww.Header().Set("X-Request-ID", requestID)

			// 处理请求
			next.ServeHTTP(ww, r)

			// 计算耗时
			duration := time.Since(start)
			status := ww.Status()
			if status == 0 {
				status = 200
			}

			// 构建日志字段
			attrs := []slog.Attr{
				slog.String("request_id", requestID),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", status),
				slog.Duration("duration", duration),
				slog.String("remote_addr", r.RemoteAddr),
				slog.Int("bytes", ww.BytesWritten()),
			}

			// 添加用户代理（如果有）
			if ua := r.Header.Get("User-Agent"); ua != "" {
				attrs = append(attrs, slog.String("user_agent", ua))
			}

			// 添加查询参数（如果有），过滤敏感参数
			if query := r.URL.Query(); len(query) > 0 {
				q := make(url.Values, len(query))
				for k, v := range query {
					if sensitiveQueryParams[strings.ToLower(k)] {
						q[k] = []string{"[REDACTED]"}
					} else {
						q[k] = v
					}
				}
				if encoded := q.Encode(); encoded != "" {
					attrs = append(attrs, slog.String("query", encoded))
				}
			}

			// 根据状态和耗时选择日志级别
			level := slog.LevelInfo
			msg := "request completed"

			if status >= 500 {
				level = slog.LevelError
				msg = "request failed"
			} else if status >= 400 {
				level = slog.LevelWarn
				msg = "request error"
			} else if duration > config.SlowThreshold {
				level = slog.LevelWarn
				msg = "slow request"
				attrs = append(attrs, slog.Duration("slow_threshold", config.SlowThreshold))
			}

			// 记录日志
			config.Logger.LogAttrs(r.Context(), level, msg, attrs...)
		})
	}
}

// RequestIDLogger 简单的请求 ID 日志中间件（不记录完整请求）
// 仅将请求 ID 添加到响应头
func RequestIDLogger() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := chiMiddleware.GetReqID(r.Context())
			if requestID != "" {
				w.Header().Set("X-Request-ID", requestID)
			}
			next.ServeHTTP(w, r)
		})
	}
}
