package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"strings"
	"syscall"

	"github.com/creamcroissant/mgpanel/internal/agent/command"
	"github.com/creamcroissant/mgpanel/internal/agent/config"
	agentupdater "github.com/creamcroissant/mgpanel/internal/agent/updater"
	agentv1 "github.com/creamcroissant/mgpanel/pkg/pb/agent/v1"
)

const (
	agentCommandActionAgentUpdate      = "agent_update"
	agentCommandActionAgentUpdateCheck = "agent_update_check"
)

type agentUpdater interface {
	Status() agentupdater.Status
	Check(ctx context.Context, req agentupdater.CheckRequest) (*agentupdater.CheckResult, error)
	Update(ctx context.Context, req agentupdater.UpdateRequest, progress agentupdater.ProgressFunc) (*agentupdater.UpdateResult, error)
	MarkHealthy() error
	RecordStartup() error
	RollbackIfHealthExpired() error
	Rollback() error
}

func newAgentUpdater(cfg config.UpdateConfig) (agentUpdater, error) {
	return agentupdater.New(agentupdater.Config{
		AutoEnabled:      cfg.AutoEnabled,
		CurrentVersion:   cfg.CurrentVersion,
		BinaryPath:       cfg.BinaryPath,
		StatePath:        cfg.StatePath,
		BackupDir:        cfg.BackupDir,
		ReleaseBaseURL:   cfg.ReleaseBaseURL,
		ReleaseRepo:      cfg.ReleaseRepo,
		ReleaseTag:       cfg.ReleaseTag,
		HealthTimeout:    cfg.HealthTimeout,
		MaxCrashCount:    cfg.MaxCrashCount,
		JitterMin:        cfg.JitterMin,
		JitterMax:        cfg.JitterMax,
		MaxDownloadBytes: cfg.MaxDownloadBytes,
	})
}

func (a *Agent) registerAgentUpdateHandlers() error {
	if a == nil || a.commandQueue == nil || a.updater == nil {
		return nil
	}
	if err := a.commandQueue.Register(agentCommandActionAgentUpdateCheck, a.handleAgentUpdateCheck); err != nil {
		return err
	}
	return a.commandQueue.Register(agentCommandActionAgentUpdate, a.handleAgentUpdate)
}

func (a *Agent) handleAgentUpdateCheck(ctx context.Context, task command.Task, reporter command.Reporter) command.Result {
	var req agentupdater.CheckRequest
	if err := decodeAgentUpdatePayload(task.RequestPayload, &req); err != nil {
		return agentUpdateFailureResult(err, "invalid_payload")
	}
	_ = reporter.Report(ctx, command.Event{EventType: command.EventTypeProgress, Status: command.StatusInProgress, Phase: agentupdater.PhaseChecking, Level: command.LevelInfo, Message: "agent update check started"})
	result, err := a.updater.Check(ctx, req)
	if err != nil {
		return agentUpdateFailureResult(err, agentupdater.PhaseChecking)
	}
	payload, _ := json.Marshal(result)
	phase, level := agentUpdateCheckResultPhase(result)
	return command.Result{Status: command.StatusSuccess, Phase: phase, Level: level, Message: "agent update check completed", Payload: payload}
}

