package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// buildV2RayAPIFragmentArtifacts 为 sing-box spec 生成 experimental.v2ray_api 段独立 fragment。
//
// 触发条件：spec 的 core_specific.sing-box.v2ray_api.enabled == true（可省略 listen，
// 默认 127.0.0.1:19194，与 agent 端 ensureV2RayAPIConfig 兜底默认一致）。
//
// 生成文件 experimental-v2ray-api.json：
//
//	{"experimental":{"v2ray_api":{"listen":<listen>,"stats":{"enabled":true}}}}
//
// 与 mesh-*.json 同为 sing-box -C 合并的 fragment 文件。
//
// 幂等语义：同批次已存在同名 fragment 时跳过（由调用方 filenameSet 校验已覆盖 spec 产物；
// 此处自身去重）。编译器产物优先于 agent 端 ensureV2RayAPIConfig 兜底文件，二者文件名不同
// （experimental-v2ray-api.json vs experimental_v2ray.json），不会重复 500。
func buildV2RayAPIFragmentArtifacts(
	ctx context.Context,
	req RenderArtifactsRequest,
	coreType string,
	enabledSpecs []*repository.InboundSpec,
) ([]*repository.DesiredArtifact, error) {
	if normalizeCoreType(coreType) != normalizeCoreType(CoreTypeSingBox) {
		return nil, nil // v2ray_api 段当前仅对 sing-box 渲染；xray 由 agent ensure 兜底
	}
	var out []*repository.DesiredArtifact
	seen := false
	for _, spec := range enabledSpecs {
		if spec == nil {
			continue
		}
		coreSpecific, err := artifactDecodeJSONObject(spec.CoreSpecific)
		if err != nil {
			continue // 交给 validateSpecInput 的主渲染路径报错，这里跳过
		}
		section, _, _ := artifactExtractCoreSection(spec, normalizeCoreType(coreType), coreSpecific, "sing-box", "singbox")
		if section == nil {
			continue
		}
		rawV2, ok := section["v2ray_api"]
		if !ok {
			continue
		}
		v2Obj, ok := rawV2.(map[string]any)
		if !ok {
			continue
		}
		enabled, _ := v2Obj["enabled"].(bool)
		if !enabled {
			continue
		}
		if seen {
			continue // 同批次只生成一次（listen 取首个启用 spec 的值）
		}
		listen, _ := v2Obj["listen"].(string)
		if strings.TrimSpace(listen) == "" {
			listen = defaultV2RayAPIListen
		}
		content := renderV2RayAPIFragment(listen)
		out = append(out, &repository.DesiredArtifact{
			AgentHostID:     req.AgentHostID,
			CoreType:        normalizeCoreType(coreType),
			DesiredRevision: req.DesiredRevision,
			Filename:        "experimental-v2ray-api.json",
			SourceTag:       "v2ray_api",
			Content:         content,
			ContentHash:     artifactHash(content),
		})
		seen = true
	}
	return out, nil
}

// renderV2RayAPIFragment 生成 experimental.v2ray_api 段 JSON bytes。
func renderV2RayAPIFragment(listen string) []byte {
	data := map[string]any{
		"experimental": map[string]any{
			"v2ray_api": map[string]any{
				"listen": listen,
				"stats":  map[string]any{"enabled": true},
			},
		},
	}
	b, _ := json.Marshal(data)
	return b
}

