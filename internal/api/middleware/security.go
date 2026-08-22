// 文件路径: internal/api/middleware/security.go
// 模块说明: 安全中间件，包括 Rate Limiting、请求体大小限制、CORS
package middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RateLimiter 简单的内存限流器
type RateLimiter struct {
	mu       sync.Mutex
	requests map[string]*rateLimitEntry
	limit    int           // 请求限制
	window   time.Duration // 时间窗口
}

type rateLimitEntry struct {
	count   int
	resetAt time.Time
}

// NewRateLimiter 创建新的限流器
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string]*rateLimitEntry),
		limit:    limit,
		window:   window,
	}
	// 启动清理协程
	go rl.cleanup()
	return rl
}

// Allow 检查是否允许请求
func (rl *RateLimiter) Allow(key string) (bool, int, time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, exists := rl.requests[key]

	if !exists || now.After(entry.resetAt) {
		// 新窗口
		rl.requests[key] = &rateLimitEntry{
			count:   1,
			resetAt: now.Add(rl.window),
		}
		return true, rl.limit - 1, now.Add(rl.window)
	}

	if entry.count >= rl.limit {
		return false, 0, entry.resetAt
	}

	entry.count++
	return true, rl.limit - entry.count, entry.resetAt
}

// cleanup 定期清理过期条目
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for key, entry := range rl.requests {
			if now.After(entry.resetAt) {
				delete(rl.requests, key)
			}
		}
		rl.mu.Unlock()
	}
}

// RateLimitConfig Rate Limit 配置
type RateLimitConfig struct {
	Limit     int                        // 每个窗口的请求数
	Window    time.Duration              // 时间窗口
	KeyFunc   func(*http.Request) string // 获取限流 key 的函数
	SkipPaths []string                   // 跳过限流的路径
}

// DefaultRateLimitConfig 默认配置
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		Limit:  60,
		Window: time.Minute,
		KeyFunc: func(r *http.Request) string {
			// 默认按 IP 限流
			return getClientIP(r)
		},
		SkipPaths: []string{"/health", "/healthz", "/_internal/ready"},
	}
}

// RateLimit Rate Limiting 中间件
func RateLimit(config RateLimitConfig) func(http.Handler) http.Handler {
	if config.Limit == 0 {
		config.Limit = 60
	}
	if config.Window == 0 {
		config.Window = time.Minute
	}
	if config.KeyFunc == nil {
		config.KeyFunc = func(r *http.Request) string {
			return getClientIP(r)
		}
	}

	limiter := NewRateLimiter(config.Limit, config.Window)
	skipPaths := make(map[string]bool)
	for _, p := range config.SkipPaths {
		skipPaths[p] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 跳过特定路径
			if skipPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			key := config.KeyFunc(r)
			allowed, remaining, resetAt := limiter.Allow(key)

			// 设置 Rate Limit 响应头
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(config.Limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetAt.Unix(), 10))

			if !allowed {
				w.Header().Set("Retry-After", strconv.Itoa(int(time.Until(resetAt).Seconds())))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "rate_limit_exceeded"})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// BodyLimitConfig 请求体大小限制配置
type BodyLimitConfig struct {
	MaxBytes  int64    // 最大字节数
	SkipPaths []string // 跳过的路径
}

// DefaultBodyLimitConfig 默认配置（10MB）
func DefaultBodyLimitConfig() BodyLimitConfig {
	return BodyLimitConfig{
		MaxBytes:  10 * 1024 * 1024, // 10MB
		SkipPaths: []string{},
	}
}

// BodyLimit 请求体大小限制中间件
func BodyLimit(config BodyLimitConfig) func(http.Handler) http.Handler {
	if config.MaxBytes == 0 {
		config.MaxBytes = 10 * 1024 * 1024 // 10MB
	}

	skipPaths := make(map[string]bool)
	for _, p := range config.SkipPaths {
		skipPaths[p] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 跳过特定路径
			if skipPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			// 限制请求体大小
			r.Body = http.MaxBytesReader(w, r.Body, config.MaxBytes)

			next.ServeHTTP(w, r)
		})
	}
}

