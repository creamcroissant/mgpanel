package service

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/creamcroissant/xboard/internal/repository"
	"github.com/creamcroissant/xboard/internal/service/meshkey"
)

// AgentMeshService manages WireGuard mesh network peers.
type AgentMeshService interface {
	// JoinNetwork adds an agent to the mesh network, allocating a WG IP and generating keys.
	JoinNetwork(ctx context.Context, agentHostID int64, networkID string) (string, string, error)
	// LeaveNetwork removes an agent from the mesh network.
	LeaveNetwork(ctx context.Context, agentHostID int64) error
	// GetMeshPeer returns the mesh peer record for a specific agent.
	GetMeshPeer(ctx context.Context, agentHostID int64) (*repository.AgentMeshPeer, error)
	// ListNetworkPeers returns all peers in a mesh network.
	ListNetworkPeers(ctx context.Context, networkID string) ([]*repository.AgentMeshPeer, error)
	// ReportPeerLatency records a latency probe result for a mesh peer.
	ReportPeerLatency(ctx context.Context, srcAgentID int64, peerID string, latencyMs, packetLoss float64, totalProbes int) error
	// GetPeerLatencies returns all stored mesh peer latency results for a network.
	GetPeerLatencies(ctx context.Context, networkID string) ([]MeshPeerLatencyView, error)
	// ComputeRoutingTables runs SPF on all known agents' latency data and caches routing tables.
	ComputeRoutingTables(ctx context.Context) error
	// GetAgentRoutes returns the cached routing table for a specific agent host.
	GetAgentRoutes(ctx context.Context, agentHostID int64) ([]RouteEntry, error)
}

// MeshPeerLatencyView represents a mesh peer latency probe result for API consumption.
type MeshPeerLatencyView struct {
	PeerID      string  `json:"peer_id"`
	Endpoint    string  `json:"endpoint,omitempty"`
	LatencyMs   float64 `json:"latency_ms"`
	PacketLoss  float64 `json:"packet_loss"`
	TotalProbes int     `json:"total_probes"`
	UpdatedAt   int64   `json:"updated_at"`
}

type agentMeshService struct {
	peers         repository.AgentMeshPeerRepository
	hosts         repository.AgentHostRepository
	listenPort    int
	networkCIDR   string
	peerLatencies map[string]MeshPeerLatencyView
	peerLatMu     sync.RWMutex
	router        *MeshRouter
	lifecycleOps  AgentLifecycleOperationService
}

// NewAgentMeshService creates a new AgentMeshService.
func NewAgentMeshService(peers repository.AgentMeshPeerRepository, hosts repository.AgentHostRepository, lifecycleOps AgentLifecycleOperationService) AgentMeshService {
	return &agentMeshService{
		peers:         peers,
		hosts:         hosts,
		listenPort:    51820,
		networkCIDR:   "10.144.0.0/24",
		peerLatencies: make(map[string]MeshPeerLatencyView),
		router:        NewMeshRouter(),
		lifecycleOps:  lifecycleOps,
	}
}

func (s *agentMeshService) JoinNetwork(ctx context.Context, agentHostID int64, networkID string) (string, string, error) {
	existing, err := s.peers.FindByAgentHostID(ctx, agentHostID)
	if err != nil {
		return "", "", err
	}
	if existing != nil {
		return existing.WGIP, existing.WGPublicKey, nil
	}

	privKey, pubKey, err := meshkey.GenerateKeyPair()
	if err != nil {
		return "", "", fmt.Errorf("generate wg keypair: %w", err)
	}

	ip, err := s.allocateIP(ctx, networkID)
	if err != nil {
		return "", "", fmt.Errorf("allocate ip: %w", err)
	}

	peer := &repository.AgentMeshPeer{
		AgentHostID:  agentHostID,
		WGPrivateKey: privKey,
		WGPublicKey:  pubKey,
		WGIP:         ip,
		WGListenPort: s.listenPort,
		NetworkID:    networkID,
	}
	if err := s.peers.Upsert(ctx, peer); err != nil {
		return "", "", err
	}
	return ip, pubKey, nil
}

func (s *agentMeshService) LeaveNetwork(ctx context.Context, agentHostID int64) error {
	// Remove from DB
	if err := s.peers.Delete(ctx, agentHostID); err != nil {
		return err
	}
	// Clean up peer latency entries for this source
	s.peerLatMu.Lock()
	prefix := fmt.Sprintf("%d:", agentHostID)
	for key := range s.peerLatencies {
		if strings.HasPrefix(key, prefix) {
			delete(s.peerLatencies, key)
		}
	}
	s.peerLatMu.Unlock()
	return nil
}

func (s *agentMeshService) GetMeshPeer(ctx context.Context, agentHostID int64) (*repository.AgentMeshPeer, error) {
	return s.peers.FindByAgentHostID(ctx, agentHostID)
}

func (s *agentMeshService) ListNetworkPeers(ctx context.Context, networkID string) ([]*repository.AgentMeshPeer, error) {
	return s.peers.ListByNetworkID(ctx, networkID)
}

