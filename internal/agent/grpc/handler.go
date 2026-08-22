package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/agent/core"
	"github.com/creamcroissant/mgpanel/internal/agent/proxy"
	agentv1 "github.com/creamcroissant/mgpanel/pkg/pb/agent/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Switcher interface {
	Switch(ctx context.Context, req proxy.SwitchRequest) (*proxy.SwitchResult, error)
}

type CoreInstaller interface {
	InstallCore(ctx context.Context, req *agentv1.InstallCoreRequest) (*agentv1.InstallCoreResponse, error)
}

// Handler implements AgentServiceServer on the Agent side.
type Handler struct {
	agentv1.UnimplementedAgentServiceServer

	coreMgr    *core.Manager
	switcher   Switcher
	installer  CoreInstaller
	outputPath string
	logger     *slog.Logger
}

// NewHandler creates a new Agent gRPC handler.
func NewHandler(coreMgr *core.Manager, outputPath string, logger *slog.Logger, switcher Switcher, installer CoreInstaller) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{
		coreMgr:    coreMgr,
		switcher:   switcher,
		installer:  installer,
		outputPath: outputPath,
		logger:     logger,
	}
}

// GetCores returns available cores and instances on this Agent.
func (h *Handler) GetCores(ctx context.Context, _ *agentv1.GetCoresRequest) (*agentv1.GetCoresResponse, error) {
	if h.coreMgr == nil {
		return nil, status.Error(codes.FailedPrecondition, "core manager not initialized")
	}

	cores, err := h.coreMgr.ListCores(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list cores: %v", err)
	}

	resp := &agentv1.GetCoresResponse{
		Cores:     buildCoreInfos(cores),
		Instances: buildCoreInstances(h.coreMgr.ListInstances()),
	}
	return resp, nil
}

// SwitchCore switches from one core instance to another.
func (h *Handler) SwitchCore(ctx context.Context, req *agentv1.SwitchCoreRequest) (*agentv1.SwitchCoreResponse, error) {
	if h.coreMgr == nil {
		return &agentv1.SwitchCoreResponse{Success: false, Error: "core manager not initialized"}, nil
	}
	if req == nil {
		return &agentv1.SwitchCoreResponse{Success: false, Error: "empty request"}, nil
	}

	fromInstanceID := strings.TrimSpace(req.FromInstanceId)
	// fromInstanceID is optional for fresh start

	toCoreType := strings.TrimSpace(req.ToCoreType)
	if toCoreType == "" {
		return &agentv1.SwitchCoreResponse{Success: false, Error: "to_core_type is required"}, nil
	}

	if len(req.ConfigJson) == 0 {
		return &agentv1.SwitchCoreResponse{Success: false, Error: "config_json is required"}, nil
	}
	if !json.Valid(req.ConfigJson) {
		return &agentv1.SwitchCoreResponse{Success: false, Error: "config_json is not valid JSON"}, nil
	}

	useSwitcher := h.switcher != nil && req.ZeroDowntime

	configPath := ""
	var err error
	if !useSwitcher {
		configPath, err = h.writeConfig(req.SwitchId, req.ConfigJson)
		if err != nil {
			return &agentv1.SwitchCoreResponse{Success: false, Error: err.Error()}, nil
		}
	}

	// 端口列表由 Panel 显式下发，避免 Agent 解析配置推导端口
	listenPorts := make([]int, 0, len(req.ListenPorts))
	for _, port := range req.ListenPorts {
		listenPorts = append(listenPorts, int(port))
	}

	h.logger.Info("core switch requested",
		"from_instance", fromInstanceID,
		"to_core", toCoreType,
		"config_path", configPath,
		"switch_id", req.SwitchId,
		"listen_ports", listenPorts,
		"zero_downtime", req.ZeroDowntime,
	)

	var newInstanceID string
	if useSwitcher {
		if len(listenPorts) == 0 {
			return &agentv1.SwitchCoreResponse{Success: false, Error: "listen_ports is required for zero-downtime switch"}, nil
		}
		result, err := h.switcher.Switch(ctx, proxy.SwitchRequest{
			FromInstanceID: fromInstanceID,
			ToCoreType:     toCoreType,
			ConfigJSON:     req.ConfigJson,
			ListenPorts:    listenPorts,
		})
		if err != nil {
			h.logger.Error("zero-downtime switch failed", "error", err, "from_instance", fromInstanceID, "to_core", toCoreType)
			return &agentv1.SwitchCoreResponse{
				Success: false,
				Error:   err.Error(),
				Message: "zero-downtime switch failed",
			}, nil
		}
		if result == nil || !result.Success {
			errMsg := "zero-downtime switch failed"
			if result != nil && result.Error != "" {
				errMsg = result.Error
			}
			return &agentv1.SwitchCoreResponse{
				Success: false,
				Error:   errMsg,
				Message: "zero-downtime switch failed",
			}, nil
		}
		newInstanceID = result.NewInstanceID
	} else if fromInstanceID != "" {
		newInstanceID, err = h.coreMgr.SwitchCore(ctx, fromInstanceID, core.CoreType(toCoreType), configPath, listenPorts)
		if err != nil {
			h.logger.Error("core switch failed", "error", err, "from_instance", fromInstanceID, "to_core", toCoreType)
			return &agentv1.SwitchCoreResponse{
				Success: false,
				Error:   err.Error(),
				Message: "core switch failed",
			}, nil
		}
	} else {
		// Fresh start
		instanceID := fmt.Sprintf("%s-%d", toCoreType, time.Now().UnixNano())
		if err := h.coreMgr.StartInstance(ctx, core.CoreType(toCoreType), instanceID, configPath, listenPorts); err != nil {
			h.logger.Error("core start failed", "error", err, "to_core", toCoreType)
			return &agentv1.SwitchCoreResponse{
				Success: false,
				Error:   err.Error(),
				Message: "core start failed",
			}, nil
		}
		newInstanceID = instanceID
	}

	return &agentv1.SwitchCoreResponse{
		Success:       true,
		NewInstanceId: newInstanceID,
		Message:       "core switch completed",
	}, nil
}

