package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/creamcroissant/mgpanel/internal/agent/command"
	"github.com/creamcroissant/mgpanel/internal/agent/transport"
)

func (a *Agent) sync(ctx context.Context) {
	if !a.beginSync() {
		slog.Debug("sync already in flight, skip re-entry")
		return
	}
	defer a.endSync()

	a.rollbackExpiredUpdateIfNeeded()
	if a.conn != nil {
		state := a.conn.CheckConnection(ctx)
		if state != transport.StateConnected {
			slog.Warn("gRPC connection not ready, skip sync", "state", state.String())
			return
		}
	}
	a.syncGRPC(ctx)
}

func (a *Agent) syncGRPC(ctx context.Context) {
	a.syncAgentHost(ctx, "", "")
	a.syncRoutesThenApply(ctx)
	a.syncCoreOperations(ctx)
	a.syncAgentCommands(ctx)

	// Sync mesh configuration — auto-initialize if agent has a mesh record
	mgr, prober := a.getMesh()
	if mgr == nil {
		a.tryEnableMesh(ctx)
		mgr, prober = a.getMesh()
	}
	if mgr != nil {
		a.syncMeshConfig(ctx, mgr)
	}

	// Update mesh probe targets and sync latencies
	if prober != nil {
		prober.UpdateTargets(ctx)
	}

	// NodeID kept for compatibility; gRPC identifies agent host by token
	nodeID := int32(a.cfg.Panel.NodeID)

	// Fetch Config via gRPC
	cfgResp, err := a.grpc.GetConfig(ctx, nodeID, a.configETag)
	if err != nil {
		slog.Error("Failed to fetch config via gRPC", "error", err)
		return
	}

	if !cfgResp.NotModified {
		a.configETag = cfgResp.Etag
		slog.Info("Config updated via gRPC", "version", cfgResp.Version)
		// Apply new config
		if len(cfgResp.ConfigJson) > 0 {
			if err := a.protoMgr.ApplyConfigWithCore(ctx, "", "config.json", cfgResp.ConfigJson); err != nil {
				slog.Error("Failed to apply config", "error", err)
			} else {
				slog.Info("Successfully applied new config", "version", cfgResp.Version)
			}
		}
	}

	// Fetch Users via gRPC
	usersResp, err := a.grpc.GetUsers(ctx, nodeID, a.usersETag, 0)
	if err != nil {
		slog.Error("Failed to fetch users via gRPC", "error", err)
		return
	}

	if !usersResp.NotModified {
		a.usersETag = usersResp.Etag
		a.refreshUserEmailMapping(usersResp.Users)
		slog.Info("Users updated via gRPC", "count", len(usersResp.Users))

		// Convert users to protocol.UserConfig and inject into config
		if err := a.applyUsers(ctx, usersResp.Users); err != nil {
			slog.Error("Failed to apply users", "error", err)
		} else {
			slog.Info("Successfully applied users to config", "count", len(usersResp.Users))
		}
	}
}

func (a *Agent) syncApplyBatch(ctx context.Context) {
	if a.batchApplier == nil {
		return
	}
	if a.commandQueue == nil {
		a.syncApplyBatchDirect(ctx)
		return
	}
	if !a.beginBatchSync() {
		slog.Debug("apply batch sync already in flight, skip re-entry")
		return
	}

	currentRevision := a.getApplyRevision()
	task := command.Task{ID: fmt.Sprintf("config-apply-%d", currentRevision), OperationType: agentCommandActionConfigApply}

	done := make(chan struct{})

	var (
		casReleased atomic.Bool
		releaseCAS  = func() {
			if casReleased.CompareAndSwap(false, true) {
				a.endBatchSync()
			}
		}
	)

	err := a.commandQueue.SubmitWithHandler(ctx, task, func(ctx context.Context, task command.Task, reporter command.Reporter) command.Result {
		defer releaseCAS()
		defer close(done)
		_ = reporter.Report(ctx, command.Event{EventType: command.EventTypeProgress, Status: command.StatusInProgress, Phase: "applying", Level: command.LevelInfo, Message: "config apply sync started"})
		return a.runApplyBatch(ctx, currentRevision)
	})
	if err != nil {
		releaseCAS()
		close(done)
		slog.Warn("config apply rejected by command queue", "current_revision", currentRevision, "error", err)
		return
	}

	// Watchdog: release CAS if queue stops or times out
	a.watchdogWg.Add(1)
	go func() {
		watchdogTimer := time.NewTimer(2 * time.Minute)
		defer a.watchdogWg.Done()
		defer watchdogTimer.Stop()
		select {
		case <-done:
			// handler completed normally, CAS already released
		case <-a.commandQueue.Stopped():
			slog.Warn("command queue stopped during batch sync, releasing CAS")
			releaseCAS()
		case <-watchdogTimer.C:
			slog.Warn("batch sync watchdog timeout, releasing CAS")
			releaseCAS()
		}
	}()
}

func (a *Agent) syncApplyBatchDirect(ctx context.Context) {
	if !a.beginBatchSync() {
		slog.Debug("apply batch sync already in flight, skip re-entry")
		return
	}
	defer a.endBatchSync()
	_ = a.runApplyBatch(ctx, a.getApplyRevision())
}

func (a *Agent) runApplyBatch(ctx context.Context, currentRevision int64) command.Result {
	nextRevision, err := a.batchApplier.SyncOnce(ctx, currentRevision)
	if err != nil {
		slog.Error("Failed to sync apply batch", "current_revision", currentRevision, "error", err)
		return command.Result{Status: command.StatusFailed, Phase: "failed", Level: command.LevelError, Message: "config apply sync failed", ErrorMessage: err.Error()}
	}
	payload, _ := json.Marshal(map[string]any{"previous_revision": currentRevision, "revision": nextRevision})
	if nextRevision != currentRevision {
		a.setApplyRevision(nextRevision)
		slog.Info("Apply batch synced", "revision", nextRevision)
		return command.Result{Status: command.StatusSuccess, Phase: "completed", Level: command.LevelInfo, Message: "config apply synced", Payload: payload}
	}
	return command.Result{Status: command.StatusSuccess, Phase: "not_modified", Level: command.LevelInfo, Message: "config apply not modified", Payload: payload}
}

func (a *Agent) beginSync() bool {
	return a.syncInFlight.CompareAndSwap(false, true)
}

func (a *Agent) endSync() {
	a.syncInFlight.Store(false)
}

func (a *Agent) beginBatchSync() bool {
	return a.batchSyncInFlight.CompareAndSwap(false, true)
}

func (a *Agent) endBatchSync() {
	a.batchSyncInFlight.Store(false)
}

func (a *Agent) beginCoreOperationSync() bool {
	return a.coreOperationSyncInFlight.CompareAndSwap(false, true)
}

func (a *Agent) endCoreOperationSync() {
	a.coreOperationSyncInFlight.Store(false)
}

func (a *Agent) getApplyRevision() int64 {
	return a.applyRevision.Load()
}

func (a *Agent) setApplyRevision(revision int64) {
	a.applyRevision.Store(revision)
}
