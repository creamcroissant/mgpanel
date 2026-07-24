package tools

// Tool name constants — used as JSON-RPC method names in handlers.
const (
	ToolSystemStatus       = "system_status"
	ToolSystemSettings     = "system_settings"
	ToolAgentList          = "agent_list"
	ToolAgentStatus        = "agent_status"
	ToolAgentConfigYAML    = "agent_config_yaml"
	ToolAgentLogsFetch     = "agent_logs_fetch"
	ToolServerList         = "server_list"
	ToolServerStats        = "server_stats"
	ToolUserList           = "user_list"
	ToolUserDetail         = "user_detail"
	ToolPlanList           = "plan_list"
	ToolCDNSiteList        = "cdn_site_list"
	ToolMeshNetwork        = "mesh_network"
	ToolOperationLogsList  = "operation_logs_list"
	ToolAccessLogsList     = "access_logs_list"
	ToolServerLogList      = "server_log_list"
	ToolServerLogTail      = "server_log_tail"
	ToolConfigArtifacts    = "config_artifacts"
)
