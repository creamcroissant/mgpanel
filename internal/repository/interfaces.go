// 文件路径: internal/repository/interfaces.go
// 模块说明: 这是 internal 模块里的 interfaces 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package repository

import (
	"context"
	"encoding/json"
)

// Store 暴露每个聚合根对应的仓储接口。
type Store interface {
	CoreOperations() CoreOperationRepository
	OperationLogs() OperationLogRepository
	BinaryVersionStates() BinaryVersionStateRepository
	AgentLifecycleOperations() AgentLifecycleOperationRepository
	AgentTrafficPolicies() AgentTrafficPolicyRepository
	AgentTrafficStates() AgentTrafficStateRepository
	SubscriptionSources() SubscriptionSourceRepository
	SubscriptionFilterReasons() SubscriptionFilterReasonRepository
	Users() UserRepository
	Settings() SettingRepository
	Plugins() PluginRepository
	Plans() PlanRepository
	LoginLogs() LoginLogRepository
	Tokens() TokenRepository
	Servers() ServerRepository
	ServerGroups() ServerGroupRepository
	ServerRoutes() ServerRouteRepository
	StatUsers() StatUserRepository
	StatServers() StatServerRepository
	Notices() NoticeRepository
	Knowledge() KnowledgeRepository
	SubscriptionLogs() SubscriptionLogRepository
	AgentHosts() AgentHostRepository
	ConfigTemplates() ConfigTemplateRepository
	UserTraffic() UserTrafficRepository
	ShortLinks() ShortLinkRepository
	SubscriptionTemplates() SubscriptionTemplateRepository
	ForwardingRules() ForwardingRuleRepository
	UserNoticeReads() UserNoticeReadsRepository
	AgentCoreInstances() AgentCoreInstanceRepository
	AgentCoreSwitchLogs() AgentCoreSwitchLogRepository
	AccessLogs() AccessLogRepository
	UnlockProbeResults() UnlockProbeResultRepository
	ExitNodeSets() ExitNodeSetRepository
	RoutingPolicies() RoutingPolicyRepository
	RelayPaths() RelayPathRepository
	InboundSpecs() InboundSpecRepository
	InboundSpecRevisions() InboundSpecRevisionRepository
	DesiredArtifacts() DesiredArtifactRepository
		CoreConfigItems() CoreConfigItemRepository
	ApplyRuns() ApplyRunRepository
	TrafficReportDedups() TrafficReportDedupRepository
	AgentConfigInventories() AgentConfigInventoryRepository
	InboundIndexes() InboundIndexRepository
	DriftStates() DriftStateRepository
	CDNSites() CDNSiteRepository
	CDNEdges() CDNEdgeRepository
	CDNCacheRules() CDNCacheRuleRepository
	CDNOriginLatencies() CDNOriginLatencyRepository
	CloudflareZones() CloudflareZoneRepository
	CloudflareDNSRecords() CloudflareDNSRecordRepository
	CloudFrontDistributions() CloudFrontDistributionRepository
		MCPApiKeys() MCPApiKeyRepository
	AgentMeshPeers() AgentMeshPeerRepository
}


// MCPApiKeyRepository manages MCP API keys for LLM access.
type MCPApiKeyRepository interface {
	Create(ctx context.Context, key *MCPApiKey) error
	GetByID(ctx context.Context, id int64) (*MCPApiKey, error)
	GetByPrefix(ctx context.Context, prefix string) (*MCPApiKey, error)
	List(ctx context.Context) ([]*MCPApiKey, error)
	Update(ctx context.Context, key *MCPApiKey) error
	Delete(ctx context.Context, id int64) error
	UpdateLastUsed(ctx context.Context, id int64, at int64) error
}

// CoreOperationRepository manages asynchronous core management tasks.
type CoreOperationRepository interface {
	Create(ctx context.Context, operation *CoreOperation) error
	UpdateStatus(ctx context.Context, id, status string, resultPayload json.RawMessage, errorMessage string, claimedBy string, claimedAt, startedAt, finishedAt *int64) error
	FindByID(ctx context.Context, id string) (*CoreOperation, error)
	List(ctx context.Context, filter CoreOperationFilter) ([]*CoreOperation, error)
	Count(ctx context.Context, filter CoreOperationFilter) (int64, error)
	ClaimNext(ctx context.Context, agentHostID int64, statuses []string, claimedBy string, claimedAt int64, reclaimBefore *int64) (*CoreOperation, error)
}

// OperationLogRepository manages append-only operation event logs.
type OperationLogRepository interface {
	Append(ctx context.Context, entry *OperationLogEntry) (*OperationLogEntry, error)
	List(ctx context.Context, filter OperationLogFilter) ([]*OperationLogEntry, error)
	Count(ctx context.Context, filter OperationLogFilter) (int64, error)
	DeleteOlderThan(ctx context.Context, days int) (int64, error)
}

// BinaryVersionStateRepository manages agent and core binary version states.
type BinaryVersionStateRepository interface {
	Upsert(ctx context.Context, state *BinaryVersionState) (*BinaryVersionState, error)
	FindByHostComponent(ctx context.Context, agentHostID int64, component string) (*BinaryVersionState, error)
	List(ctx context.Context, filter BinaryVersionFilter) ([]*BinaryVersionState, error)
	UpdateCheckResult(ctx context.Context, agentHostID int64, component, remoteVersion, status, checkError string, checkedAt int64) error
}

