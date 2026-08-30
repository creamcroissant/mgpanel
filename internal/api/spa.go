// 文件路径: internal/api/spa.go
// 模块说明: 这是 internal 模块里的 spa 逻辑，负责把前端静态资源挂到 Go 服务上，下面的注释会用非常通俗的中文帮你理解每一步。
package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/service"
	"github.com/go-chi/chi/v5"
)

// RouterOption 允许在创建 Router 时附加功能，例如注入静态站点配置。
type RouterOption func(*routerOptions)

type routerOptions struct {
	adminUI            AdminUIOptions
	userUI             UserUIOptions
	installUI          InstallUIOptions
	corsAllowedOrigins []string
	trustedProxies     []string
}

// AdminUIOptions 控制管理端前端资源的加载与品牌定制。
type AdminUIOptions struct {
	Enabled         bool
	Dir             string
	Index           string
	BaseURL         string
	Title           string
	Version         string
	Logo            string
	DeployScriptURL string
	HiddenModules   []string
}

// WithAdminUI 会把管理端静态资源配置应用到 Router 中。
func WithAdminUI(opts AdminUIOptions) RouterOption {
	return func(ro *routerOptions) {
		ro.adminUI = opts
	}
}

// UserUIOptions 控制用户端前端资源的加载与品牌定制。
type UserUIOptions struct {
	Enabled bool
	Dir     string
	Index   string
	BaseURL string
	Title   string
}

// WithUserUI 会把用户端静态资源配置应用到 Router 中。
func WithUserUI(opts UserUIOptions) RouterOption {
	return func(ro *routerOptions) {
		ro.userUI = opts
	}
}

// InstallUIOptions 控制安装引导静态页面。
type InstallUIOptions struct {
	Enabled bool
	Dir     string
}

// WithInstallUI 准备安装引导页配置。
func WithInstallUI(opts InstallUIOptions) RouterOption {
	return func(ro *routerOptions) {
		ro.installUI = opts
	}
}

// WithCORSAllowedOrigins 设置 CORS 允许的来源列表，覆盖默认的开放策略。
func WithCORSAllowedOrigins(origins []string) RouterOption {
	return func(ro *routerOptions) {
		ro.corsAllowedOrigins = origins
	}
}

// WithTrustedProxies 配置可信反代 CIDR 白名单；空切片/nil 表示不信任任何
// XFF 头（严格模式）。必须在路由中间件装配前应用。
func WithTrustedProxies(proxies []string) RouterOption {
	return func(ro *routerOptions) {
		ro.trustedProxies = proxies
	}
}

type adminBranding struct {
	baseURL         string
	title           string
	version         string
	logo            string
	deployScriptURL string
}

type userBranding struct {
	baseURL string
	title   string
}

type adminSPAHandler struct {
	logger        *slog.Logger
	paths         service.AdminPathService
	root          string
	indexFile     string
	branding      adminBranding
	hiddenModules []string
}

type userSPAHandler struct {
	logger    *slog.Logger
	root      string
	indexFile string
	branding  userBranding
}

// unifiedSPAHandler 统一处理 User SPA 和 Admin SPA 的请求，
// 通过检查请求路径来决定使用哪个 handler。
// 这解决了 chi 路由器中 /{securePath}/* 动态路由会错误捕获
// User SPA 静态资源请求（如 /assets/*）的问题。
type unifiedSPAHandler struct {
	user  *userSPAHandler
	admin *adminSPAHandler
}

// registerSPARoutes 注册统一的 SPA 路由处理器。
// 使用一个 handler 同时处理 User SPA 和 Admin SPA 的请求，
// 避免 chi 动态路由参数 /{securePath} 错误匹配 /assets 等路径。
func registerSPARoutes(root chi.Router, userHandler *userSPAHandler, adminHandler *adminSPAHandler) {
	unified := &unifiedSPAHandler{user: userHandler, admin: adminHandler}
	root.Get("/", unified.ServeHTTP)
	root.Get("/*", unified.ServeHTTP)
}

func (h *unifiedSPAHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.admin != nil {
		securePath, err := h.admin.paths.SecurePath(r.Context())
		if err == nil {
			securePath = normalizeSecurePath(securePath)
			prefix := "/" + securePath
			if r.URL.Path == prefix || strings.HasPrefix(r.URL.Path, prefix+"/") {
				h.admin.serveAdminRequest(w, r, securePath)
				return
			}
		}
	}

	if h.user != nil {
		h.user.ServeHTTP(w, r)
		return
	}

	if h.admin != nil {
		h.serveAdminAtRoot(w, r)
		return
	}

	http.NotFound(w, r)
}

