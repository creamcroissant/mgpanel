package service

import (
	"context"
	"sync"
)

// 出口集内核分发（B 方案）编号契约与解析 seam。
//
// 设计见 docs/plans/20260915-egress-dispatch-l3.md：
// 「L7 决策 + L3 分发」—— 核心（sing-box/xray）按成员选定出口并给成员出站打 fwmark，
// 内核按 mark 走策略路由 → 点对点 WG 隧道（xe*/xi*）→ 出口纯内核转发 + MASQUERADE。
//
// 分段隔离（与中继链路 relay 互不干扰）：
//   - mark:  60000 + member_agent_id   （relay 用 40000 + path_id）
//   - table: 10000 + member_agent_id   （relay 用 100 + path_id）
//   - pref:  5500                      （relay 用 5000）
const (
	// EgressMarkBase 出口集内核分发 fwmark 基址（I1）。
	EgressMarkBase = 60000
	// EgressTableBase 出口集内核分发策略路由表基址（I2）。
	EgressTableBase = 10000
	// EgressRulePref 出口集内核分发 ip rule 优先级（I2）。
	EgressRulePref = 5500
	// EgressTunnelMTU 分发隧道 MTU（I7）：外层为 mesh 骨干，留出余量。
	EgressTunnelMTU = 1400
)

// EgressMemberMark 返回成员出口的 fwmark（I1）。
func EgressMemberMark(memberAgentID int64) int { return EgressMarkBase + int(memberAgentID) }

// EgressMemberTable 返回成员出口的策略路由表号（I2）。
func EgressMemberTable(memberAgentID int64) int { return EgressTableBase + int(memberAgentID) }

// EgressDispatchResolver 由面板服务实现：判定某 agent 的出口集分发模式。
//
// 判定必须包含全部闸门（I9/I13）：
//   - host 覆盖（agent_hosts.egress_dispatch）→ 全局默认（egress_dispatch_default）
//   - agent 版本门槛：老 agent 一律 "socks"
//   - 就绪门控：agent 未上报该 revision 的 egress_ready 时一律 "socks"
//
// 返回 "l3" 之外的任何值都按 socks 处理。
type EgressDispatchResolver interface {
	EgressMode(ctx context.Context, agentHostID int64) string
}

var (
	egressResolverMu sync.RWMutex
	egressResolver   EgressDispatchResolver
)

// SetEgressDispatchResolver 注入分发模式解析器（nil = 全部回落 socks）。
func SetEgressDispatchResolver(r EgressDispatchResolver) {
	egressResolverMu.Lock()
	defer egressResolverMu.Unlock()
	egressResolver = r
}

// resolveEgressMode 供编译器调用；未注入或解析失败一律返回 "socks"（行为与引入前一致）。
func resolveEgressMode(ctx context.Context, agentHostID int64) string {
	egressResolverMu.RLock()
	r := egressResolver
	egressResolverMu.RUnlock()
	if r == nil {
		return "socks"
	}
	if mode := r.EgressMode(ctx, agentHostID); mode == "l3" {
		return "l3"
	}
	return "socks"
}
