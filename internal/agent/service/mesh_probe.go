package service

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"runtime/debug"
	"time"

	"github.com/creamcroissant/xboard/internal/agent/mesh"
	probepkg "github.com/creamcroissant/xboard/internal/probe"
	agentv1 "github.com/creamcroissant/xboard/pkg/pb/agent/v1"
)

// MeshProber wraps a BackgroundProber to continuously probe mesh peer endpoints
// for latency and packet loss. Targets are derived from mesh.Manager.DumpPeers().
type MeshProber struct {
	prober          *probepkg.BackgroundProber
	meshMgr         *mesh.Manager
	logger          *slog.Logger
	targetToPeer    map[string]string // endpoint -> WGPublicKey
	mu              sync.RWMutex
	lifecycleCtx    context.Context   // agent lifecycle context, set in Start()
	keepaliveCtx    context.Context
	keepaliveCancel context.CancelFunc // cancels the previous keepalive goroutine
	keepaliveWg     sync.WaitGroup     // waits for keepalive goroutine to exit
}

// NewMeshProber creates a new MeshProber.
func NewMeshProber(meshMgr *mesh.Manager, interval, timeout time.Duration, windowSize int, logger *slog.Logger) *MeshProber {
	if logger == nil {
		logger = slog.Default()
	}
	p := probepkg.NewBackgroundProber(interval, windowSize, logger)
	p.SetTimeout(timeout)
	return &MeshProber{
		prober:  p,
		meshMgr: meshMgr,
		logger:  logger.With("component", "mesh_prober"),
	}
}

// Start begins the background probing loop.
// ctx is propagated to the underlying BackgroundProber which respects
// cancellation in the probe loop and in each BatchProbe call.
func (mp *MeshProber) Start(ctx context.Context) {
	mp.lifecycleCtx = ctx
	go mp.prober.Start(ctx)
}

// Stop signals the probing loop to stop.
func (mp *MeshProber) Stop() {
	mp.prober.Stop()
	mp.mu.Lock()
	if mp.keepaliveCancel != nil {
		mp.keepaliveCancel()
		mp.keepaliveCancel = nil
	}
	mp.mu.Unlock()
	mp.keepaliveWg.Wait()
}

// UpdateTargets fetches current mesh peers via DumpPeers and updates the probe target list.
func (mp *MeshProber) UpdateTargets(ctx context.Context) {
	peers, err := mp.meshMgr.DumpPeers(ctx)
	if err != nil {
		mp.logger.Warn("mesh probe: dump peers failed", "error", err)
		return
	}

	targets := make([]probepkg.ProbeTargetConfig, 0, len(peers))
	mp.mu.Lock()
	mp.targetToPeer = make(map[string]string, len(peers))
	for _, p := range peers {
		if p.Endpoint == "" {
			continue
		}
		mp.targetToPeer[p.Endpoint] = p.PublicKey
		targets = append(targets, probepkg.ProbeTargetConfig{
			Type:   "tcpping",
			Target: p.Endpoint,
			Label:  p.PublicKey[:16], // short label from pubkey prefix
		})
	}
	mp.mu.Unlock()

	mp.prober.SetTargets(targets)
	mp.logger.Debug("mesh probe targets updated", "count", len(targets))
}

// SyncLatencies returns the current probe results as protobuf OriginLatencyEntry slices.
func (mp *MeshProber) SyncLatencies() []*agentv1.OriginLatencyEntry {
	results := mp.prober.GetResults()
	if len(results) == 0 {
		return nil
	}

	entries := make([]*agentv1.OriginLatencyEntry, 0, len(results))
	for _, r := range results {
		mp.mu.RLock()
		peerID := mp.targetToPeer[r.Config.Target]
		mp.mu.RUnlock()
		if peerID == "" {
			peerID = r.Config.Target // fallback
		}
		entries = append(entries, &agentv1.OriginLatencyEntry{
			Domain:      peerID,
			LatencyMs:   r.AvgLatencyMs,
			PacketLoss:  r.PacketLoss,
			TotalProbes: int32(r.TotalProbes),
			UpdatedAt:   r.LastProbeAt,
		})
	}
	return entries
}

// KeepalivePeer tracks a single peer's keepalive state.
type KeepalivePeer struct {
	PeerID string
	WGIP   string
	Port   int
	Metric int
	Failed int
	Up     bool
}