// buildMeshExitArtifacts 为 mesh 网络中的 agent 生成 socks inbound/outbound + routing rule 等附加 artifacts。
func (s *artifactCompilerService) buildMeshExitArtifacts(
	ctx context.Context,
	req RenderArtifactsRequest,
	renderer artifactRenderer,
	enabledSpecs []*repository.InboundSpec,
) ([]*repository.DesiredArtifact, []ruleArtifact, error) {
	if s.meshPeers == nil {
		return nil, nil, nil
	}

	// 查询本 agent 的 mesh peer 信息（用于本机 WG IP）
	ownPeer, err := s.meshPeers.FindByAgentHostID(ctx, req.AgentHostID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil, nil // 本 agent 不在 mesh 中，跳过
		}
		return nil, nil, fmt.Errorf("mesh peer lookup: %w", err)
	}

	// 查询所有 mesh peer，构建 agentHostID → WG IP 映射
	allPeers, err := s.meshPeers.ListByNetworkID(ctx, "default")
	if err != nil {
		return nil, nil, fmt.Errorf("list mesh peers: %w", err)
	}
	if strings.TrimSpace(ownPeer.WGIP) == "" {
		// 入口自身没有 mesh 地址：出口渲染/配对都不可能成立（与面板侧 meshPeerIndex 同口径）。
		slog.Warn("mesh peer missing wg ip, skip exit rendering", "agent_host_id", req.AgentHostID)
		return nil, nil, nil
	}
	peerWGIP := make(map[int64]string, len(allPeers))
	for _, p := range allPeers {
		if strings.TrimSpace(p.WGIP) == "" {
			continue // 无 mesh 地址的 peer 不参与出口渲染（与面板侧配对过滤保持一致，GAP-7）
		}
		peerWGIP[p.AgentHostID] = p.WGIP
	}

	coreType := normalizeCoreType(req.CoreType)
	var artifacts []*repository.DesiredArtifact
	var rules []ruleArtifact

	// direct outbound：供 selector 兜底 + remote rule_set 下载直连使用，统一生成一次
	directContent := renderMeshDirectOutbound(coreType)
	artifacts = append(artifacts, &repository.DesiredArtifact{
		AgentHostID: req.AgentHostID, CoreType: coreType, DesiredRevision: req.DesiredRevision,
		Filename: "mesh-direct.json", SourceTag: "direct",
		Content: directContent, ContentHash: artifactHash(directContent),
	})

	// 核心缓存：sing-box 的 remote rule_set 需要 cache_file 才能跨重启复用，
	// 否则每次启动都会联网拉取（GitHub 不可达时规则集为空）。
	if coreType == "sing-box" {
		cacheContent := renderCoreCacheFragment()
		artifacts = append(artifacts, &repository.DesiredArtifact{
			AgentHostID: req.AgentHostID, CoreType: coreType, DesiredRevision: req.DesiredRevision,
			Filename: "mesh-core-cache.json", SourceTag: "core-cache",
			Content: cacheContent, ContentHash: artifactHash(cacheContent),
		})
	}

	// 1. socks inbound：本 agent 监听 WG IP:1080，作为 mesh 出口服务
	//    所有 mesh agent 都生成，确保能作为出口。
	inboundTag := "mesh-inbound"
	inboundContent := renderMeshSocksInbound(coreType, ownPeer.WGIP, 1080, inboundTag)
	artifacts = append(artifacts, &repository.DesiredArtifact{
		AgentHostID:     req.AgentHostID,
		CoreType:        coreType,
		DesiredRevision: req.DesiredRevision,
		Filename:        "mesh-inbound.json",
		SourceTag:       inboundTag,
		Content:         inboundContent,
		ContentHash:     artifactHash(inboundContent),
	})

	// 2. 对每个 spec 生成出口 outbound + routing rule
	//    优先 exit_node_set_id（出口集合），其次 exit_agent_host_id（固定出口）
	setMemberOutboundGen := map[int64]struct{}{} // 避免重复生成成员 socks outbound
	specRuleOrdinal := 0                         // spec 兜底规则序号（dict 序 → route-9xxx）
	relayRuleOrdinal := 0                        // 中继规则序号（route-0xxx）

	for _, spec := range enabledSpecs {
		specTag := normalizeTag(spec.Tag)

		// 2-0. relay_path_id：多跳中继链路（最高优先级，高于 exit_node_set_id / exit_agent_host_id）
		if spec.RelayPathID != nil && *spec.RelayPathID > 0 {
			var handled bool
			var err error
			var relayRules []ruleArtifact
			artifacts, relayRules, handled, err = s.buildRelaySpecRouting(ctx, req, coreType, specTag, *spec.RelayPathID, artifacts, relayRuleOrdinal)
			if err != nil {
				return nil, nil, fmt.Errorf("build relay routing for spec %s: %w", specTag, err)
			}
			if handled {
				rules = append(rules, relayRules...)
				relayRuleOrdinal += len(relayRules)
				continue
			}
			// 链路不可用（不存在/禁用/节点不足/下一跳无 mesh IP）→ 回落旧出口逻辑并告警
			slog.Warn("relay path unavailable for spec, falling back to legacy exit",
				"spec", specTag, "relay_path_id", *spec.RelayPathID, "agent_host_id", req.AgentHostID)
		}

		// 2a. exit_node_set_id：出口集合（负载均衡+故障转移）
		if spec.ExitNodeSetID != nil && *spec.ExitNodeSetID > 0 {
			var err error
			var setRules []ruleArtifact
			artifacts, setRules, err = s.buildMeshExitSetArtifacts(ctx, req, coreType, specTag, *spec.ExitNodeSetID, peerWGIP, setMemberOutboundGen, artifacts, specRoutingOrder())
			if err != nil {
				return nil, nil, fmt.Errorf("build exit set artifacts for spec %s: %w", specTag, err)
			}
			rules = append(rules, setRules...)
			if len(setRules) > 0 {
				specRuleOrdinal++
			}
			continue
		}

		// 2b. exit_agent_host_id：固定出口（原有逻辑）
		if spec.ExitAgentHostID == nil || *spec.ExitAgentHostID <= 0 {
			continue
		}
		exitID := *spec.ExitAgentHostID
		exitWGIP, ok := peerWGIP[exitID]
		if !ok {
			continue // 出口 agent 不在 mesh 中，跳过
		}
		if exitID == req.AgentHostID {
			continue // 出口节点就是本机，无需隧道
		}

		exitTag := fmt.Sprintf("mesh-exit-%d", exitID)

		// 成员出站：l3 模式下为 direct/freedom + fwmark（内核分发），否则 socks over mesh
		outboundContent := renderMeshMemberOutbound(ctx, coreType, req.AgentHostID, exitID, exitWGIP, exitTag)
		outboundFilename := fmt.Sprintf("mesh-%s-exit.json", specTag)
		artifacts = append(artifacts, &repository.DesiredArtifact{
			AgentHostID: req.AgentHostID, CoreType: coreType, DesiredRevision: req.DesiredRevision,
			Filename: outboundFilename, SourceTag: specTag + "-exit",
			Content: outboundContent, ContentHash: artifactHash(outboundContent),
		})

		// routing rule：该 inbound 的流量 → 出口 outbound（兜底段）
		routingContent := renderMeshRoutingRule(coreType, specTag, exitTag)
		rules = append(rules, ruleArtifact{
			order:     specRoutingOrder(),
			slug:      specTag + "-routing",
			sourceTag: specTag + "-routing",
			content:   routingContent,
		})
		specRuleOrdinal++
	}

	// 3. 应用 routing policies（geosite/domain → 出口集合的自动分流）
	//    求值顺序 = [scoped 策略按 priority] ++ [全局策略按 priority] ++ [spec 兜底]，
	//    由 route-<order>-<slug>.json 文件名字典序固化（见 I1 注释）。
	//    绑定到其他 agent 入站的 scoped 策略在编译其它 agent 时才生效，此处排除。
	if s.routingPolicies != nil {
		policies, err := s.routingPolicies.ListEnabledByCore(ctx, coreType)
		if err != nil {
			return nil, nil, fmt.Errorf("list routing policies: %w", err)
		}
		orderedPolicies := orderScopedFirst(policies, enabledSpecs)
		// 为 geosite 策略生成 rule_set 定义（sing-box）
		if ruleSetContent := buildMeshRoutingPolicyRuleSet(ctx, coreType, orderedPolicies); len(ruleSetContent) > 0 {
			artifacts = append(artifacts, &repository.DesiredArtifact{
				AgentHostID: req.AgentHostID, CoreType: coreType, DesiredRevision: req.DesiredRevision,
				Filename: "mesh-rule-sets.json", SourceTag: "rule-sets",
				Content: ruleSetContent, ContentHash: artifactHash(ruleSetContent),
			})
		}
		scopedOrdinal, globalOrdinal := 0, 0
		for _, p := range orderedPolicies {
			if p.TargetSetID == nil || *p.TargetSetID <= 0 {
				continue
			}
			// 规则级出口池（loadbalance/balancer）：该规则命中流量在目标出口集成员间
			// 按粘性开关分摊 —— sticky=source-hash(固定出口) / 非sticky=round-robin(轮询)。
			// 池成员复用 spec/set 阶段已生成的成员 socks；若该集未被任何 spec 绑定
			// （成员 socks 未生成），此处惰性补生成。池产物独立命名避免多规则共享冲突。
			poolArtifacts, err := s.buildRoutingPolicyPool(ctx, req, coreType, p, peerWGIP, setMemberOutboundGen)
			if err != nil {
				return nil, nil, fmt.Errorf("build routing policy pool for policy %d: %w", p.ID, err)
			}
			artifacts = append(artifacts, poolArtifacts...)
			ruleContent := renderRoutingPolicyRule(coreType, p)
			if len(ruleContent) == 0 {
				continue
			}
			scoped := p.SpecID != nil
			order := policyRuleOrder(scoped, globalOrdinal)
			if scoped {
				order = policyRuleOrder(true, scopedOrdinal)
				scopedOrdinal++
			} else {
				globalOrdinal++
			}
			rules = append(rules, ruleArtifact{
				order:     order,
				slug:      fmt.Sprintf("policy-%d", p.ID),
				sourceTag: fmt.Sprintf("policy-%d", p.ID),
				content:   ruleContent,
			})
		}
	}

	return artifacts, rules, nil
}

