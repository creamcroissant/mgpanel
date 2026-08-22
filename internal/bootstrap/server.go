// 文件路径: internal/bootstrap/server.go
// 模块说明: 这是 internal 模块里的 server 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package bootstrap

import (
	"net/http"
	"time"
)

// NewHTTPServer constructs a baseline http.Server with conservative defaults.
func NewHTTPServer(cfg *Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           handler,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MiB
	}
}