// AgentLifecycleOperationRepository manages panel-issued agent lifecycle commands.
type AgentLifecycleOperationRepository interface {
	Create(ctx context.Context, operation *AgentLifecycleOperation) error
	DeleteOlderThan(ctx context.Context, days int) (int64, error)
	UpdateStatus(ctx context.Context, id, status string, resultPayload json.RawMessage, errorMessage string, claimedBy string, claimedAt, startedAt, finishedAt *int64) error
	UpdateClaimedStatus(ctx context.Context, id, claimedBy, status string, resultPayload json.RawMessage, errorMessage string, startedAt, finishedAt *int64) error
	FindByID(ctx context.Context, id string) (*AgentLifecycleOperation, error)
	List(ctx context.Context, filter AgentLifecycleOperationFilter) ([]*AgentLifecycleOperation, error)
	Count(ctx context.Context, filter AgentLifecycleOperationFilter) (int64, error)
	ClaimNext(ctx context.Context, agentHostID int64, statuses []string, operationTypes []string, claimedBy string, claimedAt int64, reclaimBefore *int64, limit int) ([]*AgentLifecycleOperation, error)
}

// AgentTrafficPolicyRepository manages per-agent traffic threshold and reset policy.
type AgentTrafficPolicyRepository interface {
	Upsert(ctx context.Context, policy *AgentTrafficPolicy) (*AgentTrafficPolicy, error)
	FindByAgentHostID(ctx context.Context, agentHostID int64) (*AgentTrafficPolicy, error)
	List(ctx context.Context, filter AgentTrafficPolicyFilter) ([]*AgentTrafficPolicy, error)
	UpdateThresholdReached(ctx context.Context, agentHostID int64, reached bool, updatedAt int64) error
	UpdateResetState(ctx context.Context, agentHostID int64, lastResetAt int64, cycleKey string, updatedAt int64) error
}

// AgentTrafficStateRepository manages trusted traffic counter accumulation state.
type AgentTrafficStateRepository interface {
	Upsert(ctx context.Context, state *AgentTrafficState) (*AgentTrafficState, error)
	FindByAgentHostID(ctx context.Context, agentHostID int64) (*AgentTrafficState, error)
	List(ctx context.Context, filter AgentTrafficStateFilter) ([]*AgentTrafficState, error)
	ResetCycle(ctx context.Context, agentHostID int64, resetAt int64) error
}

// SubscriptionSourceRepository manages imported and custom subscription sources.
type SubscriptionSourceRepository interface {
	Create(ctx context.Context, source *SubscriptionSource) (*SubscriptionSource, error)
	Update(ctx context.Context, source *SubscriptionSource) error
	Delete(ctx context.Context, id int64) error
	FindByID(ctx context.Context, id int64) (*SubscriptionSource, error)
	List(ctx context.Context, filter SubscriptionSourceFilter) ([]*SubscriptionSource, error)
	Count(ctx context.Context, filter SubscriptionSourceFilter) (int64, error)
	UpdateSyncResult(ctx context.Context, id int64, content string, syncErr string, syncedAt int64) error
}

// SubscriptionFilterReasonRepository manages explainable subscription filtering results.
type SubscriptionFilterReasonRepository interface {
	ReplaceForSource(ctx context.Context, sourceType string, sourceID int64, reasons []*SubscriptionFilterReason) error
	List(ctx context.Context, filter SubscriptionFilterReasonFilter) ([]*SubscriptionFilterReason, error)
	Count(ctx context.Context, filter SubscriptionFilterReasonFilter) (int64, error)
	DeleteBySource(ctx context.Context, sourceType string, sourceID int64) error
}

// UserRepository 定义用户相关数据访问方法。
type UserRepository interface {
	FindByID(ctx context.Context, id int64) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	FindByUsername(ctx context.Context, username string) (*User, error)
	FindByToken(ctx context.Context, token string) (*User, error)
	Save(ctx context.Context, user *User) error
	Create(ctx context.Context, user *User) (*User, error)
	HasAdmin(ctx context.Context) (bool, error)
	ActiveCountByPlan(ctx context.Context, planID int64, nowUnix int64) (int64, error)
	AdjustBalance(ctx context.Context, userID int64, deltaCents int64) (bool, error)
	IncrementTraffic(ctx context.Context, userID int64, uploadDelta, downloadDelta int64) error
	ListActiveForGroups(ctx context.Context, groupIDs []int64, nowUnix int64) ([]*NodeUser, error)
	PlanCounts(ctx context.Context, planIDs []int64, nowUnix int64) (map[int64]PlanUserCount, error)
	Search(ctx context.Context, filter UserSearchFilter) ([]*User, error)
	CountFiltered(ctx context.Context, filter UserSearchFilter) (int64, error)
	Count(ctx context.Context) (int64, error)
	CountActive(ctx context.Context, nowUnix int64) (int64, error)
	CountCreatedBetween(ctx context.Context, startUnix, endUnix int64) (int64, error)
	SetTrafficExceeded(ctx context.Context, userID int64, exceeded bool) error
	GetExceededUserIDs(ctx context.Context) ([]int64, error)
	Delete(ctx context.Context, id int64) error
}

// SettingRepository 处理系统配置的存取。
type SettingRepository interface {
	Get(ctx context.Context, key string) (*Setting, error)
	Upsert(ctx context.Context, setting *Setting) error
	List(ctx context.Context) ([]Setting, error)
	ListByCategory(ctx context.Context, category string) ([]Setting, error)
}

// PluginRepository 提供插件元数据与配置访问。
type PluginRepository interface {
	FindEnabledByCode(ctx context.Context, code string) (*Plugin, error)
}

