// Package protocol 提供 sing-box 协议配置管理能力。
package protocol

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/creamcroissant/mgpanel/internal/agent/initsys"
	"github.com/creamcroissant/mgpanel/internal/agent/protocol/parser"
)

// Config 定义协议管理器的配置。
type Config struct {
	// ConfigDir 是兼容旧配置的主目录（优先指向 managed）。
	ConfigDir string `yaml:"config_dir"`

	// LegacyConfigDir 是历史配置目录。
	LegacyConfigDir string `yaml:"legacy_config_dir"`

	// ManagedConfigDir 是托管配置目录。
	ManagedConfigDir string `yaml:"managed_config_dir"`

	// MergeOutputFile 是单文件核心模式下的 merged 输出文件名。
	MergeOutputFile string `yaml:"merge_output_file"`

	// ServiceName 是 sing-box 服务名称
	ServiceName string `yaml:"service_name"`

	// ValidateCmd 是应用配置前执行的校验命令
	ValidateCmd string `yaml:"validate_cmd"`

	// ServiceAction controls whether changes trigger reload/restart/none.
	ServiceAction string `yaml:"service_action"`

	// AutoRestart 是旧字段兼容映射：true -> reload, false -> none。
	AutoRestart bool `yaml:"auto_restart"`

	// PreHook 是应用配置前执行的钩子命令
	PreHook string `yaml:"pre_hook"`

	// PostHook 是应用配置后执行的钩子命令
	PostHook string `yaml:"post_hook"`
}

// DefaultConfig 返回默认配置。
func DefaultConfig() Config {
	return Config{
		ConfigDir:        "/etc/sing-box/conf",
		LegacyConfigDir:  "/etc/sing-box/conf",
		ManagedConfigDir: "/etc/sing-box/conf",
		MergeOutputFile:  "config.json",
		ServiceName:      "sing-box",
		ValidateCmd:      "",
		ServiceAction:    string(ReloadServiceAction),
		AutoRestart:      true,
	}
}

// ConfigFile 描述协议配置文件元信息。
type ConfigFile struct {
	Filename    string    `json:"filename"`
	ModTime     time.Time `json:"mod_time"`
	Size        int64     `json:"size"`
	ContentHash string    `json:"content_hash"`
	Content     []byte    `json:"content,omitempty"`
}

// Manager 管理 sing-box 协议配置。
type Manager struct {
	cfg      Config
	init     initsys.InitSystem
	registry *parser.Registry
	applyMu  sync.Mutex
}

// NewManager 创建协议管理器实例。
func NewManager(cfg Config, initSys initsys.InitSystem) *Manager {
	if initSys == nil {
		initSys = initsys.Detect()
	}
	if strings.TrimSpace(cfg.ConfigDir) == "" {
		if strings.TrimSpace(cfg.ManagedConfigDir) != "" {
			cfg.ConfigDir = cfg.ManagedConfigDir
		} else if strings.TrimSpace(cfg.LegacyConfigDir) != "" {
			cfg.ConfigDir = cfg.LegacyConfigDir
		} else {
			cfg.ConfigDir = "/etc/sing-box/conf"
		}
	}
	cfg.ConfigDir = strings.TrimSpace(cfg.ConfigDir)
	if strings.TrimSpace(cfg.ManagedConfigDir) == "" {
		cfg.ManagedConfigDir = cfg.ConfigDir
	}
	cfg.ManagedConfigDir = strings.TrimSpace(cfg.ManagedConfigDir)
	if strings.TrimSpace(cfg.LegacyConfigDir) == "" {
		cfg.LegacyConfigDir = cfg.ConfigDir
	}
	cfg.LegacyConfigDir = strings.TrimSpace(cfg.LegacyConfigDir)
	if strings.TrimSpace(cfg.MergeOutputFile) == "" {
		cfg.MergeOutputFile = "config.json"
	}
	cfg.MergeOutputFile = strings.TrimSpace(cfg.MergeOutputFile)
	return &Manager{
		cfg:      cfg,
		init:     initSys,
		registry: parser.NewRegistry(),
	}
}

// InitSystemType 返回当前使用的 init 系统类型。
func (m *Manager) InitSystemType() string {
	return m.init.Type()
}