// checkPeerHealth checks if a WireGuard peer is alive by inspecting the latest
// handshake timestamp. A peer is considered healthy if a handshake occurred
// within the last 120 seconds.
func (mp *MeshProber) checkPeerHealth(ctx context.Context, pubkey string) bool {
	cmd := exec.CommandContext(ctx, mp.meshMgr.WgBinary(), "show", mp.meshMgr.InterfaceName(), "latest-handshakes")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	now := time.Now().Unix()
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[0] == pubkey {
			handshake, err := strconv.ParseInt(parts[1], 10, 64)
			if err != nil || handshake == 0 {
				return false
			}
			return now-handshake < 120
		}
	}
	return false
}

// StartKeepalive starts the keepalive loop to monitor top N routes.
// It checks WireGuard handshake timestamps every 5 seconds. After 3 consecutive
// failures (no recent handshake), the route is removed (kernel auto-switches to
// a lower metric route).
func (mp *MeshProber) StartKeepalive(ctx context.Context, routes []struct {
	PeerID    string
	WGIP      string
	Port      int
	PublicKey string
}) {
	if len(routes) == 0 {
		return
	}

	// Cancel any existing keepalive goroutine
	mp.mu.Lock()
	if mp.keepaliveCancel != nil {
		mp.keepaliveCancel()
	}
	// Use agent lifecycle context so keepalive survives the caller's lifetime.
	if mp.lifecycleCtx == nil {
		mp.logger.Warn("mesh keepalive: lifecycle context is nil, cannot start keepalive")
		mp.mu.Unlock()
		return
	}
	keepaliveCtx, cancel := context.WithCancel(mp.lifecycleCtx)
	mp.keepaliveCancel = cancel
	mp.mu.Unlock()
	mp.keepaliveWg.Wait()

	maxRoutes := 3
	if len(routes) < maxRoutes {
		maxRoutes = len(routes)
	}

	mp.keepaliveWg.Add(1)
	// keepaliveCtx is captured as a goroutine parameter — do NOT re-read mp.keepaliveCtx.
	go func(ctx context.Context) {
		defer mp.keepaliveWg.Done()
		defer func() {
			if r := recover(); r != nil {
				mp.logger.Error("mesh keepalive panic recovered", "panic", r, "stack", string(debug.Stack()))
			}
		}()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		peers := make([]KeepalivePeer, 0, maxRoutes)
		for i := 0; i < maxRoutes; i++ {
			peers = append(peers, KeepalivePeer{
				PeerID: routes[i].PublicKey,
				WGIP:   routes[i].WGIP,
				Port:   routes[i].Port,
				Metric: (i + 1) * 10,
				Up:     true,
			})
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for i, peer := range peers {
					healthy := mp.checkPeerHealth(ctx, peer.PeerID)

					if !healthy {
						peers[i].Failed++
						mp.logger.Warn("mesh keepalive: peer unreachable",
							"peer", peer.PeerID,
							"fail_count", peers[i].Failed)
						if peers[i].Failed >= 3 && peers[i].Up {
							peers[i].Up = false
							// Remove route — kernel auto-switches to lower metric
							if out, err := exec.CommandContext(ctx, "ip", "route", "del", mp.meshMgr.NetworkCIDR(),
								"via", peer.WGIP, "dev", mp.meshMgr.InterfaceName(),
								"metric", fmt.Sprintf("%d", peer.Metric)).CombinedOutput(); err != nil {
								mp.logger.Warn("mesh keepalive: route removal failed",
									"peer", peer.PeerID, "error", string(out))
							}
							mp.logger.Warn("mesh keepalive: route removed (peer down)",
								"peer", peer.PeerID, "metric", peer.Metric)
						}
					} else {
						peers[i].Failed = 0
						if !peers[i].Up {
							peers[i].Up = true
							cmd := exec.CommandContext(ctx, "ip", "route", "add", mp.meshMgr.NetworkCIDR(),
								"via", peer.WGIP, "dev", mp.meshMgr.InterfaceName(),
								"metric", fmt.Sprintf("%d", peer.Metric))
							if out, err := cmd.CombinedOutput(); err != nil {
								mp.logger.Warn("mesh keepalive: restore route failed",
									"peer", peer.PeerID, "error", string(out))
							} else {
								mp.logger.Info("mesh keepalive: route restored (peer up)",
									"peer", peer.PeerID, "metric", peer.Metric)
							}
						}
					}
				}
			}
		}
	}(keepaliveCtx)
}