// PlanRepository 管理订阅套餐相关数据。
type PlanRepository interface {
	ListVisible(ctx context.Context) ([]*Plan, error)
	ListAll(ctx context.Context) ([]*Plan, error)
	FindByID(ctx context.Context, id int64) (*Plan, error)
	Create(ctx context.Context, plan *Plan) (*Plan, error)
	Update(ctx context.Context, plan *Plan) error
	Delete(ctx context.Context, id int64) error
	Sort(ctx context.Context, ids []int64, updatedAt int64) error
	BindGroups(ctx context.Context, planID int64, groupIDs []int64) error
	UnbindGroups(ctx context.Context, planID int64) error
	ReplaceGroups(ctx context.Context, planID int64, groupIDs []int64) error
	UpdateWithGroups(ctx context.Context, plan *Plan, groupIDs []int64) error
	GetGroups(ctx context.Context, planID int64) ([]int64, error)
}

// ServerRepository 管理节点相关数据。
type ServerRepository interface {
	FindAllVisible(ctx context.Context) ([]*Server, error)
	FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*Server, error)
	FindByIdentifier(ctx context.Context, identifier string, nodeType string) (*Server, error)
	FindByID(ctx context.Context, id int64) (*Server, error)
	FindByAgentHostID(ctx context.Context, agentHostID int64) ([]*Server, error)
	ListAll(ctx context.Context) ([]*Server, error)
	Create(ctx context.Context, server *Server) error
	Update(ctx context.Context, server *Server) error
	UpdateHeartbeat(ctx context.Context, id int64, heartbeatAt int64) error
	Delete(ctx context.Context, id int64) error
	Count(ctx context.Context) (int64, error)
}

// ServerGroupRepository 提供节点分组信息。
type ServerGroupRepository interface {
	List(ctx context.Context) ([]*ServerGroup, error)
}

// ServerRouteRepository 提供节点路由信息。
type ServerRouteRepository interface {
	List(ctx context.Context) ([]*ServerRoute, error)
	FindByIDs(ctx context.Context, ids []int64) ([]*ServerRoute, error)
}

// StatUserRepository 管理用户流量聚合统计。
type StatUserRepository interface {
	Upsert(ctx context.Context, record StatUserRecord) error
	UpsertBatch(ctx context.Context, records []StatUserRecord) error
	ListByRecord(ctx context.Context, recordType int, recordAt int64, agentHostID *int64, limit int) ([]StatUserRecord, error)
	ListByUserSince(ctx context.Context, userID int64, since int64, limit int) ([]StatUserRecord, error)
	SumByRange(ctx context.Context, filter StatUserSumFilter) (StatUserSumResult, error)
	TopByRange(ctx context.Context, filter StatUserTopFilter) ([]StatUserAggregate, error)

	// 多节点聚合查询
	ListByAgentHost(ctx context.Context, agentHostID int64, recordType int, since int64, limit int) ([]StatUserRecord, error)
	SumByAgentHost(ctx context.Context, agentHostID int64, recordType int, startAt, endAt int64) (StatUserSumResult, error)
}

// NoticeRepository 管理站点公告数据。
type NoticeRepository interface {
	List(ctx context.Context) ([]*Notice, error)
	FindByID(ctx context.Context, id int64) (*Notice, error)
	Create(ctx context.Context, notice *Notice) (*Notice, error)
	Update(ctx context.Context, notice *Notice) error
	Delete(ctx context.Context, id int64) error
	Sort(ctx context.Context, ids []int64, updatedAt int64) error
}

// UserNoticeReadsRepository 管理用户已读公告记录。
type UserNoticeReadsRepository interface {
	// MarkRead 记录用户已读公告（幂等）
	MarkRead(ctx context.Context, userID, noticeID int64) error

	// HasRead 判断用户是否已读该公告
	HasRead(ctx context.Context, userID, noticeID int64) (bool, error)

	// GetUnreadPopupNoticeIDs 返回未读弹窗公告 ID 列表
	GetUnreadPopupNoticeIDs(ctx context.Context, userID int64) ([]int64, error)
}

// KnowledgeRepository 管理知识库条目。
type KnowledgeRepository interface {
	List(ctx context.Context) ([]*Knowledge, error)
	FindByID(ctx context.Context, id int64) (*Knowledge, error)
	Create(ctx context.Context, knowledge *Knowledge) (*Knowledge, error)
	Update(ctx context.Context, knowledge *Knowledge) error
	Delete(ctx context.Context, id int64) error
	Sort(ctx context.Context, ids []int64, updatedAt int64) error
	Categories(ctx context.Context) ([]string, error)
	ListVisible(ctx context.Context, filter KnowledgeVisibleFilter) ([]*Knowledge, error)
}

// LoginLogRepository 保存登录日志。
type LoginLogRepository interface {
	Create(ctx context.Context, log *LoginLog) error
	DeleteOlderThan(ctx context.Context, days int) (int64, error)
}

// TokenRepository 管理访问/刷新令牌。
type TokenRepository interface {
	Create(ctx context.Context, token *AccessToken) (*AccessToken, error)
	FindByRefreshToken(ctx context.Context, refreshToken string) (*AccessToken, error)
	DeleteByRefreshToken(ctx context.Context, refreshToken string) error
	DeleteByUser(ctx context.Context, userID int64) error
}

// SubscriptionLogRepository 记录订阅访问日志。
type SubscriptionLogRepository interface {
	Log(ctx context.Context, log *SubscriptionLog) error
	GetRecentLogs(ctx context.Context, userID int64, limit int) ([]*SubscriptionLog, error)
	DeleteOlderThan(ctx context.Context, days int) (int64, error)
}

