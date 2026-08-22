package job

import (
	"context"
	"fmt"

	"github.com/creamcroissant/mgpanel/internal/service"
)

type AgentHostMetricsFlushJob struct {
	AgentHosts service.AgentHostService
}

func NewAgentHostMetricsFlushJob(agentHosts service.AgentHostService) *AgentHostMetricsFlushJob {
	return &AgentHostMetricsFlushJob{AgentHosts: agentHosts}
}

func (j *AgentHostMetricsFlushJob) Name() string { return "agent_host.metrics.flush" }

func (j *AgentHostMetricsFlushJob) Run(ctx context.Context) error {
	if j == nil || j.AgentHosts == nil {
		return fmt.Errorf("agent host metrics flush job dependencies not configured / agent host metrics flush 依赖未配置")
	}
	return j.AgentHosts.FlushMetrics(ctx)
}
