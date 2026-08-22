package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/agent/command"
	"github.com/creamcroissant/mgpanel/internal/agent/core"
	"github.com/creamcroissant/mgpanel/internal/agent/protocol"
	agentv1 "github.com/creamcroissant/mgpanel/pkg/pb/agent/v1"
)

const (
	operationEventScopeCoreOperation = "core_operation"

	operationEventLevelInfo  = "info"
	operationEventLevelWarn  = "warn"
	operationEventLevelError = "error"
)

type coreOperationClient interface {
	GetCoreOperations(ctx context.Context, statuses []string, limit int32) (*agentv1.GetCoreOperationsResponse, error)
	ReportCoreOperation(ctx context.Context, report *agentv1.ReportCoreOperationRequest) (*agentv1.ReportCoreOperationResponse, error)
}

type operationEventReporter interface {
	ReportOperationEvent(ctx context.Context, events []*agentv1.OperationEvent) (*agentv1.ReportOperationEventResponse, error)
}

func (a *Agent) syncCoreOperations(ctx context.Context) {
	if a.coreOperations == nil {
		slog.Debug("skip core operation sync: grpc client unavailable")
		return
	}
	if a.commandQueue == nil {
		a.syncCoreOperationsDirect(ctx)
		return
	}
	if !a.beginCoreOperationSync() {
		slog.Debug("core operation already queued or in flight, skip re-entry")
		return
	}

	slog.Debug("sync core operations: requesting pending/claimed tasks")
	resp, err := a.coreOperations.GetCoreOperations(ctx, []string{"pending", "claimed"}, 1)
	if err != nil {
		a.endCoreOperationSync()
		slog.Error("Failed to fetch core operations", "error", err)
		return
	}
	if len(resp.GetOperations()) == 0 {
		a.endCoreOperationSync()
		slog.Debug("sync core operations: no tasks returned")
		return
	}
	for _, operation := range resp.GetOperations() {
		if operation == nil {
			continue
		}
		a.submitCoreOperation(ctx, operation)
		return
	}
	a.endCoreOperationSync()
}

func (a *Agent) syncCoreOperationsDirect(ctx context.Context) {
	slog.Debug("sync core operations: requesting pending/claimed tasks")
	resp, err := a.coreOperations.GetCoreOperations(ctx, []string{"pending", "claimed"}, 1)
	if err != nil {
		slog.Error("Failed to fetch core operations", "error", err)
		return
	}
	if len(resp.GetOperations()) == 0 {
		slog.Debug("sync core operations: no tasks returned")
		return
	}
	for _, operation := range resp.GetOperations() {
		if operation == nil {
			continue
		}
		a.runCoreOperation(ctx, operation)
	}
}

func (a *Agent) submitCoreOperation(ctx context.Context, operation *agentv1.CoreOperation) {
	task := command.Task{ID: operation.GetId(), OperationType: agentCommandActionCoreOperation, RequestPayload: operation.GetRequestPayload()}
	err := a.commandQueue.SubmitWithHandler(ctx, task, func(ctx context.Context, task command.Task, reporter command.Reporter) command.Result {
		defer a.endCoreOperationSync()
		_ = reporter.Report(ctx, command.Event{EventType: command.EventTypeProgress, Status: command.StatusInProgress, Phase: "executing", Level: command.LevelInfo, Message: "core operation execution started"})
		return a.runCoreOperation(ctx, operation)
	})
	if err != nil {
		a.endCoreOperationSync()
		a.handleCoreOperationQueueRejection(ctx, operation, err)
	}
}

func (a *Agent) runCoreOperation(ctx context.Context, operation *agentv1.CoreOperation) command.Result {
	slog.Info("sync core operations: claimed task", "operation_id", operation.GetId(), "type", operation.GetOperationType(), "core_type", operation.GetCoreType(), "status", operation.GetStatus())
	a.reportCoreOperationEvent(ctx, operation, "claimed", operationEventLevelInfo, "core operation claimed", map[string]any{"status": operation.GetStatus()})
	a.reportCoreOperationEvent(ctx, operation, "executing", operationEventLevelInfo, "core operation execution started", nil)
	statusValue, resultPayload, errMessage := a.executeCoreOperation(ctx, operation)
	a.reportCoreOperationTerminalEvent(ctx, operation, statusValue, errMessage)
	if _, err := a.coreOperations.ReportCoreOperation(ctx, &agentv1.ReportCoreOperationRequest{OperationId: operation.GetId(), Status: statusValue, ResultPayload: resultPayload, ErrorMessage: errMessage}); err != nil {
		slog.Error("Failed to report core operation", "operation_id", operation.GetId(), "error", err)
	}
	slog.Info("sync core operations: reported task result", "operation_id", operation.GetId(), "status", statusValue)
	return commandResultFromCoreOperation(statusValue, resultPayload, errMessage)
}