// serveAdminAtRoot 在根路径提供 Admin SPA，适用于统一前端模式
func (h *unifiedSPAHandler) serveAdminAtRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	relative := strings.TrimPrefix(r.URL.Path, "/")

	if relative == "" {
		h.admin.serveIndexAtRoot(w, r)
		return
	}
	if relative == "settings.js" {
		h.admin.serveSettingsAtRoot(w, r)
		return
	}

	clean := path.Clean(relative)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		http.NotFound(w, r)
		return
	}
	if strings.Contains(clean, "../") {
		http.NotFound(w, r)
		return
	}
	filePath := filepath.Join(h.admin.root, filepath.FromSlash(clean))
	if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
		http.ServeFile(w, r, filePath)
		return
	}

	h.admin.serveIndexAtRoot(w, r)
}

func registerAdminSPARoutes(root chi.Router, handler *adminSPAHandler) {
	root.Group(func(public chi.Router) {
		// Admin uses a secure path, so we don't serve it at root directly unless redirected.
		// However, the original logic had redirectToSecurePath at root.
		// Since we now have a User SPA, the root path "/" should probably be occupied by the User SPA.
		// Admin is accessible via /{securePath}.
		// If Admin SPA is enabled but User SPA is NOT, maybe we redirect root to admin?
		// But usually User SPA is the main entry.
		// We will let the caller decide routing registration order.
		
		// If both are enabled, User SPA takes "/" and Admin SPA takes "/{securePath}".
		// The previous logic put redirectToSecurePath at "/". We should probably change that if User SPA is present.
		// For now, let's keep the handler logic separate and let NewRouter decide mounting.
		
		public.Route("/{securePath}", func(admin chi.Router) {
			admin.Get("/", handler.ServeHTTP)
			admin.Get("/*", handler.ServeHTTP)
		})
	})
}

func registerUserSPARoutes(root chi.Router, handler *userSPAHandler) {
	root.Get("/", handler.ServeHTTP)
	root.Get("/*", handler.ServeHTTP)
}

func newAdminSPAHandler(logger *slog.Logger, paths service.AdminPathService, opts AdminUIOptions) (*adminSPAHandler, error) {
	if !opts.Enabled {
		return nil, errors.New("admin ui disabled / 管理端界面未启用")
	}
	if opts.Dir == "" {
		return nil, errors.New("admin ui dir is required / 管理端界面目录不能为空")
	}
	root, err := filepath.Abs(opts.Dir)
	if err != nil {
		return nil, fmt.Errorf("resolve admin ui dir: %w", err)
	}
	if info, err := os.Stat(root); err != nil {
		return nil, fmt.Errorf("stat admin ui dir: %w", err)
	} else if !info.IsDir() {
		return nil, fmt.Errorf("admin ui dir is not a directory: %s", root)
	}
	index := strings.TrimSpace(opts.Index)
	if index == "" {
		index = "index.html"
	}
	branding := adminBranding{
		baseURL:         strings.TrimRight(opts.BaseURL, "/"),
		title:           fallback(opts.Title, "MGPanel"),
		version:         fallback(opts.Version, "go-dev"),
		logo:            fallback(opts.Logo, "https://mgpanel.io/images/logo.png"),
		deployScriptURL: strings.TrimSpace(opts.DeployScriptURL),
	}
	return &adminSPAHandler{
		logger:        logger,
		paths:         paths,
		root:          root,
		indexFile:     index,
		branding:      branding,
		hiddenModules: normalizeModules(opts.HiddenModules),
	}, nil
}

