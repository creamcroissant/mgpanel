package core

import "context"

type CoreType string

type Status string

const (
	CoreTypeSingBox CoreType = "sing-box"
	CoreTypeXray    CoreType = "xray"
)

const (
	StatusStopped  Status = "stopped"
	StatusStarting Status = "starting"
	StatusRunning  Status = "running"
	StatusStopping Status = "stopping"
	StatusError    Status = "error"
)

type CoreInstance struct {
	ID          string   `json:"id"`
	CoreType    CoreType `json:"core_type"`
	Status      Status   `json:"status"`
	ListenPorts []int    `json:"listen_ports,omitempty"`
	ConfigPath  string   `json:"config_path"`
	ConfigHash  string   `json:"config_hash"`
	PID         int      `json:"pid,omitempty"`
	StartedAt   int64    `json:"started_at,omitempty"`
	Error       string   `json:"error,omitempty"`
}

type CoreMetrics struct {
	ActiveConnections int64  `json:"active_connections,omitempty"`
	TotalTrafficUp    uint64 `json:"total_traffic_up,omitempty"`
	TotalTrafficDown  uint64 `json:"total_traffic_down,omitempty"`
	MemoryUsage       uint64 `json:"memory_usage,omitempty"`
}

type TrafficSample struct {
	UserID   int64  `json:"user_id"`
	Email    string `json:"email,omitempty"`
	Upload   int64  `json:"upload"`
	Download int64  `json:"download"`
}

type ProxyCore interface {
	Type() CoreType
	Version(ctx context.Context) (string, error)
	Capabilities(ctx context.Context) ([]string, error)
	IsInstalled(ctx context.Context) bool
	ValidateConfig(ctx context.Context, configPath string) error
	// Start 启动核心实例。listenPorts 由调用方显式指定，避免解析配置推导端口。
	Start(ctx context.Context, instanceID, configPath string, listenPorts []int) error
	Stop(ctx context.Context, instanceID string) error
	Restart(ctx context.Context, instanceID string) error
	Reload(ctx context.Context, instanceID string) error
	Status(ctx context.Context, instanceID string) (*CoreInstance, error)
	ListInstances(ctx context.Context) ([]*CoreInstance, error)
	CollectTraffic(ctx context.Context, instanceID string) ([]TrafficSample, error)
}
