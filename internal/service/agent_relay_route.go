package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// RelayRouteAssignment 描述某台 agent 在中继链路中的内核路由角色
// （与 internal/agent/relayroute.Role 消费契约一致）。
type RelayRouteAssignment struct {
	PathID        int64  `json:"path_id"`
	Mark          int    `json:"mark"`
	Table         int    `json:"table"`
	Role          string `json:"role"`             // "entry" | "mid"
	NextHopMeshIP string `json:"next_hop_mesh_ip"` // entry 必填；mid 为空
}

// AgentRelayRouteService 按 host_token 回答"本机应生效哪些链路角色"。
type AgentRelayRouteService interface {
	RoutesForToken(ctx context.Context, token string) ([]RelayRouteAssignment, error)
}

type agentRelayRouteService struct {
	agentHosts repository.AgentHostRepository
	relayPaths repository.RelayPathRepository
	meshPeers  repository.AgentMeshPeerRepository
	logger     *slog.Logger
}

func NewAgentRelayRouteService(
	agentHosts repository.AgentHostRepository,
	relayPaths repository.RelayPathRepository,
	meshPeers repository.AgentMeshPeerRepository,
	logger *slog.Logger,
) AgentRelayRouteService {
	if logger == nil {
		logger = slog.Default()
	}
	return &agentRelayRouteService{
		agentHosts: agentHosts,
		relayPaths: relayPaths,
		meshPeers:  meshPeers,
		logger:     logger,
	}
}

const (
	relayMarkBase  = 40000 // fwmark = base + path_id
	relayTableBase = 100   // 路由表号 = base + path_id
	meshNetworkID  = "default"
)

// RoutesForToken 展开本机参与的 enabled 链路：
// hop0 → entry（下一跳 mesh IP），1..N-2 → mid，N-1(出口) 不返回。
func (s *agentRelayRouteService) RoutesForToken(ctx context.Context, token string) ([]RelayRouteAssignment, error) {
	host, err := s.agentHosts.FindByToken(ctx, token)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound // 翻译哨兵：handler 按 service.ErrNotFound 映射 401
		}
		return nil, fmt.Errorf("locate agent by token: %w", err)
	}
	paths, err := s.relayPaths.List(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("list relay paths: %w", err)
	}
	peers, err := s.meshPeers.ListByNetworkID(ctx, meshNetworkID)
	if err != nil {
		return nil, fmt.Errorf("list mesh peers: %w", err)
	}
	wgIP := make(map[int64]string, len(peers))
	for _, p := range peers {
		wgIP[p.AgentHostID] = p.WGIP
	}

	var out []RelayRouteAssignment
	for _, p := range paths {
		if !p.Enabled || len(p.Nodes) < 2 {
			continue
		}
		nodes := append([]repository.RelayPathNode(nil), p.Nodes...)
		sort.Slice(nodes, func(i, j int) bool { return nodes[i].Sequence < nodes[j].Sequence })
		for i, n := range nodes {
			if n.AgentHostID != host.ID || i == len(nodes)-1 {
				continue // 出口跳或非本机节点
			}
			next := nodes[i+1]
			nextIP := wgIP[next.AgentHostID]
			role := RelayRouteAssignment{
				PathID: p.ID,
				Mark:   relayMarkBase + int(p.ID),
				Table:  relayTableBase + int(p.ID),
				Role:   "mid",
			}
			if i == 0 {
				role.Role = "entry"
				role.NextHopMeshIP = nextIP
			}
			if role.Role == "entry" && nextIP == "" {
				s.logger.Warn("relay-route: next hop has no mesh WG IP, skipping assignment",
					slog.Int64("path_id", p.ID), slog.Int64("next_agent_host_id", next.AgentHostID))
				continue
			}
			out = append(out, role)
		}
	}
	if out == nil {
		out = []RelayRouteAssignment{}
	}
	// 第二遍：为出口跳（序列最后）添加 exit 角色，触发出口侧转发+NAT 基础设施
	for _, p := range paths {
		if !p.Enabled || len(p.Nodes) < 2 {
			continue
		}
		nodes := append([]repository.RelayPathNode(nil), p.Nodes...)
		sort.Slice(nodes, func(i, j int) bool { return nodes[i].Sequence < nodes[j].Sequence })
		last := nodes[len(nodes)-1]
		if last.AgentHostID == host.ID {
			// 检查是否已有 entry/mid 角色复用（避免重复）
			dup := false
			for _, r := range out {
				if r.PathID == p.ID {
					dup = true
					break
				}
			}
			if !dup {
				out = append(out, RelayRouteAssignment{
					PathID: p.ID,
					Mark:   relayMarkBase + int(p.ID),
					Table:  relayTableBase + int(p.ID),
					Role:   "exit",
				})
			}
		}
	}
	return out, nil
}
