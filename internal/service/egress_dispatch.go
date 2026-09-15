package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// 出口集内核分发（B 方案）面板侧：模式判定（含能力门控）+ 隧道配对分配。
// 设计：docs/plans/20260915-egress-dispatch-l3.md（§3.2 I1–I14、§10.6）。

const (
	// EgressDispatchModeInherit 跟随全局默认。
	EgressDispatchModeInherit = "inherit"
	// EgressDispatchModeSocks 传统 socks over mesh（默认，行为与引入前一致）。
	EgressDispatchModeSocks = "socks"
	// EgressDispatchModeL3 核心打 mark + 内核分发（Lane C agent 侧执行）。
	EgressDispatchModeL3 = "l3"

	// SettingKeyEgressDispatchDefault 全局默认分发模式。
	SettingKeyEgressDispatchDefault = "egress_dispatch_default"
	// DefaultEgressDispatchMode 全局默认取 socks：未显式开启时行为零变化。
	DefaultEgressDispatchMode = EgressDispatchModeSocks

	// EgressDispatchCapabilityTTL 能力门控窗口（I13）：
	// agent 必须在该时间内成功拉取过 /api/v1/agent/egress-routes，
	// 才认为其支持内核分发；否则一律回落 socks（不对老 agent 渲染 mark 出站）。
	//
	// 取值需显著大于 agent 拉取间隔（server_pull_interval 可配，默认 60s）：过短会让
	// 面板在 socks/l3 间反复翻转渲染（核心配置抖动）。取 30min 兼顾两者。
	EgressDispatchCapabilityTTL = 30 * time.Minute

	egressMeshNetworkID = "default"
)

// NormalizeEgressDispatchMode 归一化模式；无法识别一律 inherit。
func NormalizeEgressDispatchMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case EgressDispatchModeSocks:
		return EgressDispatchModeSocks
	case EgressDispatchModeL3:
		return EgressDispatchModeL3
	default:
		return EgressDispatchModeInherit
	}
}

// IsValidEgressDispatchMode 校验管理端写入的模式取值。
func IsValidEgressDispatchMode(mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case EgressDispatchModeInherit, EgressDispatchModeSocks, EgressDispatchModeL3:
		return true
	default:
		return false
	}
}

// EgressRouteAssignment 下发给 agent 的单条内核分发指派
// （JSON 契约与 internal/agent/egressroute.Assignment 一一对应）。
type EgressRouteAssignment struct {
	Role          string  `json:"role"`            // "entry" | "exit"
	EntryAgentID  int64   `json:"entry_agent_id"`  // 入口主机 ID
	MemberAgentID int64   `json:"member_agent_id"` // 成员（出口）主机 ID；exit 角色为 0
	Mark          int     `json:"mark"`            // 60000 + member_agent_id（I1）
	Table         int     `json:"table"`           // 10000 + member_agent_id（I2）
	Iface         string  `json:"iface"`           // 入口 xe<memberID> / 出口 xi<entryID>（I3）
	ListenPort    int     `json:"listen_port"`     // 21000 + seq（I5）
	LocalAddr     string  `json:"local_addr"`      // 隧道网段本端地址（入口 .1 / 出口 .2）
	OwnPrivateKey string  `json:"own_private_key"`
	PeerPublicKey string  `json:"peer_public_key"`
	PeerEndpoint  string  `json:"peer_endpoint"` // 对端 mesh IP:listen_port（I6）
	TunnelNet     string  `json:"tunnel_net"`    // 10.220.x.y/30（I4）
	MTU           int     `json:"mtu"`           // I7
	Sets          []int64 `json:"sets"`          // 仅供观测
}

