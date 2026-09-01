package service

import (
	"context"
	"math"
	"strconv"
	"sync"
	"testing"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// ---------------------------------------------------------------------------
// fakeMeshPeerRepo — in-memory implementation of AgentMeshPeerRepository
// ---------------------------------------------------------------------------

type fakeMeshPeerRepo struct {
	mu    sync.Mutex
	peers map[int64]*repository.AgentMeshPeer // agentHostID -> peer
}

func (f *fakeMeshPeerRepo) Upsert(ctx context.Context, peer *repository.AgentMeshPeer) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.peers[peer.AgentHostID] = peer
	return nil
}

func (f *fakeMeshPeerRepo) FindByAgentHostID(ctx context.Context, agentHostID int64) (*repository.AgentMeshPeer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.peers[agentHostID]
	if !ok {
		return nil, nil
	}
	return p, nil
}

func (f *fakeMeshPeerRepo) ListByNetworkID(ctx context.Context, networkID string) ([]*repository.AgentMeshPeer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []*repository.AgentMeshPeer
	for _, p := range f.peers {
		if p.NetworkID == networkID {
			result = append(result, p)
		}
	}
	return result, nil
}

func (f *fakeMeshPeerRepo) Delete(ctx context.Context, agentHostID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.peers, agentHostID)
	return nil
}

// ---------------------------------------------------------------------------
// fakeAgentHostRepo — minimal fake for AgentHostRepository
// ---------------------------------------------------------------------------

type fakeAgentHostRepo struct{}

func (f *fakeAgentHostRepo) Create(ctx context.Context, host *repository.AgentHost) error             { return nil }
func (f *fakeAgentHostRepo) FindByID(ctx context.Context, id int64) (*repository.AgentHost, error)    { return nil, nil }
func (f *fakeAgentHostRepo) FindByHost(ctx context.Context, host string) (*repository.AgentHost, error) { return nil, nil }
func (f *fakeAgentHostRepo) FindByToken(ctx context.Context, token string) (*repository.AgentHost, error) { return nil, nil }
func (f *fakeAgentHostRepo) Update(ctx context.Context, host *repository.AgentHost) error              { return nil }
func (f *fakeAgentHostRepo) Delete(ctx context.Context, id int64) error                                 { return nil }
func (f *fakeAgentHostRepo) ListAll(ctx context.Context) ([]*repository.AgentHost, error)               { return nil, nil }
func (f *fakeAgentHostRepo) UpdateStatus(ctx context.Context, id int64, status int, heartbeatAt int64) error { return nil }
func (f *fakeAgentHostRepo) UpdateMetrics(ctx context.Context, id int64, metrics repository.AgentHostMetrics) error { return nil }
func (f *fakeAgentHostRepo) UpdateCapabilities(ctx context.Context, id int64, coreVersion string, capabilities, buildTags []string) error { return nil }
func (f *fakeAgentHostRepo) Count(ctx context.Context) (int64, error)                                  { return 0, nil }
func (f *fakeAgentHostRepo) CountOnline(ctx context.Context) (int64, error)                            { return 0, nil }

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func newTestMeshService() *agentMeshService {
	return &agentMeshService{
		peers:         &fakeMeshPeerRepo{peers: make(map[int64]*repository.AgentMeshPeer)},
		hosts:         &fakeAgentHostRepo{},
		listenPort:    51820,
		networkCIDR:   "10.144.0.0/24",
		peerLatencies: make(map[string]MeshPeerLatencyView),
		router:        NewMeshRouter(),
	}
}

// ---------------------------------------------------------------------------
// allocateIP tests
// ---------------------------------------------------------------------------

func TestAllocateIP_FirstAllocation(t *testing.T) {
	s := newTestMeshService()
	ip, err := s.allocateIP(context.Background(), "default")
	if err != nil {
		t.Fatalf("allocateIP() unexpected error: %v", err)
	}
	if ip != "10.144.0.1" {
		t.Fatalf("expected 10.144.0.1, got %s", ip)
	}
}

func TestAllocateIP_Sequential(t *testing.T) {
	s := newTestMeshService()
	ctx := context.Background()

	// Simulate 5 existing peers
	for i := int64(1); i <= 5; i++ {
		s.peers.Upsert(ctx, &repository.AgentMeshPeer{
			AgentHostID: i,
			WGIP:        "10.144.0." + string(rune('0'+i)),
			NetworkID:   "default",
		})
	}

	ip, err := s.allocateIP(ctx, "default")
	if err != nil {
		t.Fatalf("allocateIP() error: %v", err)
	}
	// The fake stores 10.144.0.1 .. 10.144.0.5, so next is 10.144.0.6
	if ip != "10.144.0.6" {
		t.Fatalf("expected 10.144.0.6, got %s", ip)
	}
}

