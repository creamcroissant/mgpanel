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
	Role          string `json:"role"`             // "entry" | "mid" | "exit"
	NextHopMeshIP string `json:"next_hop_mesh_ip"` // 兼容字段：entry 时为首跳 mesh IP

	// v2 独立隧道契约（entry/exit 必带；mid 不建隧道）
	IfaceName     string `json:"iface_name"`
	ListenPort    int    `json:"listen_port"`
	LocalAddr     string `json:"local_addr"`
	OwnPrivateKey string `json:"own_private_key"`
	PeerPublicKey string `json:"peer_public_key"`
	PeerEndpoint  string `json:"peer_endpoint"` // 对端 mesh WG IP:listenPort
	MeshCIDR      string `json:"mesh_cidr"`
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
		entry, exit := nodes[0], nodes[len(nodes)-1]

		// 出口跳（序列最后）：exit 角色 + 对端=入口的隧道
		if exit.AgentHostID == host.ID {
			r := RelayRouteAssignment{
				PathID: p.ID,
				Mark:   relayMarkBase + int(p.ID),
				Table:  relayTableBase + int(p.ID),
				Role:   "exit",
			}
			fillTunnel(&r, p.ID, exit, entry, wgIP, s.logger)
			out = append(out, r)
			continue
		}

		for i, n := range nodes {
			if n.AgentHostID != host.ID || i == len(nodes)-1 {
				continue // 非本机节点或出口已处理
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
				// v2: 入口隧道对端 = 直达出口（外层 UDP 经 mesh 骨干多跳转发）
				fillTunnel(&role, p.ID, entry, exit, wgIP, s.logger)
			} else if role.Role == "mid" {
				out = append(out, role)
				continue
			}
			// entry：出口/入口缺 mesh IP 或任一端缺钥则跳过（无法建隧道）
			if role.Role == "entry" && (nextIP == "" || role.PeerEndpoint == "" ||
				role.OwnPrivateKey == "" || role.PeerPublicKey == "") {
				s.logger.Warn("relay-route: skip tunnel assignment",
					slog.Int64("path_id", p.ID), slog.Int64("agent", n.AgentHostID))
				continue
			}
			if role.Role == "exit" && (role.PeerEndpoint == "" ||
				role.OwnPrivateKey == "" || role.PeerPublicKey == "") {
				s.logger.Warn("relay-route: skip exit tunnel assignment",
					slog.Int64("path_id", p.ID), slog.Int64("agent", n.AgentHostID))
				continue
			}
			out = append(out, role)
		}
	}
	if out == nil {
		out = []RelayRouteAssignment{}
	}
	return out, nil
}

// relayLocalAddr 入口取 .1、其余(出口)取 .2；网段按 path_id 取模避免重叠。
func relayLocalAddr(pathID int64, isEntry bool) string {
	host := 2
	if isEntry {
		host = 1
	}
	return fmt.Sprintf("10.200.%d.%d/30", pathID%250, host)
}

// fillTunnel 填充 v2 独立隧道字段：iface/listen_port/local_addr/密钥/对端 endpoint。
// self=本机节点，peer=隧道对端（entry↔exit）。对端无 mesh WG IP 时 PeerEndpoint 留空，
// 由调用方跳过该 assignment。
func fillTunnel(r *RelayRouteAssignment, pathID int64, self, peer repository.RelayPathNode, wgIP map[int64]string, logger *slog.Logger) {
	isEntry := r.Role == "entry"
	r.IfaceName = fmt.Sprintf("xr%d", pathID)
	r.ListenPort = 30000 + int(pathID)
	r.LocalAddr = relayLocalAddr(pathID, isEntry)
	r.MeshCIDR = "10.144.0.0/24"
	r.OwnPrivateKey = self.PrivateKey
	r.PeerPublicKey = peer.PublicKey
	peerIP := wgIP[peer.AgentHostID]
	if peerIP == "" {
		logger.Warn("relay-route: tunnel peer has no mesh WG IP",
			slog.Int64("path_id", pathID), slog.Int64("peer_agent", peer.AgentHostID))
		return // PeerEndpoint 留空 → 调用方跳过
	}
	r.PeerEndpoint = fmt.Sprintf("%s:%d", peerIP, r.ListenPort)
}
