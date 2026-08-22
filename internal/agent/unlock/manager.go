package unlock

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Reporter 负责将解锁检测结果上报到面板。
type Reporter interface {
	Report(ctx context.Context, results []Result) error
}

// HTTPReporter 通过面板 HTTP 端点上报结果（复用 agent token 鉴权）。
type HTTPReporter struct {
	baseURL   string
	hostToken string
	client    *http.Client
	logger    *slog.Logger
}

// NewHTTPReporter 创建 HTTP 上报器。
// baseURL 是面板 HTTP 地址（如 http://127.0.0.1:18080）。
func NewHTTPReporter(baseURL, hostToken string, logger *slog.Logger) *HTTPReporter {
	return &HTTPReporter{
		baseURL:   baseURL,
		hostToken: hostToken,
		client:    &http.Client{Timeout: 15 * time.Second},
		logger:    logger,
	}
}

func (r *HTTPReporter) Report(ctx context.Context, results []Result) error {
	if r == nil || r.baseURL == "" || r.hostToken == "" || len(results) == 0 {
		return nil
	}
	body, err := json.Marshal(map[string]any{"results": results})
	if err != nil {
		return err
	}
	reportURL := stringsTrimRight(r.baseURL, "/") + "/api/v1/agent/unlock?token=" + url.QueryEscape(r.hostToken)
	req, err := http.NewRequestWithContext(ctx, "POST", reportURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("report unlock results rejected: HTTP %d", resp.StatusCode)
	}
	return nil
}

func stringsTrimRight(s, cut string) string {
	for len(s) > 0 && s[len(s)-1] == cut[0] {
		s = s[:len(s)-1]
	}
	return s
}

// Manager 管理解锁检测的触发与上报。
// 支持两种触发方式：每日自查（内部 timer）与外部命令触发（面板下发）。
type Manager struct {
	detector *Detector
	reporter Reporter
	logger   *slog.Logger

	// services 是本次要检测的平台列表；空则使用默认列表。
	services []Service

	interval time.Duration // 每日自查间隔
	stopCh   chan struct{}
	stopOnce sync.Once
	mu       sync.Mutex
	// lastReported 缓存上次上报结果，避免结果未变化时重复上报。
	lastReported []Result
}

// NewManager 创建解锁检测管理器。
// interval 为每日自查间隔（如 24h）；reporter 可为 nil（仅本地检测不上报）。
func NewManager(detector *Detector, reporter Reporter, logger *slog.Logger, services []Service, interval time.Duration) *Manager {
	if detector == nil {
		detector = NewDetector(15 * time.Second)
	}
	if len(services) == 0 {
		services = AllServices()
	}
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	return &Manager{
		detector: detector,
		reporter: reporter,
		logger:   logger,
		services: services,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start 启动每日自查循环。
func (m *Manager) Start() {
	go m.runLoop()
}

// Stop 停止自查循环。
func (m *Manager) Stop() {
	m.stopOnce.Do(func() { close(m.stopCh) })
}

// runLoop 每日执行一次自查并上报；若上报失败则下个周期重试。
func (m *Manager) runLoop() {
	timer := time.NewTimer(m.interval)
	defer timer.Stop()
	for {
		select {
		case <-m.stopCh:
			return
		case <-timer.C:
			m.ProbeAndReport(context.Background())
			timer.Reset(m.interval)
		}
	}
}

// ProbeAndReport 立即执行一次检测并上报（供外部命令触发调用）。
func (m *Manager) ProbeAndReport(ctx context.Context) {
	m.probeAndReport(ctx, m.services)
}

// ProbeAndReportWith 用指定平台列表立即检测并上报。services 为空时回退到默认列表。
func (m *Manager) ProbeAndReportWith(ctx context.Context, services []Service) {
	if len(services) == 0 {
		services = m.services
	}
	m.probeAndReport(ctx, services)
}

func (m *Manager) probeAndReport(ctx context.Context, services []Service) {
	results := m.detector.ProbeAll(ctx, services)
	slog.Info("unlock probe completed", "count", len(results))

	m.mu.Lock()
	unchanged := sameResults(results, m.lastReported)
	m.mu.Unlock()
	if unchanged {
		slog.Debug("unlock results unchanged, skip report")
		return
	}

	if m.reporter != nil {
		if err := m.reporter.Report(ctx, results); err != nil {
			slog.Warn("unlock report failed", "error", err)
			return
		}
	}
	m.mu.Lock()
	m.lastReported = results
	m.mu.Unlock()
}

// sameResults 比较两次结果是否完全一致（不含 detail/error 细节的稳定字段）。
func sameResults(a, b []Result) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Service != b[i].Service || a[i].Status != b[i].Status || a[i].Region != b[i].Region {
			return false
		}
	}
	return true
}