func TestAllocateIP_FullNetwork(t *testing.T) {
	s := newTestMeshService()
	ctx := context.Background()

	// Fill all 254 usable IPs in /24
	for i := 1; i <= 254; i++ {
		ip := "10.144.0." + strconv.Itoa(i)
		s.peers.Upsert(ctx, &repository.AgentMeshPeer{
			AgentHostID: int64(i),
			WGIP:        ip,
			NetworkID:   "default",
		})
	}

	_, err := s.allocateIP(ctx, "default")
	if err == nil {
		t.Fatal("expected error when all IPs exhausted")
	}
}

func TestAllocateIP_WithGaps(t *testing.T) {
	s := newTestMeshService()
	ctx := context.Background()

	// Peer at .5, .10, .20
	for _, id := range []int64{5, 10, 20} {
		s.peers.Upsert(ctx, &repository.AgentMeshPeer{
			AgentHostID: id,
			WGIP:        "10.144.0." + strconv.Itoa(int(id)),
			NetworkID:   "default",
		})
	}

	ip, err := s.allocateIP(ctx, "default")
	if err != nil {
		t.Fatalf("allocateIP() error: %v", err)
	}
	// Should pick the first gap: 10.144.0.1
	if ip != "10.144.0.1" {
		t.Fatalf("expected 10.144.0.1, got %s", ip)
	}
}

func TestAllocateIP_InvalidCIDR(t *testing.T) {
	s := newTestMeshService()
	s.networkCIDR = "invalid-cidr"
	_, err := s.allocateIP(context.Background(), "default")
	if err == nil {
		t.Fatal("expected error for invalid CIDR")
	}
}

func TestAllocateIP_IPv6NotSupported(t *testing.T) {
	s := newTestMeshService()
	s.networkCIDR = "fd00::/8"
	_, err := s.allocateIP(context.Background(), "default")
	if err == nil {
		t.Fatal("expected error for IPv6")
	}
}

// ---------------------------------------------------------------------------
// ReportPeerLatency tests
// ---------------------------------------------------------------------------

func TestReportPeerLatency_NewEntry(t *testing.T) {
	s := newTestMeshService()
	ctx := context.Background()

	if err := s.ReportPeerLatency(ctx, 1, "agent-42", 10.0, 0.0, 5); err != nil {
		t.Fatal(err)
	}

	s.peerLatMu.RLock()
	lat, ok := s.peerLatencies["1:agent-42"]
	s.peerLatMu.RUnlock()
	if !ok {
		t.Fatal("latency entry not created")
	}
	if lat.LatencyMs != 10.0 {
		t.Fatalf("expected LatencyMs=10, got %f", lat.LatencyMs)
	}
	if lat.TotalProbes != 5 {
		t.Fatalf("expected TotalProbes=5, got %d", lat.TotalProbes)
	}
}

func TestReportPeerLatency_UpdateExisting(t *testing.T) {
	s := newTestMeshService()
	ctx := context.Background()

	s.ReportPeerLatency(ctx, 1, "peer1", 10.0, 0.0, 5)
	s.ReportPeerLatency(ctx, 1, "peer1", 20.0, 0.5, 8)

	s.peerLatMu.RLock()
	lat := s.peerLatencies["1:peer1"]
	s.peerLatMu.RUnlock()
	if lat.LatencyMs != 20.0 {
		t.Fatalf("expected updated LatencyMs=20, got %f", lat.LatencyMs)
	}
	if lat.PacketLoss != 0.5 {
		t.Fatalf("expected updated PacketLoss=0.5, got %f", lat.PacketLoss)
	}
	if lat.TotalProbes != 8 {
		t.Fatalf("expected updated TotalProbes=8, got %d", lat.TotalProbes)
	}
	if lat.PeerID != "peer1" {
		t.Fatalf("PeerID changed to %s", lat.PeerID)
	}
}