// renderCoreCacheFragment 生成 sing-box 的 cache_file 片段，
// 使 remote rule_set 的下载结果跨重启复用。
func renderCoreCacheFragment() []byte {
	data := map[string]any{
		"experimental": map[string]any{
			"cache_file": map[string]any{
				"enabled": true,
				"path":    "/opt/mgpanel/agent/sing-box-cache.db",
			},
		},
	}
	b, _ := json.Marshal(data)
	return b
}

// buildMeshExitSetArtifacts 为出口集合（ExitNodeSetID）生成
// selector outbound + 成员 socks outbound + routing rule。
func (s *artifactCompilerService) buildMeshExitSetArtifacts(
	ctx context.Context,
	req RenderArtifactsRequest,
	coreType, specTag string,
	setID int64,
	peerWGIP map[int64]string,
	setMemberOutboundGen map[int64]struct{},
	artifacts []*repository.DesiredArtifact,
	specRuleOrder int,
) ([]*repository.DesiredArtifact, []ruleArtifact, error) {
	if s.exitNodeSets == nil {
		return artifacts, nil, nil
	}
	members, err := s.exitNodeSets.ListMembers(ctx, setID)
	if err != nil {
		return artifacts, nil, err
	}
	if len(members) == 0 {
		return artifacts, nil, nil
	}

	// 出口集自身策略（I2）：决定 sing-box loadbalance / xray balancer 的选路方式
	setStrategy := "round_robin"
	if set, err := s.exitNodeSets.FindByID(ctx, setID); err == nil && set != nil {
		setStrategy = set.Strategy
	}

	var memberOutboundTags []string
	for _, m := range members {
		if !m.Enabled {
			continue
		}
		wgIP, ok := peerWGIP[m.AgentHostID]
		if !ok {
			continue // 该成员不在 mesh 中，跳过
		}
		if m.AgentHostID == req.AgentHostID {
			continue // 跳过自身
		}
		exitTag := fmt.Sprintf("mesh-exit-%d", m.AgentHostID)
		memberOutboundTags = append(memberOutboundTags, exitTag)

		// 每个成员只生成一次 socks outbound（可被多个集共享）
		if _, gen := setMemberOutboundGen[m.AgentHostID]; gen {
			continue
		}
		setMemberOutboundGen[m.AgentHostID] = struct{}{}

		outboundContent := renderMeshMemberOutbound(ctx, coreType, req.AgentHostID, m.AgentHostID, wgIP, exitTag)
		artifacts = append(artifacts, &repository.DesiredArtifact{
			AgentHostID: req.AgentHostID, CoreType: coreType, DesiredRevision: req.DesiredRevision,
			Filename: fmt.Sprintf("mesh-exit-%d.json", m.AgentHostID), SourceTag: fmt.Sprintf("exit-%d", m.AgentHostID),
			Content: outboundContent, ContentHash: artifactHash(outboundContent),
		})
	}

	if len(memberOutboundTags) == 0 {
		return artifacts, nil, nil
	}

	setTag := fmt.Sprintf("mesh-exit-set-%d", setID)
	var setContent []byte
	switch {
	case coreType == "xray":
		// xray balancer：策略映射（least_connections→leastLoad；哈希策略降级为 roundRobin）
		setContent = renderXrayBalancer(setTag, memberOutboundTags, setStrategy)
	case len(memberOutboundTags) > 1:
		// sing-box 多成员：按出口集策略做负载均衡，direct 仅作 fallback
		setContent = renderMeshLoadBalanceOutbound(coreType, setTag, memberOutboundTags, singBoxLoadBalanceStrategy(setStrategy))
	default:
		// sing-box 单成员：保留 selector（URLTest 式故障转移，direct 作兑底）
		selectorMembers := append([]string{}, memberOutboundTags...)
		selectorMembers = append(selectorMembers, "direct")
		setContent = renderMeshSelectorOutbound(coreType, setTag, selectorMembers)
	}
	artifacts = append(artifacts, &repository.DesiredArtifact{
		AgentHostID: req.AgentHostID, CoreType: coreType, DesiredRevision: req.DesiredRevision,
		Filename: fmt.Sprintf("mesh-%s-set.json", specTag), SourceTag: specTag + "-set",
		Content: setContent, ContentHash: artifactHash(setContent),
	})

	// 生成 routing rule → 集合出口（兜底段，位于所有策略规则之后）
	routingContent := renderMeshRoutingRule(coreType, specTag, setTag)
	rules := []ruleArtifact{{
		order:     specRuleOrder,
		slug:      specTag + "-routing",
		sourceTag: specTag + "-routing",
		content:   routingContent,
	}}
	return artifacts, rules, nil
}