func (a *Agent) handleAgentUpdate(ctx context.Context, task command.Task, reporter command.Reporter) command.Result {
	var req agentupdater.UpdateRequest
	if err := decodeAgentUpdatePayload(task.RequestPayload, &req); err != nil {
		return agentUpdateFailureResult(err, "invalid_payload")
	}
	result, err := a.updater.Update(ctx, req, func(ctx context.Context, event agentupdater.Event) error {
		return reporter.Report(ctx, command.Event{EventType: command.EventTypeProgress, Status: command.StatusInProgress, Phase: event.Phase, Level: agentUpdateCommandLevel(event.Level), Message: event.Message, Payload: event.Payload})
	})
	if err != nil {
		return agentUpdateFailureResult(err, a.updateStatusProto().GetPhase())
	}
	payload, _ := json.Marshal(result)
	// Self-restart: exec the new binary to replace the current process.
	// The updater has already swapped the binary on disk. We exec it
	// with the same args so the new version takes over immediately.
	// Guard: never self-exec inside `go test` — the test binary would
	// re-run itself endlessly (hermetic tests).
	if testing.Testing() {
		slog.Info("skip self-exec under go test", "target", result.TargetVersion)
		return command.Result{Status: command.StatusSuccess, Phase: agentupdater.PhaseHealthPending, Level: command.LevelInfo, Message: "agent update waiting for health confirmation", Payload: payload}
	}
	execPath, err := os.Executable()
	if err == nil && execPath != "" {
		slog.Info("agent binary replaced, self-exec restarting",
			"target", result.TargetVersion,
			"path", execPath,
		)
		// Exec replaces the current process in-place (PID preserved, fds inherited).
		if execErr := syscall.Exec(execPath, os.Args, os.Environ()); execErr != nil {
			slog.Error("self-exec failed", "error", execErr)
		}
	} else {
		slog.Warn("cannot determine executable path, skipping self-exec", "error", err)
	}
	// If exec fails, attempt os.Exit as fallback so the process manager (Docker/systemd) restarts us.
	os.Exit(0)
	return command.Result{Status: command.StatusSuccess, Phase: agentupdater.PhaseHealthPending, Level: command.LevelInfo, Message: "agent update waiting for health confirmation", Payload: payload}
}

func decodeAgentUpdatePayload(payload []byte, target any) error {
	if len(payload) == 0 {
		return nil
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("decode agent update payload: %w", err)
	}
	return nil
}

func agentUpdateFailureResult(err error, phase string) command.Result {
	if strings.TrimSpace(phase) == "" {
		phase = "failed"
	}
	level := command.LevelError
	if errors.Is(err, agentupdater.ErrLockedBadVersion) {
		phase = agentupdater.PhaseLocked
		level = command.LevelWarn
	}
	return command.Result{Status: command.StatusFailed, Phase: phase, Level: level, Message: "agent update command failed", ErrorMessage: err.Error()}
}

func agentUpdateCheckResultPhase(result *agentupdater.CheckResult) (string, string) {
	if result == nil {
		return agentupdater.PhaseChecking, command.LevelWarn
	}
	switch {
	case result.Locked:
		return agentupdater.PhaseLocked, command.LevelWarn
	case result.UpToDate:
		return agentupdater.PhaseUpToDate, command.LevelInfo
	default:
		return agentupdater.PhaseCompatible, command.LevelInfo
	}
}

func agentUpdateCommandLevel(level string) string {
	switch strings.TrimSpace(level) {
	case command.LevelWarn:
		return command.LevelWarn
	case command.LevelError:
		return command.LevelError
	default:
		return command.LevelInfo
	}
}

func (a *Agent) updateStatusProto() *agentv1.AgentUpdateStatus {
	if a == nil || a.updater == nil {
		return nil
	}
	return agentUpdateStatusToProto(a.updater.Status())
}

func agentUpdateStatusToProto(status agentupdater.Status) *agentv1.AgentUpdateStatus {
	return &agentv1.AgentUpdateStatus{
		CurrentVersion:    status.CurrentVersion,
		TargetVersion:     status.TargetVersion,
		Status:            status.Status,
		Phase:             status.Phase,
		PreviousVersion:   status.PreviousVersion,
		ErrorMessage:      status.ErrorMessage,
		StartedAt:         status.StartedAt,
		FinishedAt:        status.FinishedAt,
		LastCheckedAt:     status.LastCheckedAt,
		LastCheckError:    status.LastCheckError,
		RollbackAvailable: status.RollbackAvailable,
		RolledBack:        status.RolledBack,
		LockedBadVersion:  status.LockedBadVersion,
		CrashCount:        status.CrashCount,
		HealthDeadlineAt:  status.HealthDeadlineAt,
	}
}

func (a *Agent) confirmUpdaterHealthy() {
	if a == nil || a.updater == nil {
		return
	}
	if err := a.updater.MarkHealthy(); err != nil {
		slog.Warn("agent updater health confirmation failed", "error", err)
	}
}

func (a *Agent) rollbackExpiredUpdateIfNeeded() {
	if a == nil || a.updater == nil {
		return
	}
	if err := a.updater.RollbackIfHealthExpired(); err != nil && !errors.Is(err, agentupdater.ErrRollbackUnavailable) {
		slog.Error("agent updater rollback check failed", "error", err)
	}
}
