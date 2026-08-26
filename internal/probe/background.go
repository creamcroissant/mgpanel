package probepkg

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"
)

// ProbeResultMeta holds metadata for a probe target.
type ProbeTargetConfig struct {
	Type     string `json:"type"`   // "tcpping", "httpget", "icmpping"
	Target   string `json:"target"` // host:port for TCP, URL for HTTP, host for ICMP
	Port     int    `json:"port,omitempty"`
	Province string `json:"province,omitempty"` // e.g. "guangdong"
	ISP      string `json:"isp,omitempty"`      // "mobile", "unicom", "telecom"
	Label    string `json:"label,omitempty"`    // human-readable label
	Timeout  int    `json:"timeout,omitempty"`  // seconds, default 5
}

// ToProbeTarget converts a config to a ProbeTarget.
func (c ProbeTargetConfig) ToProbeTarget() ProbeTarget {
	t := ProbeTarget{Target: c.Target, Port: c.Port}
	switch c.Type {
	case "httpget":
		t.Type = ProbeTypeHTTPGet
	case "icmpping":
		t.Type = ProbeTypeICMPPing
	default:
		t.Type = ProbeTypeTCPPing
	}
	if c.Timeout > 0 {
		t.Timeout = time.Duration(c.Timeout) * time.Second
	}
	return t
}

// TargetHistory holds the sliding window of probe results for one target.
type TargetHistory struct {
	Config     ProbeTargetConfig `json:"config"`
	Window     []ProbeResult     `json:"window"`
	WindowSize int               `json:"window_size"`

	// Computed metrics
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	PacketLoss   float64 `json:"packet_loss"` // 0.0 – 1.0
	LastProbeAt  int64   `json:"last_probe_at"`
	TotalProbes  int     `json:"total_probes"`
	FailedProbes int     `json:"failed_probes"`
}

// LatencyScore returns a composite score (lower = better).
// Penalises packet loss: a target with 20% loss gets its latency doubled.
func (h *TargetHistory) LatencyScore() float64 {
	if h.TotalProbes == 0 {
		return 99999
	}
	penalty := 1.0 + h.PacketLoss*5.0
	return h.AvgLatencyMs * penalty
}

// BackgroundProber continually probes configured targets and maintains
// a sliding window of results for latency + packet-loss analysis.
type BackgroundProber struct {
	mu       sync.RWMutex
	targets  []ProbeTargetConfig
	history  map[string]*TargetHistory // key = target string
	interval time.Duration
	windowSz int
	timeout  time.Duration
	stopCh   chan struct{}
	stopOnce sync.Once
	running  bool
	wg       sync.WaitGroup
	logger   *slog.Logger
}

// NewBackgroundProber creates a new background prober.
//
//	interval: how often to re-probe all targets
//	windowSz: number of recent results to retain per target
func NewBackgroundProber(interval time.Duration, windowSz int, logger *slog.Logger) *BackgroundProber {
	if logger == nil {
		logger = slog.Default()
	}
	if windowSz <= 0 {
		windowSz = 10
	}
	return &BackgroundProber{
		interval: interval,
		windowSz: windowSz,
		timeout:  5 * time.Second,
		logger:   logger.With("component", "background_prober"),
	}
}

// SetTargets sets the probe targets, replacing any previous set.
// Old history entries for removed targets are pruned.
func (p *BackgroundProber) SetTargets(targets []ProbeTargetConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.targets = targets
	// Rebuild history — remove entries for targets no longer in the list
	fresh := make(map[string]*TargetHistory, len(targets))
	for _, t := range targets {
		key := targetKey(t)
		if existing, ok := p.history[key]; ok {
			fresh[key] = existing
		} else {
			fresh[key] = &TargetHistory{
				Config:     t,
				Window:     make([]ProbeResult, 0, p.windowSz),
				WindowSize: p.windowSz,
			}
		}
	}
	p.history = fresh
}

// Start begins the background probing loop.
// Blocks until the context is cancelled.
func (p *BackgroundProber) Start(ctx context.Context) {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return
	}
	p.running = true
	p.stopCh = make(chan struct{})
	p.wg.Add(1)
	p.mu.Unlock()
	defer p.wg.Done()

	p.logger.Info("background prober started",
		"interval", p.interval,
		"window_size", p.windowSz,
		"targets", len(p.targets))

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	// Run an initial probe immediately
	p.probeAll(ctx)

	for {
		select {
		case <-ticker.C:
			p.probeAll(ctx)
		case <-p.stopCh:
			p.mu.Lock()
			p.running = false
			p.mu.Unlock()
			p.logger.Info("background prober stopped via Stop()")
			return
		case <-ctx.Done():
			p.mu.Lock()
			p.running = false
			p.mu.Unlock()
			p.logger.Info("background prober stopped")
			return
		}
	}
}