// buildRoutingPolicyPool 为一条路由规则生成独立 loadbalance 出口池（粘性分摊）。
// 产物：
//   - 缺失成员 socks outbound（mesh-exit-{agentID}.json，复用全局去重 map）
//   - 池 outbound：mesh-policy-{id}-pool.json
//     sing-box: type=loadbalance，sticky→source-hash，否则 round-robin，
//     成员=[mesh-exit-{agentID}...] + direct 兑底
//     xray: 无 loadbalance → 返回空（规则仍指向 mesh-exit-set-{id} 的 balancer）
//
// 仅对 sing-box 生成（xray 规则渲染沿用原出口集 balancer）。
func (s *artifactCompilerService) buildRoutingPolicyPool(
	ctx context.Context,
	req RenderArtifactsRequest,
	coreType string,
	p *repository.RoutingPolicy,
	peerWGIP map[int64]string,
	setMemberOutboundGen map[int64]struct{},
) ([]*repository.DesiredArtifact, error) {
	if p == nil || p.TargetSetID == nil || *p.TargetSetID <= 0 {
		return nil, nil
	}
	if (coreType != "xray" && coreType != "sing-box") || s.exitNodeSets == nil || s.meshPeers == nil {
		return nil, nil
	}

	setID := *p.TargetSetID
	members, err := s.exitNodeSets.ListMembers(ctx, setID)
	if err != nil {
		return nil, err
	}

	var memberOutboundTags []string
	var out []*repository.DesiredArtifact
	for _, m := range members {
		if !m.Enabled {
			continue
		}
		wgIP, ok := peerWGIP[m.AgentHostID]
		if !ok {
			continue // 成员不在 mesh 中，跳过
		}
		if m.AgentHostID == req.AgentHostID {
			continue // 跳过自身
		}
		exitTag := fmt.Sprintf("mesh-exit-%d", m.AgentHostID)
		memberOutboundTags = append(memberOutboundTags, exitTag)

		// 每个成员只生成一次成员出站（可能已被 spec 出口集或其它规则生成）
		if _, gen := setMemberOutboundGen[m.AgentHostID]; gen {
			continue
		}
		setMemberOutboundGen[m.AgentHostID] = struct{}{}
		outboundContent := renderMeshMemberOutbound(ctx, coreType, req.AgentHostID, m.AgentHostID, wgIP, exitTag)
		out = append(out, &repository.DesiredArtifact{
			AgentHostID: req.AgentHostID, CoreType: coreType, DesiredRevision: req.DesiredRevision,
			Filename: fmt.Sprintf("mesh-exit-%d.json", m.AgentHostID), SourceTag: fmt.Sprintf("exit-%d", m.AgentHostID),
			Content: outboundContent, ContentHash: artifactHash(outboundContent),
		})
	}
	if len(memberOutboundTags) == 0 {
		return nil, nil
	}

	strategy := "round-robin"
	if p.Sticky {
		strategy = "source-hash"
	}

	if coreType == "xray" {
		// xray 无 loadbalance → 规则级 balancer（tag 与策略规则目标一致）
		balancerContent := renderXrayBalancer(policyBalancerTag(p.ID), memberOutboundTags, strategy)
		out = append(out, &repository.DesiredArtifact{
			AgentHostID: req.AgentHostID, CoreType: coreType, DesiredRevision: req.DesiredRevision,
			Filename: fmt.Sprintf("mesh-policy-%d-balancer.json", p.ID), SourceTag: fmt.Sprintf("policy-%d-balancer", p.ID),
			Content: balancerContent, ContentHash: artifactHash(balancerContent),
		})
		return out, nil
	}

	// sing-box：direct 只作为 fallback（不参与分摊，避免流量直连泄漏）
	poolContent := renderMeshLoadBalanceOutbound(coreType, policyPoolTag(p.ID), memberOutboundTags, strategy)
	out = append(out, &repository.DesiredArtifact{
		AgentHostID: req.AgentHostID, CoreType: coreType, DesiredRevision: req.DesiredRevision,
		Filename: fmt.Sprintf("mesh-policy-%d-pool.json", p.ID), SourceTag: fmt.Sprintf("policy-%d-pool", p.ID),
		Content: poolContent, ContentHash: artifactHash(poolContent),
	})
	return out, nil
}