// StatServerRepository 管理节点维度统计。
type StatServerRepository interface {
	Upsert(ctx context.Context, record StatServerRecord) error
	ListByServer(ctx context.Context, serverID int64, recordType int, since int64, limit int) ([]StatServerRecord, error)
	SumByRange(ctx context.Context, filter StatServerSumFilter) (StatServerSumResult, error)
	TopByRange(ctx context.Context, filter StatServerTopFilter) ([]StatServerAggregate, error)
}

// StatServerSumFilter 定义节点流量汇总筛选条件。
type StatServerSumFilter struct {
	ServerID   *int64 // nil = all servers
	RecordType int
	StartAt    int64
	EndAt      int64
}

// StatServerTopFilter 定义节点流量排行筛选条件。
type StatServerTopFilter struct {
	RecordType int
	StartAt    int64
	EndAt      int64
	Limit      int
}

// AgentHostRepository 管理 Agent 主机信息。
type AgentHostRepository interface {
	// CRUD 操作
	Create(ctx context.Context, host *AgentHost) error
	FindByID(ctx context.Context, id int64) (*AgentHost, error)
	FindByHost(ctx context.Context, host string) (*AgentHost, error)
	FindByToken(ctx context.Context, token string) (*AgentHost, error)
	Update(ctx context.Context, host *AgentHost) error
	Delete(ctx context.Context, id int64) error
	ListAll(ctx context.Context) ([]*AgentHost, error)

	// 状态更新
	UpdateStatus(ctx context.Context, id int64, status int, heartbeatAt int64) error
	UpdateMetrics(ctx context.Context, id int64, metrics AgentHostMetrics) error
	UpdateCapabilities(ctx context.Context, id int64, coreVersion string, capabilities, buildTags []string) error

	// 统计查询
	Count(ctx context.Context) (int64, error)
	CountOnline(ctx context.Context) (int64, error)
}

// ConfigTemplateRepository 管理配置模板数据。
type ConfigTemplateRepository interface {
	Create(ctx context.Context, tpl *ConfigTemplate) error
	Update(ctx context.Context, tpl *ConfigTemplate) error
	Delete(ctx context.Context, id int64) error
	FindByID(ctx context.Context, id int64) (*ConfigTemplate, error)
	ListAll(ctx context.Context) ([]*ConfigTemplate, error)
}

// AgentHostMetrics contains real-time metrics reported by an agent.
type AgentHostMetrics struct {
	CPUTotal              float64
	CPUUsed               float64
	MemTotal              int64
	MemUsed               int64
	DiskTotal             int64
	DiskUsed              int64
	UploadTotal           int64
	DownloadTotal         int64
	UploadRateBps         int64
	DownloadRateBps       int64
	RawUploadTotalBytes   int64
	RawDownloadTotalBytes int64
	BootID                string
	LastRealtimeReportAt  int64
	LastRestartAt         int64
	AgentVersion          string
	CurrentCoreType       string
}

// ServerClientConfigRepository 管理客户端订阅配置。
type ServerClientConfigRepository interface {
	// Create 插入新的客户端订阅配置
	Create(ctx context.Context, cfg *ServerClientConfig) error

	// FindByServerID 获取指定节点的全部订阅配置
	FindByServerID(ctx context.Context, serverID int64) ([]*ServerClientConfig, error)

	// FindByServerIDAndFormat 获取指定节点 + 格式的订阅配置
	FindByServerIDAndFormat(ctx context.Context, serverID int64, format string) (*ServerClientConfig, error)

	// Upsert 新增或更新订阅配置
	Upsert(ctx context.Context, cfg *ServerClientConfig) error

	// DeleteByServerID 删除指定节点的全部订阅配置
	DeleteByServerID(ctx context.Context, serverID int64) error

	// DeleteByServerIDAndFormat 删除指定节点的某种格式订阅配置
	DeleteByServerIDAndFormat(ctx context.Context, serverID int64, format string) error
}

// UserTrafficRepository 管理用户流量周期与节点选择。
type UserTrafficRepository interface {
	// 节点选择相关操作
	AddServerSelection(ctx context.Context, userID, serverID int64) error
	RemoveServerSelection(ctx context.Context, userID, serverID int64) error
	GetUserServerIDs(ctx context.Context, userID int64) ([]int64, error)
	ClearUserSelections(ctx context.Context, userID int64) error
	ReplaceUserSelections(ctx context.Context, userID int64, serverIDs []int64) error

	// 流量周期相关操作
	GetCurrentPeriod(ctx context.Context, userID int64) (*UserTrafficPeriod, error)
	CreatePeriod(ctx context.Context, period *UserTrafficPeriod) error
	IncrementPeriodTraffic(ctx context.Context, userID int64, uploadDelta, downloadDelta int64) error
	MarkPeriodExceeded(ctx context.Context, userID int64, periodStart int64) error
	GetExpiredPeriodUserIDs(ctx context.Context, nowUnix int64) ([]int64, error)
	ApplyTrafficBatchAtomic(ctx context.Context, traffic []UserTrafficDelta, nowUnix int64) ([]UserTrafficDelta, []int64, error)

	// 查询相关操作
	GetExceededUserIDs(ctx context.Context) ([]int64, error)
	GetUserTrafficStats(ctx context.Context, userID int64) (*UserTrafficStats, error)
}

