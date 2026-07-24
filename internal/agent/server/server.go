package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/creamcroissant/xboard/internal/agent/protocol"
)

// Server 表示 Agent 侧 HTTP 服务实例。
type Server struct {
	httpServer *http.Server
	handler    *Handler
}

// Config 描述 HTTP 服务配置。
type Config struct {
	// Listen 监听地址（例如 ":8081", "127.0.0.1:8081"）
	Listen string

	// AuthToken API 认证令牌
	AuthToken string
}

// NewServer 创建 Agent HTTP 服务。
func NewServer(cfg Config, protoMgr *protocol.Manager) *Server {
	handler := NewHandler(protoMgr, cfg.AuthToken)

	mux := http.NewServeMux()

	// 健康检查（无鉴权）
	mux.HandleFunc("GET /health", handler.HealthCheck)

	// 协议管理接口（需要鉴权）
	mux.Handle("GET /api/v1/protocols", handler.AuthMiddleware(http.HandlerFunc(handler.ListConfigs)))
	mux.Handle("GET /api/v1/protocols/inbounds", handler.AuthMiddleware(http.HandlerFunc(handler.ListInbounds)))
	mux.Handle("GET /api/v1/protocols/{filename}", handler.AuthMiddleware(http.HandlerFunc(handler.GetConfig)))
	mux.Handle("POST /api/v1/protocols", handler.AuthMiddleware(http.HandlerFunc(handler.ApplyConfig)))
	mux.Handle("POST /api/v1/protocols/template", handler.AuthMiddleware(http.HandlerFunc(handler.ApplyTemplate)))
	mux.Handle("DELETE /api/v1/protocols/{filename}", handler.AuthMiddleware(http.HandlerFunc(handler.DeleteConfig)))

	// 服务管理接口（需要鉴权）
	mux.Handle("GET /api/v1/service/status", handler.AuthMiddleware(http.HandlerFunc(handler.ServiceStatus)))
	mux.Handle("POST /api/v1/service/reload", handler.AuthMiddleware(http.HandlerFunc(handler.ReloadService)))

	return &Server{
		httpServer: &http.Server{
			Addr:         cfg.Listen,
			Handler:      recoverMiddleware(mux),
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		handler: handler,
	}
}

// Start 启动 HTTP 服务。
func (s *Server) Start() error {
	slog.Info("Agent HTTP server starting", "addr", s.httpServer.Addr)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}

// Shutdown 优雅关闭 HTTP 服务。
func (s *Server) Shutdown(ctx context.Context) error {
	slog.Info("Agent HTTP server shutting down")
	return s.httpServer.Shutdown(ctx)
}

// recoverMiddleware 捕获 HTTP handler panic，避免进程崩溃。
func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("http handler panic recovered", "error", err, "path", r.URL.Path)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
