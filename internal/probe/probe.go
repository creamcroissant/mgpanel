package probepkg

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// icmpReqID is an atomic counter for generating unique ICMP Echo request IDs.
var icmpReqID atomic.Uint32

func init() {
	icmpReqID.Store(1)
}

// ProbeType defines the type of probe.
type ProbeType int

const (
	ProbeTypeTCPPing  ProbeType = iota + 1 // TCP connect latency
	ProbeTypeHTTPGet                       // HTTP GET latency
	ProbeTypeICMPPing                      // ICMP echo latency
)

// ProbeResult holds the result of a single probe.
type ProbeResult struct {
	Type    ProbeType     `json:"type"`
	Target  string        `json:"target"`
	Latency time.Duration `json:"latency"`
	Success bool          `json:"success"`
	Error   string        `json:"error,omitempty"`
}

// TCPPing measures TCP connection latency to host:port.
func TCPPing(ctx context.Context, addr string, timeout time.Duration) (time.Duration, error) {
	dialer := net.Dialer{Timeout: timeout}
	start := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return 0, fmt.Errorf("tcp ping %s: %w", addr, err)
	}
	conn.Close()
	return time.Since(start), nil
}

// HTTPGet measures HTTP GET latency to a URL (only header reception).
func HTTPGet(ctx context.Context, url string, timeout time.Duration) (time.Duration, int, error) {
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
		// Don't follow redirects — we only care about the first response
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("http get %s: create request: %w", url, err)
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("http get %s: %w", url, err)
	}
	defer resp.Body.Close()

	latency := time.Since(start)
	return latency, resp.StatusCode, nil
}

// ICMPPing sends an ICMP echo request and measures round-trip time.
// Requires root/CAP_NET_RAW on Linux.
func ICMPPing(ctx context.Context, addr string, timeout time.Duration) (time.Duration, error) {
	// Resolve IP first
	ip, err := net.ResolveIPAddr("ip4", addr)
	if err != nil {
		return 0, fmt.Errorf("icmp ping %s: resolve: %w", addr, err)
	}

	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return 0, fmt.Errorf("icmp ping %s: listen: %w", addr, err)
	}
	defer conn.Close()

	// Set deadline
	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline)
	} else {
		conn.SetDeadline(time.Now().Add(timeout))
	}

	// Assign unique request ID to avoid accepting unrelated ICMP replies
	reqID := int(icmpReqID.Add(1) & 0xffff)

	// Build ICMP echo request
	echo := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   reqID,
			Seq:  1,
			Data: []byte("xboard-cdn-probe"),
		},
	}

	data, err := echo.Marshal(nil)
	if err != nil {
		return 0, fmt.Errorf("icmp ping %s: marshal: %w", addr, err)
	}

	start := time.Now()
	if _, err := conn.WriteTo(data, &net.IPAddr{IP: ip.IP}); err != nil {
		return 0, fmt.Errorf("icmp ping %s: write: %w", addr, err)
	}

	// Read reply
	reply := make([]byte, 1500)
	n, _, err := conn.ReadFrom(reply)
	if err != nil {
		return 0, fmt.Errorf("icmp ping %s: read: %w", addr, err)
	}
	// Check context cancellation after blocking ReadFrom
	if ctx.Err() != nil {
		return 0, fmt.Errorf("icmp ping %s: context cancelled: %w", addr, ctx.Err())
	}

	// Parse reply
	parsed, err := icmp.ParseMessage(ipv4.ICMPTypeEchoReply.Protocol(), reply[:n])
	if err != nil {
		return 0, fmt.Errorf("icmp ping %s: parse: %w", addr, err)
	}

	if parsed.Type != ipv4.ICMPTypeEchoReply {
		return 0, fmt.Errorf("icmp ping %s: unexpected reply type %v", addr, parsed.Type)
	}

	echoReply, ok := parsed.Body.(*icmp.Echo)
	if !ok {
		return 0, fmt.Errorf("icmp ping %s: unexpected reply body type %T", addr, parsed.Body)
	}
	if echoReply.ID != reqID {
		return 0, fmt.Errorf("icmp ping %s: echo id mismatch (got %d, want %d)", addr, echoReply.ID, reqID)
	}

	return time.Since(start), nil
}

// ProbeConfig defines a single probe configuration.
type ProbeTarget struct {
	Type ProbeType `json:"type"`
	// For TCPPing: "host:port" (e.g. "1.2.3.4:443" or "example.com:80")
	// For HTTPGet: full URL (e.g. "https://example.com/")
	// For ICMPPing: host (e.g. "1.2.3.4" or "example.com")
	Target  string        `json:"target"`
	Port    int           `json:"port,omitempty"` // used for TCPPing when target is host-only
	Timeout time.Duration `json:"timeout,omitempty"`
}

// Probe runs a single probe against a target and returns the result.
func Probe(ctx context.Context, cfg ProbeTarget, defaultTimeout time.Duration) ProbeResult {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	result := ProbeResult{
		Type:   cfg.Type,
		Target: cfg.Target,
	}

	switch cfg.Type {
	case ProbeTypeTCPPing:
		addr := cfg.Target
		// If target doesn't include port, append it
		if cfg.Port > 0 {
			addr = fmt.Sprintf("%s:%d", cfg.Target, cfg.Port)
		}
		latency, err := TCPPing(ctx, addr, timeout)
		if err != nil {
			result.Error = err.Error()
			return result
		}
		result.Success = true
		result.Latency = latency

	case ProbeTypeHTTPGet:
		latency, statusCode, err := HTTPGet(ctx, cfg.Target, timeout)
		if err != nil {
			result.Error = err.Error()
			return result
		}
		result.Success = true
		result.Latency = latency
		// Append status code to error field (reuse for metadata)
		result.Error = fmt.Sprintf("http_%d", statusCode)

	case ProbeTypeICMPPing:
		latency, err := ICMPPing(ctx, cfg.Target, timeout)
		if err != nil {
			result.Error = err.Error()
			return result
		}
		result.Success = true
		result.Latency = latency
	default:
		result.Error = fmt.Sprintf("unknown probe type: %d", cfg.Type)
		return result
	}

	return result
}

// BatchProbe runs multiple probes concurrently and returns all results.
// At most 32 goroutines run simultaneously to avoid exhausting system resources.
func BatchProbe(ctx context.Context, targets []ProbeTarget, timeout time.Duration) []ProbeResult {
	results := make([]ProbeResult, len(targets))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 32) // max 32 concurrent probes

	for i, target := range targets {
		wg.Add(1)
		go func(i int, t ProbeTarget) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = Probe(ctx, t, timeout)
		}(i, target)
	}
	wg.Wait()
	return results
}
