package traffic

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/creamcroissant/mgpanel/internal/agent/api"
	"github.com/creamcroissant/mgpanel/internal/agent/config"
)

type Collector interface {
	Collect(ctx context.Context) ([]api.TrafficPayload, error)
}

func NewCollector(cfg config.TrafficConfig) (Collector, error) {
	switch cfg.Type {
	case "none", "netio":
		// "netio" 由 NetIOCollector 单独处理（系统级流量）
		// 这里的 Collector 仅用于用户级流量（如 xray_api）
		return &NoOpCollector{}, nil
	case "dummy":
		return &DummyCollector{}, nil
	case "xray_api":
		return NewXrayCollector(cfg.Address)
	default:
		return &NoOpCollector{}, fmt.Errorf("unknown traffic type: %s", cfg.Type)
	}
}

type NoOpCollector struct{}

func (c *NoOpCollector) Collect(ctx context.Context) ([]api.TrafficPayload, error) {
	return nil, nil
}

type DummyCollector struct{}

func (c *DummyCollector) Collect(ctx context.Context) ([]api.TrafficPayload, error) {
	slog.Info("DummyCollector: collecting traffic stats (mock)")
	return []api.TrafficPayload{
		{
			UserID:   3,
			Upload:   1024 * 1024, // 1 MB
			Download: 2048 * 1024, // 2 MB
		},
	}, nil
}