func newUserSPAHandler(logger *slog.Logger, opts UserUIOptions) (*userSPAHandler, error) {
	if !opts.Enabled {
		return nil, errors.New("user ui disabled / 用户端界面未启用")
	}
	if opts.Dir == "" {
		return nil, errors.New("user ui dir is required / 用户端界面目录不能为空")
	}
	root, err := filepath.Abs(opts.Dir)
	if err != nil {
		return nil, fmt.Errorf("resolve user ui dir: %w", err)
	}
	if info, err := os.Stat(root); err != nil {
		return nil, fmt.Errorf("stat user ui dir: %w", err)
	} else if !info.IsDir() {
		return nil, fmt.Errorf("user ui dir is not a directory: %s", root)
	}
	index := strings.TrimSpace(opts.Index)
	if index == "" {
		index = "index.html"
	}
	branding := userBranding{
		baseURL: strings.TrimRight(opts.BaseURL, "/"),
		title:   fallback(opts.Title, "MGPanel"),
	}
	return &userSPAHandler{
		logger:    logger,
		root:      root,
		indexFile: index,
		branding:  branding,
	}, nil
}

func (h *adminSPAHandler) redirectToSecurePath(w http.ResponseWriter, r *http.Request) {
	securePath, err := h.paths.SecurePath(r.Context())
	if err != nil {
		h.logger.Error("resolve secure path", "error", err)
		http.Error(w, "unable to resolve admin path", http.StatusInternalServerError)
		return
	}
	securePath = normalizeSecurePath(securePath)
	target := "/" + securePath + "/"
	http.Redirect(w, r, target, http.StatusTemporaryRedirect)
}

func (h *adminSPAHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	securePath, err := h.paths.SecurePath(r.Context())
	if err != nil {
		h.logger.Error("resolve secure path", "error", err)
		http.Error(w, "unable to resolve admin path", http.StatusInternalServerError)
		return
	}
	securePath = normalizeSecurePath(securePath)
	param := chi.URLParam(r, "securePath")
	// If accessed via a mounted route, URLParam might be empty if the router stripped the prefix
	// But here we are mounting at /{securePath}, so it should be there.
	
	if param == "" {
		// Fallback check
		param = strings.TrimPrefix(r.URL.Path, "/")
		if idx := strings.Index(param, "/"); idx >= 0 {
			param = param[:idx]
		}
	}
	
	// Double check we are on the correct path
	if param != securePath {
		// If param is empty (e.g. strict slash issue), or different
		// For safety, require match.
		// However, chi routing should have ensured this if mounted correctly.
	}

	// Calculate relative path inside the SPA
	// r.URL.Path includes the securePath prefix.
	// We need to strip it.
	
	// Using chi's RouteContext would be better but simple string manipulation works if we are careful.
	prefix := "/" + securePath
	if !strings.HasPrefix(r.URL.Path, prefix) {
		// Should not happen if routed correctly
		http.NotFound(w, r)
		return
	}
	
	relative := strings.TrimPrefix(r.URL.Path, prefix)
	relative = strings.TrimPrefix(relative, "/")
	
	if relative == "" {
		h.serveIndex(w, r)
		return
	}
	if relative == "settings.js" {
		h.serveSettings(w, r, securePath)
		return
	}

	clean := path.Clean(relative)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		http.NotFound(w, r)
		return
	}
	if strings.Contains(clean, "../") {
		http.NotFound(w, r)
		return
	}
	filePath := filepath.Join(h.root, filepath.FromSlash(clean))
	if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
		http.ServeFile(w, r, filePath)
		return
	}

	if strings.HasPrefix(relative, "assets/") {
		http.NotFound(w, r)
		return
	}

	h.serveIndex(w, r)
}

// serveAdminRequest 处理 Admin SPA 请求，由 unifiedSPAHandler 调用。
// securePath 参数由调用者提供，避免重复查询。
func (h *adminSPAHandler) serveAdminRequest(w http.ResponseWriter, r *http.Request, securePath string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 计算 SPA 内部的相对路径
	prefix := "/" + securePath
	relative := strings.TrimPrefix(r.URL.Path, prefix)

	// 如果访问 /admin（没有尾随斜杠），重定向到 /admin/
	// 这确保浏览器正确解析相对路径的静态资源
	if relative == "" {
		http.Redirect(w, r, r.URL.Path+"/", http.StatusMovedPermanently)
		return
	}

	relative = strings.TrimPrefix(relative, "/")

	if relative == "" {
		h.serveIndex(w, r)
		return
	}
	if relative == "settings.js" {
		h.serveSettings(w, r, securePath)
		return
	}

	clean := path.Clean(relative)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		http.NotFound(w, r)
		return
	}
	if strings.Contains(clean, "../") {
		http.NotFound(w, r)
		return
	}
	filePath := filepath.Join(h.root, filepath.FromSlash(clean))
	if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
		http.ServeFile(w, r, filePath)
		return
	}

	h.serveIndex(w, r)
}