// ShortLinkRepository 管理短链接映射。
type ShortLinkRepository interface {
	// Create 插入新的短链接
	Create(ctx context.Context, link *ShortLink) error

	// FindByCode 按 code 查询短链接
	FindByCode(ctx context.Context, code string) (*ShortLink, error)

	// FindByID 按 ID 查询短链接
	FindByID(ctx context.Context, id int64) (*ShortLink, error)

	// FindByUserID 查询用户创建的所有短链接
	FindByUserID(ctx context.Context, userID int64) ([]*ShortLink, error)

	// Update 更新短链接记录
	Update(ctx context.Context, link *ShortLink) error

	// Delete 删除指定 ID 的短链接
	Delete(ctx context.Context, id int64) error

	// DeleteByUserID 删除用户所有短链接
	DeleteByUserID(ctx context.Context, userID int64) error

	// IncrementAccessCount 增加访问次数并更新最近访问时间
	IncrementAccessCount(ctx context.Context, id int64, accessTime int64) error

	// CodeExists 判断短码是否已存在
	CodeExists(ctx context.Context, code string) (bool, error)
}

// SubscriptionTemplateRepository 管理订阅模板。
type SubscriptionTemplateRepository interface {
	// Create 插入新的订阅模板
	Create(ctx context.Context, tpl *SubscriptionTemplate) error

	// FindByID 按 ID 查询订阅模板
	FindByID(ctx context.Context, id int64) (*SubscriptionTemplate, error)

	// FindDefaultByType 获取指定类型的默认模板 (clash, singbox 等)
	FindDefaultByType(ctx context.Context, templateType string) (*SubscriptionTemplate, error)

	// ListByType 获取指定类型的全部模板
	ListByType(ctx context.Context, templateType string) ([]*SubscriptionTemplate, error)

	// ListPublic 获取公开模板列表
	ListPublic(ctx context.Context) ([]*SubscriptionTemplate, error)

	// Update 更新订阅模板
	Update(ctx context.Context, tpl *SubscriptionTemplate) error

	// Delete 删除指定 ID 的订阅模板
	Delete(ctx context.Context, id int64) error

	// SetDefault 将模板设为默认
	SetDefault(ctx context.Context, id int64) error
}

// AgentMeshPeerRepository manages WireGuard mesh peer records.
type AgentMeshPeerRepository interface {
	Upsert(ctx context.Context, peer *AgentMeshPeer) error
	FindByAgentHostID(ctx context.Context, agentHostID int64) (*AgentMeshPeer, error)
	ListByNetworkID(ctx context.Context, networkID string) ([]*AgentMeshPeer, error)
	Delete(ctx context.Context, agentHostID int64) error
}

// ForwardingRuleRepository 管理端口转发规则。
type ForwardingRuleRepository interface {
	// CRUD 操作
	Create(ctx context.Context, rule *ForwardingRule) error
	Update(ctx context.Context, rule *ForwardingRule) error
	Delete(ctx context.Context, id int64) error
	FindByID(ctx context.Context, id int64) (*ForwardingRule, error)

	// 查询操作
	ListByAgentHostID(ctx context.Context, agentHostID int64) ([]*ForwardingRule, error)
	ListEnabledByAgentHostID(ctx context.Context, agentHostID int64) ([]*ForwardingRule, error)

	// 版本管理
	GetMaxVersion(ctx context.Context, agentHostID int64) (int64, error)

	// 冲突检测
	CheckPortConflict(ctx context.Context, agentHostID int64, listenPort int, protocol string, excludeID int64) (bool, error)
}

// ForwardingRuleLogFilter 定义转发规则日志筛选条件。
type ForwardingRuleLogFilter struct {
	AgentHostID int64
	RuleID      *int64
	StartAt     *int64
	EndAt       *int64
	Limit       int
	Offset      int
}

// ForwardingRuleLogRepository 管理转发规则审计日志。
type ForwardingRuleLogRepository interface {
	// 新增审计日志
	Create(ctx context.Context, log *ForwardingRuleLog) error

	// 按筛选条件返回日志列表
	List(ctx context.Context, filter ForwardingRuleLogFilter) ([]*ForwardingRuleLog, error)

	// 统计筛选条件下的日志总数
	Count(ctx context.Context, filter ForwardingRuleLogFilter) (int64, error)

	// 查询指定规则的日志列表
	ListByRuleID(ctx context.Context, ruleID int64, limit int) ([]*ForwardingRuleLog, error)
}

// AgentCoreInstanceRepository 管理核心实例记录。
type AgentCoreInstanceRepository interface {
	Create(ctx context.Context, instance *AgentCoreInstance) error
	Update(ctx context.Context, instance *AgentCoreInstance) error
	Delete(ctx context.Context, id int64) error
	FindByID(ctx context.Context, id int64) (*AgentCoreInstance, error)
	FindByInstanceID(ctx context.Context, agentHostID int64, instanceID string) (*AgentCoreInstance, error)
	ListByAgentHostID(ctx context.Context, agentHostID int64) ([]*AgentCoreInstance, error)
	ReplaceSnapshot(ctx context.Context, agentHostID int64, instances []*AgentCoreInstance) error
	UpdateHeartbeat(ctx context.Context, agentHostID int64, instanceID string, heartbeatAt int64) error
}

// AgentCoreSwitchLogFilter 定义核心切换日志筛选条件。
type AgentCoreSwitchLogFilter struct {
	AgentHostID int64
	Status      *string
	StartAt     *int64
	EndAt       *int64
	Limit       int
	Offset      int
}

// AgentCoreSwitchLogRepository 管理核心切换审计日志。
type AgentCoreSwitchLogRepository interface {
	Create(ctx context.Context, log *AgentCoreSwitchLog) error
	UpdateStatus(ctx context.Context, id int64, status string, detail string, completedAt *int64) error
	List(ctx context.Context, filter AgentCoreSwitchLogFilter) ([]*AgentCoreSwitchLog, error)
	Count(ctx context.Context, filter AgentCoreSwitchLogFilter) (int64, error)
}

