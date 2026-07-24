package forwarding

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/creamcroissant/xboard/internal/agent/transport"
	agentv1 "github.com/creamcroissant/xboard/pkg/pb/agent/v1"
)

// Manager 周期性同步转发规则并通过 nftables 应用。
type Manager struct {
	client    *transport.GRPCClient
	executor  *NFTablesExecutor
	version   int64
	interval  time.Duration
	logger    *slog.Logger
	conn      *transport.ConnectionManager
	available bool
}

// NewManager 创建转发规则管理器。
func NewManager(client *transport.GRPCClient, executor *NFTablesExecutor, interval time.Duration, logger *slog.Logger) *Manager {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	if logger == nil {
		logger = slog.Default()
	}

	m := &Manager{
		client:    client,
		executor:  executor,
		interval:  interval,
		logger:    logger,
		conn:      transport.NewConnectionManager(client, logger),
		available: true,
	}

	availCtx, availCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer availCancel()
	if err := executor.CheckAvailability(availCtx); err != nil {
		m.available = false
		logger.Error("nftables not available, forwarding disabled", "error", err)
	}

	return m
}

// Run 启动转发规则同步循环。
func (m *Manager) Run(ctx context.Context) {
	m.syncOnce(ctx)

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("forwarding manager stopped")
			return
		case <-ticker.C:
			m.syncOnce(ctx)
		}
	}
}

func (m *Manager) syncOnce(ctx context.Context) {
	if m.client == nil {
		m.logger.Warn("forwarding manager skipped: grpc client not set")
		return
	}
	if m.conn != nil {
		state := m.conn.CheckConnection(ctx)
		if state != transport.StateConnected {
			m.logger.Warn("forwarding sync skipped: grpc not ready", "state", state.String())
			return
		}
	}

	resp, err := m.client.GetForwardingRules(ctx, m.version)
	if err != nil {
		m.logger.Error("forwarding sync failed", "error", err)
		return
	}
	if !resp.GetSuccess() {
		m.logger.Warn("forwarding sync rejected", "error", resp.GetErrorMessage())
		return
	}
	if resp.GetNotModified() {
		return
	}

	if !m.available {
		m.reportStatus(ctx, resp.GetVersion(), false, "nftables not available")
		return
	}

	if err := m.executor.Apply(ctx, resp.GetRules()); err != nil {
		reportErr := m.reportStatus(ctx, resp.GetVersion(), false, err.Error())
		if reportErr != nil {
			m.logger.Error("report forwarding status failed", "error", reportErr)
		}
		m.logger.Error("apply forwarding rules failed", "error", err)
		return
	}

	if err := m.reportStatus(ctx, resp.GetVersion(), true, ""); err != nil {
		m.logger.Error("report forwarding status failed", "error", err)
		// 上报失败，不更新版本，下次重新同步
		return
	}
	m.version = resp.GetVersion()
}

func (m *Manager) reportStatus(ctx context.Context, version int64, success bool, errMsg string) error {
	if m.client == nil {
		return fmt.Errorf("grpc client not set")
	}
	report := &agentv1.ForwardingStatusReport{
		Version:      version,
		Success:      success,
		ErrorMessage: errMsg,
		AppliedAt:    time.Now().Unix(),
	}
	_, err := m.client.ReportForwardingStatus(ctx, report)
	return err
}