func (a *Agent) handleCoreOperationQueueRejection(ctx context.Context, operation *agentv1.CoreOperation, err error) {
	phase := "queue_rejected"
	message := "core operation rejected by command queue"
	if errors.Is(err, command.ErrQueueFull) {
		phase = "queue_full"
		message = "core operation queue full"
	}
	errMessage := err.Error()
	a.reportCoreOperationEvent(ctx, operation, phase, operationEventLevelWarn, message, map[string]any{"error": errMessage})
	a.reportCoreOperationTerminalEvent(ctx, operation, "failed", errMessage)
	if _, reportErr := a.coreOperations.ReportCoreOperation(ctx, &agentv1.ReportCoreOperationRequest{OperationId: operation.GetId(), Status: "failed", ErrorMessage: errMessage}); reportErr != nil {
		slog.Error("Failed to report core operation queue rejection", "operation_id", operation.GetId(), "error", reportErr)
	}
	slog.Warn("core operation rejected by command queue", "operation_id", operation.GetId(), "error", err)
}

func commandResultFromCoreOperation(statusValue string, resultPayload []byte, errMessage string) command.Result {
	result := command.Result{Payload: resultPayload, ErrorMessage: strings.TrimSpace(errMessage)}
	switch strings.TrimSpace(statusValue) {
	case "completed":
		result.Status = command.StatusSuccess
		result.Phase = "completed"
		result.Level = command.LevelInfo
		result.Message = "core operation completed"
	case "rolled_back":
		result.Status = command.StatusFailed
		result.Phase = "rolled_back"
		result.Level = command.LevelWarn
		result.Message = "core operation rolled back"
	default:
		result.Status = command.StatusFailed
		result.Phase = "failed"
		result.Level = command.LevelError
		result.Message = "core operation failed"
	}
	return result
}

func (a *Agent) executeCoreOperation(ctx context.Context, operation *agentv1.CoreOperation) (string, []byte, string) {
	if operation == nil {
		return "failed", nil, "empty core operation"
	}
	switch strings.TrimSpace(operation.GetOperationType()) {
	case "install":
		a.reportCoreOperationEvent(ctx, operation, "installing", operationEventLevelInfo, "core install started", nil)
		return a.executeInstallCore(ctx, operation)
	case "switch":
		a.reportCoreOperationEvent(ctx, operation, "config_applying", operationEventLevelInfo, "core switch config apply started", nil)
		return a.executeSwitchCore(ctx, operation)
	case "create":
		a.reportCoreOperationEvent(ctx, operation, "config_applying", operationEventLevelInfo, "core instance config apply started", nil)
		return a.executeCreateCoreInstance(ctx, operation)
	case "ensure":
		a.reportCoreOperationEvent(ctx, operation, "ensuring", operationEventLevelInfo, "core ensure started", nil)
		return a.executeEnsureCore(ctx, operation)
	default:
		return "failed", nil, fmt.Sprintf("unsupported core operation type %q", operation.GetOperationType())
	}
}

func (a *Agent) executeInstallCore(ctx context.Context, operation *agentv1.CoreOperation) (string, []byte, string) {
	if a == nil {
		return "failed", nil, "agent is nil"
	}
	var payload agentv1.InstallCorePayload
	if err := json.Unmarshal(operation.GetRequestPayload(), &payload); err != nil {
		return "failed", nil, err.Error()
	}
	installer := core.NewInstaller(core.InstallerConfig{
			ScriptPath: a.cfg.Core.InstallScriptPath,
			SingBoxBinaryPath: a.cfg.Core.SingBoxBinaryPath,
			XrayBinaryPath: a.cfg.Core.XrayBinaryPath,
			ServiceName: a.cfg.Protocol.ServiceName,
			SingBoxReleaseRepo: a.cfg.Core.SingBoxReleaseRepo,
			XrayReleaseRepo: a.cfg.Core.XrayReleaseRepo,
			ReleaseBaseURL: a.cfg.Core.ReleaseBaseURL,
			CoreInstallDir: a.cfg.Core.CoreInstallDir,
	}, nil, nil)
	resp, err := installer.InstallCore(ctx, &agentv1.InstallCoreRequest{CoreType: operation.GetCoreType(), Action: payload.Action, Version: payload.Version, Channel: payload.Channel, Flavor: payload.Flavor, Activate: payload.Activate, RequestId: payload.RequestId})
	a.invalidateCapabilitiesCache()
	if err != nil {
		return "failed", nil, err.Error()
	}
	data, _ := json.Marshal(resp)
	if resp.GetRolledBack() {
		return "rolled_back", data, resp.GetError()
	}
	if !resp.GetSuccess() {
		return "failed", data, resp.GetError()
	}
	return "completed", data, ""
}

