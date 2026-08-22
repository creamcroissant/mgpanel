package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	agentv1 "github.com/creamcroissant/mgpanel/pkg/pb/agent/v1"
)

// OriginLatencyProbeConfig controls origin latency probing.
type OriginLatencyProbeConfig struct {
	Interval time.Duration
	Timeout  time.Duration
}

// DefaultOriginLatencyProbeConfig returns default probe settings.
func DefaultOriginLatencyProbeConfig() OriginLatencyProbeConfig {
	return OriginLatencyProbeConfig{
		Interval: 5 * time.Minute,
		Timeout:  5 * time.Second,
	}
}

// probeAndReportOriginLatency measures latency to each CDN site's origin
// and reports via the gRPC transport.
func (a *Agent) probeAndReportOriginLatency(ctx context.Context) {
	if a == nil || a.cdnManager == nil {
		return
	}

	sites := a.cdnManager.ListSites()
	if len(sites) == 0 {
		return
	}

	for _, site := range sites {
		origin := site.OriginURL
		if origin == "" {
			continue
		}

		for _, stack := range []string{"v4", "v6"} {
			latency, err := probeHTTPLatency(ctx, origin, stack, 5*time.Second)
			if err != nil {
				slog.Debug("origin latency probe failed",
					"site", site.Domain, "stack", stack, "error", err)
				continue
			}
			slog.Debug("origin latency probed",
				"site", site.Domain, "stack", stack, "latency_ms", latency.Milliseconds())

			a.reportOriginLatencyToPanel(ctx, int64(site.ID), site.Domain, stack, latency.Milliseconds())
		}
	}
}

// probeHTTPLatency measures time to receive response headers.
func probeHTTPLatency(ctx context.Context, targetURL, stack string, timeout time.Duration) (time.Duration, error) {
	url := normalizeProbeURL(targetURL)
	start := time.Now()

	dialer := &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	tr := &http.Transport{
		DisableKeepAlives: true,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			switch stack {
			case "v4", "ipv4":
				return dialer.DialContext(ctx, "tcp4", addr)
			case "v6", "ipv6":
				return dialer.DialContext(ctx, "tcp6", addr)
			default:
				return dialer.DialContext(ctx, network, addr)
			}
		},
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   timeout,
	}

	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	return time.Since(start), nil
}

// normalizeProbeURL ensures a valid HTTP URL for probing.
func normalizeProbeURL(origin string) string {
	if strings.HasPrefix(origin, "http://") || strings.HasPrefix(origin, "https://") {
		return origin
	}
	return "https://" + origin
}

// reportOriginLatencyToPanel stores latency data for the next gRPC status report.
func (a *Agent) reportOriginLatencyToPanel(ctx context.Context, siteID int64, domain, stack string, latencyMs int64) {
	domainKey := domain + "|" + stack
	a.cdnProbeResultsMu.Lock()
	defer a.cdnProbeResultsMu.Unlock()
	for i, e := range a.cdnProbeResults {
		if e.Domain == domainKey {
			a.cdnProbeResults[i] = &agentv1.OriginLatencyEntry{
				Domain:      domainKey,
				LatencyMs:   float64(latencyMs),
				PacketLoss:  0,
				TotalProbes: 1,
				UpdatedAt:   time.Now().Unix(),
				SiteId:      siteID,
				Stack:       stack,
			}
			return
		}
	}
	a.cdnProbeResults = append(a.cdnProbeResults, &agentv1.OriginLatencyEntry{
		Domain:      domainKey,
		LatencyMs:   float64(latencyMs),
		PacketLoss:  0,
		TotalProbes: 1,
		UpdatedAt:   time.Now().Unix(),
		SiteId:      siteID,
		Stack:       stack,
	})
}