// renderXrayBalancer 生成 xray balancer（规则级出口池）。
func renderXrayBalancer(tag string, memberOutboundTags []string, setStrategy string) []byte {
	strategy, _ := xrayBalancerStrategy(setStrategy)
	data := map[string]any{
		"routing": map[string]any{
			"balancers": []any{
				map[string]any{
					"tag":         tag,
					"selector":    memberOutboundTags,
					"strategy":    map[string]any{"type": strategy},
					"fallbackTag": "direct",
				},
			},
		},
	}
	b, _ := json.Marshal(data)
	return b
}

// renderMeshLoadBalanceOutbound 生成 sing-box loadbalance outbound（粘性/分摊池）。
// direct 通过 fallback 提供可达性兑底，不进入分摊成员，否则轮询/哈希会把部分流量直连。
func renderMeshLoadBalanceOutbound(coreType, tag string, memberOutboundTags []string, strategy string) []byte {
	if coreType != "sing-box" {
		return nil
	}
	data := map[string]any{
		"outbounds": []any{
			map[string]any{
				"type":      "loadbalance",
				"tag":       tag,
				"outbounds": memberOutboundTags,
				"strategy":  strategy,
				"fallback":  "direct",
				"url":       loadBalanceHealthCheckURL,
				"interval":  loadBalanceHealthInterval,
			},
		},
	}
	b, _ := json.Marshal(data)
	return b
}

// orderScopedFirst 将策略按求值顺序排列：
// [绑定本 agent 所宿 spec 的 scoped 策略（保持 priority 序）] ++ [全局策略（保持 priority 序）]。
// 绑定到其它 agent 入站的 scoped 策略被排除（由宿主 agent 的编译轮次渲染）。
func orderScopedFirst(policies []*repository.RoutingPolicy, enabledSpecs []*repository.InboundSpec) []*repository.RoutingPolicy {
	scopedSet := make(map[int64]struct{}, len(enabledSpecs))
	for _, sp := range enabledSpecs {
		scopedSet[sp.ID] = struct{}{}
	}
	ordered := make([]*repository.RoutingPolicy, 0, len(policies))
	for _, p := range policies {
		if p.SpecID != nil {
			if _, ok := scopedSet[*p.SpecID]; ok {
				ordered = append(ordered, p)
			}
		}
	}
	for _, p := range policies {
		if p.SpecID == nil {
			ordered = append(ordered, p)
		}
	}
	return ordered
}

