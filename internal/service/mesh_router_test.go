package service

import (
	"testing"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

func makePeerMap(ids []int64, wgIPs []string, pubKeys []string) map[int64]repository.AgentMeshPeer {
	m := make(map[int64]repository.AgentMeshPeer, len(ids))
	for i, id := range ids {
		pubKey := ""
		if i < len(pubKeys) {
			pubKey = pubKeys[i]
		}
		wgIP := "10.144.0.x"
		if i < len(wgIPs) {
			wgIP = wgIPs[i]
		}
		m[id] = repository.AgentMeshPeer{
			AgentHostID:  id,
			WGPublicKey:  pubKey,
			WGIP:         wgIP,
			WGListenPort: 51820,
		}
	}
	return m
}

func makeLatencyMap(srcIDs []int64, peerIDKeys [][]string, latencies [][]MeshPeerLatencyView) map[int64]map[string]MeshPeerLatencyView {
	m := make(map[int64]map[string]MeshPeerLatencyView, len(srcIDs))
	for i, src := range srcIDs {
		inner := make(map[string]MeshPeerLatencyView, len(peerIDKeys[i]))
		for j, key := range peerIDKeys[i] {
			inner[key] = latencies[i][j]
		}
		m[src] = inner
	}
	return m
}

// ---------- Tests ----------

func TestMeshRouter_Empty(t *testing.T) {
	r := NewMeshRouter()
	result := r.Compute(nil, nil)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(result))
	}

	routes, ok := r.GetRoutes(1)
	if ok {
		t.Fatal("expected false for unknown host")
	}
	if routes != nil {
		t.Fatal("expected nil routes for unknown host")
	}
}

func TestMeshRouter_SingleNode(t *testing.T) {
	r := NewMeshRouter()
	peerMap := makePeerMap([]int64{1}, []string{"10.144.0.1"}, []string{"key1"})
	latencyMap := makeLatencyMap(
		[]int64{1},
		[][]string{{}},
		[][]MeshPeerLatencyView{{}},
	)

	result := r.Compute(latencyMap, peerMap)
	routes, ok := result[1]
	if !ok {
		t.Fatal("expected entry for host 1")
	}
	if len(routes) != 0 {
		t.Fatalf("expected 0 routes for single node, got %d", len(routes))
	}
}

func TestMeshRouter_ThreeNode(t *testing.T) {
	r := NewMeshRouter()
	peerMap := makePeerMap(
		[]int64{1, 2, 3},
		[]string{"10.144.0.1", "10.144.0.2", "10.144.0.3"},
		[]string{"key1", "key2", "key3"},
	)
	latencyMap := makeLatencyMap(
		[]int64{1, 2, 3},
		[][]string{
			{"key2", "key3"},
			{"key1", "key3"},
			{"key1", "key2"},
		},
		[][]MeshPeerLatencyView{
			{
				{PeerID: "key2", LatencyMs: 10, PacketLoss: 0, TotalProbes: 5},
				{PeerID: "key3", LatencyMs: 100, PacketLoss: 0, TotalProbes: 5},
			},
			{
				{PeerID: "key1", LatencyMs: 10, PacketLoss: 0, TotalProbes: 5},
				{PeerID: "key3", LatencyMs: 10, PacketLoss: 0, TotalProbes: 5},
			},
			{
				{PeerID: "key1", LatencyMs: 100, PacketLoss: 0, TotalProbes: 5},
				{PeerID: "key2", LatencyMs: 10, PacketLoss: 0, TotalProbes: 5},
			},
		},
	)

	result := r.Compute(latencyMap, peerMap)

	// Node 1 should prefer node 2 as first hop to all destinations (2 is the hub).
	t.Run("node1_routes", func(t *testing.T) {
		routes, ok := result[1]
		if !ok {
			t.Fatal("expected routes for host 1")
		}
		if len(routes) != 2 {
			t.Fatalf("expected 2 routes, got %d", len(routes))
		}
		// Priority 1 should be node 2 (direct, 10ms)
		if routes[0].Priority != 1 {
			t.Fatalf("expected priority 1, got %d", routes[0].Priority)
		}
		if routes[0].PeerID != "key2" {
			t.Fatalf("expected first hop key2, got %s", routes[0].PeerID)
		}
		if routes[0].PeerWGIP != "10.144.0.2" {
			t.Fatalf("expected WGIP 10.144.0.2, got %s", routes[0].PeerWGIP)
		}
		if routes[0].LatencyMs != 10 {
			t.Fatalf("expected latency 10, got %f", routes[0].LatencyMs)
		}
		// Priority 2 should be node 3 (via 2, 20ms)
		if len(routes) >= 2 {
			if routes[1].Priority != 2 {
				t.Fatalf("expected priority 2, got %d", routes[1].Priority)
			}
			if routes[1].PeerWGIP != "10.144.0.2" && routes[1].PeerWGIP != "10.144.0.3" {
				t.Fatalf("unexpected WGIP for route 2: %s", routes[1].PeerWGIP)
			}
		}
	})
}

