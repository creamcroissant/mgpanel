import { adminApi } from "./client";

/**
 * Fetch agent's reported config YAML from Panel.
 * Returns the raw YAML string or empty string if not reported yet.
 */
export async function getAgentConfigYAML(agentHostId: number): Promise<string> {
  const response = await adminApi.get<{ data: string }>(
    `/agent-hosts/${agentHostId}/config`
  );
  return response.data.data ?? "";
}

/**
 * Push edited config YAML to Panel, which forwards it to Agent.
 * Backend contract: PUT /agent-hosts/{id}/config with body {config_yaml}.
 */
export async function updateAgentConfig(agentHostId: number, configYAML: string): Promise<void> {
  await adminApi.put(`/agent-hosts/${agentHostId}/config`, {
    config_yaml: configYAML,
  });
}

/**
 * Request agent to re-read and report its config.yml immediately.
 */
export async function reportAgentConfig(agentHostId: number): Promise<void> {
  await adminApi.post(`/agent-hosts/${agentHostId}/report-config`);
}