// relayFwmark 中继链路的 fwmark：40000 + pathID（避开系统保留段）。
// agent 侧 RelayRouteManager 据此下发 ip rule/ip route；路由表号 = 100 + pathID。
func relayFwmark(pathID int64) int64 { return 40000 + pathID }

// renderRelayMarkOutbound 生成中继出站 artifact：direct/freedom 出站打 fwmark，
// 由内核策略路由接管物理路径。双核各一分支——这是 L3 与代理核心解耦的唯一耦合点。
func renderRelayMarkOutbound(coreType string, tag string, pathID int64) []byte {
	mark := relayFwmark(pathID)
	var data map[string]any
	if coreType == "xray" {
		data = map[string]any{"outbounds": []any{map[string]any{
			"tag": tag, "protocol": "freedom", "settings": map[string]any{},
			"streamSettings": map[string]any{"sockopt": map[string]any{"mark": mark}},
		}}}
	} else {
		data = map[string]any{"outbounds": []any{map[string]any{
			"tag": tag, "type": "direct", "routing_mark": mark,
		}}}
	}
	b, _ := json.Marshal(data)
	return b
}

// buildRelaySpecRouting 为绑定中继链路的 spec 生成入口路由规则 + mark 出站。
// 返回 handled=false 表示链路不可用（不存在/禁用/节点不足/任一跳无 mesh IP），调用方回落旧出口逻辑。
// 产物（均渲染在入口 agent）：
//   - mesh-{specTag}-relay-{pathID}.json：inbound[specTag] → relay-mk-{pathID} 路由规则
//   - relay-{pathID}-out.json：relay-mk-{pathID} direct/freedom 出站带 fwmark=40000+pathID
//     （物理路径由 agent 侧 RelayRouteManager 下发的 nftables 策略路由实现）
func (s *artifactCompilerService) buildRelaySpecRouting(
	ctx context.Context,
	req RenderArtifactsRequest,
	coreType, specTag string,
	pathID int64,
	artifacts []*repository.DesiredArtifact,
	relayOrdinal int,
) ([]*repository.DesiredArtifact, []ruleArtifact, bool, error) {
	if s.relayPaths == nil || s.meshPeers == nil {
		return artifacts, nil, false, nil
	}
	path, err := s.relayPaths.GetByID(ctx, pathID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return artifacts, nil, false, nil
		}
		return artifacts, nil, false, err
	}
	if !path.Enabled || len(path.Nodes) < 2 {
		return artifacts, nil, false, nil
	}
	nodes := append([]repository.RelayPathNode(nil), path.Nodes...)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Sequence < nodes[j].Sequence })

	// 全链路下一跳可达性预检：任一跳缺 mesh IP 则整体回落，避免半成品链路吞流量
	allPeers, err := s.meshPeers.ListByNetworkID(ctx, "default")
	if err != nil {
		return artifacts, nil, false, fmt.Errorf("list mesh peers: %w", err)
	}
	peerWGIP := make(map[int64]string, len(allPeers))
	for _, p := range allPeers {
		peerWGIP[p.AgentHostID] = p.WGIP
	}
	for i := 0; i < len(nodes)-1; i++ {
		dst := nodes[i+1]
		ip, ok := peerWGIP[dst.AgentHostID]
		if !ok || ip == "" {
			slog.Warn("relay path hop unreachable, falling back to legacy exit",
				"path_id", pathID, "path_name", path.Name,
				"next_agent", dst.AgentHostID, "spec", specTag)
			return artifacts, nil, false, nil
		}
	}

	entryOutboundTag := fmt.Sprintf("relay-mk-%d", pathID)
	ruleContent := renderMeshRoutingRule(coreType, specTag, entryOutboundTag)
	rules := []ruleArtifact{{
		order:     relayRuleOrder() + relayOrdinal*ruleOrderStep,
		slug:      fmt.Sprintf("%s-relay-%d", specTag, pathID),
		sourceTag: fmt.Sprintf("%s-relay-%d", specTag, pathID),
		content:   ruleContent,
	}}

	outContent := renderRelayMarkOutbound(coreType, entryOutboundTag, pathID)
	artifacts = append(artifacts, &repository.DesiredArtifact{
		AgentHostID:     req.AgentHostID,
		CoreType:        coreType,
		DesiredRevision: req.DesiredRevision,
		Filename:        fmt.Sprintf("relay-%d-out.json", pathID),
		SourceTag:       fmt.Sprintf("relay-mk-%d", pathID),
		Content:         outContent,
		ContentHash:     artifactHash(outContent),
	})
	return artifacts, rules, true, nil
}

