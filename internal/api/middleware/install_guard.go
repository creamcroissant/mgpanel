// 文件路径: internal/api/middleware/install_guard.go
// 模块说明: 这是负责安装阶段路由拦截的中间件，让用户在未初始化时强制进入安装向导。
package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/service"
)

// InstallGuard 在系统尚未创建管理员之前，拦截所有业务请求并重定向到 /install。
func InstallGuard(logger *slog.Logger, install service.InstallService) func(http.Handler) http.Handler {
	if install == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			needs, err := install.NeedsBootstrap(r.Context())
			if err != nil {
				if logger != nil {
					logger.Error("install guard check failed", "error", err)
				}
				http.Error(w, "install guard unavailable", http.StatusInternalServerError)
				return
			}
			path := r.URL.Path
			if !needs {
				if isInstallPath(path) {
					http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			if allowDuringInstall(path) || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			if strings.HasPrefix(path, "/api/") {
				writeInstallJSON(w)
				return
			}
			http.Redirect(w, r, "/install", http.StatusTemporaryRedirect)
		})
	}
}

func allowDuringInstall(path string) bool {
	if isInstallPath(path) {
		return true
	}
	if strings.HasPrefix(path, "/api/install") {
		return true
	}
	if path == "/healthz" || strings.HasPrefix(path, "/_internal/") {
		return true
	}
	// Allow static assets (JS, CSS, images, fonts, etc.)
	if strings.HasPrefix(path, "/assets/") {
		return true
	}
	// Allow common static files
	if strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".css") ||
		strings.HasSuffix(path, ".svg") || strings.HasSuffix(path, ".png") ||
		strings.HasSuffix(path, ".ico") || strings.HasSuffix(path, ".woff") ||
		strings.HasSuffix(path, ".woff2") || strings.HasSuffix(path, ".ttf") {
		return true
	}
	return false
}

func isInstallPath(path string) bool {
	return path == "/install" || strings.HasPrefix(path, "/install/")
}

func writeInstallJSON(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusPreconditionRequired)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":           "setup_required",
		"needs_bootstrap": true,
	})
}
