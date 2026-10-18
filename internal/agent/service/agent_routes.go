package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/agent/egressroute"
	"github.com/creamcroissant/mgpanel/internal/agent/protocol"
	"github.com/creamcroissant/mgpanel/internal/agent/relayroute"
)

// getRelayRouteMgr 返回中继链路内核路由管理器（惰性构造，与 mesh 共用锁）。
func (a *Agent) getRelayRouteMgr() *relayroute.Manager {
	a.meshMu.Lock()
	defer a.meshMu.Unlock()
	if a.relayRouteMgr == nil {
		a.relayRouteMgr = relayroute.NewManager(relayroute.Config{Logger: slog.Default()})
	}
	return a.relayRouteMgr
}

// getEgressRouteMgr 返回出口集分发管理器（惰性构造，与 mesh/relay 共用锁）。
func (a *Agent) getEgressRouteMgr() egressRouteManager {
	a.meshMu.Lock()
	defer a.meshMu.Unlock()
	if a.egressRouteMgr == nil {
		a.egressRouteMgr = egressroute.NewManager(egressroute.Config{
			Logger:             slog.Default(),
			AppliedConfigPaths: a.egressAppliedConfigPaths(),
		})
	}
	return a.egressRouteMgr
}

// egressAppliedConfigPaths 返回"核心实际读取的配置"路径列表，供出口集分发精确判定
// "某个 fwmark 是否仍被已应用配置引用"（见 egressroute/applied_config.go）。
//
// 取 ManagedDir（staged apply 把 stageDir rename 到这里，核心读的是它）：
//   - xray:     ManagedDir/<mergeOutputFile>（单文件 merged 输出）
//   - sing-box: ManagedDir 下的一批 fragment（无 merged 文件）→ 交给目录扫描
//
// 注意**不要**取 LegacyDir：那是 merge base 输入，apply 不会更新它，读它会得到
// 恒空的 mark 集合 → 误判为"可以拆除"（泄漏）。
func (a *Agent) egressAppliedConfigPaths() []string {
	if a == nil || a.cfg == nil {
		return nil
	}
	paths, err := protocol.ResolveStagedApplyPaths(protocol.Config{
		ConfigDir:        a.cfg.Protocol.ConfigDir,
		LegacyConfigDir:  a.cfg.Protocol.LegacyConfigDir,
		ManagedConfigDir: a.cfg.Protocol.ManagedConfigDir,
		MergeOutputFile:  a.cfg.Protocol.MergeOutputFile,
	})
	if err != nil {
		return nil
	}
	out := []string{paths.ManagedDir}
	if merged := filepath.Join(paths.ManagedDir, paths.MergeOutputFile); merged != paths.ManagedDir {
		out = append(out, merged)
	}
	return out
}

// syncRoutesThenApply 按 I12 顺序同步内核路由再应用配置，并按 I14 做就绪门控：
// egress 未就绪（Apply 失败或 Probe 发现缺项）时本轮跳过 syncApplyBatch，
// 核心继续跑旧 revision，避免 mark 生效但表/规则不存在导致流量从入口直出。
func (a *Agent) syncRoutesThenApply(ctx context.Context) {
	egressReady := a.syncEgressRoutes(ctx)
	a.syncRelayRoutes(ctx)
	if !egressReady {
		slog.Warn("egress-route: not ready, skip apply batch this round")
		return
	}
	// 延迟拆除（GAP-2/I15）：syncApplyBatch 在生产走异步入队（commandQueue），
	// 本轮拿不到结果，因此这里**每轮**按"自记账以来 revision 是否前进"判定：
	// 只要核心确已换上不含该 mark 的新配置，就安全拆除陈旧规则/表/隧道。
	a.syncApplyBatch(ctx)
	if mgr := a.getEgressRouteMgr(); mgr != nil {
		if err := mgr.CommitRemovals(ctx, a.getApplyRevision()); err != nil {
			slog.Warn("egress-route: commit removals failed", slog.String("err", err.Error()))
		}
	}
}