func TestReportPeerLatency_UniquePeers(t *testing.T) {
	s := newTestMeshService()
	ctx := context.Background()

	s.ReportPeerLatency(ctx, 1, "a", 5.0, 0, 1)
	s.ReportPeerLatency(ctx, 1, "b", 10.0, 0, 1)
	s.ReportPeerLatency(ctx, 1, "c", 15.0, 0, 1)

	s.peerLatMu.RLock()
	count := len(s.peerLatencies)
	s.peerLatMu.RUnlock()
	if count != 3 {
		t.Fatalf("expected 3 latency entries, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// GetPeerLatencies tests
// ---------------------------------------------------------------------------

func TestGetPeerLatencies_FiltersByNetwork(t *testing.T) {
	s := newTestMeshService()
	ctx := context.Background()

	// Add peers to the repository
	s.peers.Upsert(ctx, &repository.AgentMeshPeer{
		AgentHostID: 1, WGPublicKey: "key1", WGIP: "10.144.0.1", NetworkID: "default",
	})
	s.peers.Upsert(ctx, &repository.AgentMeshPeer{
		AgentHostID: 2, WGPublicKey: "key2", WGIP: "10.144.0.2", NetworkID: "default",
	})
	s.peers.Upsert(ctx, &repository.AgentMeshPeer{
		AgentHostID: 3, WGPublicKey: "key3", WGIP: "10.144.0.3", NetworkID: "other",
	})

	// Report latencies for both keys
	s.ReportPeerLatency(ctx, 1, "key1", 10.0, 0, 5)
	s.ReportPeerLatency(ctx, 1, "key2", 20.0, 0, 5)
	s.ReportPeerLatency(ctx, 2, "key3", 30.0, 0, 5)

	latencies, err := s.GetPeerLatencies(ctx, "default")
	if err != nil {
		t.Fatal(err)
	}
	if len(latencies) != 2 {
		t.Fatalf("expected 2 latencies for 'default' network, got %d", len(latencies))
	}
}

func TestGetPeerLatencies_EmptyNetwork(t *testing.T) {
	s := newTestMeshService()
	latencies, err := s.GetPeerLatencies(context.Background(), "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if len(latencies) != 0 {
		t.Fatalf("expected 0 latencies, got %d", len(latencies))
	}
}

// ---------------------------------------------------------------------------
// ComputeRoutingTables tests
// ---------------------------------------------------------------------------

func TestComputeRoutingTables_Empty(t *testing.T) {
	s := newTestMeshService()
	err := s.ComputeRoutingTables(context.Background())
	if err != nil {
		t.Fatalf("ComputeRoutingTables() error: %v", err)
	}
}


// ---------------------------------------------------------------------------
// NaN/Inf propagation protection test
// ---------------------------------------------------------------------------

func TestComputeRoutingTables_NanLatency(t *testing.T) {
	s := newTestMeshService()
	ctx := context.Background()

	s.peers.Upsert(ctx, &repository.AgentMeshPeer{
		AgentHostID: 1, WGPublicKey: "k1", WGIP: "10.144.0.1", NetworkID: "default", WGListenPort: 51820,
	})
	s.peers.Upsert(ctx, &repository.AgentMeshPeer{
		AgentHostID: 2, WGPublicKey: "k2", WGIP: "10.144.0.2", NetworkID: "default", WGListenPort: 51820,
	})

	// NaN latency must not corrupt Dijkstra
	s.ReportPeerLatency(ctx, 1, "k2", math.NaN(), 0, 5)
	s.ReportPeerLatency(ctx, 2, "k1", 10.0, 0, 5)

	if err := s.ComputeRoutingTables(ctx); err != nil {
		t.Fatal(err)
	}
	// Should not panic; routing tables computed gracefully
}


func TestComputeRoutingTables_TwoNodes(t *testing.T) {
	s := newTestMeshService()
	ctx := context.Background()

	s.peers.Upsert(ctx, &repository.AgentMeshPeer{
		AgentHostID: 1, WGPublicKey: "k1", WGIP: "10.144.0.1", NetworkID: "default", WGListenPort: 51820,
	})
	s.peers.Upsert(ctx, &repository.AgentMeshPeer{
		AgentHostID: 2, WGPublicKey: "k2", WGIP: "10.144.0.2", NetworkID: "default", WGListenPort: 51820,
	})

	// Feed the router directly
	peerMap := makePeerMap([]int64{1, 2}, []string{"10.144.0.1", "10.144.0.2"}, []string{"k1", "k2"})
	latencyMap := makeLatencyMap(
		[]int64{1, 2},
		[][]string{{"k2"}, {"k1"}},
		[][]MeshPeerLatencyView{
			{{PeerID: "k2", LatencyMs: 15, PacketLoss: 0, TotalProbes: 5}},
			{{PeerID: "k1", LatencyMs: 15, PacketLoss: 0, TotalProbes: 5}},
		},
	)
	s.router.Compute(latencyMap, peerMap)

	routes, err := s.GetAgentRoutes(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
	if routes[0].LatencyMs != 15.0 {
		t.Fatalf("expected LatencyMs=15, got %f", routes[0].LatencyMs)
	}
	if routes[0].PeerID != "k2" {
		t.Fatalf("expected PeerID=k2, got %s", routes[0].PeerID)
	}
}

func TestGetAgentRoutes_NoData(t *testing.T) {
	s := newTestMeshService()
	routes, err := s.GetAgentRoutes(context.Background(), 99)
	if err != nil {
		t.Fatal(err)
	}
	if routes != nil {
		t.Fatal("expected nil for unknown agent")
	}
}
