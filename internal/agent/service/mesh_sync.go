package service

import (
	"context"
	"log/slog"
	"net"
	"strconv"

	"github.com/creamcroissant/mgpanel/internal/agent/mesh"
	agentv1 "github.com/creamcroissant/mgpanel/pkg/pb/agent/v1"
)

// syncMeshConfig implements the full mesh auto-join and peer sync loop
// using gRPC communication with the Panel.
// Called periodically from syncGRPC.
func (a *Agent) syncMeshConfig(ctx context.Context) {
	if a.meshManager == nil {
		return
	}

	// Step 1: Check local config
	localCfg, err := a.meshManager.ReadLocalConfig()
	if err != nil {
		slog.Warn("mesh sync: read local config failed", "error", err)
		return
	}

	// Step 2: Auto-join if not yet joined
	if localCfg == nil {
		slog.Info("mesh sync: not joined, auto-joining network via gRPC")
		resp, err := a.grpc.JoinMesh(ctx, &agentv1.JoinMeshRequest{
			NetworkId: "default",
		})
		if err != nil {
			slog.Warn("mesh sync: auto-join via gRPC failed", "error", err)
			return
		}

		localCfg = &mesh.LocalConfig{
			PrivateKey: resp.WgPrivateKey,
			Address:    resp.WgIp + "/" + strconv.Itoa(a.cidrPrefixLen()),
			ListenPort: int(resp.WgListenPort),
		}

		if err := a.meshManager.WriteLocalConfig(*localCfg); err != nil {
			slog.Warn("mesh sync: write local config failed", "error", err)
			return
		}

		// Start 创建 wireguard 接口、应用配置、分配 IP、拉起接口
		if err := a.meshManager.Start(ctx); err != nil {
			slog.Warn("mesh sync: auto-join start failed, rolling back local config", "error", err)
			if rmErr := a.meshManager.RemoveLocalConfig(); rmErr != nil {
				slog.Warn("mesh sync: failed to remove local config after start failure", "error", rmErr)
			}
			return
		}

		slog.Info("mesh sync: auto-joined successfully via gRPC",
			"ip", resp.WgIp,
			"port", resp.WgListenPort,
		)
	}

	// Step 3: 若接口丢失（如重启后），重建接口和应用配置
	// 通过 DumpPeers（依赖 wg show dump）检查接口是否存在
	if _, err := a.meshManager.DumpPeers(ctx); err != nil {
		slog.Info("mesh sync: interface not ready, restarting", "error", err)
		if err := a.meshManager.Start(ctx); err != nil {
			slog.Warn("mesh sync: restart wireguard interface failed", "error", err)
			return
		}
	}

	// Step 4: Fetch all peers via gRPC and sync
	peersResp, err := a.grpc.GetMeshPeers(ctx, &agentv1.GetMeshPeersRequest{
		NetworkId: "default",
	})
	if err != nil {
		slog.Warn("mesh sync: fetch peers via gRPC failed", "error", err)
		return
	}

	if len(peersResp.Peers) > 0 {
		peerCfgs := make([]mesh.PeerConfig, 0, len(peersResp.Peers))
		for _, p := range peersResp.Peers {
			peerCfgs = append(peerCfgs, mesh.PeerConfig{
				ID:         p.Id,
				PublicKey:  p.PublicKey,
				Endpoint:   p.Endpoint,
				AllowedIPs: p.AllowedIps,
				Keepalive:  int(p.Keepalive),
			})
		}
		if err := a.meshManager.SyncPeers(ctx, peerCfgs); err != nil {
			slog.Warn("mesh sync: sync peers failed", "error", err)
		} else {
			slog.Debug("mesh sync: peers synced", "count", len(peerCfgs))
		}
	}
}

// cidrPrefixLen returns the prefix length from the configured NetworkCIDR.
// Falls back to 24 if parsing fails.
func (a *Agent) cidrPrefixLen() int {
	_, cidrNet, err := net.ParseCIDR(a.meshManager.NetworkCIDR())
	if err != nil {
		slog.Warn("mesh sync: invalid NetworkCIDR, falling back to /24",
			"cidr", a.meshManager.NetworkCIDR(),
		)
		return 24
	}
	prefixLen, _ := cidrNet.Mask.Size()
	return prefixLen
}