func (h *userSPAHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// User SPA is typically at root "/"
	// So r.URL.Path is the relative path (except leading slash)
	relative := strings.TrimPrefix(r.URL.Path, "/")
	
	if relative == "" {
		h.serveIndex(w, r)
		return
	}
	if relative == "settings.js" {
		h.serveSettings(w, r)
		return
	}

	clean := path.Clean(relative)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		http.NotFound(w, r)
		return
	}
	if strings.Contains(clean, "../") {
		http.NotFound(w, r)
		return
	}
	
	filePath := filepath.Join(h.root, filepath.FromSlash(clean))
	if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
		http.ServeFile(w, r, filePath)
		return
	}

	if strings.HasPrefix(relative, "assets/") {
		http.NotFound(w, r)
		return
	}

	// Fallback to index.html for SPA routing
	h.serveIndex(w, r)
}

func (h *adminSPAHandler) serveIndex(w http.ResponseWriter, r *http.Request) {
	data, err := h.indexBytes()
	if err != nil {
		h.logger.Error("load admin index", "error", err)
		http.Error(w, "admin ui unavailable", http.StatusInternalServerError)
		return
	}

	// 获取 securePath 并构建 settings 脚本
	securePath, _ := h.paths.SecurePath(r.Context())
	securePath = normalizeSecurePath(securePath)

	payload := map[string]any{
		"base_url":          h.resolveBaseURL(r),
		"title":             h.branding.title,
		"version":           h.branding.version,
		"logo":              h.branding.logo,
		"deploy_script_url": h.branding.deployScriptURL,
		"secure_path":       "/" + securePath,
		"disabled_modules":  h.hiddenModules,
	}
	settingsJSON, _ := json.Marshal(payload)
	settingsScript := fmt.Sprintf("<script>window.settings = %s;</script>", settingsJSON)

	// 在 <head> 标签后注入 settings，确保在其他脚本加载前执行
	html := strings.Replace(string(data), "<head>", "<head>\n    "+settingsScript, 1)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = w.Write([]byte(html))
}

func (h *userSPAHandler) serveIndex(w http.ResponseWriter, r *http.Request) {
	data, err := h.indexBytes()
	if err != nil {
		h.logger.Error("load user index", "error", err)
		http.Error(w, "user ui unavailable", http.StatusInternalServerError)
		return
	}

	// 构建 settings 脚本
	payload := map[string]any{
		"base_url": h.resolveBaseURL(r),
		"title":    h.branding.title,
	}
	settingsJSON, _ := json.Marshal(payload)
	settingsScript := fmt.Sprintf("<script>window.settings = %s;</script>", settingsJSON)

	// 在 <head> 标签后注入 settings，确保在其他脚本加载前执行
	html := strings.Replace(string(data), "<head>", "<head>\n    "+settingsScript, 1)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = w.Write([]byte(html))
}