func TestMeshRouter_AsymmetricLatency(t *testing.T) {
	r := NewMeshRouter()
	peerMap := makePeerMap(
		[]int64{1, 2, 3},
		[]string{"10.144.0.1", "10.144.0.2", "10.144.0.3"},
		[]string{"key1", "key2", "key3"},
	)
	// Node 2 → 3 is fast, 1 → 3 is slow, 1 → 2 is medium.
	latencyMap := makeLatencyMap(
		[]int64{1, 2, 3},
		[][]string{
			{"key2", "key3"},
			{"key1", "key3"},
			{"key1", "key2"},
		},
		[][]MeshPeerLatencyView{
			{
				{PeerID: "key2", LatencyMs: 30, PacketLoss: 0, TotalProbes: 5},
				{PeerID: "key3", LatencyMs: 100, PacketLoss: 0, TotalProbes: 5},
			},
			{
				{PeerID: "key1", LatencyMs: 30, PacketLoss: 0, TotalProbes: 5},
				{PeerID: "key3", LatencyMs: 10, PacketLoss: 0, TotalProbes: 5},
			},
			{
				{PeerID: "key1", LatencyMs: 100, PacketLoss: 0, TotalProbes: 5},
				{PeerID: "key2", LatencyMs: 10, PacketLoss: 0, TotalProbes: 5},
			},
		},
	)

	result := r.Compute(latencyMap, peerMap)

	routes := result[1]
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes for node 1, got %d", len(routes))
	}
	// Priority 1: direct to node2 (30ms)
	if routes[0].PeerID != "key2" {
		t.Fatalf("expected first hop key2, got %s", routes[0].PeerID)
	}
	// Priority 2: to node3 via node2 (30+10=40ms vs direct 100ms)
	if routes[1].PeerID != "key2" {
		t.Fatalf("expected second hop also via key2 (key3 via key2), got %s", routes[1].PeerID)
	}
}

func TestMeshRouter_PacketLossPenalty(t *testing.T) {
	r := NewMeshRouter()
	peerMap := makePeerMap(
		[]int64{1, 2, 3},
		[]string{"10.144.0.1", "10.144.0.2", "10.144.0.3"},
		[]string{"key1", "key2", "key3"},
	)
	// Node 1 → 2: low latency but high packet loss
	// Node 1 → 3: higher latency but no loss
	latencyMap := makeLatencyMap(
		[]int64{1, 2, 3},
		[][]string{
			{"key2", "key3"},
			{"key1", "key3"},
			{"key1", "key2"},
		},
		[][]MeshPeerLatencyView{
			{
				{PeerID: "key2", LatencyMs: 10, PacketLoss: 0.5, TotalProbes: 10},
				{PeerID: "key3", LatencyMs: 30, PacketLoss: 0, TotalProbes: 10},
			},
			{
				{PeerID: "key1", LatencyMs: 10, PacketLoss: 0.5, TotalProbes: 10},
				{PeerID: "key3", LatencyMs: 20, PacketLoss: 0, TotalProbes: 10},
			},
			{
				{PeerID: "key1", LatencyMs: 30, PacketLoss: 0, TotalProbes: 10},
				{PeerID: "key2", LatencyMs: 20, PacketLoss: 0, TotalProbes: 10},
			},
		},
	)

	result := r.Compute(latencyMap, peerMap)

	// Weights: 1→2 = 10*(1+2.5) = 35, 1→3 = 30*(1+0) = 30
	// So node 3 should be priority 1
	routes := result[1]
	if len(routes) == 0 {
		t.Fatal("expected routes for node 1")
	}
	if routes[0].PeerID != "key3" {
		t.Fatalf("expected priority 1 to be key3 (30ms no loss > 10ms 50%% loss), got %s",
			routes[0].PeerID)
	}
}

