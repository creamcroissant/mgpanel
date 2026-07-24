// Admin API module exports
export { adminApi, AdminApiError, isAdminApiError } from "./client";

// Agent Host API
export {
  getAgentHosts,
  getAgentHost,
  createAgentHost,
  updateAgentHost,
  deleteAgentHost,
  refreshAgentHosts,
} from "./agentHost";

// User API
export {
  getUsers,
  getUser,
  createUser,
  updateUser,
  deleteUser,
  toggleUserBan,
  resetUserPassword,
} from "./user";

// Plan API
export {
  getPlans,
  getPlan,
  createPlan,
  updatePlan,
  deletePlan,
  updatePlanSort,
} from "./plan";

// Notice API
export {
  getNotices,
  getNotice,
  createNotice,
  updateNotice,
  deleteNotice,
  toggleNoticeVisibility,
} from "./notice";

// Knowledge API
export {
  getKnowledgeList,
  getKnowledgeDetail,
  getKnowledgeCategories,
  saveKnowledgeArticle,
  toggleKnowledgeVisibility,
  deleteKnowledgeArticle,
  sortKnowledgeArticles,
} from "./knowledge";

// Forwarding API
export {
  listForwardingRules,
  createForwardingRule,
  updateForwardingRule,
  deleteForwardingRule,
  listForwardingLogs,
} from "./forwarding";

// Agent Core API
export {
  listAgentCores,
  listAgentCoreInstances,
  listAgentCoreOperations,
  createAgentCoreInstance,
  deleteAgentCoreInstance,
  switchAgentCore,
  installAgentCore,
  convertAgentCoreConfig,
  listAgentCoreSwitchLogs,
} from "./cores";

// Agent observability API
export {
  listOperationLogs,
  getOperationLogStreamURL,
  listAgentBinaryVersions,
  refreshAgentBinaryVersion,
} from "./operationLogs";

// Agent lifecycle API
export {
  listAgentLifecycleOperations,
  createAgentUpdateCheckOperation,
  createAgentUpdateOperation,
  createAgentTrafficResetOperation,
} from "./lifecycle";

// Agent traffic API
export {
  getAgentTrafficPolicy,
  updateAgentTrafficPolicy,
  getAgentTrafficStatus,
  resetAgentTrafficCycle,
} from "./traffic";

// Subscription diagnostics API
export {
  listSubscriptionSources,
  createSubscriptionSource,
  getSubscriptionSource,
  updateSubscriptionSource,
  deleteSubscriptionSource,
  syncSubscriptionSource,
  listSubscriptionFilterReasons,
  getSubscriptionFilterSummary,
} from "./subscription";

// Access Logs API
export { fetchAccessLogs, getAccessLogStats, cleanupAccessLogs } from "./accessLog";

// System API
export {
  getSystemStatus,
  getQueueStats,
  getSystemConfig,
  updateSystemConfig,
} from "./system";

// Config Center API
export {
  listConfigCenterSpecs,
  createConfigCenterSpec,
  updateConfigCenterSpec,
  getConfigCenterSpecHistory,
  deleteConfigCenterSpec,
  importConfigCenterSpecsFromApplied,
  listConfigCenterArtifacts,
  getConfigCenterTextDiff,
  getConfigCenterSemanticDiff,
  createConfigCenterApplyRun,
  listConfigCenterApplyRuns,
  getConfigCenterApplyRunDetail,
  listConfigCenterAppliedSnapshot,
  listConfigCenterDriftStates,
  listConfigCenterRecoveryStates,
} from "./configCenter";

// Agent config API
export { getAgentConfigYAML, reportAgentConfig } from "./agentConfig";

// CDN API
export {
  fetchCDNSites,
  createCDNSite,
  updateCDNSite,
  deleteCDNSite,
  deployCDNSite,
  undeployCDNSite,
  fetchProbeReport,
} from "./cdn";

export type { ProbeReportItem, ProbeReportResponse } from "./cdn";

// MCP API Key management
export {
  fetchMCPKeys,
  createMCPKey,
  revokeMCPKey,
  deleteMCPKey,
} from "./mcp";

export type { MCPApiKey, CreateKeyRequest } from "./mcp";

// Core Config API
export {
  listCoreConfigItems,
  createCoreConfigItem,
  updateCoreConfigItem,
  deleteCoreConfigItem,
} from "./configCenter";
