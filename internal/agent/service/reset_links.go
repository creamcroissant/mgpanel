package service

import (
	"context"
	"log/slog"

	"github.com/creamcroissant/xboard/internal/agent/command"
)

const agentCommandActionResetLinks = "reset_links"

// registerResetLinksHandler registers the reset_links lifecycle operation handler.
// The actual link reset happens Panel-side; this handler acknowledges the operation
// so it does not stay pending forever.
func (a *Agent) registerResetLinksHandler() error {
	if a == nil || a.commandQueue == nil {
		return nil
	}
	return a.commandQueue.Register(agentCommandActionResetLinks, a.handleResetLinks)
}

// handleResetLinks acknowledges a reset_links operation from the Panel.
// The operation is informational — the agent confirms receipt and completes.
func (a *Agent) handleResetLinks(ctx context.Context, task command.Task, reporter command.Reporter) command.Result {
	slog.Info("handling reset links command", "command_id", task.ID)

	_ = reporter.Report(ctx, command.Event{
		EventType: command.EventTypeProgress,
		Status:    command.StatusInProgress,
		Phase:     "acknowledged",
		Level:     command.LevelInfo,
		Message:   "reset links acknowledged",
	})

	return command.Result{
		Status:  command.StatusSuccess,
		Phase:   "completed",
		Level:   command.LevelInfo,
		Message: "reset links completed",
	}
}