// AccessLogRepository manages access log data.
type AccessLogRepository interface {
	Create(ctx context.Context, log *AccessLog) error
	BatchCreate(ctx context.Context, logs []*AccessLog) error
	List(ctx context.Context, filter AccessLogFilter) ([]*AccessLog, error)
	Count(ctx context.Context, filter AccessLogFilter) (int64, error)
	DeleteByRetentionDays(ctx context.Context, days int) (int64, error)
	GetStats(ctx context.Context, filter AccessLogFilter) (*AccessLogStats, error)
}

// UnlockProbeResultRepository manages streaming unlock probe results.
type UnlockProbeResultRepository interface {
	// Upsert 覆盖某个 agent+service 的最新结果（无历史保留）。
	Upsert(ctx context.Context, r *UnlockProbeResult) error
	// ListByAgentHost 返回某个 agent 的最新结果（每个 service 一行）。
	ListByAgentHost(ctx context.Context, agentHostID int64) ([]*UnlockProbeResult, error)
	// ListAll 返回所有 agent 的最新结果（每个 agent+service 一行）。
	ListAll(ctx context.Context) ([]*UnlockProbeResult, error)
	// CountByAgentHost 返回某个 agent 的结果行数。
	CountByAgentHost(ctx context.Context, agentHostID int64) (int64, error)
}

// ExitNodeSetRepository 管理出口节点集合及其成员。
type ExitNodeSetRepository interface {
	Create(ctx context.Context, set *ExitNodeSet) error
	Update(ctx context.Context, set *ExitNodeSet) error
	Delete(ctx context.Context, id int64) error
	FindByID(ctx context.Context, id int64) (*ExitNodeSet, error)
	FindByName(ctx context.Context, name string) (*ExitNodeSet, error)
	List(ctx context.Context) ([]*ExitNodeSet, error)

	// 成员管理
	AddMember(ctx context.Context, m *ExitNodeSetMember) error
	RemoveMember(ctx context.Context, setID, agentHostID int64) error
	UpdateMember(ctx context.Context, m *ExitNodeSetMember) error
	ListMembers(ctx context.Context, setID int64) ([]*ExitNodeSetMember, error)
	ListMembersByAgent(ctx context.Context, agentHostID int64) ([]*ExitNodeSetMember, error)
}

// RoutingPolicyRepository 管理路由策略。
type RoutingPolicyRepository interface {
	Create(ctx context.Context, p *RoutingPolicy) error
	Update(ctx context.Context, p *RoutingPolicy) error
	Delete(ctx context.Context, id int64) error
	FindByID(ctx context.Context, id int64) (*RoutingPolicy, error)
	List(ctx context.Context, filter RoutingPolicyFilter) ([]*RoutingPolicy, error)
	ListEnabledByCore(ctx context.Context, coreType string) ([]*RoutingPolicy, error)
	// ReorderPriorities 事务内按给定 ID 顺序重写 priority（首项 100，逐项递增 100），
	// 返回成功更新的行数；ID 不存在时静默跳过（影响 0 行）。
	ReorderPriorities(ctx context.Context, orderedIDs []int64) (int64, error)
}

// RelayPathRepository 管理服务器中继链路及其有序节点。
// nodes 在事务内随 path 整体写（先删后插）。
type RelayPathRepository interface {
	Create(ctx context.Context, p *RelayPath) (int64, error)
	Update(ctx context.Context, p *RelayPath) error
	Delete(ctx context.Context, id int64) error
	GetByID(ctx context.Context, id int64) (*RelayPath, error)
	List(ctx context.Context, coreType string) ([]*RelayPath, error)
}

// RoutingPolicyFilter 定义路由策略查询过滤条件。
type RoutingPolicyFilter struct {
	CoreType *string
	Enabled  *bool
	// SpecID 非 nil 时仅返回该 spec 的策略
	SpecID *int64
	// OnlyGlobal 为 true 时仅返回全局策略（spec_id IS NULL）
	OnlyGlobal *bool
}

// InboundSpecRepository manages desired inbound specs.
type InboundSpecRepository interface {
	Create(ctx context.Context, spec *InboundSpec) error
	Update(ctx context.Context, spec *InboundSpec) error
	UpdateWithRevision(ctx context.Context, spec *InboundSpec, expectedRevision int64) error
	Delete(ctx context.Context, id int64) error
	FindByID(ctx context.Context, id int64) (*InboundSpec, error)
	FindByHostCoreTag(ctx context.Context, agentHostID int64, coreType, tag string) (*InboundSpec, error)
	List(ctx context.Context, filter InboundSpecFilter) ([]*InboundSpec, error)
	Count(ctx context.Context, filter InboundSpecFilter) (int64, error)

	// FindByCoreTag 全局查询（模板 spec 唯一性校验）。
	FindByCoreTag(ctx context.Context, coreType, tag string) (*InboundSpec, error)
	// ListByAgentHost 返回主机绑定的 specs（host-specific + 模板 spec 绑定到该主机的）。
	ListByAgentHost(ctx context.Context, agentHostID int64, filter InboundSpecFilter) ([]*InboundSpec, error)
	CountByAgentHost(ctx context.Context, agentHostID int64, filter InboundSpecFilter) (int64, error)
}

// SpecHostBindingRepository manages spec-to-host bindings for template specs.
type SpecHostBindingRepository interface {
	Bind(ctx context.Context, specID, agentHostID int64) error
	Unbind(ctx context.Context, specID, agentHostID int64) error
	UnbindAll(ctx context.Context, specID int64) error
	ListBySpec(ctx context.Context, specID int64) ([]int64, error)
	ListByHost(ctx context.Context, agentHostID int64) ([]int64, error)
}