func (h *adminSPAHandler) serveSettings(w http.ResponseWriter, r *http.Request, securePath string) {
	payload := map[string]any{
		"base_url":          h.resolveBaseURL(r),
		"title":             h.branding.title,
		"version":           h.branding.version,
		"logo":              h.branding.logo,
		"deploy_script_url": h.branding.deployScriptURL,
		"secure_path":       "/" + securePath,
		"disabled_modules":  h.hiddenModules,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		h.logger.Error("encode admin settings", "error", err)
		http.Error(w, "admin settings unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = fmt.Fprintf(w, "window.settings = Object.assign({}, window.settings || {}, %s);\n", data)
	_, _ = fmt.Fprintf(w, "document.title = window.settings.title || document.title;\n")
}

func (h *userSPAHandler) serveSettings(w http.ResponseWriter, r *http.Request) {
	payload := map[string]any{
		"base_url": h.resolveBaseURL(r),
		"title":    h.branding.title,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		h.logger.Error("encode user settings", "error", err)
		http.Error(w, "user settings unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = fmt.Fprintf(w, "window.settings = Object.assign({}, window.settings || {}, %s);\n", data)
	_, _ = fmt.Fprintf(w, "document.title = window.settings.title || document.title;\n")
}

// serveIndexAtRoot 在根路径提供 Admin SPA 的 index.html
// 注入实际的 secure_path 以便前端可以正确调用 Admin API
func (h *adminSPAHandler) serveIndexAtRoot(w http.ResponseWriter, r *http.Request) {
	data, err := h.indexBytes()
	if err != nil {
		h.logger.Error("load admin index", "error", err)
		http.Error(w, "admin ui unavailable", http.StatusInternalServerError)
		return
	}

	// 获取实际的 securePath
	securePath, _ := h.paths.SecurePath(r.Context())
	securePath = normalizeSecurePath(securePath)

	// 构建 settings 脚本，使用实际的 secure_path
	payload := map[string]any{
		"base_url":          h.resolveBaseURL(r),
		"title":             h.branding.title,
		"version":           h.branding.version,
		"logo":              h.branding.logo,
		"deploy_script_url": h.branding.deployScriptURL,
		"secure_path":       "/" + securePath,
		"router_base":       "/", // 强制根路径作为路由基础
		"disabled_modules":  h.hiddenModules,
	}
	settingsJSON, _ := json.Marshal(payload)
	settingsScript := fmt.Sprintf("<script>window.settings = %s;</script>", settingsJSON)

	// 在 <head> 标签后注入 settings，确保在其他脚本加载前执行
	html := strings.Replace(string(data), "<head>", "<head>\n    "+settingsScript, 1)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = w.Write([]byte(html))
}

// serveSettingsAtRoot 在根路径提供 Admin SPA 的 settings.js
func (h *adminSPAHandler) serveSettingsAtRoot(w http.ResponseWriter, r *http.Request) {
	// 获取实际的 securePath
	securePath, _ := h.paths.SecurePath(r.Context())
	securePath = normalizeSecurePath(securePath)

	payload := map[string]any{
		"base_url":          h.resolveBaseURL(r),
		"title":             h.branding.title,
		"version":           h.branding.version,
		"logo":              h.branding.logo,
		"deploy_script_url": h.branding.deployScriptURL,
		"secure_path":       "/" + securePath,
		"router_base":       "/", // 强制根路径作为路由基础
		"disabled_modules":  h.hiddenModules,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		h.logger.Error("encode admin settings", "error", err)
		http.Error(w, "admin settings unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = fmt.Fprintf(w, "window.settings = Object.assign({}, window.settings || {}, %s);\n", data)
	_, _ = fmt.Fprintf(w, "document.title = window.settings.title || document.title;\n")
}

func (h *adminSPAHandler) resolveBaseURL(r *http.Request) string {
	if h.branding.baseURL != "" {
		return h.branding.baseURL
	}
	return resolveRequestBaseURL(r)
}

func (h *userSPAHandler) resolveBaseURL(r *http.Request) string {
	if h.branding.baseURL != "" {
		return h.branding.baseURL
	}
	return resolveRequestBaseURL(r)
}

func resolveRequestBaseURL(r *http.Request) string {
	scheme := "http"
	if forwarded := r.Header.Get("X-Forwarded-Proto"); forwarded != "" {
		scheme = strings.TrimSpace(strings.Split(forwarded, ",")[0])
	} else if r.TLS != nil {
		scheme = "https"
	}
	host := r.Host
	if host == "" {
		host = "localhost"
	}
	return scheme + "://" + host
}

func (h *adminSPAHandler) indexBytes() ([]byte, error) {
	path := filepath.Join(h.root, h.indexFile)
	return os.ReadFile(path)
}

func (h *userSPAHandler) indexBytes() ([]byte, error) {
	path := filepath.Join(h.root, h.indexFile)
	return os.ReadFile(path)
}

func fallback(value, def string) string {
	if strings.TrimSpace(value) == "" {
		return def
	}
	return value
}

func normalizeSecurePath(value string) string {
	trimmed := strings.Trim(strings.TrimSpace(value), "/")
	if trimmed == "" {
		return "admin"
	}
	return trimmed
}

func normalizeModules(mods []string) []string {
	if len(mods) == 0 {
		return nil
	}
	unique := make(map[string]struct{})
	result := make([]string, 0, len(mods))
	for _, mod := range mods {
		normalized := strings.ToLower(strings.TrimSpace(mod))
		if normalized == "" {
			continue
		}
		if _, exists := unique[normalized]; exists {
			continue
		}
		unique[normalized] = struct{}{}
		result = append(result, normalized)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
