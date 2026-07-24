package service

import (
	"context"
	"log/slog"

	"github.com/creamcroissant/xboard/internal/agent/command"
)

const agentCommandActionReportConfig = "report_config"

// registerReportConfigHandler registers the report_config handler.
func (a *Agent) registerReportConfigHandler() error {
	if a == nil || a.commandQueue == nil {
		return nil
	}
	return a.commandQueue.Register(agentCommandActionReportConfig, a.handleReportConfig)
}

// handleReportConfig triggers a re-read of the config file.
// The next StatusReport will include the fresh config content.
func (a *Agent) handleReportConfig(ctx context.Context, task command.Task, reporter command.Reporter) command.Result {
	slog.Info("handling report config command", "command_id", task.ID)

	if _, err := a.readConfigFile(); err != nil {
		slog.Error("re-read config failed", "error", err)
		return command.Result{
			Status:       command.StatusFailed,
			Phase:        "completed",
			Level:        command.LevelError,
			Message:      "config re-read failed",
			ErrorMessage: err.Error(),
			Terminal:     true,
		}
	}

	return command.Result{
		Status:   command.StatusSuccess,
		Phase:    "completed",
		Level:    command.LevelInfo,
		Message:  "config file re-read, will be sent on next status report",
		Terminal: true,
	}
}