// InboundSpecRevisionRepository manages immutable spec revisions.
type InboundSpecRevisionRepository interface {
	Create(ctx context.Context, revision *InboundSpecRevision) error
	FindBySpecAndRevision(ctx context.Context, specID int64, revision int64) (*InboundSpecRevision, error)
	ListBySpecID(ctx context.Context, specID int64, limit, offset int) ([]*InboundSpecRevision, error)
	GetMaxRevision(ctx context.Context, specID int64) (int64, error)
}

// CoreConfigItemRepository manages non-inbound core config items (outbound/routing/dns/core_settings).
type CoreConfigItemRepository interface {
	Create(ctx context.Context, item *CoreConfigItem) error
	Update(ctx context.Context, item *CoreConfigItem) error
	Delete(ctx context.Context, id int64) error
	FindByID(ctx context.Context, id int64) (*CoreConfigItem, error)
	FindByHostCoreTypeTag(ctx context.Context, agentHostID int64, coreType, configType, tag string) (*CoreConfigItem, error)
	FindByCoreTypeTag(ctx context.Context, coreType, configType, tag string) (*CoreConfigItem, error)
	ListByHost(ctx context.Context, agentHostID int64, coreType string, configType *string) ([]*CoreConfigItem, error)
	List(ctx context.Context, filter CoreConfigItemFilter) ([]*CoreConfigItem, error)
	Count(ctx context.Context, filter CoreConfigItemFilter) (int64, error)
}

// DesiredArtifactRepository manages rendered artifact files.
type DesiredArtifactRepository interface {
	CreateBatch(ctx context.Context, artifacts []*DesiredArtifact) error
	DeleteByHostCoreRevision(ctx context.Context, agentHostID int64, coreType string, desiredRevision int64, sourceTags ...string) error
	ReplaceRevision(ctx context.Context, agentHostID int64, coreType string, desiredRevision int64, artifacts []*DesiredArtifact, sourceTags ...string) (int64, error)
	List(ctx context.Context, filter DesiredArtifactFilter) ([]*DesiredArtifact, error)
	Count(ctx context.Context, filter DesiredArtifactFilter) (int64, error)
	GetLatestRevision(ctx context.Context, agentHostID int64, coreType string) (int64, error)
	FindByHostCoreRevisionFilename(ctx context.Context, agentHostID int64, coreType string, desiredRevision int64, filename string) (*DesiredArtifact, error)
	PruneOldRevisions(ctx context.Context, keep int) (int64, error)
}

// ApplyRunRepository manages apply lifecycle records.
type ApplyRunRepository interface {
	Create(ctx context.Context, run *ApplyRun) error
	UpdateStatus(ctx context.Context, runID, status, errorMessage string, rollbackRevision int64, finishedAt int64) error
	MarkStarted(ctx context.Context, runID, status string, startedAt int64) error
	FindByRunID(ctx context.Context, runID string) (*ApplyRun, error)
	List(ctx context.Context, filter ApplyRunFilter) ([]*ApplyRun, error)
	Count(ctx context.Context, filter ApplyRunFilter) (int64, error)
	// ExpireStale marks stale non-terminal apply runs (pending/applying whose started_at is
	// older than deadline) as failed with errorMessage, keeping the audit record. Returns
	// the number of runs expired.
	ExpireStale(ctx context.Context, deadline int64, errorMessage string) (int64, error)
}

// TrafficReportDedupRepository manages idempotency keys for traffic reports.
type TrafficReportDedupRepository interface {
	// MarkHandled records report_id for an agent host. Returns false if already exists.
	MarkHandled(ctx context.Context, agentHostID int64, reportID string, handledAt int64) (bool, error)
	DeleteOlderThan(ctx context.Context, days int) (int64, error)
}

// AgentConfigInventoryRepository manages applied file inventory.
type AgentConfigInventoryRepository interface {
	UpsertBatch(ctx context.Context, inventories []*AgentConfigInventory) error
	List(ctx context.Context, filter AgentConfigInventoryFilter) ([]*AgentConfigInventory, error)
	DeleteStaleByHostCoreBefore(ctx context.Context, agentHostID int64, coreType string, beforeLastSeenAt int64) error
}

// InboundIndexRepository manages parsed inbound semantic index.
type InboundIndexRepository interface {
	UpsertBatch(ctx context.Context, indexes []*InboundIndex) error
	List(ctx context.Context, filter InboundIndexFilter) ([]*InboundIndex, error)
	DeleteStaleByHostCoreBefore(ctx context.Context, agentHostID int64, coreType string, beforeLastSeenAt int64) error
}

// DriftStateRepository manages desired-applied drift records.
type DriftStateRepository interface {
	Upsert(ctx context.Context, drift *DriftState) error
	MarkRecoveredByHostCore(ctx context.Context, agentHostID int64, coreType string, recoveredAt int64) error
	List(ctx context.Context, filter DriftStateFilter) ([]*DriftState, error)
	Count(ctx context.Context, filter DriftStateFilter) (int64, error)
}

// ----------------------------------------------------------------
// CDN 数据模型
// ----------------------------------------------------------------

// CDNSite represents a CDN site/domain configuration.
type CDNSite struct {
	ID               int64
	Name             string
	Description      string
	Domain           string
	OriginType       string // e.g. "ip", "domain", "s3"
	OriginURL        string
	CacheTTL         int    // default cache TTL in seconds
	SSLMode          string // "off", "flexible", "full", "full_strict"
	Status           string // "active", "deploying", "error"
	CustomCertPEM    string
	CustomKeyPEM     string
	AccelerationMode string // "xhttp" for acceleration
	InboundSpecID    *int64 // optional link to InboundSpec
	Provider         string // "cloudflare", "cloudfront", "generic"
	OriginPath       string // path prefix for origin
	OriginProtocol   string // "http", "https"
	Enabled          bool
	AsnTags          string
	LastDeployedAt   *int64 // timestamp of last deploy
	CreatedAt        int64
	UpdatedAt        int64
}

