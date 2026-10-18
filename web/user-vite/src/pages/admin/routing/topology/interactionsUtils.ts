import { useMutation } from "@tanstack/react-query";
import { toast } from "sonner";
import { reorderPolicies } from "@/lib/topology/api";
import type { TopologyIssue, TopologyPolicy, TopologySnapshot } from "@/lib/topology/types";

/**
 * 拓扑交互的纯函数与 hook：从 interactions.tsx 拆出，保证那个文件只导出组件
 * （react-refresh 要求组件文件不要混入非组件导出）。
 */

export function sortedPolicies(policies: TopologyPolicy[]): TopologyPolicy[] {
  return [...policies].sort((a, b) => a.priority - b.priority || a.id - b.id);
}

export function issueNodeId(issue: TopologyIssue): string | null {
  switch (issue.entity_type) {
    case "policy":
      return `rule-${issue.entity_id}`;
    case "exit_set":
    case "set":
      return `set-${issue.entity_id}`;
    case "spec":
    case "inbound":
      return `inbound-${issue.entity_id}`;
    default:
      return null;
  }
}

export function groupIssuesByNode(issues: TopologyIssue[]): Record<string, TopologyIssue[]> {
  const map: Record<string, TopologyIssue[]> = {};
  for (const iss of issues) {
    const nid = issueNodeId(iss);
    if (!nid) continue;
    (map[nid] ??= []).push(iss);
  }
  return map;
}

export function useReorderCommit(coreType: string, onDone: () => void) {
  return useMutation({
    mutationFn: (orderedIds: number[]) => reorderPolicies(orderedIds),
    onError: (e) => {
      toast.error(`优先级保存失败：${e instanceof Error ? e.message : String(e)}`);
    },
    onSuccess: (n) => {
      toast.success(`优先级已更新（${n} 条）`);
      onDone();
    },
    // mutation key 绑定 core 便于调试观察
    mutationKey: ["topology-reorder", coreType],
  });
}

export function policyName(snapshot: TopologySnapshot | undefined, id: number): string {
  return snapshot?.policies.find((p) => p.id === id)?.name ?? `#${id}`;
}
