// 文件路径: internal/service/compiler_rule_order.go
// 模块说明: 分流规则产物的求值顺序约定 + 匹配值/策略/资源小工具。
//
// 顺序为什么必须显式化（实测结论，勿改）：
//   - agent 侧 normalizeArtifacts 按 filename 升序排列（internal/agent/configcenter/applier.go）
//   - sing-box `run -C <dir>` 以 badjson.MergeJSON 按 os.ReadDir 字典序合并各 JSON 文件：
//     顶层键累加、对象深合并、数组按文件顺序追加
//   - 因此 route.rules / routing.rules 的求值顺序 == 承载规则的产物文件名字典序
//
// 以前规则产物名为 mesh-policy-<id>.json / mesh-<tag>-routing.json，顺序取决于 policy id
// 字符串序与 spec tag 首字母，属于巧合而非契约（tag 首字母 < "p" 时策略规则被 spec 兜底规则
// 抢先命中 → 策略永不生效）。本文件用 order 段把顺序固定下来。
package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// 规则产物 order 段（4 位十进制零填充，字典序 == 数值序）。
const (
	ruleOrderRelayBase       = 0    // 中继 mark 规则：inbound → relay-mk-<pathID>
	ruleOrderScopedBase      = 1000 // 绑定本 host 所宿 spec 的策略（按 priority 升序）
	ruleOrderGlobalBase      = 5000 // 全局策略（按 priority 升序）
	ruleOrderSpecRoutingBase = 9000 // spec 兜底：inbound → 出口集合/固定出口/中继
	ruleOrderStep            = 10
	// maxRuleOrderIndex 限制单段内策略数量，避免序号溢出到下一段基值。
	maxRuleOrderIndex = 400
)

// ruleArtifact 是一条待定序的规则产物。
type ruleArtifact struct {
	order     int
	slug      string
	sourceTag string
	content   []byte
}

// ruleArtifactFilename 规则产物文件名：route-<order>-<slug>.json。
func ruleArtifactFilename(order int, slug string) string {
	return fmt.Sprintf("route-%04d-%s.json", order, slug)
}

// specRoutingOrder spec 兜底规则段（同段内多条按 slug 字典序，互不冲突：inbound tag 各不相同）。
func specRoutingOrder() int { return ruleOrderSpecRoutingBase }

// relayRuleOrder 中继规则段。
func relayRuleOrder() int { return ruleOrderRelayBase }

// policyRuleOrder 策略规则 order：scoped（绑定本 host 入站）与全局分段，段内按 priority 下标递增。
func policyRuleOrder(scoped bool, index int) int {
	if index < 0 {
		index = 0
	}
	if index > maxRuleOrderIndex {
		index = maxRuleOrderIndex
	}
	if scoped {
		return ruleOrderScopedBase + index*ruleOrderStep
	}
	return ruleOrderGlobalBase + index*ruleOrderStep
}

// defaultRuleSetBaseURL 规则集默认来源（Lane B 注册同名设置项 route_rule_set_base_url 覆盖）。
const (
	defaultRuleSetBaseURL     = "https://raw.githubusercontent.com/SagerNet"
	ruleSetBaseURLSettingKey  = "route_rule_set_base_url"
	maxMatchValuesPerPolicy   = 64
	singBoxCacheFileArtifact  = "mesh-core-cache.json"
	singBoxCacheFilePath      = "/opt/mgpanel/agent/sing-box-cache.db"
	loadBalanceHealthCheckURL = "https://www.gstatic.com/generate_204"
	loadBalanceHealthInterval = "3m"
)

// ruleSetBaseURLProvider 由装配层注入（serve.go 读取设置项），nil 时用默认值。
// 用包级注入而非构造参数：避免改动既有构造签名导致大量测试装配点连锁改动。
var ruleSetBaseURLProvider func(context.Context) string

// SetRuleSetBaseURLProvider 注入规则集来源解析器（nil 恢复默认）。
func SetRuleSetBaseURLProvider(fn func(context.Context) string) { ruleSetBaseURLProvider = fn }

// resolveRuleSetBaseURL 解析规则集来源前缀（去掉尾部斜杠）。
func resolveRuleSetBaseURL(ctx context.Context) string {
	if ruleSetBaseURLProvider != nil {
		if v := strings.TrimSpace(ruleSetBaseURLProvider(ctx)); v != "" {
			return strings.TrimRight(v, "/")
		}
	}
	return defaultRuleSetBaseURL
}

// splitMatchValues 拆分逗号分隔匹配值：去空、去重、保序、上限 maxMatchValuesPerPolicy。
func splitMatchValues(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
		if len(out) >= maxMatchValuesPerPolicy {
			break
		}
	}
	return out
}

// singBoxLoadBalanceStrategy 把出口集合策略词表（I2）映射为 sing-box loadbalance strategy。
// 旧值（least_ping/random/weighted_random）做兼容映射，便于迁移期混合数据。
func singBoxLoadBalanceStrategy(setStrategy string) string {
	switch strings.ToLower(strings.TrimSpace(setStrategy)) {
	case "least_connections", "least-connections":
		return "least-connections"
	case "source_hash", "source-hash":
		return "source-hash"
	case "consistent_hash", "consistent-hash":
		return "consistent-hash"
	case "least_ping", "least-ping":
		return "least-connections"
	default:
		return "round-robin"
	}
}

// xrayBalancerStrategy 映射为 xray balancer strategy；lossy=true 表示核心无等价策略（降级）。
func xrayBalancerStrategy(setStrategy string) (strategy string, lossy bool) {
	switch strings.ToLower(strings.TrimSpace(setStrategy)) {
	case "least_connections", "least-connections", "least_ping", "least-ping":
		return "leastLoad", false
	case "source_hash", "source-hash", "consistent_hash", "consistent-hash":
		// xray 无源地址哈希策略 → 降级轮询（粘性在 xray 上不成立）
		return "roundRobin", true
	default:
		return "roundRobin", false
	}
}

// policyPoolTag sing-box 规则级 loadbalance 池 tag。
func policyPoolTag(policyID int64) string { return fmt.Sprintf("mesh-policy-%d-pool", policyID) }

// policyBalancerTag xray 规则级 balancer tag（与池语义对齐，标签不带 -pool 后缀）。
func policyBalancerTag(policyID int64) string { return fmt.Sprintf("mesh-policy-%d", policyID) }

// ruleArtifactsFrom 把规则产物列表转换为仓库产物（按文件名升序，保证入库顺序即求值序）。
func ruleArtifactsFrom(items []ruleArtifact, hostID int64, coreType string, revision int64) []*repository.DesiredArtifact {
	if len(items) == 0 {
		return nil
	}
	sorted := append([]ruleArtifact(nil), items...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].order != sorted[j].order {
			return sorted[i].order < sorted[j].order
		}
		return sorted[i].slug < sorted[j].slug
	})
	out := make([]*repository.DesiredArtifact, 0, len(sorted))
	for _, item := range sorted {
		content := item.content
		out = append(out, &repository.DesiredArtifact{
			AgentHostID:     hostID,
			CoreType:        coreType,
			DesiredRevision: revision,
			Filename:        ruleArtifactFilename(item.order, item.slug),
			SourceTag:       item.sourceTag,
			Content:         content,
			ContentHash:     artifactHash(content),
		})
	}
	return out
}