// CDNEdge represents an edge node assignment for a CDN site.
type CDNEdge struct {
	ID          int64
	SiteID      int64
	AgentHostID int64
	Weight      int
	Enabled     bool
	Status      string // "pending", "active", "error"
	LastError   string
	DeployedAt  *int64
	CreatedAt   int64
	UpdatedAt   int64
}

// CDNCacheRule represents a per-path cache rule for a CDN site.
type CDNCacheRule struct {
	ID         int64
	SiteID     int64
	MatchType  string // "prefix", "exact", "regex"
	MatchValue string
	CacheTTL   int  // TTL in seconds, 0 = no-cache
	Bypass     bool // bypass CDN entirely
	Priority   int
	CreatedAt  int64
}

// CloudflareZone represents a Cloudflare zone integration.
type CloudflareZone struct {
	ID        int64
	AccountID string // maps to api_token_encrypted column (legacy name, keep for backward compat)
	APITokenEncrypted string
	ZoneID    string
	ZoneName  string
	Status    string
	Plan      string
	Enabled   bool
	CreatedAt int64
	UpdatedAt int64
}

// CloudflareDNSRecord represents a Cloudflare DNS record managed by the panel.
type CloudflareDNSRecord struct {
	ID        int64
	ZoneID    int64 // internal CloudflareZone ID
	RecordID  string
	Name      string
	Type      string // A, AAAA, CNAME, etc.
	Content   string
	TTL       int
	Proxied   bool
	SyncedAt  int64
	CreatedAt int64
	UpdatedAt int64
}

// CloudFrontDistribution represents a CloudFront distribution integration.
type CloudFrontDistribution struct {
	ID              int64
	SiteID          int64  // FK to cdn_sites
	DistributionID  string // CloudFront Distribution ID (E1234567890)
	DistributionARN string
	DomainName      string // dxxx.cloudfront.net
	CertARN         string
	PriceClass      string
	Enabled         bool
	Status          string
	LastSyncedAt    int64
	CreatedAt       int64
	UpdatedAt       int64
}

// ----------------------------------------------------------------
// CDN Repository 接口
// ----------------------------------------------------------------

// CDNSiteRepository manages CDN site configurations.
type CDNSiteRepository interface {
	Create(ctx context.Context, site *CDNSite) error
	Update(ctx context.Context, site *CDNSite) error
	FindByID(ctx context.Context, id int64) (*CDNSite, error)
	FindByInboundSpecID(ctx context.Context, specID int64) (*CDNSite, error)
	FindByDomain(ctx context.Context, domain string) (*CDNSite, error)
	List(ctx context.Context, filter CDNSiteFilter) ([]*CDNSite, error)
	Count(ctx context.Context, filter CDNSiteFilter) (int64, error)
	Delete(ctx context.Context, id int64) error
}

// CDNEdgeRepository manages edge node assignments for CDN sites.
type CDNEdgeRepository interface {
	Create(ctx context.Context, edge *CDNEdge) error
	Update(ctx context.Context, edge *CDNEdge) error
	FindByID(ctx context.Context, id int64) (*CDNEdge, error)
	ListBySiteID(ctx context.Context, siteID int64) ([]*CDNEdge, error)
	Delete(ctx context.Context, id int64) error
}

// CDNCacheRuleRepository manages per-path cache rules for CDN sites.
type CDNCacheRuleRepository interface {
	Create(ctx context.Context, rule *CDNCacheRule) error
	Update(ctx context.Context, rule *CDNCacheRule) error
	FindByID(ctx context.Context, id int64) (*CDNCacheRule, error)
	ListBySiteID(ctx context.Context, siteID int64) ([]*CDNCacheRule, error)
	Delete(ctx context.Context, id int64) error
}

// CDNOriginLatencyRepository manages origin latency measurements for CDN sites.
type CDNOriginLatencyRepository interface {
	Upsert(ctx context.Context, siteID int64, stack string, latencyMs int64) error
	ListBySiteID(ctx context.Context, siteID int64) ([]*CDNOriginLatency, error)
	ListAll(ctx context.Context) ([]*CDNOriginLatency, error)
}

// CloudflareZoneRepository manages Cloudflare zone integrations.
type CloudflareZoneRepository interface {
	Create(ctx context.Context, zone *CloudflareZone) error
	FindByID(ctx context.Context, id int64) (*CloudflareZone, error)
	List(ctx context.Context) ([]*CloudflareZone, error)
	Delete(ctx context.Context, id int64) error
}

// CloudflareDNSRecordRepository manages Cloudflare DNS record sync.
type CloudflareDNSRecordRepository interface {
	Create(ctx context.Context, record *CloudflareDNSRecord) error
	FindByID(ctx context.Context, id int64) (*CloudflareDNSRecord, error)
	ListByZoneID(ctx context.Context, zoneID int64) ([]*CloudflareDNSRecord, error)
	Delete(ctx context.Context, id int64) error
}

// CloudFrontDistributionRepository manages CloudFront distribution integrations.
type CloudFrontDistributionRepository interface {
	Create(ctx context.Context, dist *CloudFrontDistribution) error
	FindByID(ctx context.Context, id int64) (*CloudFrontDistribution, error)
	List(ctx context.Context) ([]*CloudFrontDistribution, error)
	Delete(ctx context.Context, id int64) error
}