// renderMeshSocksInbound 生成 sing-box / Xray 的 socks inbound 配置（监听 WG IP:port）。
func renderMeshSocksInbound(coreType, listenIP string, port int, tag string) []byte {
	if coreType == "xray" {
		data := map[string]any{
			"inbounds": []any{
				map[string]any{
					"protocol": "socks",
					"tag":      tag,
					"listen":   listenIP,
					"port":     port,
					"settings": map[string]any{
						"udp":  false,
						"auth": "noauth",
					},
				},
			},
		}
		b, _ := json.Marshal(data)
		return b
	}
	// sing-box
	data := map[string]any{
		"inbounds": []any{
			map[string]any{
				"type":        "socks",
				"tag":         tag,
				"listen":      listenIP,
				"listen_port": port,
			},
		},
	}
	b, _ := json.Marshal(data)
	return b
}

// renderMeshSocksOutbound 生成 sing-box / Xray 的 socks outbound 配置。
// renderMeshEgressMarkOutbound 渲染"内核分发"成员出站（I11）：
// 出站本身只是本机 direct/freedom + fwmark，实际出口由内核按 mark 的策略路由决定
// （详见 docs/plans/20260915-egress-dispatch-l3.md）。
func renderMeshEgressMarkOutbound(coreType, tag string, memberAgentID int64) []byte {
	mark := EgressMemberMark(memberAgentID)
	if coreType == "xray" {
		data := map[string]any{
			"outbounds": []any{
				map[string]any{
					"protocol": "freedom",
					"tag":      tag,
					"streamSettings": map[string]any{
						"sockopt": map[string]any{"mark": mark},
					},
				},
			},
		}
		b, _ := json.Marshal(data)
		return b
	}
	// sing-box
	data := map[string]any{
		"outbounds": []any{
			map[string]any{
				"type":         "direct",
				"tag":          tag,
				"routing_mark": mark,
			},
		},
	}
	b, _ := json.Marshal(data)
	return b
}

// renderMeshMemberOutbound 按主机分发模式渲染成员出站（I11）：
//   - l3:   direct/freedom + fwmark（内核分发；出口无需 sing-box）
//   - socks: 传统 socks over mesh（默认，行为不变）
func renderMeshMemberOutbound(ctx context.Context, coreType string, agentHostID int64, memberAgentID int64, memberWGIP string, tag string) []byte {
	if resolveEgressMode(ctx, agentHostID) == "l3" {
		return renderMeshEgressMarkOutbound(coreType, tag, memberAgentID)
	}
	return renderMeshSocksOutbound(coreType, memberWGIP, 1080, tag)
}

func renderMeshSocksOutbound(coreType, serverIP string, port int, tag string) []byte {
	if coreType == "xray" {
		data := map[string]any{
			"outbounds": []any{
				map[string]any{
					"protocol": "socks",
					"tag":      tag,
					"settings": map[string]any{
						"servers": []any{
							map[string]any{
								"address": serverIP,
								"port":    port,
							},
						},
					},
				},
			},
		}
		b, _ := json.Marshal(data)
		return b
	}
	// sing-box
	data := map[string]any{
		"outbounds": []any{
			map[string]any{
				"type":        "socks",
				"tag":         tag,
				"server":      serverIP,
				"server_port": port,
			},
		},
	}
	b, _ := json.Marshal(data)
	return b
}

// renderMeshRoutingRule 生成 sing-box / Xray 的路由规则，将指定 inbound tag 的流量路由到出口 outbound。
func renderMeshRoutingRule(coreType, inboundTag, outboundTag string) []byte {
	if coreType == "xray" {
		data := map[string]any{
			"routing": map[string]any{
				"rules": []any{
					map[string]any{
						"type":        "field",
						"inboundTag":  []string{inboundTag},
						"outboundTag": outboundTag,
					},
				},
			},
		}
		b, _ := json.Marshal(data)
		return b
	}
	// sing-box
	data := map[string]any{
		"route": map[string]any{
			"rules": []any{
				map[string]any{
					"inbound":  []string{inboundTag},
					"outbound": outboundTag,
				},
			},
		},
	}
	b, _ := json.Marshal(data)
	return b
}

