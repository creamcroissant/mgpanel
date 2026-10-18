/** 画布节点纯工具：从 shared.tsx 拆出，保证 shared.tsx 只导出组件与常量。 */

export function disabledClass(enabled?: boolean): string {
  return enabled === false ? "opacity-55 saturate-50" : "";
}

export function nodeIssues(d: Record<string, unknown>): { severity: string; message: string }[] {
  const raw = d.issues;
  return Array.isArray(raw) ? (raw as { severity: string; message: string }[]) : [];
}
