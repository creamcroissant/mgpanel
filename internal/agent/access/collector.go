package access

import (
	"context"
	"time"
)

// AccessLogEntry represents a single access record collected from the proxy core.
type AccessLogEntry struct {
	UserEmail       string
	SourceIP        string
	TargetDomain    string
	TargetIP        string
	TargetPort      int
	Protocol        string
	Upload          int64
	Download        int64
	ConnectionStart time.Time
	ConnectionEnd   time.Time
}

// Collector defines the interface for collecting access logs from different proxy cores.
type Collector interface {
	// Collect retrieves access logs from the underlying core.
	Collect(ctx context.Context) ([]AccessLogEntry, error)
	// Type returns the core type this collector supports (e.g., "xray", "sing-box").
	Type() string
}
