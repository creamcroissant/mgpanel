package configcenter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/agent/proxy"
	"github.com/creamcroissant/mgpanel/internal/agent/transport"
	agentv1 "github.com/creamcroissant/mgpanel/pkg/pb/agent/v1"
)

// SwitcherClient defines the zero-downtime switcher operations used by SwitcherApplier.
type SwitcherClient interface {
	AddInstance(ctx context.Context, req proxy.AddInstanceRequest) (*proxy.AddInstanceResult, error)
	ReplaceInstance(ctx context.Context, req proxy.ReplaceInstanceRequest) (*proxy.AddInstanceResult, error)
	RemoveInstance(ctx context.Context, req proxy.RemoveInstanceRequest) error
	GetInstance(instanceID string) *proxy.SlotInfo
}

// SwitcherApplier wraps AgentBatchApplier's SyncOnce pattern to use the zero-downtime
// Switcher for multi-core deployments. Each core type (sing-box, xray) gets its own
// SwitcherApplier instance that uses AddInstance/ReplaceInstance instead of
// staged-apply directory swapping.
type SwitcherApplier struct {
	coreType string

	client   BatchClient
	switcher SwitcherClient
	logger   *slog.Logger
}

// NewSwitcherApplier creates a SwitcherApplier for multi-core agent deployments.
func NewSwitcherApplier(
	coreType string,
	client BatchClient,
	switcher SwitcherClient,
	logger *slog.Logger,
) (*SwitcherApplier, error) {
	if client == nil {
		return nil, fmt.Errorf("batch client is required")
	}
	if switcher == nil {
		return nil, fmt.Errorf("switcher is required")
	}
	normalized := normalizeCoreType(coreType)
	if normalized == "" {
		return nil, fmt.Errorf("core_type must be sing-box or xray")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &SwitcherApplier{
		coreType: normalized,
		client:   client,
		switcher: switcher,
		logger:   logger,
	}, nil
}

// SyncOnce fetches one apply batch and deploys it through the zero-downtime Switcher.
// It returns the latest applied revision on success, otherwise currentRevision with error.
func (a *SwitcherApplier) SyncOnce(ctx context.Context, currentRevision int64) (int64, error) {
	if currentRevision < 0 {
		return currentRevision, fmt.Errorf("current revision must be >= 0")
	}

	batchResp, err := a.client.GetApplyBatch(ctx, a.coreType, currentRevision)
	if err != nil {
		return currentRevision, fmt.Errorf("fetch apply batch: %w", err)
	}
	if batchResp == nil {
		return currentRevision, fmt.Errorf("fetch apply batch: empty response")
	}
	if !batchResp.GetSuccess() {
		message := strings.TrimSpace(batchResp.GetErrorMessage())
		if message == "" {
			message = "panel rejected apply batch"
		}
		return currentRevision, fmt.Errorf("fetch apply batch: %s", message)
	}
	if batchResp.GetNotModified() {
		return currentRevision, nil
	}

	runID := strings.TrimSpace(batchResp.GetRunId())
	if runID == "" {
		return currentRevision, fmt.Errorf("apply batch missing run_id")
	}
	targetRevision := batchResp.GetTargetRevision()
	previousRevision := batchResp.GetPreviousRevision()
	if targetRevision <= currentRevision {
		return currentRevision, fmt.Errorf("invalid target revision: %d <= current revision: %d", targetRevision, currentRevision)
	}

	artifacts, err := a.normalizeArtifacts(batchResp.GetArtifacts())
	if err != nil {
		reportErr := a.reportApplyResult(ctx, runID, targetRevision, false, applyRunStatusFailed, err.Error(), 0)
		if reportErr != nil {
			return currentRevision, errors.Join(err, fmt.Errorf("report failed status: %w", reportErr))
		}
		return currentRevision, err
	}

	// Assemble individual artifacts into a single core config JSON
	assembledConfig, err := assembleArtifactConfig(artifacts)
	if err != nil {
		reportErr := a.reportApplyResult(ctx, runID, targetRevision, false, applyRunStatusFailed, err.Error(), 0)
		if reportErr != nil {
			return currentRevision, errors.Join(err, fmt.Errorf("report failed status: %w", reportErr))
		}
		return currentRevision, err
	}

	// Report applying status
	if reportErr := a.reportApplyResult(ctx, runID, targetRevision, false, applyRunStatusApplying, "", 0); reportErr != nil {
		a.logger.Warn("failed to report applying status",
			"run_id", runID, "error", reportErr,
			"error_category", transport.ClassifyError(reportErr).String(),
		)
	}

	// Deploy via Switcher — first-time or update
	externPorts := extractExternalPorts(assembledConfig)
	if len(externPorts) == 0 {
		// If no port info found, use default listen port
		externPorts = []int{443}
		a.logger.Warn("no listen ports found in config, defaulting to 443", "core_type", a.coreType)
	}

	if a.isFirstDeployment() {
		result, addErr := a.switcher.AddInstance(ctx, proxy.AddInstanceRequest{
			CoreType:    a.coreType,
			ConfigJSON:  assembledConfig,
			ListenPorts: externPorts,
		})
		if addErr != nil {
			status := applyRunStatusFailed
			reportErr := a.reportApplyResult(ctx, runID, targetRevision, false, status, addErr.Error(), 0)
			if reportErr != nil {
				return currentRevision, errors.Join(addErr, fmt.Errorf("report failed status: %w", reportErr))
			}
			return currentRevision, addErr
		}
		a.logger.Info("added core instance via switcher",
			"core_type", a.coreType, "instance_id", result.InstanceID,
			"port_mappings", result.PortMappings)
	} else {
		oldInstanceID := findExistingInstanceID(a.switcher, a.coreType)
		result, replaceErr := a.switcher.ReplaceInstance(ctx, proxy.ReplaceInstanceRequest{
			OldInstanceID: oldInstanceID,
			NewCoreType:   a.coreType,
			NewConfigJSON: assembledConfig,
			ListenPorts:   externPorts,
		})
		if replaceErr != nil {
			status := applyRunStatusFailed
			if oldInstanceID != "" {
				status = applyRunStatusRolledBack
			}
			reportErr := a.reportApplyResult(ctx, runID, targetRevision, false, status, replaceErr.Error(), previousRevision)
			if reportErr != nil {
				return currentRevision, errors.Join(replaceErr, fmt.Errorf("report %s status: %w", status, reportErr))
			}
			return currentRevision, replaceErr
		}
		a.logger.Info("replaced core instance via switcher",
			"core_type", a.coreType, "old_instance_id", oldInstanceID,
			"new_instance_id", result.InstanceID)
	}

	// Report success
	reportErr := a.reportApplyResult(ctx, runID, targetRevision, true, applyRunStatusSuccess, "", 0)
	if reportErr != nil {
		return currentRevision, fmt.Errorf("apply succeeded but report success failed: %w", reportErr)
	}

	return targetRevision, nil
}

// isFirstDeployment returns true if no instance of this core type exists yet.
func (a *SwitcherApplier) isFirstDeployment() bool {
	return findExistingInstanceID(a.switcher, a.coreType) == ""
}

// findExistingInstanceID searches the Switcher for an active instance of the given core type.
func findExistingInstanceID(switcher SwitcherClient, coreType string) string {
	// Try common instance ID prefixes
	candidates := []string{
		coreType,
		fmt.Sprintf("%s-0", coreType),
	}
	for _, c := range candidates {
		if slot := switcher.GetInstance(c); slot != nil && slot.CoreType == coreType {
			return slot.InstanceID
		}
	}
	return ""
}

// normalizeArtifacts validates artifacts from the apply batch.
func (a *SwitcherApplier) normalizeArtifacts(artifacts []*agentv1.ApplyArtifact) ([]normalizedArtifact, error) {
	return normalizeArtifacts(artifacts)
}

// assembleArtifactConfig merges individual artifact JSON fragments into a
// single core config JSON document by combining top-level keys.
func assembleArtifactConfig(artifacts []normalizedArtifact) ([]byte, error) {
	merged := make(map[string]any)
	for _, artifact := range artifacts {
		var doc map[string]any
		if err := json.Unmarshal(artifact.content, &doc); err != nil {
			return nil, fmt.Errorf("parse artifact %s: %w", artifact.filename, err)
		}
		for key, value := range doc {
			if existing, ok := merged[key]; ok {
				// Merge arrays by appending (for inbounds, routing rules, etc.)
				switch existingArr := existing.(type) {
				case []any:
					if newArr, ok := value.([]any); ok {
						merged[key] = append(existingArr, newArr...)
						continue
					}
				}
				// For scalars/objects, last one wins
				merged[key] = value
			} else {
				merged[key] = value
			}
		}
	}
	return json.Marshal(merged)
}

// extractExternalPorts attempts to extract listening ports from a core config JSON.
func extractExternalPorts(configJSON []byte) []int {
	var doc struct {
		Inbounds []struct {
			Port     int `json:"port"`
			PortSing int `json:"listen_port"`
		} `json:"inbounds"`
	}
	if err := json.Unmarshal(configJSON, &doc); err != nil {
		return nil
	}
	seen := make(map[int]bool)
	var ports []int
	for _, inbound := range doc.Inbounds {
		port := inbound.Port
		if port <= 0 {
			port = inbound.PortSing
		}
		if port > 0 && !seen[port] {
			seen[port] = true
			ports = append(ports, port)
		}
	}
	return ports
}

func (a *SwitcherApplier) reportApplyResult(
	ctx context.Context,
	runID string,
	targetRevision int64,
	success bool,
	statusValue string,
	errorMessage string,
	rollbackRevision int64,
) error {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return fmt.Errorf("report apply result: run_id is required")
	}
	report := &agentv1.ApplyRunReport{
		RunId:            runID,
		CoreType:         a.coreType,
		TargetRevision:   targetRevision,
		Success:          success,
		Status:           statusValue,
		ErrorMessage:     strings.TrimSpace(errorMessage),
		RollbackRevision: rollbackRevision,
		FinishedAt:       time.Now().Unix(),
	}
	resp, err := a.client.ReportApplyRun(ctx, report)
	if err != nil {
		return fmt.Errorf("report apply result: %w", err)
	}
	if resp != nil && !resp.GetSuccess() {
		message := strings.TrimSpace(resp.GetMessage())
		if message == "" {
			message = "panel rejected apply report"
		}
		return fmt.Errorf("report apply result: %s", message)
	}
	return nil
}

// Ensure normalizeCoreType is available — it's defined in the configcenter package.
