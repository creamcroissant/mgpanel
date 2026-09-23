package tools

// Tool name constants — used as JSON-RPC method names in handlers.
const (
	ToolSystemStatus      = "system_status"
	ToolSystemSettings    = "system_settings"
	ToolAgentList         = "agent_list"
	ToolAgentStatus       = "agent_status"
	ToolAgentConfigYAML   = "agent_config_yaml"
	ToolAgentLogsFetch    = "agent_logs_fetch"
	ToolServerList        = "server_list"
	ToolServerStats       = "server_stats"
	ToolUserList          = "user_list"
	ToolUserDetail        = "user_detail"
	ToolCDNSiteList       = "cdn_site_list"
	ToolMeshNetwork       = "mesh_network"
	ToolOperationLogsList = "operation_logs_list"
	ToolAccessLogsList    = "access_logs_list"
	ToolServerLogList     = "server_log_list"
	ToolServerLogTail     = "server_log_tail"
	ToolConfigArtifacts   = "config_artifacts"

	// 写操作工具（scope=ops）
	ToolConfigRender      = "config_render"
	ToolConfigApply       = "config_apply"
	ToolConfigApplyStatus = "config_apply_status"
	ToolConfigSync        = "config_sync"
	ToolInboundSpecUpsert = "inbound_spec_upsert"
	ToolEgressMode        = "egress_mode"
	ToolRoutingPolicy     = "routing_policy"
	ToolExitNodeSet       = "exit_node_set"

	// 就绪/拓扑读取
	ToolEgressPairList    = "egress_pair_list"
	ToolInboundSpecList   = "inbound_spec_list"
	ToolEgressModeGet     = "egress_mode_get"
	ToolRoutingPolicyList = "routing_policy_list"
	ToolExitNodeSetList   = "exit_node_set_list"
)