// ListConfigs 返回所有协议配置文件。
func (m *Manager) ListConfigs() ([]ConfigFile, error) {
	dir, err := m.dirForSource("current")
	if err != nil {
		return nil, err
	}
	files, err := m.listConfigFiles(dir)
	if err != nil {
		return nil, err
	}
	return m.buildConfigFiles(files), nil
}

// ListInbounds 仅返回 inbound 相关配置文件。
func (m *Manager) ListInbounds() ([]ConfigFile, error) {
	configs, err := m.ListConfigs()
	if err != nil {
		return nil, err
	}

	inbounds := make([]ConfigFile, 0)
	for _, c := range configs {
		if strings.Contains(c.Filename, "inbounds") {
			inbounds = append(inbounds, c)
		}
	}
	return inbounds, nil
}

// ListConfigsBySource 按来源返回配置文件。
func (m *Manager) ListConfigsBySource(source string) ([]ConfigFile, error) {
	dir, err := m.dirForSource(source)
	if err != nil {
		return nil, err
	}
	files, err := m.listConfigFiles(dir)
	if err != nil {
		return nil, err
	}
	return m.buildConfigFiles(files), nil
}

// ReadConfig 读取指定配置文件内容。
func (m *Manager) ReadConfig(filename string) ([]byte, error) {
	return m.ReadConfigFromSource("current", filename)
}

// ReadConfigFromSource 按来源读取配置文件内容。
func (m *Manager) ReadConfigFromSource(source, filename string) ([]byte, error) {
	name, err := sanitizeFilename(filename)
	if err != nil {
		return nil, err
	}
	dir, err := m.dirForSource(source)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, name)
	return os.ReadFile(path)
}

// WriteConfig 写入配置文件。
func (m *Manager) WriteConfig(filename string, content []byte) error {
	return m.WriteConfigToSource("current", filename, content)
}

// WriteConfigToSource 按来源写入配置文件。
func (m *Manager) WriteConfigToSource(source, filename string, content []byte) error {
	name, err := sanitizeFilename(filename)
	if err != nil {
		return err
	}
	dir, err := m.dirForSource(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	path := filepath.Join(dir, name)

	formattedContent, err := formatConfigContent(content)
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, formattedContent, 0644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	return nil
}

// DeleteConfig 删除配置文件。
func (m *Manager) DeleteConfig(filename string) error {
	return m.DeleteConfigFromSource("current", filename)
}

// DeleteConfigFromSource 按来源删除配置文件。
func (m *Manager) DeleteConfigFromSource(source, filename string) error {
	name, err := sanitizeFilename(filename)
	if err != nil {
		return err
	}
	dir, err := m.dirForSource(source)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, name)
	return os.Remove(path)
}

// CreateFromTemplate 根据模板生成配置文件。
func (m *Manager) CreateFromTemplate(filename, tmplContent string, vars map[string]any) error {
	content, err := renderTemplateContent(tmplContent, vars)
	if err != nil {
		return err
	}
	return m.WriteConfig(filename, content)
}

// ValidateConfig 校验当前配置（若未配置校验命令则跳过）。
func (m *Manager) ValidateConfig(ctx context.Context) error {
	dir, err := m.dirForSource("current")
	if err != nil {
		return err
	}
	return m.ValidateConfigInDir(ctx, dir)
}

// ValidateConfigInDir 在指定目录校验配置。
func (m *Manager) ValidateConfigInDir(ctx context.Context, dir string) error {
	if m.cfg.ValidateCmd == "" {
		return nil
	}

	cmd := strings.ReplaceAll(m.cfg.ValidateCmd, "{config_dir}", dir)
	cmd = strings.ReplaceAll(cmd, "{legacy_config_dir}", m.cfg.LegacyConfigDir)
	cmd = strings.ReplaceAll(cmd, "{managed_config_dir}", m.cfg.ManagedConfigDir)
	return runCommand(ctx, cmd)
}

// ReloadService 重载或重启 sing-box 服务。
func (m *Manager) ReloadService(ctx context.Context) error {
	validateDir, err := m.dirForSource("current")
	if err != nil {
		return err
	}
	return m.reloadServiceWithValidationDir(ctx, validateDir)
}

// ReloadServiceWithValidationDir 在指定目录完成校验后重载或重启服务。
func (m *Manager) ReloadServiceWithValidationDir(ctx context.Context, dir string) error {
	return m.reloadServiceWithValidationDir(ctx, dir)
}