func (a *Agent) executeSwitchCore(ctx context.Context, operation *agentv1.CoreOperation) (string, []byte, string) {
	var payload agentv1.SwitchCorePayload
	if err := json.Unmarshal(operation.GetRequestPayload(), &payload); err != nil {
		return "failed", nil, err.Error()
	}
	configPath, err := a.resolveOperationConfigPath(payload.GetToCoreType(), payload.GetFromInstanceId(), "panel-core-switch")
	if err != nil {
		return "failed", nil, err.Error()
	}
	filename := strings.TrimSuffix(filepathBase(configPath), ".json") + ".json"
	if err := a.protoMgr.ApplyConfigWithCore(ctx, payload.GetToCoreType(), filename, payload.GetConfigJson()); err != nil {
		return "failed", nil, err.Error()
	}
	a.reportCoreOperationEvent(ctx, operation, "switching", operationEventLevelInfo, "core switch started", map[string]any{"to_core_type": payload.GetToCoreType()})
	listenPorts := int32SliceToInt(payload.GetListenPorts())
	if len(listenPorts) == 0 {
		listenPorts = a.determineListenPort()
	}
	newInstanceID, err := a.coreMgr.SwitchCore(ctx, payload.GetFromInstanceId(), core.CoreType(payload.GetToCoreType()), configPath, listenPorts)
	if err != nil {
		return "failed", nil, err.Error()
	}
	resultPayload, _ := json.Marshal(map[string]any{"new_instance_id": newInstanceID, "core_type": payload.GetToCoreType(), "config_path": configPath})
	return "completed", resultPayload, ""
}

func (a *Agent) executeCreateCoreInstance(ctx context.Context, operation *agentv1.CoreOperation) (string, []byte, string) {
	var payload agentv1.CreateCoreInstancePayload
	if err := json.Unmarshal(operation.GetRequestPayload(), &payload); err != nil {
		return "failed", nil, err.Error()
	}
	configPath, err := a.resolveOperationConfigPath(operation.GetCoreType(), payload.GetInstanceId(), "panel-core-create")
	if err != nil {
		return "failed", nil, err.Error()
	}
	filename := filepathBase(configPath)
	if err := a.protoMgr.ApplyConfigWithCore(ctx, operation.GetCoreType(), filename, payload.GetConfigJson()); err != nil {
		return "failed", nil, err.Error()
	}
	a.reportCoreOperationEvent(ctx, operation, "starting_instance", operationEventLevelInfo, "core instance start requested", map[string]any{"instance_id": payload.GetInstanceId()})
	if err := a.coreMgr.StartInstance(ctx, core.CoreType(operation.GetCoreType()), payload.GetInstanceId(), configPath, a.determineListenPort()); err != nil {
		return "failed", nil, err.Error()
	}
	resultPayload, _ := json.Marshal(map[string]any{"instance_id": payload.GetInstanceId(), "core_type": operation.GetCoreType(), "config_path": configPath})
	return "completed", resultPayload, ""
}

func (a *Agent) executeEnsureCore(ctx context.Context, operation *agentv1.CoreOperation) (string, []byte, string) {
	var payload agentv1.EnsureCorePayload
	if err := json.Unmarshal(operation.GetRequestPayload(), &payload); err != nil {
		return "failed", nil, err.Error()
	}
	installer := core.NewInstaller(core.InstallerConfig{
			ScriptPath: a.cfg.Core.InstallScriptPath,
			SingBoxBinaryPath: a.cfg.Core.SingBoxBinaryPath,
			XrayBinaryPath: a.cfg.Core.XrayBinaryPath,
			ServiceName: a.cfg.Protocol.ServiceName,
			SingBoxReleaseRepo: a.cfg.Core.SingBoxReleaseRepo,
			XrayReleaseRepo: a.cfg.Core.XrayReleaseRepo,
			ReleaseBaseURL: a.cfg.Core.ReleaseBaseURL,
			CoreInstallDir: a.cfg.Core.CoreInstallDir,
	}, nil, nil)
	resp, err := installer.InstallCore(ctx, &agentv1.InstallCoreRequest{CoreType: operation.GetCoreType(), Action: "ensure", Version: payload.Version, Channel: payload.Channel, Flavor: payload.Flavor})
	a.invalidateCapabilitiesCache()
	if err != nil {
		return "failed", nil, err.Error()
	}
	data, _ := json.Marshal(resp)
	if !resp.GetSuccess() {
		return "failed", data, resp.GetError()
	}
	return "completed", data, ""
}

