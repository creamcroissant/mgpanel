// 文件路径: internal/repository/filters.go
// 模块说明: 这是 internal 模块里的 filters 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package repository

// UserSearchFilter constrains admin user listings.
type UserSearchFilter struct {
	Keyword string // Changed from Query to Keyword to match usage
	Status  *int
	PlanID  *int64
	Limit   int
	Offset  int
}

// StatUserSumFilter constrains traffic summations.
type StatUserSumFilter struct {
	UserID      *int64 // nil = all users
	AgentHostID *int64 // nil = all hosts
	RecordType  int
	StartAt     int64
	EndAt       int64
}

// StatUserTopFilter selects the top-N traffic users.
type StatUserTopFilter struct {
	AgentHostID *int64 // nil = all hosts
	RecordType  int
	StartAt     int64
	EndAt       int64
	Limit       int
}

// InboundSpecFilter constrains inbound spec listing queries.
type InboundSpecFilter struct {
	AgentHostID *int64  // nil = no host filter
	CoreType    *string
	Tag         *string
	Enabled     *bool
	IsTemplate  *bool   // true = only templates (agent_host_id IS NULL), false = only host-specific, nil = all
	Limit       int
	Offset      int
}

// DesiredArtifactFilter constrains artifact listing queries.
type DesiredArtifactFilter struct {
	AgentHostID     int64
	CoreType        *string
	DesiredRevision *int64
	SourceTag       *string
	Filename        *string
	ExcludeContent  bool
	Limit           int
	Offset          int
}

// OperationLogFilter constrains operation log listing queries.
type OperationLogFilter struct {
	Scope       *string
	TargetID    *string
	AgentHostID *int64
	AfterID     *int64
	Level       *string
	Limit       int
	Offset      int
}

// AgentLifecycleOperationFilter constrains agent lifecycle operation queries.
type AgentLifecycleOperationFilter struct {
	AgentHostID   *int64
	OperationType *string
	Status        *string
	Statuses      []string
	ClaimedBy     *string
	Source        *string
	CreatedAfter  *int64
	CreatedBefore *int64
	Limit         int
	Offset        int
}

// AgentTrafficPolicyFilter constrains traffic policy queries.
type AgentTrafficPolicyFilter struct {
	AgentHostID      *int64
	Enabled          *bool
	ThresholdReached *bool
	ResetMode        *string
	Limit            int
	Offset           int
}

// AgentTrafficStateFilter constrains traffic state queries.
type AgentTrafficStateFilter struct {
	AgentHostID *int64
	BootID      *string
	Limit       int
	Offset      int
}

// SubscriptionSourceFilter constrains imported/custom subscription source queries.
type SubscriptionSourceFilter struct {
	Type    *string
	Enabled *bool
	Keyword string
	Limit   int
	Offset  int
}

// SubscriptionFilterReasonFilter constrains subscription filtering explanation queries.
type SubscriptionFilterReasonFilter struct {
	SourceType    *string
	SourceID      *int64
	ServerID      *int64
	Reason        *string
	CreatedAfter  *int64
	CreatedBefore *int64
	Limit         int
	Offset        int
}

// BinaryVersionFilter constrains binary version state listing queries.
type BinaryVersionFilter struct {
	AgentHostID *int64
	Component   *string
	Status      *string
	Limit       int
	Offset      int
}

// CoreOperationFilter constrains core operation listing and claim queries.
type CoreOperationFilter struct {
	AgentHostID   *int64
	OperationType *string
	CoreType      *string
	Status        *string
	Statuses      []string
	ClaimedBy     *string
	CreatedAfter  *int64
	CreatedBefore *int64
	Limit         int
	Offset        int
}

// ApplyRunFilter constrains apply run listing queries.
type ApplyRunFilter struct {
	AgentHostID    *int64
	CoreType       *string
	TargetRevision *int64
	Status         *string
	Statuses       []string
	Limit          int
	Offset         int
}

// AgentConfigInventoryFilter constrains inventory listing queries.
type AgentConfigInventoryFilter struct {
	AgentHostID *int64
	CoreType    *string
	Source      *string
	Filename    *string
	ParseStatus *string
	Limit       int
	Offset      int
}

// InboundIndexFilter constrains inbound index listing queries.
type InboundIndexFilter struct {
	AgentHostID *int64
	CoreType    *string
	Source      *string
	Tag         *string
	Protocol    *string
	Filename    *string
	Limit       int
	Offset      int
}

// DriftStateFilter constrains drift listing queries.
type DriftStateFilter struct {
	AgentHostID *int64
	CoreType    *string
	Status      *string
	DriftType   *string
	Tag         *string
	Filename    *string
	Limit       int
	Offset      int
}

// CoreConfigItemFilter constrains core config item listing queries.
type CoreConfigItemFilter struct {
	AgentHostID *int64
	CoreType    *string
	ConfigType  *string
	Tag         *string
	Enabled     *bool
	IsTemplate  *bool
	Limit       int
	Offset      int
}