func (m *Manager) reloadServiceWithValidationDir(ctx context.Context, validateDir string) error {
	action, err := normalizeServiceAction(m.cfg)
	if err != nil {
		return err
	}

	// 执行预处理钩子
	if m.cfg.PreHook != "" {
		if err := runCommand(ctx, m.cfg.PreHook); err != nil {
			return fmt.Errorf("pre-hook failed: %w", err)
		}
	}

	if err := m.ValidateConfigInDir(ctx, validateDir); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	switch action {
	case ReloadServiceAction:
		if err := m.init.Reload(ctx, m.cfg.ServiceName); err != nil {
			return fmt.Errorf("reload service: %w", err)
		}
	case RestartServiceAction:
		if err := m.init.Restart(ctx, m.cfg.ServiceName); err != nil {
			return fmt.Errorf("restart service: %w", err)
		}
	case NoneServiceAction:
		// no-op
		return m.runPostHook(ctx)
	default:
		return fmt.Errorf("invalid service_action %q", action)
	}

	return m.runPostHook(ctx)
}

// ServiceStatus 返回 sing-box 服务是否运行中。
func (m *Manager) runPostHook(ctx context.Context) error {
	if m.cfg.PostHook != "" {
		if err := runCommand(ctx, m.cfg.PostHook); err != nil {
			return fmt.Errorf("post-hook failed: %w", err)
		}
	}
	return nil
}

func (m *Manager) validateServiceAction() error {
	_, err := normalizeServiceAction(m.cfg)
	return err
}

func (m *Manager) ServiceStatus(ctx context.Context) (bool, error) {
	return m.init.Status(ctx, m.cfg.ServiceName)
}

// ApplyConfig 写入指定配置并重载服务。
func (m *Manager) ApplyConfig(ctx context.Context, filename string, content []byte) error {
	return m.applyPatchWithCore(ctx, "", filename, content)
}

// ApplyConfigWithCore 在指定核心模式下写入配置并重载服务。
func (m *Manager) ApplyConfigWithCore(ctx context.Context, coreType, filename string, content []byte) error {
	return m.applyPatchWithCore(ctx, coreType, filename, content)
}

// ApplyFromTemplate 根据模板生成配置并重载服务。
func (m *Manager) ApplyFromTemplate(ctx context.Context, filename, tmpl string, vars map[string]any) error {
	content, err := renderTemplateContent(tmpl, vars)
	if err != nil {
		return err
	}
	return m.applyPatchWithCore(ctx, "", filename, content)
}

// RemoveConfig 删除配置并重载服务。
func (m *Manager) RemoveConfig(ctx context.Context, filename string) error {
	if err := m.validateServiceAction(); err != nil {
		return err
	}
	targetFilename, err := defaultPatchFilename(filename)
	if err != nil {
		return err
	}
	if _, err := m.ReadConfig(targetFilename); err != nil {
		return err
	}
	_, err = m.ExecuteStagedApply(ctx, StagedApplyRequest{
		Mode: StagedApplyModePatch,
		PatchOperations: []StagedApplyPatchOperation{
			{
				Type:     StagedApplyPatchOperationDelete,
				Filename: targetFilename,
			},
		},
	})
	return err
}

// templateFuncs 返回模板函数集合。
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"default": func(def, val any) any {
			if val == nil || val == "" {
				return def
			}
			return val
		},
		"json": func(v any) string {
			b, err := json.Marshal(v)
			if err != nil {
				return ""
			}
			return string(b)
		},
	}
}

func formatConfigContent(content []byte) ([]byte, error) {
	if !json.Valid(content) {
		return nil, fmt.Errorf("invalid JSON content")
	}

	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, content, "", "  "); err != nil {
		return nil, fmt.Errorf("format JSON: %w", err)
	}
	return prettyJSON.Bytes(), nil
}

func renderTemplateContent(tmplContent string, vars map[string]any) ([]byte, error) {
	tmpl, err := template.New("config").Funcs(templateFuncs()).Parse(tmplContent)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}
	return buf.Bytes(), nil
}

func defaultPatchFilename(filename string) (string, error) {
	targetFilename := strings.TrimSpace(filename)
	if targetFilename == "" {
		targetFilename = "config.json"
	}
	return sanitizeFilename(targetFilename)
}