// EgressDispatchService 出口集内核分发的模式判定与配对分配。
type EgressDispatchService interface {
	// EgressMode 实现 EgressDispatchResolver（编译器门控）：返回 "l3" 或 "socks"。
	EgressMode(ctx context.Context, agentHostID int64) string
	// AssignmentsForHost 返回该主机需要的内核分发指派（入口侧 + 出口侧）。
	AssignmentsForHost(ctx context.Context, agentHostID int64) ([]EgressRouteAssignment, error)
	// MarkCapabilitySynced 记录一次成功拉取（能力门控刷新，I13）。
	MarkCapabilitySynced(ctx context.Context, agentHostID int64) error
	// ReconcilePairs 按当前期望集合回收不再需要的配对。
	ReconcilePairs(ctx context.Context) error
	// SetHostMode 管理端写入主机模式；SetDefaultMode 写全局默认。
	SetHostMode(ctx context.Context, agentHostID int64, mode string) error
	SetDefaultMode(ctx context.Context, mode string) error
	// DefaultMode 返回全局默认模式。
	DefaultMode(ctx context.Context) string
}

type egressDispatchService struct {
	agentHosts repository.AgentHostRepository
	specs      repository.InboundSpecRepository
	policies   repository.RoutingPolicyRepository
	sets       repository.ExitNodeSetRepository
	relayPaths repository.RelayPathRepository
	pairs      repository.EgressDispatchPairRepository
	meshPeers  repository.AgentMeshPeerRepository
	settings   repository.SettingRepository
	logger     *slog.Logger
}

// NewEgressDispatchService 构造分发服务；settings 可为 nil（此时全局默认取 DefaultEgressDispatchMode）。
func NewEgressDispatchService(
	agentHosts repository.AgentHostRepository,
	specs repository.InboundSpecRepository,
	policies repository.RoutingPolicyRepository,
	sets repository.ExitNodeSetRepository,
	relayPaths repository.RelayPathRepository,
	pairs repository.EgressDispatchPairRepository,
	meshPeers repository.AgentMeshPeerRepository,
	settings repository.SettingRepository,
	logger *slog.Logger,
) EgressDispatchService {
	if logger == nil {
		logger = slog.Default()
	}
	return &egressDispatchService{
		agentHosts: agentHosts, specs: specs, policies: policies, sets: sets,
		relayPaths: relayPaths, pairs: pairs, meshPeers: meshPeers, settings: settings, logger: logger,
	}
}

// DefaultMode 全局默认模式（settings 缺失/非法 → socks）。
func (s *egressDispatchService) DefaultMode(ctx context.Context) string {
	if s.settings == nil {
		return DefaultEgressDispatchMode
	}
	setting, err := s.settings.Get(ctx, SettingKeyEgressDispatchDefault)
	if err != nil || setting == nil {
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			s.logger.Warn("egress-dispatch: read default mode failed", slog.String("err", err.Error()))
		}
		return DefaultEgressDispatchMode
	}
	mode := NormalizeEgressDispatchMode(setting.Value)
	switch mode {
	case EgressDispatchModeSocks, EgressDispatchModeL3:
		return mode
	default:
		return DefaultEgressDispatchMode
	}
}

// desiredMode 主机"期望模式"：host 覆盖 → 全局默认（不含能力门控）。
func (s *egressDispatchService) desiredMode(ctx context.Context, host *repository.AgentHost) string {
	mode := NormalizeEgressDispatchMode(host.EgressDispatch)
	if mode == EgressDispatchModeInherit {
		mode = s.DefaultMode(ctx)
	}
	if mode != EgressDispatchModeL3 {
		return EgressDispatchModeSocks
	}
	return EgressDispatchModeL3
}

// EgressMode 编译器门控入口：期望模式 + 能力门控（I13）。
func (s *egressDispatchService) EgressMode(ctx context.Context, agentHostID int64) string {
	host, err := s.agentHosts.FindByID(ctx, agentHostID)
	if err != nil || host == nil {
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			s.logger.Warn("egress-dispatch: load host failed",
				slog.Int64("agent_host_id", agentHostID), slog.String("err", err.Error()))
		}
		return EgressDispatchModeSocks
	}
	if s.desiredMode(ctx, host) != EgressDispatchModeL3 {
		return EgressDispatchModeSocks
	}
	if !s.capable(host) {
		s.logger.Debug("egress-dispatch: capability gate blocks l3",
			slog.Int64("agent_host_id", agentHostID), slog.Int64("egress_synced_at", host.EgressSyncedAt))
		return EgressDispatchModeSocks
	}
	return EgressDispatchModeL3
}

