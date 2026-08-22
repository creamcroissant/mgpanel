// 文件路径: internal/api/install_page.go
// 模块说明: 这里提供安装引导页的静态文件服务，结构与 admin SPA 相似但更轻量。
package api

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

type installPageHandler struct {
	index []byte
	fs    http.Handler
}

func newInstallPageHandler(dir string) (*installPageHandler, error) {
	cleanDir := filepath.Clean(dir)
	if cleanDir == "" {
		return nil, errors.New("install ui dir is empty / 安装页面目录为空")
	}
	root, err := filepath.Abs(cleanDir)
	if err != nil {
		return nil, fmt.Errorf("resolve install ui dir: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("stat install ui dir: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("install ui dir is not a directory: %s", root)
	}
	indexPath := filepath.Join(root, "index.html")
	index, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, fmt.Errorf("read install index: %w", err)
	}
	fs := http.StripPrefix("/install", http.FileServer(http.Dir(root)))
	return &installPageHandler{
		index: index,
		fs:    fs,
	}, nil
}

func (h *installPageHandler) serveIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = w.Write(h.index)
}

func (h *installPageHandler) serveAssets(w http.ResponseWriter, r *http.Request) {
	h.fs.ServeHTTP(w, r)
}
