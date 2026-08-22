package service

import (
	"math"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// RouteEntry represents one entry in a computed mesh routing table.
// Each entry describes the first-hop peer for traffic destined to a reachable node,
// ordered by shortest-path distance.
type RouteEntry struct {
	DestPeerID  string  `json:"dest_peer_id"`
	PeerID      string  `json:"peer_id"`
	Priority    int     `json:"priority"`
	PeerWGIP    string  `json:"peer_wg_ip"`
	PeerPort    int     `json:"peer_port"`
	LatencyMs   float64 `json:"latency_ms"`
	PacketLoss  float64 `json:"packet_loss"`
}

// MeshRouter computes SPF (Shortest Path First) routes from mesh latency probe data
// using Dijkstra's algorithm on a weighted undirected graph.
type MeshRouter struct {
	mu      sync.RWMutex
	tables  map[int64][]RouteEntry // agentHostID → priority-ordered routes
	lastRun time.Time
}

// NewMeshRouter creates a new MeshRouter with an empty route table cache.
func NewMeshRouter() *MeshRouter {
	return &MeshRouter{
		tables: make(map[int64][]RouteEntry),
	}
}

// GetRoutes returns the cached routing table for a given agent host.
// The boolean indicates whether routes exist for this host.
func (r *MeshRouter) GetRoutes(agentHostID int64) ([]RouteEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	routes, ok := r.tables[agentHostID]
	return routes, ok
}

// adjacency represents a weighted edge between two graph nodes.
type adjacency struct {
	to         int64
	weight     float64
	latencyMs  float64
	packetLoss float64
}

// Compute runs SPF (Dijkstra) on the mesh latency map and peer map, caches the
// resulting routing tables, and returns them.
//
// Algorithm:
//  1. Build an undirected weighted graph from latencyMap using weight formula:
//     latency_ms × (1 + packet_loss × 5)
//  2. For each source node, run Dijkstra to find shortest paths to all others.
//  3. For each reachable destination, record the first-hop peer.
//  4. Sort by total distance ascending, take top 5, assign priority 1-5.
func (r *MeshRouter) Compute(
	latencyMap map[int64]map[string]MeshPeerLatencyView,
	peerMap map[int64]repository.AgentMeshPeer,
) map[int64][]RouteEntry {
	// Step 0: Build peerID → agentHostID reverse lookup.
	// The gRPC handler sends peer ID as "agent-{hostID}" to agents, but
	// WGPublicKey is also supported for backward compatibility.
	reverseID := make(map[string]int64, len(peerMap)*2)
	for hostID, peer := range peerMap {
		reverseID["agent-"+strconv.FormatInt(hostID, 10)] = hostID
		if peer.WGPublicKey != "" {
			reverseID[peer.WGPublicKey] = hostID
		}
	}

	// Step 1: Build weighted undirected graph (adjacency list indexed by agentHostID).
	graph := make(map[int64][]adjacency)

	// Collect all known nodes (including those only appearing in peerMap).
	nodeSet := make(map[int64]bool)
	for hostID := range peerMap {
		nodeSet[hostID] = true
	}

	for srcID, peerLatencies := range latencyMap {
		nodeSet[srcID] = true
		for peerID, view := range peerLatencies {
			dstID, ok := reverseID[peerID]
			if !ok || srcID == dstID {
				continue
			}
			nodeSet[dstID] = true
			packetLoss := view.PacketLoss
			if packetLoss < 0 {
				packetLoss = 0
			} else if packetLoss > 1 {
				packetLoss = 1
			}
			weight := view.LatencyMs * (1 + packetLoss*5)
			if math.IsNaN(weight) || math.IsInf(weight, 0) || weight < 0 {
				weight = 0
			}
			edge := adjacency{
				to:         dstID,
				weight:     weight,
				latencyMs:  view.LatencyMs,
				packetLoss: view.PacketLoss,
			}
			// Undirected: add both directions.
			graph[srcID] = append(graph[srcID], edge)
			graph[dstID] = append(graph[dstID], adjacency{
				to:         srcID,
				weight:     weight,
				latencyMs:  view.LatencyMs,
				packetLoss: view.PacketLoss,
			})
		}
	}

	// Ensure every peerMap node has a graph entry (even if empty).
	for hostID := range peerMap {
		if _, ok := graph[hostID]; !ok {
			graph[hostID] = nil
		}
	}

	// Build a fast direct-link lookup: graphDirect[src][dst] → adjacency.
	// When multiple edges exist to the same dst (from bidirectional probes that
	// may differ), keep the lowest weight to match Dijkstra's min-weight path.
	graphDirect := make(map[int64]map[int64]adjacency, len(graph))
	for srcID, edges := range graph {
		m := make(map[int64]adjacency, len(edges))
		for _, e := range edges {
			existing, ok := m[e.to]
			if !ok || e.weight < existing.weight {
				m[e.to] = e
			}
		}
		graphDirect[srcID] = m
	}

	// Step 2: Run Dijkstra from each source node.
	tables := make(map[int64][]RouteEntry, len(nodeSet))

	for srcID := range nodeSet {
		if len(graph[srcID]) == 0 {
			tables[srcID] = []RouteEntry{}
			continue
		}

		const inf = math.MaxFloat64
		dist := make(map[int64]float64, len(nodeSet))
		firstHop := make(map[int64]int64, len(nodeSet)) // firstHop[dest] = immediate next hop from src

		for n := range nodeSet {
			dist[n] = inf
		}
		dist[srcID] = 0

		// Mark direct neighbors' first hop.
		for _, e := range graph[srcID] {
			firstHop[e.to] = e.to
		}

		// Simple O(V^2) Dijkstra — sufficient for typical mesh sizes (< 100 nodes).
		unvisited := make(map[int64]bool, len(nodeSet))
		for n := range nodeSet {
			unvisited[n] = true
		}

		for len(unvisited) > 0 {
			// Extract min-distance unvisited node.
			var u int64
			first := true
			for n := range unvisited {
				if first || dist[n] < dist[u] {
					u = n
				}
				first = false
			}
			if first {
				break
			}
			if dist[u] == inf {
				break
			}
			delete(unvisited, u)

			for _, e := range graph[u] {
				if !unvisited[e.to] {
					continue
				}
				alt := dist[u] + e.weight
				if alt < dist[e.to] {
					dist[e.to] = alt
					if u == srcID {
						firstHop[e.to] = e.to
					} else {
						firstHop[e.to] = firstHop[u]
					}
				}
			}
		}

		// Collect reachable destinations.
		type candidate struct {
			dstID    int64
			distance float64
		}
		var candidates []candidate
		for dstID := range nodeSet {
			if dstID == srcID || dist[dstID] == inf {
				continue
			}
			if _, ok := firstHop[dstID]; !ok {
				continue
			}
			candidates = append(candidates, candidate{dstID: dstID, distance: dist[dstID]})
		}

		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].distance < candidates[j].distance
		})

		// Take top 5, build RouteEntry with first-hop peer info and direct-link metrics.
		n := len(candidates)
		if n > 5 {
			n = 5
		}
		routes := make([]RouteEntry, 0, n)
		for i := 0; i < n; i++ {
			fh := firstHop[candidates[i].dstID]
			fhPeer, ok := peerMap[fh]
			if !ok {
				continue
			}
			// Check that the destination peer itself is still in peerMap
			candidateDest, ok := peerMap[candidates[i].dstID]
			if !ok {
				continue
			}
			// Direct-link metrics from src to first hop.
			direct, ok := graphDirect[srcID][fh]
			if !ok {
				continue
			}
			routes = append(routes, RouteEntry{
				DestPeerID:  candidateDest.WGPublicKey,
				PeerID:      fhPeer.WGPublicKey,
				Priority:    i + 1,
				PeerWGIP:    fhPeer.WGIP,
				PeerPort:    fhPeer.WGListenPort,
				LatencyMs:   direct.latencyMs,
				PacketLoss:  direct.packetLoss,
			})
		}
		tables[srcID] = routes
	}

	// Cache results.
	r.mu.Lock()
	r.tables = tables
	r.lastRun = time.Now()
	r.mu.Unlock()

	return tables
}