func (s *agentMeshService) ReportPeerLatency(ctx context.Context, srcAgentID int64, peerID string, latencyMs, packetLoss float64, totalProbes int) error {
	s.peerLatMu.Lock()
	defer s.peerLatMu.Unlock()
	key := fmt.Sprintf("%d:%s", srcAgentID, peerID)
	existing, ok := s.peerLatencies[key]
	if ok {
		existing.LatencyMs = latencyMs
		existing.PacketLoss = packetLoss
		existing.TotalProbes = totalProbes
		existing.UpdatedAt = time.Now().Unix()
		s.peerLatencies[key] = existing
	} else {
		s.peerLatencies[key] = MeshPeerLatencyView{
			PeerID:      peerID,
			LatencyMs:   latencyMs,
			PacketLoss:  packetLoss,
			TotalProbes: totalProbes,
			UpdatedAt:   time.Now().Unix(),
		}
	}
	return nil
}

func (s *agentMeshService) GetPeerLatencies(ctx context.Context, networkID string) ([]MeshPeerLatencyView, error) {
	// Snapshot latency data under short RLock, release before DB query
	s.peerLatMu.RLock()
	snapshot := make([]MeshPeerLatencyView, 0, len(s.peerLatencies))
	for _, v := range s.peerLatencies {
		snapshot = append(snapshot, v)
	}
	s.peerLatMu.RUnlock()

	peers, err := s.peers.ListByNetworkID(ctx, networkID)
	if err != nil {
		return nil, err
	}
	peerSet := make(map[string]bool, len(peers))
	for _, p := range peers {
		peerSet[p.WGPublicKey] = true
	}
	result := make([]MeshPeerLatencyView, 0, len(snapshot))
	for _, v := range snapshot {
		if peerSet[v.PeerID] {
			result = append(result, v)
		}
	}
	return result, nil
}

// allocateIP finds the next available IP in the mesh network CIDR.
func (s *agentMeshService) allocateIP(ctx context.Context, networkID string) (string, error) {
	_, cidr, err := net.ParseCIDR(s.networkCIDR)
	if err != nil {
		return "", fmt.Errorf("parse cidr %s: %w", s.networkCIDR, err)
	}
	ones, bits := cidr.Mask.Size()
	base := cidr.IP.To4()
	if base == nil {
		return "", fmt.Errorf("only IPv4 supported for mesh")
	}

	peers, err := s.peers.ListByNetworkID(ctx, networkID)
	if err != nil {
		return "", err
	}

	used := make(map[string]bool)
	for _, p := range peers {
		used[p.WGIP] = true
	}

	totalHosts := (1 << uint(bits-ones)) - 2 // exclude network and broadcast
	for i := 1; i <= totalHosts; i++ {
		ip := make(net.IP, 4)
		copy(ip, base)
		offset := i
		ip[3] = base[3] + byte(offset%256)
		if offset >= 256 {
			ip[2] = base[2] + byte(offset/256)
		}
		s := ip.String()
		if !used[s] {
			return s, nil
		}
	}
	return "", fmt.Errorf("no available IP in %s", s.networkCIDR)
}

func (s *agentMeshService) ComputeRoutingTables(ctx context.Context) error {
	s.peerLatMu.Lock()
	defer s.peerLatMu.Unlock()

	peers, err := s.peers.ListByNetworkID(ctx, "default")
	if err != nil {
		return err
	}

	peerMap := make(map[int64]repository.AgentMeshPeer)
	for _, p := range peers {
		peerMap[p.AgentHostID] = *p
	}

	latencyMap := make(map[int64]map[string]MeshPeerLatencyView)
	for _, p := range peers {
		latencyMap[p.AgentHostID] = make(map[string]MeshPeerLatencyView)
	}

	// Track which keys are used for pruning
	usedKeys := make(map[string]bool, len(s.peerLatencies))

	for key, lat := range s.peerLatencies {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 {
			continue
		}
		srcID, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			continue
		}
		// Skip latencies from departed nodes not in the current peer set
		if _, ok := peerMap[srcID]; !ok {
			continue
		}

		peerID := parts[1]

		for _, p := range peers {
			expectedAgentID := "agent-" + strconv.FormatInt(p.AgentHostID, 10)
			if p.WGPublicKey == peerID || expectedAgentID == peerID {
				if _, ok := latencyMap[srcID]; !ok {
					latencyMap[srcID] = make(map[string]MeshPeerLatencyView)
				}
				latencyMap[srcID][peerID] = lat
				usedKeys[key] = true
				break
			}
		}
	}

	// Prune unused entries (departed agents)
	for key := range s.peerLatencies {
		if !usedKeys[key] {
			delete(s.peerLatencies, key)
		}
	}

	s.router.Compute(latencyMap, peerMap)
	return nil
}

func (s *agentMeshService) GetAgentRoutes(ctx context.Context, agentHostID int64) ([]RouteEntry, error) {
	s.peerLatMu.RLock()
	defer s.peerLatMu.RUnlock()
	routes, _ := s.router.GetRoutes(agentHostID)
	return routes, nil
}