// renderRoutingPolicyRule 根据路由策略生成 sing-box / Xray 的 route rule。
// matchType: geosite / domain / ip_cidr，匹配成功 → outbound 到目标出口集合的 selector。
func renderRoutingPolicyRule(coreType string, p *repository.RoutingPolicy) []byte {
	if p == nil || p.TargetSetID == nil {
		return nil
	}
	values := splitMatchValues(p.MatchValue)
	if len(values) == 0 {
		return nil
	}
	// sing-box 规则指向规则级 loadbalance 池（粘性/分摊在池内执行）；
	// xray 指向规则级 balancer（策略级故障转移 + 轮询/最低负载）。
	outboundTag := policyPoolTag(p.ID)
	if coreType == "xray" {
		outboundTag = policyBalancerTag(p.ID)
	}

	if coreType == "xray" {
		// Xray field rule：domain 用 domain:/geosite: 前缀，网段必须落 ip 字段
		var field string
		var list []string
		switch p.MatchType {
		case "domain":
			field = "domain"
			for _, v := range values {
				list = append(list, "domain:"+v)
			}
		case "ip_cidr":
			field = "ip"
			list = append(list, values...)
		default: // geosite
			field = "domain"
			for _, v := range values {
				list = append(list, "geosite:"+v)
			}
		}
		rule := map[string]any{
			"type":        "field",
			"outboundTag": outboundTag,
		}
		rule[field] = list
		data := map[string]any{
			"routing": map[string]any{
				"rules": []any{rule},
			},
		}
		b, _ := json.Marshal(data)
		return b
	}

	// sing-box route rule（多值 → 数组）
	var rule map[string]any
	switch p.MatchType {
	case "domain":
		rule = map[string]any{"domain_suffix": values, "outbound": outboundTag}
	case "ip_cidr":
		rule = map[string]any{"ip_cidr": values, "outbound": outboundTag}
	default: // geosite：rule_set 引用，定义由 buildMeshRoutingPolicyRuleSet 注入
		tags := make([]string, 0, len(values))
		for _, v := range values {
			tags = append(tags, routingRuleSetTag(v))
		}
		rule = map[string]any{"rule_set": tags, "outbound": outboundTag}
	}
	data := map[string]any{
		"route": map[string]any{
			"rules": []any{rule},
		},
	}
	b, _ := json.Marshal(data)
	return b
}

// routingRuleSetTag geosite 规则集 tag。
func routingRuleSetTag(value string) string { return "geosite-" + value }

// buildMeshRoutingPolicyRuleSet 为所有 geosite 类型策略生成 rule_set 定义（sing-box）。
// 每个值独立成集；来源基址由设置 route_rule_set_base_url 提供（可换镜像）。
func buildMeshRoutingPolicyRuleSet(ctx context.Context, coreType string, policies []*repository.RoutingPolicy) []byte {
	if coreType == "xray" {
		return nil // Xray 不使用 rule_set（走 geosite.dat）
	}
	base := resolveRuleSetBaseURL(ctx)
	ruleSets := make([]map[string]any, 0)
	seen := map[string]struct{}{}
	for _, p := range policies {
		if p == nil || p.MatchType != "geosite" || p.TargetSetID == nil {
			continue
		}
		for _, v := range splitMatchValues(p.MatchValue) {
			tag := routingRuleSetTag(v)
			if _, ok := seen[tag]; ok {
				continue
			}
			seen[tag] = struct{}{}
			ruleSets = append(ruleSets, map[string]any{
				"type":            "remote",
				"tag":             tag,
				"url":             fmt.Sprintf("%s/sing-geosite/rule-set/geosite-%s.srs", base, v),
				"download_detour": "direct",
				"update_interval": "24h",
			})
		}
	}
	if len(ruleSets) == 0 {
		return nil
	}
	data := map[string]any{
		"route": map[string]any{
			"rule_set": ruleSets,
		},
	}
	b, _ := json.Marshal(data)
	return b
}

// renderMeshDirectOutbound 生成 sing-box / Xray 的 direct outbound（用于 rule_set 直连下载）。
func renderMeshDirectOutbound(coreType string) []byte {
	if coreType == "xray" {
		data := map[string]any{
			"outbounds": []any{
				map[string]any{"protocol": "freedom", "tag": "direct"},
			},
		}
		b, _ := json.Marshal(data)
		return b
	}
	data := map[string]any{
		"outbounds": []any{
			map[string]any{"type": "direct", "tag": "direct"},
		},
	}
	b, _ := json.Marshal(data)
	return b
}

// renderMeshSelectorOutbound 生成 sing-box / Xray 的 selector outbound。
// sing-box: type=selector（自动故障转移+负载均衡）；Xray: balancing（fallback）。
func renderMeshSelectorOutbound(coreType, tag string, memberOutboundTags []string) []byte {
	if coreType == "xray" {
		// Xray 使用 balancing 策略组做负载均衡 + balancerSettings 做故障转移
		data := map[string]any{
			"routing": map[string]any{
				"balancers": []any{
					map[string]any{
						"tag":         tag,
						"selector":    memberOutboundTags,
						"strategy":    map[string]any{"type": "roundRobin"},
						"fallbackTag": firstOrEmpty(memberOutboundTags),
					},
				},
			},
		}
		b, _ := json.Marshal(data)
		return b
	}
	// sing-box: selector outbound，URL 测试自动选可达节点，天然支持故障转移
	data := map[string]any{
		"outbounds": []any{
			map[string]any{
				"type":                        "selector",
				"tag":                         tag,
				"outbounds":                   memberOutboundTags,
				"default":                     firstOrEmpty(memberOutboundTags),
				"interrupt_exist_connections": false,
			},
		},
	}
	b, _ := json.Marshal(data)
	return b
}

// firstOrEmpty 返回切片第一个元素，空则返回空串。
func firstOrEmpty(items []string) string {
	if len(items) == 0 {
		return ""
	}
	return items[0]
}
