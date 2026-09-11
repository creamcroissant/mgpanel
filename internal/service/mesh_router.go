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
	DestPeerID string  `json:"dest_peer_id"`
	PeerID     string  `json:"peer_id"`
	Priority   int     `json:"priority"`
	PeerWGIP   string  `json:"peer_wg_ip"`
	PeerPort   int     `json:"peer_port"`
	LatencyMs  float64 `json:"latency_ms"`
	PacketLoss float64 `json:"packet_loss"`
}

// meshLatencyStaleSeconds 探测值新鲜度阈值:超过该时长未收到有效上报的边
// 视为过期,不进路由图(真断链 peer 在迟滞 + 过期双重机制下从图中退出)。
const meshLatencyStaleSeconds = 300

// routeHysteresisRatio 排序滞回阈值:相邻候选可达距离的相对差在此以内时,
// 保持上一轮优先级顺序,避免探测平均值微波动引发 priority 互换与无谓下发。
const routeHysteresisRatio = 0.20

// nearlyEqualDistance 判定两个可达距离是否"接近"(相对差 ≤ routeHysteresisRatio)。
func nearlyEqualDistance(a, b float64) bool {
	if a == b {
		return true
	}
	maxDist := a
	if b > maxDist {
		maxDist = b
	}
	if maxDist <= 0 {
		return true
	}
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff/maxDist <= routeHysteresisRatio
}

// MeshRouter computes SPF (Shortest Path First) routes from mesh latency probe data
// using Dijkstra's algorithm on a weighted undirected graph.
type MeshRouter struct {
	mu      sync.RWMutex
	tables  map[int64][]RouteEntry // agentHostID → priority-ordered routes
	lastRun time.Time

	// 路径滞回状态:上一轮每个 (src → dest) 的首跳与路径距离,
	// 用于抑制等价路径之间因延迟微波动产生的首跳抖动。
	lastHop  map[int64]map[int64]int64
	lastDist map[int64]map[int64]float64
}