// Stop signals the probing loop to stop.
func (p *BackgroundProber) Stop() {
	p.stopOnce.Do(func() {
		p.mu.Lock()
		ch := p.stopCh
		p.mu.Unlock()
		if ch != nil {
			close(ch)
		}
	})
	p.wg.Wait()
}

// GetResults returns all target histories.
func (p *BackgroundProber) GetResults() []*TargetHistory {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]*TargetHistory, 0, len(p.history))
	for _, h := range p.history {
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Config.Label < out[j].Config.Label
	})
	return out
}

// GetResultsByISP returns target histories filtered by ISP.
func (p *BackgroundProber) GetResultsByISP(isp string) []*TargetHistory {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var out []*TargetHistory
	for _, h := range p.history {
		if h.Config.ISP == isp {
			out = append(out, h)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].AvgLatencyMs < out[j].AvgLatencyMs
	})
	return out
}

// GetRanking returns all targets with non-zero data, sorted by composite score.
func (p *BackgroundProber) GetRanking() []*TargetHistory {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var out []*TargetHistory
	for _, h := range p.history {
		if h.TotalProbes > 0 {
			out = append(out, h)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].LatencyScore() < out[j].LatencyScore()
	})
	return out
}

// GetISPLatency returns the average latency to a given ISP across all targets.
// Returns 0 if no data.
func (p *BackgroundProber) GetISPLatency(isp string) float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var total float64
	var count int
	for _, h := range p.history {
		if h.Config.ISP == isp && h.AvgLatencyMs > 0 {
			total += h.AvgLatencyMs
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return total / float64(count)
}

// SetTimeout sets a custom probe timeout. For testing.
func (p *BackgroundProber) SetTimeout(d time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.timeout = d
}

// ProbeOnce runs a single probe cycle against all configured targets.
// Useful for testing and manual triggers.
func (p *BackgroundProber) ProbeOnce(ctx context.Context) {
	p.probeAll(ctx)
}

func (p *BackgroundProber) probeAll(ctx context.Context) {
	p.mu.RLock()
	targets := p.targets
	p.mu.RUnlock()

	if len(targets) == 0 {
		p.logger.Debug("no probe targets configured, skipping")
		return
	}

	probeTargets := make([]ProbeTarget, len(targets))
	for i, t := range targets {
		probeTargets[i] = t.ToProbeTarget()
	}

	p.mu.RLock()
	timeout := p.timeout
	p.mu.RUnlock()
	results := BatchProbe(ctx, probeTargets, timeout)

	// Check if the context was cancelled during probing
	if ctx.Err() != nil {
		p.logger.Debug("probe cancelled during batch probe", "error", ctx.Err())
		return
	}

	p.mu.Lock()
	var failedCount int
	var firstErrs []string
	for i, res := range results {
		key := targetKey(targets[i])
		h, ok := p.history[key]
		if !ok {
			continue
		}
		h.TotalProbes++
		h.Window = append(h.Window, res)
		// Slide window
		if len(h.Window) > p.windowSz {
			h.Window = h.Window[len(h.Window)-p.windowSz:]
		}
		h.LastProbeAt = time.Now().UnixMilli()
		if !res.Success {
			h.FailedProbes++
			failedCount++
			if len(firstErrs) < 3 {
				firstErrs = append(firstErrs, fmt.Sprintf("%s: %s", key, res.Error))
			}
		}
		// Recompute metrics
		h.computeMetrics()
	}
	p.mu.Unlock()

	if failedCount > 0 {
		p.logger.Warn("probe batch had failures",
			"total", len(results),
			"failed", failedCount,
			"sample_errors", firstErrs)
	}
}

func (h *TargetHistory) computeMetrics() {
	var totalLatency time.Duration
	var successCount int

	for _, r := range h.Window {
		if r.Success {
			totalLatency += r.Latency
			successCount++
		}
	}

	windowLen := len(h.Window)
	if windowLen == 0 {
		h.AvgLatencyMs = 0
		h.PacketLoss = 0
		return
	}

	if successCount > 0 {
		h.AvgLatencyMs = float64(totalLatency.Microseconds()) / float64(successCount) / 1000.0
	} else {
		h.AvgLatencyMs = 0
	}
	h.PacketLoss = float64(windowLen-successCount) / float64(windowLen)
}

func targetKey(t ProbeTargetConfig) string {
	return fmt.Sprintf("%s:%s:%d", t.Type, t.Target, t.Port)
}