func patchFilenameForCore(_ Config, _ string, filename string) (string, error) {
	return defaultPatchFilename(filename)
}

func (m *Manager) applyPatchWithCore(ctx context.Context, coreType, filename string, content []byte) error {
	if err := m.validateServiceAction(); err != nil {
		return err
	}
	normalizedCore, err := normalizeStagedApplyCoreType(coreType)
	if err != nil {
		return err
	}
	targetFilename, err := patchFilenameForCore(m.cfg, normalizedCore, filename)
	if err != nil {
		return err
	}
	formattedContent, err := formatConfigContent(content)
	if err != nil {
		return err
	}
	_, err = m.ExecuteStagedApply(ctx, StagedApplyRequest{
		Mode:     StagedApplyModePatch,
		CoreType: normalizedCore,
		PatchOperations: []StagedApplyPatchOperation{
			{
				Type:     StagedApplyPatchOperationUpsert,
				Filename: targetFilename,
				Content:  formattedContent,
			},
		},
	})
	return err
}

func sanitizeFilename(filename string) (string, error) {
	trimmed := strings.TrimSpace(filename)
	if trimmed == "" {
		return "", fmt.Errorf("filename is required")
	}
	if strings.Contains(trimmed, "..") {
		return "", fmt.Errorf("invalid filename")
	}
	cleaned := filepath.Clean(trimmed)
	if cleaned == "." || cleaned == string(filepath.Separator) {
		return "", fmt.Errorf("invalid filename")
	}
	base := filepath.Base(cleaned)
	if base != cleaned {
		return "", fmt.Errorf("invalid filename")
	}
	if !strings.HasSuffix(strings.ToLower(base), ".json") {
		return "", fmt.Errorf("invalid filename")
	}
	return base, nil
}

func normalizeCoreType(coreType string) string {
	switch strings.ToLower(strings.TrimSpace(coreType)) {
	case "xray":
		return "xray"
	case "sing-box", "singbox", "sing_box":
		return "sing-box"
	default:
		return ""
	}
}

// SanitizeFilename normalizes and validates config filenames for safe local writes.
func SanitizeFilename(filename string) (string, error) {
	return sanitizeFilename(filename)
}

// NormalizeCoreType converts core aliases to canonical values (sing-box/xray).
func NormalizeCoreType(coreType string) string {
	return normalizeCoreType(coreType)
}

func (m *Manager) dirForSource(source string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "managed":
		if strings.TrimSpace(m.cfg.ManagedConfigDir) == "" {
			return "", fmt.Errorf("managed config dir is required")
		}
		return m.cfg.ManagedConfigDir, nil
	case "legacy":
		if strings.TrimSpace(m.cfg.LegacyConfigDir) == "" {
			return "", fmt.Errorf("legacy config dir is required")
		}
		return m.cfg.LegacyConfigDir, nil
	case "merged":
		if strings.TrimSpace(m.cfg.ManagedConfigDir) == "" {
			return "", fmt.Errorf("managed config dir is required")
		}
		return m.cfg.ManagedConfigDir, nil
	case "current", "":
		if strings.TrimSpace(m.cfg.ConfigDir) == "" {
			return "", fmt.Errorf("config dir is required")
		}
		return m.cfg.ConfigDir, nil
	default:
		return "", fmt.Errorf("invalid source")
	}
}

func (m *Manager) dirForCore(coreType string) (string, error) {
	switch normalizeCoreType(coreType) {
	case "xray":
		return m.dirForSource("merged")
	case "sing-box":
		return m.dirForSource("managed")
	default:
		return m.dirForSource("current")
	}
}

func (m *Manager) listConfigFiles(dir string) ([]string, error) {
	pattern := filepath.Join(dir, "*.json")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob config files: %w", err)
	}
	return files, nil
}

func (m *Manager) buildConfigFiles(files []string) []ConfigFile {
	configs := make([]ConfigFile, 0, len(files))
	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		content, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		hash := md5.Sum(content)
		configs = append(configs, ConfigFile{
			Filename:    filepath.Base(f),
			ModTime:     info.ModTime(),
			Size:        info.Size(),
			ContentHash: hex.EncodeToString(hash[:]),
		})
	}
	sort.Slice(configs, func(i, j int) bool {
		return configs[i].Filename < configs[j].Filename
	})
	return configs
}

