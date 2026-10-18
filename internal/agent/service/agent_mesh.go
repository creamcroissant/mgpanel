package service

import (
	"context"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/creamcroissant/mgpanel/internal/agent/mesh"
	agentv1 "github.com/creamcroissant/mgpanel/pkg/pb/agent/v1"
)

// tryEnableMesh checks if this agent has a mesh peer record on the panel.
// If yes, it dynamically initializes the mesh manager and prober so the agent
// can auto-join without requiring mesh.enabled: true in the local config.yml.
// setMesh 原子更新 mesh 组件对（构造后动态启用场景）。
func (a *Agent) setMesh(mgr *mesh.Manager, prober *MeshProber) {
	a.meshMu.Lock()
	defer a.meshMu.Unlock()
	a.meshManager = mgr
	a.meshProber = prober
}

// getMesh 返回当前 mesh 组件快照；调用方基于快照操作即可保证一致视图。
func (a *Agent) getMesh() (*mesh.Manager, *MeshProber) {
	a.meshMu.Lock()
	defer a.meshMu.Unlock()
	return a.meshManager, a.meshProber
}

// cachedAdvertiseHost 的并发安全读写。
func (a *Agent) getCachedAdvertiseHost() string {
	a.meshMu.Lock()
	defer a.meshMu.Unlock()
	return a.cachedAdvertiseHost
}

func (a *Agent) setCachedAdvertiseHost(host string) {
	a.meshMu.Lock()
	defer a.meshMu.Unlock()
	a.cachedAdvertiseHost = host
}

func (a *Agent) tryEnableMesh(ctx context.Context) {
	joinCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	resp, err := a.grpc.JoinMesh(joinCtx, &agentv1.JoinMeshRequest{NetworkId: "default"})
	if err != nil {
		// mesh service not available on panel, or agent not in mesh — nothing to do
		return
	}
	if resp == nil || !resp.Success {
		return
	}

	meshDir := filepath.Join(a.cfg.Core.CoreInstallDir, "..", "mesh")
	meshDir = filepath.Clean(meshDir)

	newMgr := mesh.NewManager(mesh.Config{
		ConfigDir:     meshDir,
		InterfaceName: "wgmesh0",
		ListenPort:    51820,
		WgBinary:      "wg",
		NetworkCIDR:   "10.144.0.0/24",
	}, slog.Default())

	newProber := NewMeshProber(newMgr, 30*time.Second, 5*time.Second, 10, slog.Default())
	a.setMesh(newMgr, newProber)
	// 动态创建后需显式启动 prober，否则 lifecycleCtx 为 nil，keepalive 无法启动。
	// 必须使用 agent 生命周期上下文（a.ctx），而非本轮 sync 的临时 ctx，避免随 sync 结束被取消。
	newProber.Start(a.ctx)
	slog.Info("mesh: dynamically enabled after discovering panel-side mesh record",
		"ip", resp.WgIp,
	)
}

// meshPeerLatenciesProto returns the current mesh peer latency probe results
// as protobuf OriginLatencyEntry slice for the status report.
func (a *Agent) meshPeerLatenciesProto() []*agentv1.OriginLatencyEntry {
	_, prober := a.getMesh()
	if prober == nil {
		return nil
	}
	return prober.SyncLatencies()
}