func (a *Agent) reportCoreOperationTerminalEvent(ctx context.Context, operation *agentv1.CoreOperation, statusValue, errMessage string) {
	phase := "completed"
	level := operationEventLevelInfo
	message := "core operation completed"
	payload := map[string]any{"status": statusValue}
	if strings.TrimSpace(errMessage) != "" {
		payload["error"] = errMessage
	}
	switch strings.TrimSpace(statusValue) {
	case "completed":
	case "rolled_back":
		phase = "rolled_back"
		level = operationEventLevelWarn
		message = "core operation rolled back"
	case "failed":
		phase = "failed"
		level = operationEventLevelError
		message = "core operation failed"
	default:
		phase = "finished"
		level = operationEventLevelWarn
		message = "core operation finished with unexpected status"
	}
	a.reportCoreOperationEvent(ctx, operation, phase, level, message, payload)
}

func (a *Agent) reportCoreOperationEvent(ctx context.Context, operation *agentv1.CoreOperation, phase, level, message string, payload map[string]any) {
	if a == nil || a.operationEvents == nil || operation == nil {
		return
	}
	targetID := strings.TrimSpace(operation.GetId())
	if targetID == "" {
		return
	}
	eventPayload := encodeOperationEventPayload(payload)
	resp, err := a.operationEvents.ReportOperationEvent(ctx, []*agentv1.OperationEvent{{
		Scope:       operationEventScopeCoreOperation,
		TargetId:    targetID,
		Phase:       strings.TrimSpace(phase),
		Level:       strings.TrimSpace(level),
		Message:     strings.TrimSpace(message),
		OccurredAt:  time.Now().Unix(),
		PayloadJson: eventPayload,
	}})
	if err != nil {
		slog.Warn("failed to report core operation event", "operation_id", targetID, "phase", phase, "error", err)
		return
	}
	if resp != nil && !resp.GetSuccess() {
		slog.Warn("panel rejected core operation event", "operation_id", targetID, "phase", phase, "message", resp.GetMessage())
	}
}

func encodeOperationEventPayload(payload map[string]any) []byte {
	if len(payload) == 0 {
		return []byte(`{}`)
	}
	data, err := json.Marshal(payload)
	if err != nil || !json.Valid(data) {
		return []byte(`{}`)
	}
	return data
}

func (a *Agent) resolveOperationConfigPath(coreType, instanceID, fallbackPrefix string) (string, error) {
	trimmedCore := protocol.NormalizeCoreType(coreType)
	trimmedInstance := strings.TrimSpace(instanceID)
	if trimmedInstance == "" {
		trimmedInstance = fallbackPrefix
	}
	filename, err := protocol.SanitizeFilename(trimmedInstance + ".json")
	if err != nil {
		return "", err
	}
	dir := strings.TrimSpace(a.cfg.Protocol.ConfigDir)
	if dir == "" {
		dir = "/etc/sing-box/conf"
	}
	switch trimmedCore {
	case "xray":
		if strings.TrimSpace(a.cfg.Protocol.ManagedConfigDir) != "" {
			dir = a.cfg.Protocol.ManagedConfigDir
		}
	case "sing-box":
		if strings.TrimSpace(a.cfg.Protocol.ManagedConfigDir) != "" {
			dir = a.cfg.Protocol.ManagedConfigDir
		}
	}
	return dir + "/" + filename, nil
}

func filepathBase(path string) string {
	idx := strings.LastIndex(path, "/")
	if idx == -1 {
		return path
	}
	return path[idx+1:]
}

func (a *Agent) invalidateCapabilitiesCache() {
	if a == nil {
		return
	}
	a.capsMu.Lock()
	a.cachedCaps = nil
	a.capsDetectedAt = 0
	a.capsMu.Unlock()
}

func int32SliceToInt(input []int32) []int {
	if len(input) == 0 {
		return nil
	}
	result := make([]int, 0, len(input))
	for _, value := range input {
		result = append(result, int(value))
	}
	return result
}
