import { z } from "zod";

/**
 * 拓扑 Drawer 节点字段 schema 注册表。
 * 只覆盖可编辑实体 rule(policy) / set(exit_node_set)；
 * agent/inbound/direct 为只读展示节点，不进 Drawer。
 * 字段标签用 t(key, defaultValue) 内联缺省文案，i18n 键正式化由 polish 波次统一收编。
 */

// ===== 枚举选项 =====

export const MATCH_TYPE_OPTIONS = [
  { value: "geosite", labelKey: "admin.topology.matchType.geosite", fallback: "Geosite 站点集" },
  { value: "domain", labelKey: "admin.topology.matchType.domain", fallback: "Domain 域名" },
  { value: "ip_cidr", labelKey: "admin.topology.matchType.ipCidr", fallback: "IP-CIDR 网段" },
] as const;

export const SET_STRATEGY_OPTIONS = [
  { value: "round_robin", labelKey: "admin.topology.strategy.roundRobin", fallback: "轮询" },
  { value: "weighted_random", labelKey: "admin.topology.strategy.weightedRandom", fallback: "加权随机" },
  { value: "least_ping", labelKey: "admin.topology.strategy.leastPing", fallback: "最低延迟优先" },
  { value: "random", labelKey: "admin.topology.strategy.random", fallback: "随机" },
] as const;

export type MatchTypeValue = (typeof MATCH_TYPE_OPTIONS)[number]["value"];
export type SetStrategyValue = (typeof SET_STRATEGY_OPTIONS)[number]["value"];

// ===== Zod 校验 =====

/** match_value 支持逗号分隔多值；校验拆分后无空段、无重复 */
const matchValueSchema = z
  .string()
  .trim()
  .min(1, "匹配值不能为空")
  .refine(
    (v) => {
      const parts = v
        .split(",")
        .map((s) => s.trim())
        .filter(Boolean);
      return parts.length > 0 && new Set(parts).size === parts.length;
    },
    { message: "匹配值存在空段或重复项" }
  );

export const ruleFormSchema = z.object({
  name: z.string().trim().min(1, "策略名称不能为空").max(64, "名称最长 64 字符"),
  match_type: z.enum(["geosite", "domain", "ip_cidr"]),
  match_value: matchValueSchema,
  priority: z.coerce.number().int("优先级须为整数").min(0, "优先级不能为负").max(100000),
  enabled: z.boolean(),
});

export const memberRowSchema = z.object({
  agent_host_id: z.coerce.number().int().positive("必须选择 Agent"),
  weight: z.coerce.number().int("权重须为整数").min(1, "权重至少为 1").max(100, "权重最多 100"),
});

export const setFormSchema = z
  .object({
    name: z.string().trim().min(1, "集合名称不能为空").max(64, "名称最长 64 字符"),
    description: z.string().trim().max(255, "描述最长 255 字符"),
    strategy: z.enum(["round_robin", "weighted_random", "least_ping", "random"]),
    enabled: z.boolean(),
    members: z.array(memberRowSchema).min(1, "出口集至少需要一个成员（否则流量无处可去）"),
  })
  .refine(
    (v) => new Set(v.members.map((m) => m.agent_host_id)).size === v.members.length,
    { message: "成员中存在重复 Agent" }
  );

export type RuleFormValues = z.infer<typeof ruleFormSchema>;
export type SetFormValues = z.infer<typeof setFormSchema>;
export type MemberRow = z.infer<typeof memberRowSchema>;

// ===== 字段元数据（驱动渲染；表单控件由 DrawerPanel 按 type 分派）=====

export interface FieldSpec {
  key: string;
  labelKey: string;
  labelFallback: string;
  type: "text" | "textarea" | "number" | "select" | "switch";
  options?: readonly { value: string; labelKey: string; fallback: string }[];
  placeholder?: string;
  required?: boolean;
  readOnly?: boolean;
  help?: string;
}

export const RULE_FIELDS: FieldSpec[] = [
  { key: "name", labelKey: "admin.topology.rule.name", labelFallback: "策略名称", type: "text", required: true },
  {
    key: "match_type",
    labelKey: "admin.topology.rule.matchType",
    labelFallback: "匹配类型",
    type: "select",
    options: MATCH_TYPE_OPTIONS,
    required: true,
  },
  {
    key: "match_value",
    labelKey: "admin.topology.rule.matchValue",
    labelFallback: "匹配值",
    type: "textarea",
    required: true,
    placeholder: "netflix 或 netflix,disney,hbo（逗号分隔多值）",
    help: "逗号分隔多个值，保存后按顺序生成匹配项",
  },
  { key: "priority", labelKey: "admin.topology.rule.priority", labelFallback: "优先级（越小越先匹配）", type: "number", required: true },
  // target_set_id 由画布连线维护（f4 波次），Drawer 内只读展示语义归属
  { key: "target_set_name", labelKey: "admin.topology.rule.targetSet", labelFallback: "目标出口集（由画布连线决定）", type: "text", readOnly: true },
  { key: "enabled", labelKey: "admin.topology.common.enabled", labelFallback: "启用", type: "switch" },
];

export const SET_FIELDS: FieldSpec[] = [
  { key: "name", labelKey: "admin.topology.set.name", labelFallback: "集合名称", type: "text", required: true },
  { key: "description", labelKey: "admin.topology.set.description", labelFallback: "描述", type: "text" },
  {
    key: "strategy",
    labelKey: "admin.topology.set.strategy",
    labelFallback: "负载均衡/故障转移策略",
    type: "select",
    options: SET_STRATEGY_OPTIONS,
    required: true,
  },
  { key: "enabled", labelKey: "admin.topology.common.enabled", labelFallback: "启用", type: "switch" },
];

/** 把 zod error 平铺成 fieldKey -> 首条错误消息，供表单逐字段标红 */
export function flattenZodErrors(err: z.ZodError): Record<string, string> {
  const out: Record<string, string> = {};
  for (const issue of err.issues) {
    const key = issue.path.join(".") || "_form";
    if (!out[key]) out[key] = issue.message;
  }
  return out;
}
