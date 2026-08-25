// 文件路径: internal/repository/types.go
// 模块说明: 这是 internal 模块里的 types 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package repository

import "encoding/json"

// User represents a subset of the v2_user columns migrated to SQLite.
type User struct {
	ID                int64
	UUID              string
	Token             string
	Username          string
	Email             string
	Password          string
	PasswordAlgo      string
	PasswordSalt      string
	BalanceCents      int64
	PlanID            int64
	GroupID           int64
	ExpiredAt         int64
	U                 int64
	D                 int64
	TransferEnable    int64
	SpeedLimit        *int64
	DeviceLimit       *int64
	IsAdmin           bool
	Status            int
	Banned            bool
	TrafficExceeded   bool
	TelegramID        int64
	LastLoginAt       int64
	Remarks           string
	Tags              []string
	CreatedAt         int64
	UpdatedAt         int64
}

// NodeUser represents the limited subset of user columns shared with nodes.
type NodeUser struct {
	ID          int64
	UUID        string
	Email       string
	SpeedLimit  *int64
	DeviceLimit *int64
}

// PlanUserCount aggregates user totals per plan for admin analytics.
type PlanUserCount struct {
	Total  int64
	Active int64
}

// AccessToken stores refresh/access session metadata.
type AccessToken struct {
	ID               int64
	UserID           int64
	Token            string
	RefreshToken     string
	ExpiresAt        int64
	RefreshExpiresAt int64
	IP               string
	UserAgent        string
	Revoked          bool
	CreatedAt        int64
	UpdatedAt        int64
}

// LoginLog captures a single login attempt for auditing purposes.
type LoginLog struct {
	ID        int64
	UserID    *int64
	Email     string
	IP        string
	UserAgent string
	Success   bool
	Reason    string
	CreatedAt int64
	UpdatedAt int64
}

// Setting mirrors the admin settings KV pairs.
type Setting struct {
	Key       string
	Value     string
	Category  string
	UpdatedAt int64
}

// Plugin models enabled plugin metadata and configuration payloads.
type Plugin struct {
	ID        int64
	Code      string
	Type      string
	IsEnabled bool
	Config    string
}

// Plan models the plans table for subscription listings.
type Plan struct {
	ID                 int64
	GroupID            *int64
	Name               string
	Prices             map[string]float64
	Sell               bool
	TransferEnable     int64
	SpeedLimit         *int64
	DeviceLimit        *int64
	Show               bool
	Renew              bool
	Content            string
	Tags               []string
	ResetTrafficMethod *int64
	CapacityLimit      *int64
	Sort               int64
	CreatedAt          int64
	UpdatedAt          int64
}

// ServerGroup represents a logical grouping of servers.
type ServerGroup struct {
	ID        int64
	Name      string
	Type      string
	Sort      int64
	CreatedAt int64
	UpdatedAt int64
}

// ServerRoute captures custom routing rules applied to servers.
type ServerRoute struct {
	ID          int64
	Remarks     string
	Match       json.RawMessage
	Action      string
	ActionValue string
	CreatedAt   int64
	UpdatedAt   int64
}

// Server describes a single proxy/server entry exposed to clients.
type Server struct {
	ID              int64
	Code            string
	GroupID         int64
	RouteID         int64
	ParentID        int64
	AgentHostID     int64 // 关联的 Agent 主机 ID
	Tags            json.RawMessage
	Name            string
	Rate            string
	Host            string
	Port            int
	ServerPort      int
	Cipher          string
	Obfs            string
	ObfsSettings    json.RawMessage
	Show            int
	Sort            int64
	Status          int
	Type            string
	Settings        json.RawMessage
	LastHeartbeatAt int64
	CreatedAt       int64
	UpdatedAt       int64
}