// capable 判断主机是否具备分发能力（I13：最近一次成功拉取在窗口内）。
func (s *egressDispatchService) capable(host *repository.AgentHost) bool {
	if host == nil || host.EgressSyncedAt <= 0 {
		return false
	}
	return time.Since(time.Unix(host.EgressSyncedAt, 0)) <= EgressDispatchCapabilityTTL
}

// MarkCapabilitySynced 刷新能力时间戳（每次成功响应下发端点时调用）。
func (s *egressDispatchService) MarkCapabilitySynced(ctx context.Context, agentHostID int64) error {
	if err := s.agentHosts.UpdateEgressSyncedAt(ctx, agentHostID, time.Now().Unix()); err != nil {
		return fmt.Errorf("update egress synced_at: %w", err)
	}
	return nil
}

// SetHostMode 写入主机模式（inherit/socks/l3）。
func (s *egressDispatchService) SetHostMode(ctx context.Context, agentHostID int64, mode string) error {
	if !IsValidEgressDispatchMode(mode) {
		return fmt.Errorf("%w: invalid egress dispatch mode %q", ErrBadRequest, mode)
	}
	host, err := s.agentHosts.FindByID(ctx, agentHostID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("load agent host: %w", err)
	}
	host.EgressDispatch = strings.ToLower(strings.TrimSpace(mode))
	if err := s.agentHosts.Update(ctx, host); err != nil {
		return fmt.Errorf("update agent host: %w", err)
	}
	return nil
}

// SetDefaultMode 写全局默认模式。
func (s *egressDispatchService) SetDefaultMode(ctx context.Context, mode string) error {
	if !IsValidEgressDispatchMode(mode) {
		return fmt.Errorf("%w: invalid egress dispatch mode %q", ErrBadRequest, mode)
	}
	if s.settings == nil {
		return fmt.Errorf("%w: settings repository unavailable", ErrBadRequest)
	}
	return s.settings.Upsert(ctx, &repository.Setting{
		Key: SettingKeyEgressDispatchDefault, Value: strings.ToLower(strings.TrimSpace(mode)),
	})
}

// meshPeerInfo 隧道所需的对端信息。
type meshPeerInfo struct {
	WGIP       string
	PublicKey  string
	PrivateKey string
}

// meshPeerIndex 返回 agentHostID → mesh peer 信息（缺 mesh 记录的主机不参与分发）。
func (s *egressDispatchService) meshPeerIndex(ctx context.Context) (map[int64]meshPeerInfo, error) {
	peers, err := s.meshPeers.ListByNetworkID(ctx, egressMeshNetworkID)
	if err != nil {
		return nil, fmt.Errorf("list mesh peers: %w", err)
	}
	index := make(map[int64]meshPeerInfo, len(peers))
	for _, p := range peers {
		if p == nil || strings.TrimSpace(p.WGIP) == "" {
			continue
		}
		index[p.AgentHostID] = meshPeerInfo{WGIP: p.WGIP, PublicKey: p.WGPublicKey, PrivateKey: p.WGPrivateKey}
	}
	return index, nil
}

// desiredPairs 计算"期望的 (入口, 成员) 配对集合"，过滤条件必须与编译器渲染一致：
// 成员 enabled + 在 mesh + 非自身。
// desiredPairSet 期望配对集合及其关联的出口集（供观测字段 sets 使用）。
type desiredPairSet struct {
	keys []repository.EgressDispatchPairKey
	sets map[repository.EgressDispatchPairKey][]int64
}