// InstallCore performs controlled core install or upgrade on this Agent.
func (h *Handler) InstallCore(ctx context.Context, req *agentv1.InstallCoreRequest) (*agentv1.InstallCoreResponse, error) {
	if h.installer == nil {
		return &agentv1.InstallCoreResponse{
			Success: false,
			Error:   "core installer not initialized",
			Message: "core installer not initialized",
		}, nil
	}
	if req == nil {
		return &agentv1.InstallCoreResponse{
			Success: false,
			Error:   "empty request",
			Message: "empty request",
		}, nil
	}
	resp, err := h.installer.InstallCore(ctx, req)
	if err != nil {
		h.logger.Error("core install failed", "error", err, "core_type", strings.TrimSpace(req.CoreType), "action", strings.TrimSpace(req.Action))
		return &agentv1.InstallCoreResponse{
			Success:  false,
			Error:    err.Error(),
			Message:  "core install failed",
			CoreType: strings.TrimSpace(req.CoreType),
		}, nil
	}
	if resp == nil {
		return &agentv1.InstallCoreResponse{
			Success:  false,
			Error:    "empty installer response",
			Message:  "empty installer response",
			CoreType: strings.TrimSpace(req.CoreType),
		}, nil
	}
	return resp, nil
}

func (h *Handler) writeConfig(switchID string, content []byte) (string, error) {
	baseDir := "."
	if h.outputPath != "" {
		baseDir = filepath.Dir(h.outputPath)
	}
	if baseDir == "" {
		baseDir = "."
	}
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}

	name := "core-switch"
	if switchID != "" {
		if safe := sanitizeToken(switchID); safe != "" {
			name = name + "-" + safe
		}
	}

	path := filepath.Join(baseDir, name+".json")

	if h.outputPath != "" {
		// Use configured output path when possible to align with core expectations.
		path = h.outputPath
	}

	if err := os.WriteFile(path, content, 0644); err != nil {
		return "", fmt.Errorf("write config: %w", err)
	}
	return path, nil
}

func sanitizeToken(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, value)
}

func buildCoreInfos(infos []*core.CoreInfo) []*agentv1.CoreInfo {
	if len(infos) == 0 {
		return nil
	}
	out := make([]*agentv1.CoreInfo, 0, len(infos))
	for _, info := range infos {
		out = append(out, &agentv1.CoreInfo{
			Type:         string(info.Type),
			Version:      info.Version,
			Installed:    info.Installed,
			Capabilities: append([]string(nil), info.Capabilities...),
		})
	}
	return out
}

func buildCoreInstances(instances []*core.CoreInstance) []*agentv1.CoreInstance {
	if len(instances) == 0 {
		return nil
	}
	out := make([]*agentv1.CoreInstance, 0, len(instances))
	for _, inst := range instances {
		ports := make([]int32, 0, len(inst.ListenPorts))
		for _, port := range inst.ListenPorts {
			ports = append(ports, int32(port))
		}
		out = append(out, &agentv1.CoreInstance{
			Id:          inst.ID,
			CoreType:    string(inst.CoreType),
			Status:      string(inst.Status),
			ListenPorts: ports,
			ConfigPath:  inst.ConfigPath,
			ConfigHash:  inst.ConfigHash,
			Pid:         int32(inst.PID),
			StartedAt:   inst.StartedAt,
			Error:       inst.Error,
		})
	}
	return out
}