// AgentHost represents a physical server where Agents are deployed.
type AgentHost struct {
	ID                    int64
	Name                  string   // 服务器名称
	Host                  string   // 服务器 IP 或域名
	Token                 string   // Agent 认证令牌
	Status                int      // 0: 离线, 1: 在线, 2: 警告
	ProvisionStatus       int      // 0: active, 1: pending
	TemplateID            int64    // Config Template ID
	CoreVersion           string   // 核心版本 (如 "1.10.0")
	Capabilities          []string // 支持的能力 (如 ["reality", "multiplex"])
	BuildTags             []string // 构建标签 (如 ["with_v2ray_api"])
	CPUTotal              float64  // CPU 核心数
	CPUUsed               float64  // CPU 使用率 (%)
	MemTotal              int64    // 内存总量 (bytes)
	MemUsed               int64    // 内存使用量 (bytes)
	DiskTotal             int64    // 磁盘总量 (bytes)
	DiskUsed              int64    // 磁盘使用量 (bytes)
	UploadTotal           int64    // 累计上传流量 (bytes)
	DownloadTotal         int64    // 累计下载流量 (bytes)
	UploadRateBps         int64    // 实时上传速率 (bytes/s)
	DownloadRateBps       int64    // 实时下载速率 (bytes/s)
	RawUploadTotalBytes   int64    // Agent 原始上传计数器
	RawDownloadTotalBytes int64    // Agent 原始下载计数器
	BootID                string   // Agent 启动标识
	LastRealtimeReportAt  int64    // 最后实时指标上报时间
	LastRestartAt         int64    // 最近一次检测到重启的时间
	AgentVersion          string   // Agent 二进制版本
	CurrentCoreType       string   // 当前运行核心类型
	LastHeartbeatAt       int64    // 最后心跳时间
	ConfigYAML            string   // Agent 上报的运行配置 YAML
	CreatedAt             int64
	UpdatedAt             int64
}

// AgentLifecycleOperation represents a panel-issued agent lifecycle command.
type AgentLifecycleOperation struct {
	ID             string          `json:"id"`
	AgentHostID    int64           `json:"agent_host_id"`
	OperationType  string          `json:"operation_type"`
	Status         string          `json:"status"`
	RequestPayload json.RawMessage `json:"request_payload,omitempty"`
	ResultPayload  json.RawMessage `json:"result_payload,omitempty"`
	ErrorMessage   string          `json:"error_message,omitempty"`
	ClaimedBy      string          `json:"claimed_by,omitempty"`
	ClaimedAt      *int64          `json:"claimed_at,omitempty"`
	StartedAt      *int64          `json:"started_at,omitempty"`
	FinishedAt     *int64          `json:"finished_at,omitempty"`
	OperatorID     *int64          `json:"operator_id,omitempty"`
	Source         string          `json:"source"`
	CreatedAt      int64           `json:"created_at"`
	UpdatedAt      int64           `json:"updated_at"`
}

// AgentTrafficPolicy stores threshold and reset policy for an agent host.
type AgentTrafficPolicy struct {
	AgentHostID       int64  `json:"agent_host_id"`
	Enabled           bool   `json:"enabled"`
	LimitBytes        int64  `json:"limit_bytes"`
	LimitType         string `json:"limit_type"`
	ThresholdPercent  int    `json:"threshold_percent"`
	ThresholdAction   string `json:"threshold_action"`
	ThresholdReached  bool   `json:"threshold_reached"`
	ResetMode         string `json:"reset_mode"`
	ResetDay          int    `json:"reset_day"`
	IntervalDays      int    `json:"interval_days"`
	AnchorAt          int64  `json:"anchor_at"`
	LastResetAt       int64  `json:"last_reset_at"`
	LastResetCycleKey string `json:"last_reset_cycle_key"`
	UpdatedAt         int64  `json:"updated_at"`
}

