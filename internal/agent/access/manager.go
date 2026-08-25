package access

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/creamcroissant/mgpanel/internal/agent/core"
	"github.com/creamcroissant/mgpanel/internal/agent/transport"
	agentv1 "github.com/creamcroissant/mgpanel/pkg/pb/agent/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Manager struct {
	client  *transport.GRPCClient
	manager *core.Manager
	logger  *slog.Logger

	// singboxConfigDir 是 sing-box 配置目录，用于注入 experimental.clash_api 段
	singboxConfigDir string

	collectors []Collector
	uploadQueue *UploadQueue
	stopCh     chan struct{}
	stopOnce   sync.Once
}

func NewManager(client *transport.GRPCClient, coreManager *core.Manager, singboxConfigDir string, logger *slog.Logger) *Manager {
	return &Manager{
		client:           client,
		manager:          coreManager,
		logger:           logger,
		singboxConfigDir: singboxConfigDir,
		stopCh:           make(chan struct{}),
	}
}

func (m *Manager) Start() {
	// 注入 sing-box experimental.clash_api 配置（访问日志收集依赖此段）
	if err := m.ensureClashAPIConfig(); err != nil {
		m.logger.Warn("failed to ensure sing-box clash_api config", "error", err)
	} else {
		// 写入 experimental.json 后尝试 reload sing-box 使其加载新配置段
		if err := m.reloadSingBox(context.Background()); err != nil {
			m.logger.Warn("reload sing-box after clash_api config write", "error", err)
		}
		m.logger.Info("sing-box clash_api config ensured", "dir", m.singboxConfigDir)

	// 创建有界上报队列（worker 限速批量上报）
	m.uploadQueue = NewUploadQueue(m.report, m.logger)
	}

	// Register collectors
	m.collectors = []Collector{
		NewXrayAccessCollector(m.manager, m.logger),
		NewSingboxAccessCollector(m.manager, m.singboxConfigDir, m.logger),
	}

	go m.run()
}

// ensureClashAPIConfig 在 sing-box 配置目录创建 experimental.json，
// 使 sing-box 暴露 Clash API（/connections），供访问日志收集器读取。
func (m *Manager) ensureClashAPIConfig() error {
	dir := m.singboxConfigDir
	if dir == "" {
		dir = "/etc/sing-box/conf"
	}
	target := filepath.Join(dir, "experimental.json")
	secret := m.loadOrCreateSecret(filepath.Join(dir, ".access_secret"))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create sing-box conf dir: %w", err)
	}
	payload := map[string]any{
		"experimental": map[string]any{
			"clash_api": map[string]any{
				"external_controller": "127.0.0.1:19090",
				"secret":             secret,
			},
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return os.WriteFile(target, data, 0644)
}

// loadOrCreateSecret 读取或创建访问日志使用的 Clash API secret。
func (m *Manager) loadOrCreateSecret(path string) string {
	if b, err := os.ReadFile(path); err == nil {
		if s := string(b); len(s) >= 16 {
			return s
		}
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		// 兜底：时间戳哈希
		return fmt.Sprintf("mgpanel_access_%d", time.Now().UnixNano())
	}
	s := hex.EncodeToString(buf)
	_ = os.WriteFile(path, []byte(s), 0600)
	return s
}

func (m *Manager) Stop() {
	m.stopOnce.Do(func() {
		if m.uploadQueue != nil {
			m.uploadQueue.Stop()
		}
		close(m.stopCh)
	})
}

// reloadSingBox 发送 SIGHUP 让 sing-box 重新加载配置（含 experimental.json）。
func (m *Manager) reloadSingBox(ctx context.Context) error {
	// 默认 sing-box 服务名
	return exec.CommandContext(ctx, "systemctl", "reload", "sing-box").Run()
}

func (m *Manager) run() {
	// TODO: Make interval configurable
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			// staged apply 可能切换整个配置目录（rename）冲掉 experimental.json，
			// 每次轮询前检查并自愈（缺失才重建，避免频繁 reload）
			m.EnsureClashAPIConfig()
			m.collectAndReport()
		}
	}
}

// EnsureClashAPIConfig 幂等地确保 experimental.json 存在；缺失时才重建并 reload。
func (m *Manager) EnsureClashAPIConfig() error {
	dir := m.singboxConfigDir
	if dir == "" {
		dir = "/etc/sing-box/conf"
	}
	target := filepath.Join(dir, "experimental.json")
	if _, err := os.Stat(target); err == nil {
		return nil // 已存在，不需重建
	}
	if err := m.ensureClashAPIConfig(); err != nil {
		return err
	}
	if err := m.reloadSingBox(context.Background()); err != nil {
		m.logger.Warn("reload sing-box after clash_api config restore", "error", err)
	}
	m.logger.Info("sing-box clash_api config restored", "dir", dir)
	return nil
}

func (m *Manager) collectAndReport() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var allEntries []AccessLogEntry
	for _, collector := range m.collectors {
		entries, err := collector.Collect(ctx)
		if err != nil {
			m.logger.Error("failed to collect access logs",
				"collector", collector.Type(),
				"error", err,
			)
			continue
		}
		if len(entries) > 0 {
			allEntries = append(allEntries, entries...)
		}
	}

	if len(allEntries) == 0 {
		return
	}

	m.logger.Debug("collected access logs", "count", len(allEntries))

	// 入队异步上报；队列满时丢弃最旧条目
	if m.uploadQueue != nil {
		m.uploadQueue.Enqueue(allEntries)
	} else {
		// 降级：直接同步上报（无队列时）
		if err := m.report(ctx, allEntries); err != nil {
			m.logger.Error("failed to report access logs", "error", err)
		}
	}
}

func (m *Manager) report(ctx context.Context, entries []AccessLogEntry) error {
	if !m.client.IsHealthy() {
		return nil // skip if not connected
	}

	req := &agentv1.AccessLogReport{
		Entries: make([]*agentv1.AccessLogEntry, len(entries)),
	}

	for i, entry := range entries {
		req.Entries[i] = &agentv1.AccessLogEntry{
			UserEmail:       entry.UserEmail,
			SourceIp:        entry.SourceIP,
			TargetDomain:    entry.TargetDomain,
			TargetIp:        entry.TargetIP,
			TargetPort:      int32(entry.TargetPort),
			Protocol:        entry.Protocol,
			Upload:          entry.Upload,
			Download:        entry.Download,
			ConnectionStart: timestamppb.New(entry.ConnectionStart),
		}
		if !entry.ConnectionEnd.IsZero() {
			req.Entries[i].ConnectionEnd = timestamppb.New(entry.ConnectionEnd)
		}
	}

	resp, err := m.client.ReportAccessLogs(ctx, req)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("server rejected logs: %s", resp.Message)
	}

	return nil
}