// syncEgressRoutes 周期拉取本机应生效的出口集分发 assignment 并幂等应用。
// 面板端点：GET /api/v1/agent/egress-routes?token=<host_token>。
// 返回 false 表示内核未就绪，调用方必须跳过本轮配置应用（I14）。
// 拉取/解析失败不改就绪判定：面板未回收到本机能力前不会渲染 mark 出站，
// 上一轮已生效的内核状态保持不变。
func (a *Agent) syncEgressRoutes(ctx context.Context) bool {
	if a == nil || a.cfg == nil {
		return true
	}
	hostToken := strings.TrimSpace(a.cfg.Panel.HostToken)
	if hostToken == "" {
		return true
	}
	base := strings.TrimSuffix(resolvePanelHTTPBase(a.cfg), "/")
	if base == "" {
		slog.Debug("egress-route: no panel http base, skip")
		return true
	}
	reqURL := base + "/api/v1/agent/egress-routes?token=" + url.QueryEscape(hostToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return true
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		slog.Debug("egress-route: fetch skipped", slog.String("err", err.Error()))
		return true
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		slog.Warn("egress-route: unexpected status", slog.Int("status", resp.StatusCode))
		return true
	}
	var payload struct {
		Data []egressroute.Assignment `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		slog.Warn("egress-route: decode failed", slog.String("err", err.Error()))
		return true
	}
	mgr := a.getEgressRouteMgr()
	if err := mgr.Apply(ctx, payload.Data, a.getApplyRevision()); err != nil {
		slog.Warn("egress-route: not ready, apply failed",
			slog.Int("assignments", len(payload.Data)), slog.String("err", err.Error()))
		return false
	}
	states, err := mgr.Probe(ctx, payload.Data)
	if err != nil {
		slog.Warn("egress-route: not ready, probe failed", slog.String("err", err.Error()))
		return false
	}
	if notReady := egressNotReady(states); len(notReady) > 0 {
		slog.Warn("egress-route: not ready, skip apply", slog.Any("members", notReady))
		return false
	}
	slog.Info("egress-route: assignments applied", slog.Int("count", len(payload.Data)))
	return true
}

// egressNotReady 返回内核未就绪的成员明细（就绪判定见 I14；空切片表示全部就绪）。
func egressNotReady(states []egressroute.MemberState) []egressroute.MemberState {
	var out []egressroute.MemberState
	for _, st := range states {
		if !st.OK {
			out = append(out, st)
		}
	}
	return out
}

// syncRelayRoutes 周期拉取本机应生效的中继链路角色并幂等应用。
// 面板端点:GET /api/v1/agent/relay-routes?token=<host_token>（host_token 复用注册凭据）。
func (a *Agent) syncRelayRoutes(ctx context.Context) {
	if a == nil || a.cfg == nil {
		return
	}
	hostToken := strings.TrimSpace(a.cfg.Panel.HostToken)
	if hostToken == "" {
		return
	}
	base := strings.TrimSuffix(resolvePanelHTTPBase(a.cfg), "/")
	reqURL := base + "/api/v1/agent/relay-routes?token=" + url.QueryEscape(hostToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		slog.Debug("relay-route: fetch skipped", slog.String("err", err.Error()))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		slog.Warn("relay-route: unexpected status", slog.Int("status", resp.StatusCode))
		return
	}
	var payload struct {
		Data []relayroute.Role `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		slog.Warn("relay-route: decode failed", slog.String("err", err.Error()))
		return
	}
	if err := a.getRelayRouteMgr().Apply(ctx, payload.Data); err != nil {
		slog.Warn("relay-route: apply failed", slog.String("err", err.Error()))
		return
	}
	slog.Info("relay-route: roles applied", slog.Int("count", len(payload.Data)))
}