// AgentTrafficState stores the trusted traffic accumulation state for an agent host.
type AgentTrafficState struct {
	AgentHostID          int64  `json:"agent_host_id"`
	BootID               string `json:"boot_id"`
	LastRawUploadBytes   int64  `json:"last_raw_upload_bytes"`
	LastRawDownloadBytes int64  `json:"last_raw_download_bytes"`
	CounterSeen          bool   `json:"counter_seen"`
	CycleUploadBytes     int64  `json:"cycle_upload_bytes"`
	CycleDownloadBytes   int64  `json:"cycle_download_bytes"`
	UpdatedAt            int64  `json:"updated_at"`
}

// SubscriptionSource stores imported or custom subscription material.
type SubscriptionSource struct {
	ID          int64  `json:"id"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	URL         string `json:"url,omitempty"`
	Content     string `json:"content,omitempty"`
	Enabled     bool   `json:"enabled"`
	LastSyncAt  int64  `json:"last_sync_at,omitempty"`
	LastSyncErr string `json:"last_sync_err,omitempty"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

// SubscriptionFilterReason stores explainable node exclusion decisions.
type SubscriptionFilterReason struct {
	ID         int64  `json:"id"`
	SourceType string `json:"source_type"`
	SourceID   int64  `json:"source_id"`
	ServerID   int64  `json:"server_id"`
	NodeName   string `json:"node_name"`
	Reason     string `json:"reason"`
	Detail     string `json:"detail"`
	CreatedAt  int64  `json:"created_at"`
}

// BinaryVersionState tracks local and remote versions for agent-managed binaries.
type BinaryVersionState struct {
	ID               int64  `json:"id"`
	AgentHostID      int64  `json:"agent_host_id"`
	Component        string `json:"component"`
	LocalVersion     string `json:"local_version"`
	RemoteVersion    string `json:"remote_version,omitempty"`
	Status           string `json:"status"`
	CapabilitiesJSON string `json:"-"`
	BuildTagsJSON    string `json:"-"`
	LastCheckedAt    int64  `json:"last_checked_at,omitempty"`
	LastCheckError   string `json:"last_check_error,omitempty"`
	UpdatedAt        int64  `json:"updated_at"`
}

// ConfigTemplate defines a configuration template for agents.
type ConfigTemplate struct {
	ID              int64
	Name            string
	Type            string   // sing-box, xray, etc.
	Content         string   // Template content (Go text/template format)
	Description     string   // Human-readable description
	MinVersion      string   // Minimum core version required (e.g., "1.8.0")
	Capabilities    []string // Required capabilities (e.g., ["reality", "multiplex"])
	SchemaVersion   int      // Template format version
	IsValid         bool     // Cached validation status
	ValidationError string   // Last validation error message
	CreatedAt       int64
	UpdatedAt       int64
}

// Notice mirrors announcements shown to users/admins.
type Notice struct {
	ID        int64
	Sort      int64
	Title     string
	Content   string
	ImgURL    string
	Tags      []string
	Show      bool
	Popup     bool
	CreatedAt int64
	UpdatedAt int64
}

// Knowledge mirrors v2_knowledge articles exposed to users/admins.
type Knowledge struct {
	ID        int64
	Language  string
	Category  string
	Title     string
	Body      string
	Sort      int64
	Show      bool
	CreatedAt int64
	UpdatedAt int64
}

// KnowledgeVisibleFilter narrows which knowledge entries are exposed to users.
type KnowledgeVisibleFilter struct {
	Language string
	Keyword  string
}

// StatUserRecord captures aggregated traffic usage per user per interval.
type StatUserRecord struct {
	UserID      int64
	AgentHostID int64 // Source agent host ID for multi-node aggregation
	ServerRate  float64
	RecordAt    int64
	RecordType  int // 0: hourly, 1: daily, 2: monthly
	Upload      int64
	Download    int64
	CreatedAt   int64
	UpdatedAt   int64
}

// StatUserSumResult sums upload/download amounts.
type StatUserSumResult struct {
	Upload   int64
	Download int64
}

// StatUserAggregate stores ranked traffic totals per user.
type StatUserAggregate struct {
	UserID   int64
	Upload   int64
	Download int64
}

// SubscriptionLog represents an access log for subscription endpoints.
type SubscriptionLog struct {
	ID        int64
	UserID    int64
	IP        string
	UserAgent string
	Type      string
	URL       string
	CreatedAt int64
}

// StatServerRecord captures aggregated node-level statistics per interval.
type StatServerRecord struct {
	ID          int64
	ServerID    int64
	RecordAt    int64
	RecordType  int // 0: hourly, 1: daily
	Upload      int64
	Download    int64
	CPUAvg      float64
	MemUsed     int64
	MemTotal    int64
	DiskUsed    int64
	DiskTotal   int64
	OnlineUsers int64
	CreatedAt   int64
	UpdatedAt   int64
}

// StatServerSumResult sums upload/download amounts for servers.
type StatServerSumResult struct {
	Upload   int64
	Download int64
}

// StatServerAggregate stores ranked traffic totals per server.
type StatServerAggregate struct {
	ServerID int64
	Upload   int64
	Download int64
}

// ServerClientConfig stores client-side configuration for a server/protocol.
type ServerClientConfig struct {
	ID          int64
	ServerID    int64  // FK to servers table
	Format      string // v2rayn, clash, singbox-pc, singbox-phone, etc.
	Content     string // Full config content for this format
	ContentHash string // MD5 hash for change detection
	CreatedAt   int64
	UpdatedAt   int64
}

// UserServerSelection represents a user's selection of a specific server node.
type UserServerSelection struct {
	ID        int64
	UserID    int64
	ServerID  int64
	CreatedAt int64
}

// UserTrafficPeriod tracks user traffic usage within a billing period.
type UserTrafficPeriod struct {
	ID            int64
	UserID        int64
	PeriodStart   int64 // Unix timestamp of period start (1st of month)
	PeriodEnd     int64 // Unix timestamp of period end (1st of next month)
	UploadBytes   int64
	DownloadBytes int64
	QuotaBytes    int64 // Traffic quota for this period
	Exceeded      bool  // True if user exceeded quota
	CreatedAt     int64
	UpdatedAt     int64
}

// UserTrafficDelta represents a single traffic delta sample for batch processing.
type UserTrafficDelta struct {
	UserID   int64
	Upload   int64
	Download int64
}

// UserTrafficStats provides a summary of user's traffic usage.
type UserTrafficStats struct {
	PeriodStart   int64
	PeriodEnd     int64
	UploadBytes   int64
	DownloadBytes int64
	TotalBytes    int64
	QuotaBytes    int64
	UsedPercent   float64
	Exceeded      bool
}

// ShortLink represents a short URL mapping for subscription links.
type ShortLink struct {
	ID             int64
	Code           string // Short code (e.g., "abc123")
	UserID         int64
	TargetPath     string // Target path (default: /api/v1/client/subscribe)
	CustomParams   string // Custom query parameters (JSON)
	ExpiresAt      int64  // Optional expiration timestamp
	AccessCount    int64  // Number of times accessed
	LastAccessedAt int64  // Last access timestamp
	CreatedAt      int64
	UpdatedAt      int64
}

// SubscriptionTemplate represents a customizable template for subscription output.
type SubscriptionTemplate struct {
	ID          int64
	Name        string // Template display name
	Description string // Human-readable description
	Type        string // clash, singbox, surge, general
	Content     string // Template content (Go text/template or raw config)
	IsDefault   bool   // Whether this is the default template for its type
	IsPublic    bool   // Whether this template is visible to users
	SortOrder   int    // Display order
	CreatedAt   int64
	UpdatedAt   int64
}

// ForwardingRule represents a nftables port forwarding rule.
type ForwardingRule struct {
	ID            int64
	AgentHostID   int64  // 关联的 Agent 主机 ID
	Name          string // 规则名称
	Protocol      string // tcp/udp/both
	ListenPort    int    // 本地监听端口
	TargetAddress string // 目标地址 (仅 IP)
	TargetPort    int    // 目标端口
	Enabled       bool   // 是否启用
	Priority      int    // 优先级（越小越优先）
	Remark        string // 备注
	Version       int64  // 规则版本
	CreatedAt     int64
	UpdatedAt     int64
}

// ForwardingRuleLog records audit logs for forwarding rule changes.
type ForwardingRuleLog struct {
	ID          int64
	RuleID      *int64 // 关联的规则 ID (可为空，规则删除后保留日志)
	AgentHostID int64  // 关联的 Agent 主机 ID
	Action      string // 操作类型: create, update, delete, enable, disable, apply_success, apply_failed
	OperatorID  *int64 // 操作者 ID (可为空，系统操作时)
	Detail      string // 操作详情 (JSON 格式)
	CreatedAt   int64
}

// AgentMeshPeer represents a WireGuard mesh peer in the database.
type AgentMeshPeer struct {
	ID           int64  `json:"id"`
	AgentHostID  int64  `json:"agent_host_id"`
	WGPrivateKey string `json:"wg_private_key,omitempty"`
	WGPublicKey  string `json:"wg_public_key"`
	WGIP         string `json:"wg_ip"`
	WGListenPort int    `json:"wg_listen_port"`
	NetworkID    string `json:"network_id"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
}

// CoreStatusSnapshot represents the latest known core capability snapshot reported by an agent host.
type CoreStatusSnapshot struct {
	Type         string   `json:"type"`
	Version      string   `json:"version,omitempty"`
	Installed    bool     `json:"installed"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// CoreOperation represents one asynchronous core management task tracked by Panel.
type CoreOperation struct {
	ID             string          `json:"id"`
	AgentHostID    int64           `json:"agent_host_id"`
	OperationType  string          `json:"operation_type"`
	CoreType       string          `json:"core_type"`
	Status         string          `json:"status"`
	RequestPayload json.RawMessage `json:"request_payload,omitempty"`
	ResultPayload  json.RawMessage `json:"result_payload,omitempty"`
	ErrorMessage   string          `json:"error_message,omitempty"`
	OperatorID     *int64          `json:"operator_id,omitempty"`
	ClaimedBy      string          `json:"claimed_by,omitempty"`
	ClaimedAt      *int64          `json:"claimed_at,omitempty"`
	StartedAt      *int64          `json:"started_at,omitempty"`
	FinishedAt     *int64          `json:"finished_at,omitempty"`
	CreatedAt      int64           `json:"created_at"`
	UpdatedAt      int64           `json:"updated_at"`
}

// OperationLogEntry records one append-only operation event.
type OperationLogEntry struct {
	ID            int64           `json:"id"`
	Scope         string          `json:"scope"`
	TargetID      string          `json:"target_id"`
	AgentHostID   int64           `json:"agent_host_id"`
	Sequence      int64           `json:"sequence"`
	Phase         string          `json:"phase"`
	Level         string          `json:"level"`
	Message       string          `json:"message"`
	Payload       json.RawMessage `json:"payload,omitempty"`
	SourceEventID string          `json:"source_event_id,omitempty"`
	ReportedAt    int64           `json:"reported_at"`
	CreatedAt     int64           `json:"created_at"`
}

// MCPApiKey represents a stored MCP API key for LLM access.
type MCPApiKey struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Prefix     string `json:"prefix"`
	KeyHash    string `json:"-"`
	Enabled    bool   `json:"enabled"`
	LastUsedAt int64  `json:"last_used_at,omitempty"`
	CreatedBy  int64  `json:"created_by"`
	CreatedAt  int64  `json:"created_at"`
	UpdatedAt  int64  `json:"updated_at"`
}
type OperationBlocker struct {
	Scope         string `json:"scope"`
	ID            string `json:"id"`
	AgentHostID   int64  `json:"agent_host_id"`
	OperationType string `json:"operation_type"`
	Status        string `json:"status"`
	CreatedAt     int64  `json:"created_at"`
}

// AgentCoreInstance represents a persisted core instance on an agent host.
type AgentCoreInstance struct {
	ID               int64               `json:"id"`
	AgentHostID      int64               `json:"agent_host_id"`
	InstanceID       string              `json:"instance_id"`
	CoreType         string              `json:"core_type"`
	Status           string              `json:"status"`
	ListenPorts      []int               `json:"listen_ports"`
	ConfigTemplateID *int64              `json:"config_template_id"`
	ConfigHash       string              `json:"config_hash"`
	StartedAt        *int64              `json:"started_at"`
	LastHeartbeatAt  *int64              `json:"last_heartbeat_at"`
	ErrorMessage     string              `json:"error_message"`
	CreatedAt        int64               `json:"created_at"`
	UpdatedAt        int64               `json:"updated_at"`
	CoreSnapshot     *CoreStatusSnapshot `json:"core_snapshot,omitempty"`
}

// AgentCoreSwitchLog captures core switching audit logs.
type AgentCoreSwitchLog struct {
	ID             int64   `json:"id"`
	AgentHostID    int64   `json:"agent_host_id"`
	FromInstanceID *string `json:"from_instance_id"`
	FromCoreType   *string `json:"from_core_type"`
	ToInstanceID   string  `json:"to_instance_id"`
	ToCoreType     string  `json:"to_core_type"`
	OperatorID     *int64  `json:"operator_id"`
	Status         string  `json:"status"`
	Detail         string  `json:"detail"`
	CreatedAt      int64   `json:"created_at"`
	CompletedAt    *int64  `json:"completed_at"`
}

// AccessLog records user traffic access history.
type AccessLog struct {
	ID              int64
	UserID          *int64
	UserEmail       string
	AgentHostID     int64
	SourceIP        string
	TargetDomain    string
	TargetIP        string
	TargetPort      int
	Protocol        string
	Upload          int64
	Download        int64
	ConnectionStart *int64 // Unix timestamp
	ConnectionEnd   *int64 // Unix timestamp
	CreatedAt       int64
}

// UnlockProbeResult 存储单个 agent 对某个流媒体平台的解锁检测结果。
type UnlockProbeResult struct {
	ID          int64  `json:"id"`
	AgentHostID int64  `json:"agent_host_id"`
	Service     string `json:"service"`
	Status      string `json:"status"`  // unlocked / locked / error / unknown
	Region      string `json:"region"`
	Detail      string `json:"detail"`
	ProbedAt    int64  `json:"probed_at"`
	CreatedAt   int64  `json:"created_at"`
}

// ExitNodeSet 是一组出口 agent 的集合，用于负载均衡和故障转移。
type ExitNodeSet struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Tags        string `json:"tags"`
	Strategy    string `json:"strategy"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

// ExitNodeSetMember 是出口集合的成员 agent。
type ExitNodeSetMember struct {
	ID          int64 `json:"id"`
	SetID       int64 `json:"set_id"`
	AgentHostID int64 `json:"agent_host_id"`
	Weight      int   `json:"weight"`
	Enabled     bool  `json:"enabled"`
	CreatedAt   int64 `json:"created_at"`
	UpdatedAt   int64 `json:"updated_at"`
}

// RoutingPolicy 定义一条路由规则（geosite/domain → 出口集合）。
type RoutingPolicy struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	CoreType    string `json:"core_type"`
	Priority    int    `json:"priority"`
	MatchType   string `json:"match_type"`
	MatchValue  string `json:"match_value"`
	Action      string `json:"action"`
	TargetSetID *int64 `json:"target_set_id,omitempty"`
	// SpecID 非 nil 表示仅对绑定入站生效（入站规则，渲染时排在全局规则之前）；nil 为全局。
	SpecID    *int64 `json:"spec_id,omitempty"`
	Enabled   bool   `json:"enabled"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

// RelayPath 是受控服务器之间的多跳中继链路（如 s1→s2→s3），与入口协议配置正交。
// sequence 0 = 入口，N-1 = 出口；每跳通过 mesh socks 隧道转发到下一跳。
type RelayPath struct {
	ID          int64            `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	CoreType    string           `json:"core_type"`
	Enabled     bool             `json:"enabled"`
	Nodes       []RelayPathNode  `json:"nodes"`
	CreatedAt   int64            `json:"created_at"`
	UpdatedAt   int64            `json:"updated_at"`
}

// RelayPathNode 是中继链路中的一跳。
type RelayPathNode struct {
	Sequence    int   `json:"sequence"`
	AgentHostID int64 `json:"agent_host_id"`
}

// AccessLogFilter defines filter conditions for querying access logs.
type AccessLogFilter struct {
	UserID       *int64
	AgentHostID  *int64
	TargetDomain *string // Use LIKE match
	SourceIP     *string
	Protocol     *string
	StartAt      *int64
	EndAt        *int64
	Limit        int
	Offset       int
}

// AccessLogStats provides aggregated statistics of access logs.
type AccessLogStats struct {
	TotalCount    int64
	TotalUpload   int64
	TotalDownload int64
}

// InboundSpec represents desired inbound configuration at tag granularity.
type InboundSpec struct {
	ID              int64           `json:"id"`
	AgentHostID     *int64          `json:"agent_host_id"` // nil = template spec
	ExitAgentHostID *int64          `json:"exit_agent_host_id,omitempty"` // nil = 直连出网；非 nil = 经 mesh 隧道到该 agent 出网
	ExitNodeSetID   *int64          `json:"exit_node_set_id,omitempty"`  // nil = 固定出口；非 nil = 从出口集合中选（负载均衡+故障转移）
	RelayPathID     *int64          `json:"relay_path_id,omitempty"`    // 非 nil = 走多跳中继链路（优先级高于 exit_*）
	CoreType        string          `json:"core_type"`
	Tag             string          `json:"tag"`
	Enabled         bool            `json:"enabled"`
	SemanticSpec    json.RawMessage `json:"semantic_spec"`
	CoreSpecific    json.RawMessage `json:"core_specific"`
	DesiredRevision int64           `json:"desired_revision"`
	CreatedBy       int64           `json:"created_by"`
	UpdatedBy       int64           `json:"updated_by"`
	CreatedAt       int64           `json:"created_at"`
	UpdatedAt       int64           `json:"updated_at"`
}

// InboundSpecRevision stores immutable snapshots for spec changes.
type InboundSpecRevision struct {
	ID         int64           `json:"id"`
	SpecID     int64           `json:"spec_id"`
	Revision   int64           `json:"revision"`
	Snapshot   json.RawMessage `json:"snapshot"`
	ChangeNote string          `json:"change_note"`
	OperatorID int64           `json:"operator_id"`
	CreatedAt  int64           `json:"created_at"`
}

// DesiredArtifact is a deployable rendered config file for a revision.
type DesiredArtifact struct {
	ID              int64  `json:"id"`
	AgentHostID     int64  `json:"agent_host_id"`
	CoreType        string `json:"core_type"`
	DesiredRevision int64  `json:"desired_revision"`
	Filename        string `json:"filename"`
	SourceTag       string `json:"source_tag"`
	Content         []byte `json:"content"`
	ContentHash     string `json:"content_hash"`
	GeneratedAt     int64  `json:"generated_at"`
}

// CoreConfigItem 表示非 inbound 的核心配置项（outbound/routing/DNS/core 设置）。
type CoreConfigItem struct {
	ID              int64           `json:"id"`
	AgentHostID     *int64          `json:"agent_host_id"` // nil = 模板项
	CoreType        string          `json:"core_type"`
	ConfigType      string          `json:"config_type"` // outbound | routing | dns | core_settings
	Tag             string          `json:"tag"`
	Enabled         bool            `json:"enabled"`
	ConfigData      json.RawMessage `json:"config_data"`
	DesiredRevision int64           `json:"desired_revision"`
	CreatedBy       int64           `json:"created_by"`
	UpdatedBy       int64           `json:"updated_by"`
	CreatedAt       int64           `json:"created_at"`
	UpdatedAt       int64           `json:"updated_at"`
}

// ApplyRun tracks release/apply lifecycle for a target revision.
type ApplyRun struct {
	RunID            string `json:"run_id"`
	AgentHostID      int64  `json:"agent_host_id"`
	CoreType         string `json:"core_type"`
	TargetRevision   int64  `json:"target_revision"`
	Status           string `json:"status"`
	ErrorMessage     string `json:"error_message"`
	PreviousRevision int64  `json:"previous_revision"`
	RollbackRevision int64  `json:"rollback_revision"`
	OperatorID       int64  `json:"operator_id"`
	StartedAt        int64  `json:"started_at"`
	FinishedAt       int64  `json:"finished_at"`
	CreatedAt        int64  `json:"created_at"`
}

// AgentConfigInventory is file-level applied observation reported by agents.
type AgentConfigInventory struct {
	ID          int64  `json:"id"`
	AgentHostID int64  `json:"agent_host_id"`
	CoreType    string `json:"core_type"`
	Source      string `json:"source"`
	Filename    string `json:"filename"`
	HashApplied string `json:"hash_applied"`
	ParseStatus string `json:"parse_status"`
	ParseError  string `json:"parse_error"`
	LastSeenAt  int64  `json:"last_seen_at"`
}

// InboundIndex is semantic inbound index parsed from applied files.
type InboundIndex struct {
	ID          int64           `json:"id"`
	AgentHostID int64           `json:"agent_host_id"`
	CoreType    string          `json:"core_type"`
	Source      string          `json:"source"`
	Filename    string          `json:"filename"`
	Tag         string          `json:"tag"`
	Protocol    string          `json:"protocol"`
	Listen      string          `json:"listen"`
	Port        int             `json:"port"`
	TLS         json.RawMessage `json:"tls,omitempty"`
	Transport   json.RawMessage `json:"transport,omitempty"`
	Multiplex   json.RawMessage `json:"multiplex,omitempty"`
	LastSeenAt  int64           `json:"last_seen_at"`
}

// CDNOriginLatency records a latency measurement for a CDN origin.
type CDNOriginLatency struct {
	ID         int64  `json:"id"`
	SiteID     int64  `json:"site_id"`
	Stack      string `json:"stack"`
	LatencyMs  int    `json:"latency_ms"`
	UpdatedAt  int64  `json:"updated_at"`
}

// CDNSiteFilter defines filter conditions for listing CDN sites.
type CDNSiteFilter struct {
	Keyword string
	Status  *string
	Enabled *bool
	Limit   int
	Offset  int
}

// DriftState tracks desired-applied mismatch status.
type DriftState struct {
	ID              int64           `json:"id"`
	AgentHostID     int64           `json:"agent_host_id"`
	CoreType        string          `json:"core_type"`
	Filename        string          `json:"filename"`
	Tag             string          `json:"tag"`
	DesiredRevision int64           `json:"desired_revision"`
	DesiredHash     string          `json:"desired_hash"`
	AppliedHash     string          `json:"applied_hash"`
	DriftType       string          `json:"drift_type"`
	Status          string          `json:"status"`
	Detail          json.RawMessage `json:"detail,omitempty"`
	FirstDetectedAt int64           `json:"first_detected_at"`
	LastChangedAt   int64           `json:"last_changed_at"`
}