func TestMeshRouter_DisconnectedNode(t *testing.T) {
	r := NewMeshRouter()
	peerMap := makePeerMap(
		[]int64{1, 2, 3},
		[]string{"10.144.0.1", "10.144.0.2", "10.144.0.3"},
		[]string{"key1", "key2", "key3"},
	)
	// Node 3 has no latency data (disconnected)
	latencyMap := makeLatencyMap(
		[]int64{1, 2},
		[][]string{
			{"key2"},
			{"key1"},
		},
		[][]MeshPeerLatencyView{
			{
				{PeerID: "key2", LatencyMs: 10, PacketLoss: 0, TotalProbes: 5},
			},
			{
				{PeerID: "key1", LatencyMs: 10, PacketLoss: 0, TotalProbes: 5},
			},
		},
	)

	result := r.Compute(latencyMap, peerMap)

	// Node 1 should have 1 route (to node 2)
	routes := result[1]
	if len(routes) != 1 {
		t.Fatalf("expected 1 route for node 1, got %d", len(routes))
	}
	if routes[0].PeerID != "key2" {
		t.Fatalf("expected route to key2, got %s", routes[0].PeerID)
	}

	// Node 3 has no latency data, but may still have empty route list
	routes3 := result[3]
	if routes3 == nil {
		t.Fatal("expected non-nil (maybe empty) routes for node 3")
	}
	if len(routes3) != 0 {
		t.Fatalf("expected 0 routes for disconnected node 3, got %d", len(routes3))
	}
}

func TestMeshRouter_FiveNodeTopology(t *testing.T) {
	r := NewMeshRouter()
	peerMap := makePeerMap(
		[]int64{1, 2, 3, 4, 5},
		[]string{"10.144.0.1", "10.144.0.2", "10.144.0.3", "10.144.0.4", "10.144.0.5"},
		[]string{"k1", "k2", "k3", "k4", "k5"},
	)

	// Star topology: node 1 connects to all, others only connect to node 1
	latencyMap := makeLatencyMap(
		[]int64{1, 2, 3, 4, 5},
		[][]string{
			{"k2", "k3", "k4", "k5"},
			{"k1"},
			{"k1"},
			{"k1"},
			{"k1"},
		},
		[][]MeshPeerLatencyView{
			{
				{PeerID: "k2", LatencyMs: 10, PacketLoss: 0, TotalProbes: 5},
				{PeerID: "k3", LatencyMs: 20, PacketLoss: 0, TotalProbes: 5},
				{PeerID: "k4", LatencyMs: 30, PacketLoss: 0, TotalProbes: 5},
				{PeerID: "k5", LatencyMs: 40, PacketLoss: 0, TotalProbes: 5},
			},
			{
				{PeerID: "k1", LatencyMs: 10, PacketLoss: 0, TotalProbes: 5},
			},
			{
				{PeerID: "k1", LatencyMs: 20, PacketLoss: 0, TotalProbes: 5},
			},
			{
				{PeerID: "k1", LatencyMs: 30, PacketLoss: 0, TotalProbes: 5},
			},
			{
				{PeerID: "k1", LatencyMs: 40, PacketLoss: 0, TotalProbes: 5},
			},
		},
	)

	result := r.Compute(latencyMap, peerMap)

	// Node 1 should have 4 routes, top 5 cap makes it 4
	t.Run("node1_four_routes", func(t *testing.T) {
		routes := result[1]
		if len(routes) != 4 {
			t.Fatalf("expected 4 routes for node 1, got %d", len(routes))
		}
		// Verify ordering: k2(10), k3(20), k4(30), k5(40)
		expected := []struct {
			peerID string
			prio   int
			latMs  float64
		}{
			{"k2", 1, 10},
			{"k3", 2, 20},
			{"k4", 3, 30},
			{"k5", 4, 40},
		}
		for i, exp := range expected {
			if routes[i].Priority != exp.prio {
				t.Errorf("route %d: expected priority %d, got %d", i, exp.prio, routes[i].Priority)
			}
			if routes[i].PeerID != exp.peerID {
				t.Errorf("route %d: expected peer %s, got %s", i, exp.peerID, routes[i].PeerID)
			}
			if routes[i].LatencyMs != exp.latMs {
				t.Errorf("route %d: expected latency %f, got %f", i, exp.latMs, routes[i].LatencyMs)
			}
		}
	})

	// Node 2 should have 4 routes (to 1,3,4,5 all via first-hop 1)
	t.Run("node2_four_routes", func(t *testing.T) {
		routes := result[2]
		if len(routes) != 4 {
			t.Fatalf("expected 4 routes for node 2, got %d", len(routes))
		}
		// Priority 1 should be node 1 (direct)
		if routes[0].PeerWGIP != "10.144.0.1" {
			t.Fatalf("expected first hop 10.144.0.1, got %s", routes[0].PeerWGIP)
		}
	})
}

