package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/agent/config"
)

// syncAgentHost 周期向面板上报本机公网 IP，使 agent_hosts.host 保持为真实可达地址，
// 避免 pending-* 占位符导致 mesh peer endpoint / 订阅节点 host 不可解析。
// 手动配置的 advertise_host 优先；否则探测公网 IP（带缓存，避免每次 sync 都调外网 API）。
func (a *Agent) syncAgentHost(ctx context.Context, country, region string) {
	if a == nil || a.cfg == nil {
		return
	}
	hostToken := strings.TrimSpace(a.cfg.Panel.HostToken)
	if hostToken == "" {
		return
	}

	advertiseHost := strings.TrimSpace(a.cfg.Panel.AdvertiseHost)
	if advertiseHost == "" {
		advertiseHost = a.getCachedAdvertiseHost()
		if advertiseHost == "" {
			advertiseHost = config.DetectPublicIP()
			if advertiseHost == "" {
				return // 探测失败，等下一次 sync 再试
			}
			a.setCachedAdvertiseHost(advertiseHost)
		}
	}

	base := resolvePanelHTTPBase(a.cfg)
	base = strings.TrimSuffix(base, "/")
	reportURL := base + "/api/v1/agent/host?token=" + url.QueryEscape(hostToken)

	payload := map[string]string{"host": advertiseHost, "country": strings.TrimSpace(country), "region": strings.TrimSpace(region)}
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reportURL, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		slog.Debug("report agent host failed", "error", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		slog.Debug("report agent host rejected", "status", resp.StatusCode)
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	slog.Debug("reported agent host", "host", advertiseHost)
}

// resolvePanelHTTPBase 解析面板 HTTP 基础地址。
// 优先使用 panel.url；为空时从 grpc.address 推断（与注册逻辑一致）。
func resolvePanelHTTPBase(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	base := strings.TrimSpace(cfg.Panel.URL)
	if base != "" {
		return base
	}
	grpcAddr := strings.TrimSpace(cfg.GRPC.Address)
	host, port, err := net.SplitHostPort(grpcAddr)
	if err != nil {
		return ""
	}
	scheme := "http"
	if cfg.GRPC.TLS.Enabled {
		scheme = "https"
	}
	return scheme + "://" + net.JoinHostPort(host, port)
}