func (s *egressDispatchService) desiredPairs(ctx context.Context) (*desiredPairSet, error) {
	peerIndex, err := s.meshPeerIndex(ctx)
	if err != nil {
		return nil, err
	}
	enabled := true
	specs, err := s.specs.List(ctx, repository.InboundSpecFilter{Enabled: &enabled})
	if err != nil {
		return nil, fmt.Errorf("list enabled specs: %w", err)
	}
	policiesByCore := map[string][]*repository.RoutingPolicy{}
	seen := map[repository.EgressDispatchPairKey]struct{}{}
	out := make([]repository.EgressDispatchPairKey, 0, len(specs))
	setsOf := map[repository.EgressDispatchPairKey][]int64{}

	addMember := func(entryAgentID, memberAgentID, setID int64) {
		if memberAgentID <= 0 || memberAgentID == entryAgentID {
			return
		}
		if _, ok := peerIndex[memberAgentID]; !ok {
			return // 成员不在 mesh
		}
		if _, ok := peerIndex[entryAgentID]; !ok {
			return // 入口自身不在 mesh
		}
		key := repository.EgressDispatchPairKey{EntryAgentID: entryAgentID, MemberAgentID: memberAgentID}
		if setID > 0 && !containsInt64(setsOf[key], setID) {
			setsOf[key] = append(setsOf[key], setID)
		}
		if _, dup := seen[key]; dup {
			return
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}

	addSet := func(entryAgentID int64, setID *int64) error {
		if setID == nil || *setID <= 0 {
			return nil
		}
		members, err := s.sets.ListMembers(ctx, *setID)
		if err != nil {
			return fmt.Errorf("list members of set %d: %w", *setID, err)
		}
		for _, m := range members {
			if m == nil || !m.Enabled {
				continue
			}
			addMember(entryAgentID, m.AgentHostID, *setID)
		}
		return nil
	}

	relayUsable := map[int64]bool{}
	isRelayUsable := func(pathID int64) bool {
		if usable, cached := relayUsable[pathID]; cached {
			return usable
		}
		usable := false
		if s.relayPaths != nil {
			if paths, err := s.relayPaths.List(ctx, ""); err == nil {
				for _, rp := range paths {
					if rp == nil || rp.ID != pathID || !rp.Enabled || len(rp.Nodes) < 2 {
						continue
					}
					nodes := append([]repository.RelayPathNode(nil), rp.Nodes...)
					sort.Slice(nodes, func(i, j int) bool { return nodes[i].Sequence < nodes[j].Sequence })
					// 与编译器 buildRelaySpecRouting 同口径：**每一跳**都必须有 mesh IP，
					// 否则编译器会回落固定出口/出口集渲染 mark，而这里却以为中继接管了（GAP-1 残留）。
					usable = true
					for i := 0; i+1 < len(nodes); i++ {
						if _, ok := peerIndex[nodes[i+1].AgentHostID]; !ok {
							usable = false
							break
						}
					}
					break
				}
			}
		}
		relayUsable[pathID] = usable
		return usable
	}

	// 0) 全局策略（SpecID 为空）对所有"会渲染产物的 mesh 主机"生效：
	//    编译器的策略段独立于 spec 循环，且显式不因 enabledSpecs 为空而提前返回，
	//    因此**没有 spec 的 mesh 主机**同样会渲染全局策略池的 mark 出站 → 必须有配对。
	//    核心类型取自 agent_hosts.current_core_type（面板按该类型渲染产物）。
	if len(peerIndex) > 0 {
		hosts, err := s.agentHosts.ListAll(ctx)
		if err != nil {
			return nil, fmt.Errorf("list agent hosts: %w", err)
		}
		coreOf := make(map[int64]string, len(hosts))
		for _, h := range hosts {
			if h == nil {
				continue
			}
			coreOf[h.ID] = normalizeCoreType(h.CurrentCoreType)
		}
		for hostID := range peerIndex {
			coreType := coreOf[hostID]
			if coreType == "" {
				continue // 无核心类型 → 面板不渲染该主机产物
			}
			pols, ok := policiesByCore[coreType]
			if !ok {
				pols, err = s.policies.ListEnabledByCore(ctx, coreType)
				if err != nil {
					return nil, fmt.Errorf("list policies for core %s: %w", coreType, err)
				}
				policiesByCore[coreType] = pols
			}
			for _, p := range pols {
				if p == nil || p.SpecID != nil || p.TargetSetID == nil || *p.TargetSetID <= 0 {
					continue
				}
				if err := addSet(hostID, p.TargetSetID); err != nil {
					return nil, err
				}
			}
		}
	}

	for _, spec := range specs {
		if spec == nil || spec.AgentHostID == nil {
			continue
		}
		entry := *spec.AgentHostID
		coreType := strings.TrimSpace(spec.CoreType)
		if _, ok := policiesByCore[coreType]; !ok {
			pols, err := s.policies.ListEnabledByCore(ctx, coreType)
			if err != nil {
				return nil, fmt.Errorf("list policies for core %s: %w", coreType, err)
			}
			policiesByCore[coreType] = pols
		}
		// 中继链路可用时，编译器直接在 2-0 段 continue（不渲染出口集/固定出口）→ 此处同样跳过，
		// 避免建造不承载流量的多余隧道（也会占用 seq/端口）。
		relayHandled := spec.RelayPathID != nil && *spec.RelayPathID > 0 && isRelayUsable(*spec.RelayPathID)
		// 1) spec 自身绑定的出口集（兜底段）
		if !relayHandled {
			if err := addSet(entry, spec.ExitNodeSetID); err != nil {
				return nil, err
			}
		}
		// 2) 固定出口（exit_agent_host_id）：与编译器 2b 段同条件——
		//    既无出口集、又无可用中继链路时才生效（否则该路径不参与渲染）。
		if !relayHandled && (spec.ExitNodeSetID == nil || *spec.ExitNodeSetID <= 0) {
			if spec.ExitAgentHostID != nil && *spec.ExitAgentHostID > 0 {
				addMember(entry, *spec.ExitAgentHostID, 0)
			}
		}
		// 3) 适用于该 spec 的分流策略（全局策略 + 绑该 spec 的 scoped 策略）。
		// 注意：编译器的策略段（第三步）**不受中继接管影响**（2-0 的 continue 只跳过该 spec 的
		// 出口集/固定出口），因此这里绝不能因 relayHandled 跳过——否则策略池渲染出 mark 却无配对。
		for _, p := range policiesByCore[coreType] {
			if p == nil || p.SpecID != nil && *p.SpecID != spec.ID {
				continue
			}
			if err := addSet(entry, p.TargetSetID); err != nil {
				return nil, err
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].EntryAgentID != out[j].EntryAgentID {
			return out[i].EntryAgentID < out[j].EntryAgentID
		}
		return out[i].MemberAgentID < out[j].MemberAgentID
	})
	return &desiredPairSet{keys: out, sets: setsOf}, nil
}

// containsInt64 判断切片是否已含该值（集合信息规模很小，线性足够）。
func containsInt64(items []int64, v int64) bool {
	for _, item := range items {
		if item == v {
			return true
		}
	}
	return false
}

// ReconcilePairs 回收不再期望的配对（保留仍被引用的，seq 稳定不变）。
func (s *egressDispatchService) ReconcilePairs(ctx context.Context) error {
	desired, err := s.desiredPairs(ctx)
	if err != nil {
		return err
	}
	removed, err := s.pairs.DeleteUnused(ctx, desired.keys)
	if err != nil {
		return fmt.Errorf("delete unused pairs: %w", err)
	}
	if removed > 0 {
		s.logger.Info("egress-dispatch: stale pairs removed", slog.Int64("count", removed))
	}
	return nil
}

// AssignmentsForHost 组装该主机的指派集合：
//   - 入口侧：本机期望模式为 l3 时，对每个期望配对分配/复用 pair 并生成 entry 指派；
//   - 出口侧：本机作为成员被 l3 入口使用时的镜像指派（自身模式无关）。
//
// 出口侧先建：agent 先具备隧道与 NAT，入口侧随后才可能生效（I12 顺序的语义保证）。
func (s *egressDispatchService) AssignmentsForHost(ctx context.Context, agentHostID int64) ([]EgressRouteAssignment, error) {
	peerIndex, err := s.meshPeerIndex(ctx)
	if err != nil {
		return nil, err
	}
	self, ok := peerIndex[agentHostID]
	if !ok || self.PrivateKey == "" || self.PublicKey == "" {
		return nil, nil // 不在 mesh 或无密钥 → 无法建隧道
	}

	host, err := s.agentHosts.FindByID(ctx, agentHostID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("load agent host: %w", err)
	}

	desiredSet, err := s.desiredPairs(ctx)
	if err != nil {
		return nil, err
	}
	desired := desiredSet.keys

	// 入口主机集合的期望模式（决定哪些配对值得建隧道）。
	entryMode := map[int64]string{agentHostID: s.desiredMode(ctx, host)}
	entryModeFor := func(entryID int64) (string, error) {
		if mode, ok := entryMode[entryID]; ok {
			return mode, nil
		}
		h, err := s.agentHosts.FindByID(ctx, entryID)
		if err != nil {
			return EgressDispatchModeSocks, nil
		}
		mode := s.desiredMode(ctx, h)
		entryMode[entryID] = mode
		return mode, nil
	}

	var out []EgressRouteAssignment
	for _, key := range desired {
		if key.EntryAgentID != agentHostID && key.MemberAgentID != agentHostID {
			continue
		}
		mode, err := entryModeFor(key.EntryAgentID)
		if err != nil {
			return nil, err
		}
		if mode != EgressDispatchModeL3 {
			continue // 入口未启用 l3 → 不建隧道
		}
		pair, err := s.pairs.EnsurePair(ctx, key.EntryAgentID, key.MemberAgentID)
		if err != nil {
			return nil, fmt.Errorf("ensure pair %d→%d: %w", key.EntryAgentID, key.MemberAgentID, err)
		}
		entryPeer, entryOK := peerIndex[key.EntryAgentID]
		memberPeer, memberOK := peerIndex[key.MemberAgentID]
		if !entryOK || !memberOK {
			continue
		}
		if key.MemberAgentID == agentHostID {
			out = append(out, buildExitAssignment(pair, entryPeer, self, desiredSet.sets[key]))
		}
		if key.EntryAgentID == agentHostID {
			out = append(out, buildEntryAssignment(pair, memberPeer, self, desiredSet.sets[key]))
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Role != out[j].Role {
			return out[i].Role < out[j].Role // entry 在前
		}
		return out[i].MemberAgentID < out[j].MemberAgentID
	})
	return out, nil
}

// buildEntryAssignment 入口侧：本端 .1，对端为成员。
func buildEntryAssignment(pair *repository.EgressDispatchPair, memberPeer, self meshPeerInfo, sets []int64) EgressRouteAssignment {
	mark := EgressMemberMark(pair.MemberAgentID)
	return EgressRouteAssignment{
		Role:          "entry",
		EntryAgentID:  pair.EntryAgentID,
		MemberAgentID: pair.MemberAgentID,
		Mark:          mark,
		Table:         EgressMemberTable(pair.MemberAgentID),
		Iface:         fmt.Sprintf("xe%d", pair.MemberAgentID),
		ListenPort:    pair.ListenPort,
		LocalAddr:     egressPairLocalAddr(pair.Seq, true),
		OwnPrivateKey: self.PrivateKey,
		PeerPublicKey: memberPeer.PublicKey,
		PeerEndpoint:  fmt.Sprintf("%s:%d", memberPeer.WGIP, pair.ListenPort),
		TunnelNet:     pair.LocalNet,
		MTU:           EgressTunnelMTU,
		Sets:          sets,
	}
}

// buildExitAssignment 出口侧：本端 .2，对端为入口。
func buildExitAssignment(pair *repository.EgressDispatchPair, entryPeer, self meshPeerInfo, sets []int64) EgressRouteAssignment {
	return EgressRouteAssignment{
		Role:          "exit",
		EntryAgentID:  pair.EntryAgentID,
		MemberAgentID: pair.MemberAgentID,
		Mark:          EgressMemberMark(pair.MemberAgentID),
		Table:         EgressMemberTable(pair.MemberAgentID),
		Iface:         fmt.Sprintf("xi%d", pair.EntryAgentID),
		ListenPort:    pair.ListenPort,
		LocalAddr:     egressPairLocalAddr(pair.Seq, false),
		OwnPrivateKey: self.PrivateKey,
		PeerPublicKey: entryPeer.PublicKey,
		PeerEndpoint:  fmt.Sprintf("%s:%d", entryPeer.WGIP, pair.ListenPort),
		TunnelNet:     pair.LocalNet,
		MTU:           EgressTunnelMTU,
		Sets:          sets,
	}
}

// egressPairLocalAddr 隧道本端地址（I4）：入口 .1，出口 .2。
func egressPairLocalAddr(seq int64, entry bool) string {
	host := 2
	if entry {
		host = 1
	}
	return fmt.Sprintf("10.220.%d.%d/%d", seq/64, (seq%64)*4+int64(host), 30)
}