func (m *Manager) buildConfigsWithDetails(files []string) []parser.ConfigFileWithDetails {
	results := make([]parser.ConfigFileWithDetails, 0, len(files))
	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			continue
		}

		hash := md5.Sum(content)
		filename := filepath.Base(f)
		protocols := m.registry.ParseAll(filename, content)
		if len(protocols) == 0 {
			continue
		}

		results = append(results, parser.ConfigFileWithDetails{
			Filename:    filename,
			ContentHash: hex.EncodeToString(hash[:]),
			Protocols:   protocols,
		})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Filename < results[j].Filename
	})
	return results
}

// ListConfigsWithDetails 返回带解析详情的配置文件列表。
func (m *Manager) ListConfigsWithDetails() ([]parser.ConfigFileWithDetails, error) {
	dir, err := m.dirForSource("current")
	if err != nil {
		return nil, err
	}
	files, err := m.listConfigFiles(dir)
	if err != nil {
		return nil, err
	}
	return m.buildConfigsWithDetails(files), nil
}

// ListConfigsWithDetailsBySource 按来源返回带解析详情的配置文件列表。
func (m *Manager) ListConfigsWithDetailsBySource(source string) ([]parser.ConfigFileWithDetails, error) {
	dir, err := m.dirForSource(source)
	if err != nil {
		return nil, err
	}
	files, err := m.listConfigFiles(dir)
	if err != nil {
		return nil, err
	}
	return m.buildConfigsWithDetails(files), nil
}

// GetRegistry 返回协议解析注册表。
func (m *Manager) GetRegistry() *parser.Registry {
	return m.registry
}

// UserConfig 描述注入到协议配置中的用户信息。
type UserConfig struct {
	UUID    string
	Email   string
	Enabled bool
}

// InjectUsers 将用户注入配置并重载服务。
// 流程：读取配置 -> 更新 inbounds -> 写回 -> 重载
func (m *Manager) InjectUsers(ctx context.Context, users []UserConfig) error {
	// 读取当前配置文件
	const defaultFilename = "config.json"
	content, err := m.ReadConfig(defaultFilename)
	if err != nil {
		return fmt.Errorf("read config for user injection: %w", err)
	}

	// 将配置解析为通用 JSON 对象
	var config map[string]any
	if err := json.Unmarshal(content, &config); err != nil {
		return fmt.Errorf("parse config JSON: %w", err)
	}

	// 更新 inbounds 中的用户
	if err := m.updateInboundUsers(config, users); err != nil {
		return fmt.Errorf("update inbound users: %w", err)
	}

	// 重新序列化为 JSON
	updatedContent, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshal updated config: %w", err)
	}

	if err := m.ApplyConfig(ctx, defaultFilename, updatedContent); err != nil {
		return fmt.Errorf("write updated config: %w", err)
	}

	return nil
}

// updateInboundUsers 更新配置中的所有 inbound 用户。
func (m *Manager) updateInboundUsers(config map[string]any, users []UserConfig) error {
	// 查找 "inbounds" 数组（Sing-box 格式）
	if inbounds, ok := config["inbounds"].([]any); ok {
		for _, inbound := range inbounds {
			if inboundMap, ok := inbound.(map[string]any); ok {
				m.injectUsersIntoSingboxInbound(inboundMap, users)
			}
		}
		return nil
	}

	// 预留 Xray 格式兼容（inbounds + settings.clients 结构）
	return nil
}

// injectUsersIntoSingboxInbound 注入 sing-box inbound 用户。
func (m *Manager) injectUsersIntoSingboxInbound(inbound map[string]any, users []UserConfig) {
	inboundType, _ := inbound["type"].(string) // ok check done by caller, empty string on mismatch

	// 按协议类型构建用户列表
	usersList := make([]map[string]any, 0, len(users))
	for _, u := range users {
		if !u.Enabled {
			continue
		}
		user := map[string]any{
			"name": u.Email,
		}
		switch inboundType {
		case "vless":
			user["uuid"] = u.UUID
			user["flow"] = "xtls-rprx-vision"
		case "vmess":
			user["uuid"] = u.UUID
		case "shadowsocks":
			user["password"] = u.UUID
		case "trojan", "hysteria2", "tuic":
			user["password"] = u.UUID
		default:
			// 未知协议，跳过注入但记录日志
			slog.Warn("unknown inbound type, skipping user injection", "type", inboundType)
			return
		}
		usersList = append(usersList, user)
	}

	if len(usersList) > 0 {
		inbound["users"] = usersList
	}
}

