package service

import (
	"context"
	"log/slog"

	"github.com/creamcroissant/mgpanel/internal/agent/command"
)

// agentCommandActionGeoRefresh 是 panel → agent 的"立即探测 GeoIP"命令 key。
// 与 set_routing_table 同模式，agent_lifecycle_operation type=geo_refresh。
const agentCommandActionGeoRefresh = "geo_refresh"

// registerGeoRefreshHandler 向 commandQueue 注册 geo_refresh 命令处理器。
// 当 panel 通过 agent_lifecycle_operation 派发该类型操作时，agent 在 syncGRPC
// 周期拉到本地 queue 并执行 RunOnce + syncAgentHost 上报结果。
func (a *Agent) registerGeoRefreshHandler() error {
	if a.commandQueue == nil {
		return nil
	}
	return a.commandQueue.Register(agentCommandActionGeoRefresh, a.handleGeoRefresh)
}

// handleGeoRefresh 是 geo_refresh 命令的 handler 主体。
// 1) 调 geoReporter.RunOnce 探测 (country, region)；
// 2) 立即把 (host, country, region) 走现有 syncAgentHost 通道上报 panel；
// 3) 探测失败时 country/region 留空，panel 端会用 mmdb 兜底 country。
func (a *Agent) handleGeoRefresh(ctx context.Context, task command.Task, reporter command.Reporter) command.Result {
	var country, region string
	if a.geoReporter != nil {
		c, r, err := a.geoReporter.RunOnce(ctx)
		if err != nil {
			slog.Warn("geo refresh runonce failed", "error", err)
		} else {
			country, region = c, r
		}
	}
	host := a.getCachedAdvertiseHost()
	if host != "" {
		a.syncAgentHost(ctx, country, region)
	}
	return command.Result{
		Status:  command.StatusSuccess,
		Level:   command.LevelInfo,
		Message: "geo refreshed",
	}
}
