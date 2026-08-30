// Package geoip implements the agent-side country/region reporter.
//
// The reporter periodically (or on demand) detects the agent's public IP
// and queries the free ip-api.com endpoint to translate that IP into a
// country code and region name. Results are reported back to the panel
// via the existing /api/v1/agent/host channel — the panel layer is
// responsible for any mmdb-based fallback.
package geoip

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/agent/config"
)

// DefaultInterval is the default reporting interval (one week).
const DefaultInterval = 168 * time.Hour

// publicIPEndpoints mirrors config.DetectPublicIP but adds context
// support so the reporter can be cancelled.
var publicIPEndpoints = []string{
	"https://api.ipify.org",
	"https://ifconfig.me/ip",
	"https://icanhazip.com",
	"https://ipinfo.io/ip",
}

// ipAPIBase is the free ip-api.com endpoint template. We only ask for
// the fields we need to keep the response tiny and the parser simple.
// Declared as a var so tests can point it at a local httptest server.
var ipAPIBase = "http://ip-api.com/json/%s?fields=status,country,regionName"

// Reporter periodically probes the public IP, queries ip-api.com and
// yields (country, region) for the caller to upload. Safe for concurrent
// RunOnce calls; only one Start/Stop lifecycle is supported.
type Reporter struct {
	cfg        *config.Config
	httpClient *http.Client
	interval   time.Duration
	logger     *slog.Logger
	nowFunc    func() time.Time

	stopCh chan struct{}
	doneCh chan struct{}
}

// NewReporter constructs a reporter with a private http client. If cfg
// is nil or logger is nil sensible defaults are used so the reporter
// can run under testing() too.
func NewReporter(cfg *config.Config, logger *slog.Logger) *Reporter {
	if logger == nil {
		logger = slog.Default()
	}
	return &Reporter{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 8 * time.Second},
		interval:   DefaultInterval,
		logger:     logger,
		nowFunc:    time.Now,
		stopCh:     make(chan struct{}),
		doneCh:     make(chan struct{}),
	}
}

// Start launches the periodic loop. It runs RunOnce once immediately,
// then re-runs every interval. Blocks until ctx is cancelled or Stop is
// called. The first return of doneCh signals that the loop exited.
func (r *Reporter) Start(ctx context.Context) {
	if r == nil {
		return
	}
	go r.loop(ctx)
}

func (r *Reporter) loop(ctx context.Context) {
	defer close(r.doneCh)

	// Immediate probe — agent self-reports country/region right after boot.
	country, region, err := r.RunOnce(ctx)
	if err != nil {
		r.logger.Debug("geo reporter initial probe failed", "error", err)
	} else {
		r.logger.Info("geo reporter initial probe ok", "country", country, "region", region)
	}

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopCh:
			return
		case <-ticker.C:
			country, region, err := r.RunOnce(ctx)
			if err != nil {
				r.logger.Debug("geo reporter periodic probe failed", "error", err)
				continue
			}
			r.logger.Info("geo reporter periodic probe ok", "country", country, "region", region)
		}
	}
}

// Stop signals the loop to exit and waits up to 5 seconds. Safe to call
// multiple times; subsequent calls are no-ops.
func (r *Reporter) Stop() {
	if r == nil {
		return
	}
	select {
	case <-r.stopCh:
		// already closed
	default:
		close(r.stopCh)
	}
	select {
	case <-r.doneCh:
	case <-time.After(5 * time.Second):
		r.logger.Warn("geo reporter stop timeout")
	}
}

// RunOnce performs a single public-IP -> country/region lookup.
// Returns ("", "", err) on any failure; the caller is expected to
// upload whatever it gets and let the panel fall back to mmdb.
func (r *Reporter) RunOnce(ctx context.Context) (country, region string, err error) {
	if r == nil {
		return "", "", fmt.Errorf("nil reporter")
	}
	if r.cfg == nil {
		return "", "", fmt.Errorf("nil agent config")
	}
	ip, err := r.detectPublicIP(ctx)
	if err != nil {
		return "", "", fmt.Errorf("detect public ip: %w", err)
	}
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return "", "", fmt.Errorf("empty public ip")
	}

	apiCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	url := fmt.Sprintf(ipAPIBase, ip)
	req, err := http.NewRequestWithContext(apiCtx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", fmt.Errorf("build ip-api request: %w", err)
	}
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("ip-api request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("ip-api status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
	if err != nil {
		return "", "", fmt.Errorf("read ip-api body: %w", err)
	}
	var parsed struct {
		Status     string `json:"status"`
		Country    string `json:"country"`
		RegionName string `json:"regionName"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", "", fmt.Errorf("parse ip-api body: %w", err)
	}
	if !strings.EqualFold(parsed.Status, "success") {
		return "", "", fmt.Errorf("ip-api status=%s", parsed.Status)
	}
	return strings.TrimSpace(parsed.Country), strings.TrimSpace(parsed.RegionName), nil
}

// detectPublicIP is a context-aware variant of config.DetectPublicIP.
func (r *Reporter) detectPublicIP(ctx context.Context) (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	for _, ep := range publicIPEndpoints {
		ip, err := r.fetchPublicIP(ctx, client, ep)
		if err == nil && ip != "" {
			return ip, nil
		}
	}
	return "", fmt.Errorf("all public ip endpoints failed")
}

func (r *Reporter) fetchPublicIP(ctx context.Context, client *http.Client, endpoint string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("status %d from %s", resp.StatusCode, endpoint)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil {
		return "", err
	}
	ip := strings.TrimSpace(string(body))
	if ip == "" {
		return "", fmt.Errorf("empty ip from %s", endpoint)
	}
	return ip, nil
}