func TestMeshRouter_Top5Cap(t *testing.T) {
	r := NewMeshRouter()
	// 7 nodes all connected to each other
	ids := []int64{1, 2, 3, 4, 5, 6, 7}
	wgIPs := make([]string, 7)
	keys := make([]string, 7)
	for i := range ids {
		wgIPs[i] = "10.144.0." + string(rune('0'+i+1))
		keys[i] = "key" + string(rune('0'+i+1))
	}

	peerMap := makePeerMap(ids, wgIPs, keys)

	// Full mesh: all nodes know each other
	latencyMap := make(map[int64]map[string]MeshPeerLatencyView)
	for _, src := range ids {
		inner := make(map[string]MeshPeerLatencyView)
		for _, dst := range ids {
			if src == dst {
				continue
			}
			inner["key"+string(rune('0'+dst+1))] = MeshPeerLatencyView{
				PeerID: "key" + string(rune('0'+dst+1)), LatencyMs: float64(src*10 + dst),
				PacketLoss: 0, TotalProbes: 5,
			}
		}
		latencyMap[src] = inner
	}

	result := r.Compute(latencyMap, peerMap)

	// Each node should have at most 5 routes
	for id, routes := range result {
		if len(routes) > 5 {
			t.Errorf("node %d has %d routes, expected ≤5", id, len(routes))
		}
		for i, rte := range routes {
			if rte.Priority != i+1 {
				t.Errorf("node %d route %d: expected priority %d, got %d", id, i, i+1, rte.Priority)
			}
		}
	}
}

func TestMeshRouter_GetRoutesEmpty(t *testing.T) {
	r := NewMeshRouter()
	routes, ok := r.GetRoutes(99)
	if ok {
		t.Fatal("expected ok=false for non-existent host")
	}
	if routes != nil {
		t.Fatal("expected nil routes")
	}
}

func TestMeshRouter_GetRoutesAfterCompute(t *testing.T) {
	r := NewMeshRouter()
	peerMap := makePeerMap(
		[]int64{1, 2},
		[]string{"10.144.0.1", "10.144.0.2"},
		[]string{"a", "b"},
	)
	latencyMap := makeLatencyMap(
		[]int64{1, 2},
		[][]string{{"b"}, {"a"}},
		[][]MeshPeerLatencyView{
			{{PeerID: "b", LatencyMs: 15, PacketLoss: 0, TotalProbes: 5}},
			{{PeerID: "a", LatencyMs: 15, PacketLoss: 0, TotalProbes: 5}},
		},
	)

	_ = r.Compute(latencyMap, peerMap)

	routes, ok := r.GetRoutes(1)
	if !ok {
		t.Fatal("expected ok=true after compute")
	}
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
	if routes[0].LatencyMs != 15 {
		t.Fatalf("expected latency 15, got %f", routes[0].LatencyMs)
	}
}
