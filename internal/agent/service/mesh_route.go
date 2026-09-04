package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"time"

	"github.com/creamcroissant/mgpanel/internal/agent/command"
	agentv1 "github.com/creamcroissant/mgpanel/pkg/pb/agent/v1"
)

const (
	agentCommandActionSetRoutingTable = "set_routing_table"
	meshInterfaceName                 = "wgmesh0"
	meshNetworkCIDR                   = "10.144.0.0/24"
)

// registerRoutingTableHandler registers the set_routing_table command handler.
func (a *Agent) registerRoutingTableHandler() error {
	if a.commandQueue == nil {
		return nil
	}
	return a.commandQueue.Register(agentCommandActionSetRoutingTable, func(ctx context.Context, task command.Task, reporter command.Reporter) command.Result {
		if err := a.handleSetRoutingTable(ctx, task); err != nil {
			return command.Result{
				Status:       command.StatusFailed,
				Level:        command.LevelError,
				Message:      err.Error(),
				ErrorMessage: err.Error(),
				Terminal:     true,
			}
		}
		return command.Result{
			Status:   command.StatusSuccess,
			Level:    command.LevelInfo,
			Message:  "routing table installed",
			Terminal: true,
		}
	})
}

func (a *Agent) handleSetRoutingTable(ctx context.Context, task command.Task) error {
	var payload agentv1.SetRoutingTablesPayload
	if err := json.Unmarshal(task.RequestPayload, &payload); err != nil {
		return fmt.Errorf("unmarshal set_routing_table payload: %w", err)
	}
	routes := payload.GetRoutes()
	if len(routes) == 0 {
		return nil
	}
	slog.Info("mesh route: installing routing table", "routes", len(routes))

	if err := a.installRoutingTable(ctx, routes); err != nil {
		return err
	}

	// Start/update keepalive for top 3 unique routes
	if a.meshProber != nil {
		maxKa := 3
		kaRoutes := make([]struct {
			PeerID    string
			WGIP      string
			Port      int
			PublicKey string
		}, 0, maxKa)
		seenKey := make(map[string]struct{})
		for _, r := range routes {
			if len(kaRoutes) >= maxKa {
				break
			}
			if r.PeerWgIp == "" {
				continue
			}
			if _, ok := seenKey[r.PeerWgIp]; ok {
				continue
			}
			seenKey[r.PeerWgIp] = struct{}{}
			kaRoutes = append(kaRoutes, struct {
				PeerID    string
				WGIP      string
				Port      int
				PublicKey string
			}{
				PeerID:    r.PeerId,
				WGIP:      r.PeerWgIp,
				Port:      int(r.PeerPort),
				PublicKey: r.PeerId,
			})
		}
		if _, prober := a.getMesh(); prober != nil {
			prober.StartKeepalive(ctx, kaRoutes)
		}
	}

	return nil
}

func (a *Agent) installRoutingTable(ctx context.Context, routes []*agentv1.RouteEntry) error {
	maxRoutes := 3

	// Track successfully installed routes for rollback on failure
	type installedRoute struct {
		WGIP   string
		Metric int
	}
	installed := make([]installedRoute, 0, maxRoutes)
	seenIP := make(map[string]struct{})

	for _, r := range routes {
		if len(installed) >= maxRoutes {
			break
		}
		if r.PeerWgIp == "" {
			continue
		}
		if _, ok := seenIP[r.PeerWgIp]; ok {
			continue // 相同网关仅安装最高优先级条目，避免生成重复不同 metric 路由
		}
		seenIP[r.PeerWgIp] = struct{}{}

		metric := (len(installed) + 1) * 10
		cmd := exec.CommandContext(ctx, "ip", "route", "replace", meshNetworkCIDR,
			"via", r.PeerWgIp, "dev", meshInterfaceName,
			"metric", fmt.Sprintf("%d", metric))
		out, err := cmd.CombinedOutput()
		if err != nil {
			// Rollback all previously installed routes with timeout
			// TODO: make rollback timeout configurable via mesh config
			rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), 10*time.Second)
			for _, inst := range installed {
				_ = exec.CommandContext(rollbackCtx, "ip", "route", "del", meshNetworkCIDR,
					"via", inst.WGIP, "dev", meshInterfaceName,
					"metric", fmt.Sprintf("%d", inst.Metric)).Run()
			}
			rollbackCancel()
			return fmt.Errorf("route %d failed: %s; rolled back %d routes",
				len(installed)+1, string(out), len(installed))
		}
		installed = append(installed, installedRoute{r.PeerWgIp, metric})
		slog.Debug("mesh route installed", "priority", r.Priority, "peer", r.PeerWgIp, "metric", metric)
	}

	// 清理多余 metric 残留（如历史保留了更高 metric 但本次去重后有效网关不足 maxRoutes）
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cleanupCancel()
	for m := len(installed) + 1; m <= maxRoutes; m++ {
		metric := m * 10
		_ = exec.CommandContext(cleanupCtx, "ip", "route", "del", meshNetworkCIDR,
			"dev", meshInterfaceName,
			"metric", fmt.Sprintf("%d", metric)).Run()
	}

	return nil
}