// CORSConfig CORS 配置
type CORSConfig struct {
	AllowedOrigins   []string // 允许的来源，"*" 表示所有
	AllowedMethods   []string // 允许的 HTTP 方法
	AllowedHeaders   []string // 允许的请求头
	ExposedHeaders   []string // 暴露给客户端的响应头
	AllowCredentials bool     // 是否允许携带凭证
	MaxAge           int      // 预检请求缓存时间（秒）
}

// DefaultCORSConfig 默认 CORS 配置
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Requested-With"},
		ExposedHeaders:   []string{"X-Request-ID", "X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset"},
		AllowCredentials: false,
		MaxAge:           86400, // 24 小时
	}
}

// CORS 跨域资源共享中间件
func CORS(config CORSConfig) func(http.Handler) http.Handler {
	if len(config.AllowedOrigins) == 0 {
		config.AllowedOrigins = []string{"*"}
	}
	if len(config.AllowedMethods) == 0 {
		config.AllowedMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"}
	}
	if len(config.AllowedHeaders) == 0 {
		config.AllowedHeaders = []string{"Accept", "Authorization", "Content-Type", "X-Requested-With"}
	}

	allowAll := len(config.AllowedOrigins) == 1 && config.AllowedOrigins[0] == "*"
	allowedOrigins := make(map[string]bool)
	for _, o := range config.AllowedOrigins {
		allowedOrigins[o] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// 检查来源是否允许
			var allowOrigin string
			if allowAll {
				if config.AllowCredentials {
					allowOrigin = origin
				} else {
					allowOrigin = "*"
				}
			} else if allowedOrigins[origin] {
				allowOrigin = origin
			}

			if allowOrigin != "" {
				w.Header().Set("Access-Control-Allow-Origin", allowOrigin)

				if config.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}

				if len(config.ExposedHeaders) > 0 {
					w.Header().Set("Access-Control-Expose-Headers", strings.Join(config.ExposedHeaders, ", "))
				}

				// 预检请求
				if r.Method == http.MethodOptions {
					w.Header().Set("Access-Control-Allow-Methods", strings.Join(config.AllowedMethods, ", "))
					w.Header().Set("Access-Control-Allow-Headers", strings.Join(config.AllowedHeaders, ", "))
					if config.MaxAge > 0 {
						w.Header().Set("Access-Control-Max-Age", strconv.Itoa(config.MaxAge))
					}
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeaders 设置安全响应头。
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "0")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}

// getClientIP 获取客户端真实 IP
func getClientIP(r *http.Request) string {
	// Prefer RemoteAddr unless the connection is from a trusted proxy.
	remoteIP := parseIP(r.RemoteAddr)
	if remoteIP == "" {
		return ""
	}
	if !isTrustedProxy(remoteIP) {
		return remoteIP
	}

	// 检查 X-Forwarded-For
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			if ip := strings.TrimSpace(parts[0]); ip != "" {
				return ip
			}
		}
	}

	// 检查 X-Real-IP
	if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
		return xri
	}

	return remoteIP
}

func parseIP(addr string) string {
	trimmed := strings.TrimSpace(addr)
	if trimmed == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(trimmed); err == nil {
		return host
	}
	return trimmed
}

func isTrustedProxy(remoteIP string) bool {
	if remoteIP == "127.0.0.1" || remoteIP == "::1" {
		return true
	}
	if strings.HasPrefix(remoteIP, "10.") || strings.HasPrefix(remoteIP, "192.168.") {
		return true
	}
	if strings.HasPrefix(remoteIP, "172.") {
		parts := strings.Split(remoteIP, ".")
		if len(parts) > 1 {
			if second, err := strconv.Atoi(parts[1]); err == nil {
				if second >= 16 && second <= 31 {
					return true
				}
			}
		}
	}
	return false
}