// NewMeshRouter creates a new MeshRouter with an empty route table cache.
func NewMeshRouter() *MeshRouter {
	return &MeshRouter{
		tables:   make(map[int64][]RouteEntry),
		lastHop:  make(map[int64]map[int64]int64),
		lastDist: make(map[int64]map[int64]float64),
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
	//
	// 同时快照上一轮结果与路径滞回状态(见下)。
	r.mu.RLock()
	prevTables := r.tables
	prevHopMap := r.lastHop
	prevDistMap := r.lastDist
	r.mu.RUnlock()

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
			// 边有效性:探测失败(raw ICMP 超时)上报 (latency=0, loss=1),
			// 若按 weight=latency×(1+loss×5) 计算会得到 0,把失败链路当成
			// "零代价免费边",Dijkstra 全图翻转 → 每 60s 整表路由抖动
			// (2026-09-10 生产排查:set_routing_table 每日 8600+ 次全量下发)。
			// 因此:无有效延迟/100% 丢包的边一律不进图(走其它可达路径);
			// 超过新鲜度阈值的旧值同样剔除(配合 ReportPeerLatency 迟滞,
			// 真断链的 peer 在阈值后从图中消失)。
			if !meshLatencyReportValid(view.LatencyMs, view.PacketLoss) {
				continue
			}
			if view.UpdatedAt > 0 && time.Now().Unix()-view.UpdatedAt > meshLatencyStaleSeconds {
				continue
			}
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

	// Step 2: Run Dijkstra from every source node (phase 1), then apply
	// hop-level hysteresis and materialize routing tables (phase 2).
	const inf = math.MaxFloat64

	type candidate struct {
		dstID    int64
		distance float64
	}
	type spfResult struct {
		dist       map[int64]float64
		firstHop   map[int64]int64 // firstHop[dest] = immediate next hop from src
		candidates []candidate     // distance-sorted, order hysteresis applied
	}

	results := make(map[int64]*spfResult, len(nodeSet))
	for srcID := range nodeSet {
		if len(graph[srcID]) == 0 {
			results[srcID] = &spfResult{}
			continue
		}

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

		// 排序滞回:相邻候选的可达距离接近(差值 ≤20%)时,沿用上一轮
		// 优先级顺序。纯 distance 排序对探测平均值的微小波动敏感,
		// 会让距离接近的边界节点每轮互换 priority(2026-09-11 实测:
		// tw 13.85ms ↔ xiaobai 16.17ms 每轮换位),触发无谓的整表下发。
		if prevRoutes, ok := prevTables[srcID]; ok && len(prevRoutes) > 0 {
			prevRank := make(map[int64]int, len(prevRoutes))
			for i, rt := range prevRoutes {
				if id, found := reverseID[rt.DestPeerID]; found {
					prevRank[id] = i
				}
			}
			for i := 1; i < len(candidates); i++ {
				for j := i; j > 0; j-- {
					if !nearlyEqualDistance(candidates[j-1].distance, candidates[j].distance) {
						break
					}
					rankA, okA := prevRank[candidates[j-1].dstID]
					rankB, okB := prevRank[candidates[j].dstID]
					if okA && okB && rankB < rankA {
						candidates[j-1], candidates[j] = candidates[j], candidates[j-1]
						continue
					}
					break
				}
			}
		}

		results[srcID] = &spfResult{dist: dist, firstHop: firstHop, candidates: candidates}
	}

	// Phase 2: hop-level hysteresis, then materialize routing tables.
	//
	// 等价路径抖动:当 (src → dest) 存在多条距离接近的路径时,探测平均值的
	// 微波动会让 Dijkstra 的最优路径在它们之间来回切换,导致下发的首跳
	// 每轮变化并触发无谓的全量下发(2026-09-11 生产实测:jps 的 priority
	// 3/4 首跳在 tw ↔ hk 之间每轮换位)。滞回规则:若上一轮首跳 H 本轮
	// 仍能到达目标(用本轮纯图结果验证),且上一轮路径距离不超过本轮最优
	// 距离的 20%,保持 H;旧路径明显更差或 H 已断路时采纳新最优路径。
	newHop := make(map[int64]map[int64]int64, len(nodeSet))
	newDist := make(map[int64]map[int64]float64, len(nodeSet))
	tables := make(map[int64][]RouteEntry, len(nodeSet))

	for srcID := range nodeSet {
		res := results[srcID]
		if res == nil || len(res.candidates) == 0 {
			tables[srcID] = []RouteEntry{}
			continue
		}
		dist := res.dist
		firstHop := res.firstHop
		candidates := res.candidates

		stabilized := make(map[int64]bool)
		if hopMap, ok := prevHopMap[srcID]; ok {
			distMap := prevDistMap[srcID]
			for _, cand := range candidates {
				dstID := cand.dstID
				h, okH := hopMap[dstID]
				if !okH || h == dstID {
					continue
				}
				// 旧首跳必须仍是 src 的直接邻居(边未被剔除)。
				if _, directOK := graphDirect[srcID][h]; !directOK {
					continue
				}
				// 旧首跳本轮仍能到达目标(纯图连通性检查)。
				hr, okR := results[h]
				if !okR || hr.dist == nil || hr.dist[dstID] >= inf {
					continue
				}
				oldDist, okD := distMap[dstID]
				if !okD {
					continue
				}
				if oldDist > dist[dstID]*(1+routeHysteresisRatio) {
					continue // 旧路径明显更差,接受新最优
				}
				firstHop[dstID] = h
				stabilized[dstID] = true
			}
		}

		// Take top 5, build RouteEntry with first-hop peer info and direct-link metrics.
		n := len(candidates)
		if n > 5 {
			n = 5
		}
		routes := make([]RouteEntry, 0, n)
		for i := range n {
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
				DestPeerID: candidateDest.WGPublicKey,
				PeerID:     fhPeer.WGPublicKey,
				Priority:   i + 1,
				PeerWGIP:   fhPeer.WGIP,
				PeerPort:   fhPeer.WGListenPort,
				LatencyMs:  direct.latencyMs,
				PacketLoss: direct.packetLoss,
			})
		}
		tables[srcID] = routes

		// 记录本轮全部候选目标的首跳与实际路径距离,供下一轮滞回参考。
		hopRec := make(map[int64]int64, len(candidates))
		distRec := make(map[int64]float64, len(candidates))
		for _, cand := range candidates {
			dstID := cand.dstID
			hopRec[dstID] = firstHop[dstID]
			d := dist[dstID]
			if stabilized[dstID] {
				if od, ok := prevDistMap[srcID][dstID]; ok {
					d = od
				}
			}
			distRec[dstID] = d
		}
		newHop[srcID] = hopRec
		newDist[srcID] = distRec
	}

	// Cache results.
	r.mu.Lock()
	r.tables = tables
	r.lastHop = newHop
	r.lastDist = newDist
	r.lastRun = time.Now()
	r.mu.Unlock()

	return tables
}
