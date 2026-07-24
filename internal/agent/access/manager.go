package access

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/creamcroissant/xboard/internal/agent/core"
	"github.com/creamcroissant/xboard/internal/agent/transport"
	agentv1 "github.com/creamcroissant/xboard/pkg/pb/agent/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Manager struct {
	client  *transport.GRPCClient
	manager *core.Manager
	logger  *slog.Logger

	collectors []Collector
	stopCh     chan struct{}
	stopOnce   sync.Once
}

func NewManager(client *transport.GRPCClient, coreManager *core.Manager, logger *slog.Logger) *Manager {
	return &Manager{
		client:  client,
		manager: coreManager,
		logger:  logger,
		stopCh:  make(chan struct{}),
	}
}

func (m *Manager) Start() {
	// Register collectors
	m.collectors = []Collector{
		NewXrayAccessCollector(m.manager, m.logger),
		NewSingboxAccessCollector(m.manager, m.logger),
	}

	go m.run()
}

func (m *Manager) Stop() {
	m.stopOnce.Do(func() {
		close(m.stopCh)
	})
}

func (m *Manager) run() {
	// TODO: Make interval configurable
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.collectAndReport()
		}
	}
}

func (m *Manager) collectAndReport() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var allEntries []AccessLogEntry
	for _, collector := range m.collectors {
		entries, err := collector.Collect(ctx)
		if err != nil {
			m.logger.Error("failed to collect access logs",
				"collector", collector.Type(),
				"error", err,
			)
			continue
		}
		if len(entries) > 0 {
			allEntries = append(allEntries, entries...)
		}
	}

	if len(allEntries) == 0 {
		return
	}

	m.logger.Debug("collected access logs", "count", len(allEntries))

	if err := m.report(ctx, allEntries); err != nil {
		m.logger.Error("failed to report access logs", "error", err)
	}
}

func (m *Manager) report(ctx context.Context, entries []AccessLogEntry) error {
	if !m.client.IsHealthy() {
		return nil // skip if not connected
	}

	req := &agentv1.AccessLogReport{
		Entries: make([]*agentv1.AccessLogEntry, len(entries)),
	}

	for i, entry := range entries {
		req.Entries[i] = &agentv1.AccessLogEntry{
			UserEmail:       entry.UserEmail,
			SourceIp:        entry.SourceIP,
			TargetDomain:    entry.TargetDomain,
			TargetIp:        entry.TargetIP,
			TargetPort:      int32(entry.TargetPort),
			Protocol:        entry.Protocol,
			Upload:          entry.Upload,
			Download:        entry.Download,
			ConnectionStart: timestamppb.New(entry.ConnectionStart),
		}
		if !entry.ConnectionEnd.IsZero() {
			req.Entries[i].ConnectionEnd = timestamppb.New(entry.ConnectionEnd)
		}
	}

	resp, err := m.client.ReportAccessLogs(ctx, req)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("server rejected logs: %s", resp.Message)
	}

	return nil
}