// InjectUsersXray 将用户注入 Xray 配置并重载服务。
func (m *Manager) InjectUsersXray(ctx context.Context, users []UserConfig) error {
	// 读取当前配置文件
	const defaultFilename = "config.json"
	content, err := m.ReadConfig(defaultFilename)
	if err != nil {
		return fmt.Errorf("read config for user injection: %w", err)
	}

	// 将配置解析为通用 JSON 对象
	var config map[string]any
	if err := json.Unmarshal(content, &config); err != nil {
		return fmt.Errorf("parse config JSON: %w", err)
	}

	// 更新 Xray inbounds 中的用户
	if err := m.updateXrayInboundUsers(config, users); err != nil {
		return fmt.Errorf("update xray inbound users: %w", err)
	}

	// 重新序列化为 JSON
	updatedContent, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshal updated config: %w", err)
	}

	if err := m.ApplyConfig(ctx, defaultFilename, updatedContent); err != nil {
		return fmt.Errorf("write updated config: %w", err)
	}

	return nil
}

// updateXrayInboundUsers 更新 Xray 配置中的 inbound 用户。
func (m *Manager) updateXrayInboundUsers(config map[string]any, users []UserConfig) error {
	inbounds, ok := config["inbounds"].([]any)
	if !ok {
		return nil
	}

	for _, inbound := range inbounds {
		inboundMap, ok := inbound.(map[string]any)
		if !ok {
			continue
		}
		m.injectUsersIntoXrayInbound(inboundMap, users)
	}

	return nil
}

// injectUsersIntoXrayInbound 注入 Xray inbound 用户。
func (m *Manager) injectUsersIntoXrayInbound(inbound map[string]any, users []UserConfig) {
	protocol, _ := inbound["protocol"].(string) // ok check done by caller, empty string on mismatch

	// 获取或创建 settings 对象
	settings, ok := inbound["settings"].(map[string]any)
	if !ok {
		settings = make(map[string]any)
		inbound["settings"] = settings
	}

	// 按协议类型构建 clients 列表
	clients := make([]map[string]any, 0, len(users))
	for _, u := range users {
		if !u.Enabled {
			continue
		}
		client := map[string]any{
			"email": u.Email,
			"level": 0,
		}
		switch protocol {
		case "vless":
			client["id"] = u.UUID
			client["flow"] = "xtls-rprx-vision"
		case "vmess":
			client["id"] = u.UUID
			client["alterId"] = 0
		case "shadowsocks":
			client["password"] = u.UUID
		case "trojan":
			client["password"] = u.UUID
		default:
			// 未知协议，跳过注入但记录日志
			slog.Warn("unknown inbound protocol, skipping user injection", "protocol", protocol)
			return
		}
		clients = append(clients, client)
	}

	if len(clients) > 0 {
		settings["clients"] = clients
	}
}

// DetectCoreType 尝试判断配置属于 sing-box 还是 xray。
func (m *Manager) DetectCoreType() string {
	const defaultFilename = "config.json"
	content, err := m.ReadConfig(defaultFilename)
	if err != nil {
		return "unknown"
	}
	return detectCoreTypeFromContent(content)
}

func detectCoreTypeFromContent(content []byte) string {
	var config map[string]any
	if err := json.Unmarshal(content, &config); err != nil {
		return "unknown"
	}

	return detectCoreTypeFromObject(config)
}

func detectCoreTypeFromObject(config map[string]any) string {
	if _, hasAPI := config["api"]; hasAPI {
		return "xray"
	}
	if _, hasPolicy := config["policy"]; hasPolicy {
		return "xray"
	}

	if _, hasExperimental := config["experimental"]; hasExperimental {
		return "sing-box"
	}

	if inbounds, ok := config["inbounds"].([]any); ok && len(inbounds) > 0 {
		if inbound, ok := inbounds[0].(map[string]any); ok {
			if _, hasProtocol := inbound["protocol"]; hasProtocol {
				return "xray"
			}
			if _, hasType := inbound["type"]; hasType {
				return "sing-box"
			}
		}
	}

	return "unknown"
}
