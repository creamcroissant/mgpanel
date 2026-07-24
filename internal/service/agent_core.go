package service

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/creamcroissant/xboard/internal/grpc/client"
	"github.com/creamcroissant/xboard/internal/repository"
	"github.com/creamcroissant/xboard/internal/template"
	agentv1 "github.com/creamcroissant/xboard/pkg/pb/agent/v1"
)

const (
	switchStatusPending   = "pending"
	switchStatusRunning   = "in_progress"
	switchStatusCompleted = "completed"
	switchStatusFailed    = "failed"
)

// AgentCoreService 定义 Panel 侧的核心管理逻辑。
type AgentCoreService interface {
	GetCores(ctx context.Context, agentHostID int64) ([]*agentv1.CoreInfo, error)
	GetInstances(ctx context.Context, agentHostID int64) ([]*repository.AgentCoreInstance, error)
	CreateInstance(ctx context.Context, req CreateInstanceRequest) (*repository.CoreOperation, error)
	DeleteInstance(ctx context.Context, agentHostID int64, instanceID string) error
	SwitchCore(ctx context.Context, req SwitchCoreRequest) (*repository.CoreOperation, error)
	InstallCore(ctx context.Context, req InstallCoreRequest) (*repository.CoreOperation, error)
	GetSwitchLogs(ctx context.Context, filter SwitchLogFilter) ([]*repository.AgentCoreSwitchLog, int64, error)
	ConvertConfig(ctx context.Context, req ConvertRequest) (*ConvertResult, error)
	ListOperations(ctx context.Context, req ListCoreOperationsRequest) ([]*repository.CoreOperation, int64, error)
	GetOperation(ctx context.Context, operationID string) (*repository.CoreOperation, error)
}

// CreateInstanceRequest 定义创建核心实例的请求参数。
type CreateInstanceRequest struct {
	AgentHostID      int64
	CoreType         string
	InstanceID       string
	ConfigTemplateID int64
	ConfigJSON       json.RawMessage
	OperatorID       *int64
}

// SwitchCoreRequest 定义核心切换请求参数。
type SwitchCoreRequest struct {
	AgentHostID      int64
	FromInstanceID   string
	ToCoreType       string
	ConfigTemplateID int64
	ConfigJSON       json.RawMessage
	SwitchID         string
	ListenPorts      []int
	ZeroDowntime     *bool
	OperatorID       *int64
}

// InstallCoreRequest 定义核心安装/升级请求参数。
type InstallCoreRequest struct {
	AgentHostID int64
	CoreType    string
	Action      string
	Version     string
	Channel     string
	Flavor      string
	Activate    bool
	RequestID   string
	OperatorID  *int64
}

// SwitchResult 返回切换结果与日志信息。
type SwitchResult struct {
	Success        bool   `json:"success"`
	NewInstanceID  string `json:"new_instance_id,omitempty"`
	Message        string `json:"message,omitempty"`
	Error          string `json:"error,omitempty"`
	SwitchLogID    int64  `json:"switch_log_id,omitempty"`
	CompletedAt    *int64 `json:"completed_at,omitempty"`
	FromInstanceID string `json:"from_instance_id,omitempty"`
	ToCoreType     string `json:"to_core_type,omitempty"`
}

// InstallCoreResult 返回核心安装/升级结果。
type InstallCoreResult struct {
	Success         bool   `json:"success"`
	Changed         bool   `json:"changed"`
	Message         string `json:"message,omitempty"`
	Error           string `json:"error,omitempty"`
	CoreType        string `json:"core_type,omitempty"`
	Version         string `json:"version,omitempty"`
	PreviousVersion string `json:"previous_version,omitempty"`
	Activated       bool   `json:"activated"`
	RolledBack      bool   `json:"rolled_back"`
}

// SwitchLogFilter 定义切换日志查询条件。
type SwitchLogFilter struct {
	AgentHostID int64
	Status      *string
	StartAt     *int64
	EndAt       *int64
	Limit       int
	Offset      int
}

// ConvertRequest 定义配置转换的输入参数。
type ConvertRequest struct {
	SourceCore string
	TargetCore string
	ConfigJSON json.RawMessage
}

// ConvertResult 返回转换后的配置与警告信息。
type ConvertResult struct {
	ConfigJSON json.RawMessage `json:"config_json"`
	Warnings   []string        `json:"warnings,omitempty"`
}

// agentCoreService 组合核心管理相关依赖与配置。
type agentCoreService struct {
	agentHosts          repository.AgentHostRepository
	instances           repository.AgentCoreInstanceRepository
	switchLogs          repository.AgentCoreSwitchLogRepository
	templates           repository.ConfigTemplateRepository
	converters          *template.ConverterRegistry
	logger              *slog.Logger
	grpcTLS             *client.TLSConfig
	grpcTimeout         client.TimeoutConfig
	grpcKeepalive       *client.KeepaliveConfig
	grpcPort            string
	grpcClientFunc      func(ctx context.Context, cfg client.Config) (*client.AgentClient, error)
	operations          CoreOperationService
	snapshots           CoreSnapshotService
	binaryVersionStates repository.BinaryVersionStateRepository
}

// NewAgentCoreService 组装核心管理服务。
func NewAgentCoreService(
	agentHosts repository.AgentHostRepository,
	instances repository.AgentCoreInstanceRepository,
	switchLogs repository.AgentCoreSwitchLogRepository,
	templates repository.ConfigTemplateRepository,
	converters *template.ConverterRegistry,
	logger *slog.Logger,
) AgentCoreService {
	return NewAgentCoreServiceWithOptions(agentHosts, instances, switchLogs, templates, converters, logger, AgentCoreServiceOptions{})
}

// AgentCoreServiceOptions 定义 gRPC 客户端构造参数。
type AgentCoreServiceOptions struct {
	GRPCTLS             *client.TLSConfig
	Timeout             client.TimeoutConfig
	Keepalive           *client.KeepaliveConfig
	GRPCPort            string
	ClientFactory       func(ctx context.Context, cfg client.Config) (*client.AgentClient, error)
	Operations          repository.CoreOperationRepository
	OperationGuard      AgentOperationGuard
	BinaryVersionStates repository.BinaryVersionStateRepository
}

// NewAgentCoreServiceWithOptions 构造可定制的核心管理服务。
func NewAgentCoreServiceWithOptions(
	agentHosts repository.AgentHostRepository,
	instances repository.AgentCoreInstanceRepository,
	switchLogs repository.AgentCoreSwitchLogRepository,
	templates repository.ConfigTemplateRepository,
	converters *template.ConverterRegistry,
	logger *slog.Logger,
	opts AgentCoreServiceOptions,
) AgentCoreService {
	if logger == nil {
		logger = slog.Default()
	}
	grpcPort := opts.GRPCPort
	if grpcPort == "" {
		grpcPort = ":19090"
	}
	factory := opts.ClientFactory
	if factory == nil {
		factory = client.NewAgentClient
	}
	return &agentCoreService{
		agentHosts:          agentHosts,
		instances:           instances,
		switchLogs:          switchLogs,
		templates:           templates,
		converters:          converters,
		logger:              logger,
		grpcTLS:             opts.GRPCTLS,
		grpcTimeout:         opts.Timeout,
		grpcKeepalive:       opts.Keepalive,
		grpcPort:            grpcPort,
		grpcClientFunc:      factory,
		operations:          NewCoreOperationService(opts.Operations, opts.OperationGuard),
		snapshots:           NewCoreSnapshotService(agentHosts, instances),
		binaryVersionStates: opts.BinaryVersionStates,
	}
}

func (s *agentCoreService) GetCores(ctx context.Context, agentHostID int64) ([]*agentv1.CoreInfo, error) {
	if s.agentHosts == nil {
		return nil, fmt.Errorf("agent host repository unavailable / 节点仓库不可用")
	}
	host, err := s.agentHosts.FindByID(ctx, agentHostID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	instances, err := s.GetInstances(ctx, agentHostID)
	if err != nil {
		return nil, err
	}
	snapshots, err := s.snapshots.BuildCoreSnapshots(ctx, agentHostID, instances)
	if err != nil {
		return nil, err
	}
	result := make([]*agentv1.CoreInfo, 0, len(snapshots)+2)
	seen := make(map[string]struct{}, len(snapshots)+2)
	for _, snapshot := range snapshots {
		if snapshot == nil {
			continue
		}
		coreType := strings.TrimSpace(snapshot.Type)
		if coreType == "" {
			continue
		}
		seen[coreType] = struct{}{}
		result = append(result, &agentv1.CoreInfo{Type: coreType, Version: snapshot.Version, Installed: snapshot.Installed, Capabilities: append([]string(nil), snapshot.Capabilities...)})
	}
	if s.binaryVersionStates != nil {
		s.mergeInstalledCoreStates(ctx, agentHostID, &result, seen)
	}
	if len(result) > 0 {
		return result, nil
	}
	// Ultimate fallback: single core from host record
	if strings.TrimSpace(host.CoreVersion) != "" {
		coreType := strings.TrimSpace(host.CurrentCoreType)
		if coreType == "" {
			coreType = "reported"
		}
		return []*agentv1.CoreInfo{{Type: coreType, Version: host.CoreVersion, Installed: true, Capabilities: append([]string(nil), host.Capabilities...)}}, nil
	}
	return []*agentv1.CoreInfo{}, nil
}

func (s *agentCoreService) mergeInstalledCoreStates(ctx context.Context, agentHostID int64, result *[]*agentv1.CoreInfo, seen map[string]struct{}) {
	states, err := s.binaryVersionStates.List(ctx, repository.BinaryVersionFilter{AgentHostID: &agentHostID, Limit: len(binaryVersionComponents)})
	if err != nil {
		return
	}
	byComponent := make(map[string]*repository.BinaryVersionState, len(states))
	for _, state := range states {
		if state == nil {
			continue
		}
		component, err := NormalizeBinaryVersionComponent(state.Component)
		if err != nil || component == BinaryVersionComponentAgent {
			continue
		}
		byComponent[component] = state
	}
	for _, component := range binaryVersionComponents {
		if component == BinaryVersionComponentAgent {
			continue
		}
		if _, ok := seen[component]; ok {
			continue
		}
		state := byComponent[component]
		if !isInstalledCoreBinaryState(state) {
			continue
		}
		seen[component] = struct{}{}
		*result = append(*result, &agentv1.CoreInfo{Type: component, Version: strings.TrimSpace(state.LocalVersion), Installed: true, Capabilities: unmarshalStringSlice(state.CapabilitiesJSON)})
	}
}

func isInstalledCoreBinaryState(state *repository.BinaryVersionState) bool {
	if state == nil || strings.TrimSpace(state.LocalVersion) == "" {
		return false
	}
	switch strings.TrimSpace(state.Status) {
	case BinaryVersionStatusMissing, BinaryVersionStatusUnknown, "":
		return false
	default:
		return true
	}
}

func (s *agentCoreService) GetInstances(ctx context.Context, agentHostID int64) ([]*repository.AgentCoreInstance, error) {
	if s.instances == nil {
		return nil, fmt.Errorf("agent core instance repository unavailable / 核心实例仓库不可用")
	}
	if _, err := s.agentHosts.FindByID(ctx, agentHostID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	instances, err := s.instances.ListByAgentHostID(ctx, agentHostID)
	if err != nil {
		return nil, err
	}
	if instances == nil {
		instances = []*repository.AgentCoreInstance{}
	}
	return instances, nil
}

func (s *agentCoreService) CreateInstance(ctx context.Context, req CreateInstanceRequest) (*repository.CoreOperation, error) {
	if req.AgentHostID == 0 || strings.TrimSpace(req.CoreType) == "" || strings.TrimSpace(req.InstanceID) == "" {
		return nil, ErrBadRequest
	}
	configJSON, _, _, err := s.resolveConfigJSON(ctx, req.ConfigTemplateID, req.ConfigJSON)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(&agentv1.CreateCoreInstancePayload{InstanceId: strings.TrimSpace(req.InstanceID), ConfigJson: configJSON, ConfigTemplateId: req.ConfigTemplateID})
	if err != nil {
		return nil, err
	}
	return s.operations.Create(ctx, CreateCoreOperationRequest{AgentHostID: req.AgentHostID, OperationType: coreOperationTypeCreate, CoreType: strings.TrimSpace(req.CoreType), RequestPayload: payload, OperatorID: req.OperatorID})
}

func (s *agentCoreService) DeleteInstance(ctx context.Context, agentHostID int64, instanceID string) error {
	if strings.TrimSpace(instanceID) == "" {
		return fmt.Errorf("instance id required / 需要实例 ID")
	}
	if s.instances == nil {
		return fmt.Errorf("agent core instance repository unavailable / 核心实例仓库不可用")
	}
	instance, err := s.instances.FindByInstanceID(ctx, agentHostID, instanceID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	return s.instances.Delete(ctx, instance.ID)
}

func (s *agentCoreService) SwitchCore(ctx context.Context, req SwitchCoreRequest) (*repository.CoreOperation, error) {
	if req.AgentHostID == 0 || strings.TrimSpace(req.FromInstanceID) == "" || strings.TrimSpace(req.ToCoreType) == "" {
		return nil, ErrBadRequest
	}
	var configJSON []byte
	var templateID = req.ConfigTemplateID
	var err error

	if len(req.ConfigJSON) == 0 && req.ConfigTemplateID <= 0 {
		oldConfig, sourceCore, tplID, findErr := s.findInstanceConfig(ctx, req.AgentHostID, req.FromInstanceID)
		if findErr == nil {
			targetCore := strings.TrimSpace(req.ToCoreType)
			if strings.ToLower(sourceCore) == strings.ToLower(targetCore) {
				configJSON = oldConfig
				if tplID != nil {
					templateID = *tplID
				}
			} else if s.converters != nil {
				inbounds, parseErr := s.converters.Parse(oldConfig, sourceCore)
				if parseErr == nil {
					converted, convertErr := s.converters.Convert(inbounds, targetCore)
					if convertErr == nil {
						configJSON = converted
						templateID = 0
					}
				}
			}
		}
	}

	if len(configJSON) == 0 {
		var resolvedTplID *int64
		configJSON, _, resolvedTplID, err = s.resolveConfigJSON(ctx, req.ConfigTemplateID, req.ConfigJSON)
		if err != nil {
			return nil, err
		}
		if resolvedTplID != nil {
			templateID = *resolvedTplID
		}
	}

	payload, err := json.Marshal(&agentv1.SwitchCorePayload{FromInstanceId: strings.TrimSpace(req.FromInstanceID), ToCoreType: strings.TrimSpace(req.ToCoreType), ConfigJson: configJSON, SwitchId: strings.TrimSpace(req.SwitchID), ListenPorts: listenPortsToInt32(req.ListenPorts), ZeroDowntime: ptrToBool(req.ZeroDowntime), ConfigTemplateId: templateID})
	if err != nil {
		return nil, err
	}
	return s.operations.Create(ctx, CreateCoreOperationRequest{AgentHostID: req.AgentHostID, OperationType: coreOperationTypeSwitch, CoreType: strings.TrimSpace(req.ToCoreType), RequestPayload: payload, OperatorID: req.OperatorID})
}

func (s *agentCoreService) InstallCore(ctx context.Context, req InstallCoreRequest) (*repository.CoreOperation, error) {
	if req.AgentHostID == 0 || strings.TrimSpace(req.CoreType) == "" || strings.TrimSpace(req.Action) == "" {
		return nil, ErrBadRequest
	}
	payload, err := json.Marshal(&agentv1.InstallCorePayload{Action: strings.TrimSpace(req.Action), Version: strings.TrimSpace(req.Version), Channel: strings.TrimSpace(req.Channel), Flavor: strings.TrimSpace(req.Flavor), Activate: req.Activate, RequestId: strings.TrimSpace(req.RequestID)})
	if err != nil {
		return nil, err
	}
	return s.operations.Create(ctx, CreateCoreOperationRequest{AgentHostID: req.AgentHostID, OperationType: coreOperationTypeInstall, CoreType: strings.TrimSpace(req.CoreType), RequestPayload: payload, OperatorID: req.OperatorID})
}

func (s *agentCoreService) ListOperations(ctx context.Context, req ListCoreOperationsRequest) ([]*repository.CoreOperation, int64, error) {
	return s.operations.List(ctx, req)
}

func (s *agentCoreService) GetOperation(ctx context.Context, operationID string) (*repository.CoreOperation, error) {
	return s.operations.Get(ctx, operationID)
}

func (s *agentCoreService) GetSwitchLogs(ctx context.Context, filter SwitchLogFilter) ([]*repository.AgentCoreSwitchLog, int64, error) {
	if filter.AgentHostID == 0 {
		return nil, 0, fmt.Errorf("agent host id required / 需要节点 ID")
	}
	if s.switchLogs != nil {
		logs, err := s.switchLogs.List(ctx, repository.AgentCoreSwitchLogFilter{AgentHostID: filter.AgentHostID, Status: filter.Status, StartAt: filter.StartAt, EndAt: filter.EndAt, Limit: normalizeLimit(filter.Limit, 50), Offset: normalizeOffset(filter.Offset)})
		if err != nil {
			return nil, 0, err
		}
		count, err := s.switchLogs.Count(ctx, repository.AgentCoreSwitchLogFilter{AgentHostID: filter.AgentHostID, Status: filter.Status, StartAt: filter.StartAt, EndAt: filter.EndAt, Limit: normalizeLimit(filter.Limit, 50), Offset: normalizeOffset(filter.Offset)})
		if err != nil {
			return nil, 0, err
		}
		if count > 0 {
			return logs, count, nil
		}
	}
	if s.operations == nil {
		return nil, 0, fmt.Errorf("agent core switch log repository unavailable / 核心切换日志仓库不可用")
	}
	items, total, err := s.operations.List(ctx, ListCoreOperationsRequest{AgentHostID: &filter.AgentHostID, OperationType: coreOperationTypeSwitch, Statuses: statusFilterToOperationStatuses(filter.Status), StartAt: filter.StartAt, EndAt: filter.EndAt, Limit: normalizeLimit(filter.Limit, 50), Offset: normalizeOffset(filter.Offset)})
	if err != nil {
		return nil, 0, err
	}
	return buildSwitchLogsFromOperations(items), total, nil
}

func statusFilterToOperationStatuses(status *string) []string {
	if status == nil || strings.TrimSpace(*status) == "" {
		return nil
	}
	return []string{strings.TrimSpace(*status)}
}

func buildSwitchLogsFromOperations(items []*repository.CoreOperation) []*repository.AgentCoreSwitchLog {
	logs := make([]*repository.AgentCoreSwitchLog, 0, len(items))
	for _, op := range items {
		if op == nil {
			continue
		}
		var req agentv1.SwitchCorePayload
		if err := json.Unmarshal(op.RequestPayload, &req); err != nil {
			slog.Debug("failed to unmarshal switch core request payload", "error", err)
		}
		var result map[string]any
		if err := json.Unmarshal(op.ResultPayload, &result); err != nil {
				slog.Warn("agent_core: parse ResultPayload failed", "error", err)
			}
		var fromCoreType *string
		fromInstanceID := strings.TrimSpace(req.GetFromInstanceId())
		var fromPtr *string
		if fromInstanceID != "" {
			fromPtr = &fromInstanceID
		}
		newInstanceID := strings.TrimSpace(stringValue(result["new_instance_id"]))
		if newInstanceID == "" {
			newInstanceID = strings.TrimSpace(req.GetSwitchId())
		}
		createdAt := op.CreatedAt
		log := &repository.AgentCoreSwitchLog{ID: 0, AgentHostID: op.AgentHostID, FromInstanceID: fromPtr, FromCoreType: fromCoreType, ToInstanceID: newInstanceID, ToCoreType: strings.TrimSpace(req.GetToCoreType()), OperatorID: op.OperatorID, Status: op.Status, Detail: strings.TrimSpace(op.ErrorMessage), CreatedAt: createdAt, CompletedAt: op.FinishedAt}
		if log.Detail == "" {
			log.Detail = string(op.ResultPayload)
		}
		logs = append(logs, log)
	}
	return logs
}

func stringValue(v any) string {
	switch value := v.(type) {
	case string:
		return value
	default:
		return ""
	}
}

func (s *agentCoreService) ConvertConfig(ctx context.Context, req ConvertRequest) (*ConvertResult, error) {
	if strings.TrimSpace(req.SourceCore) == "" {
		return nil, fmt.Errorf("source core required / 需要源核心")
	}
	if strings.TrimSpace(req.TargetCore) == "" {
		return nil, fmt.Errorf("target core required / 需要目标核心")
	}
	if len(req.ConfigJSON) == 0 {
		return nil, fmt.Errorf("config json required / 需要配置 JSON")
	}
	if s.converters == nil {
		return nil, fmt.Errorf("converter registry unavailable / 转换器注册表不可用")
	}
	inbounds, err := s.converters.Parse(req.ConfigJSON, req.SourceCore)
	if err != nil {
		return nil, err
	}
	converted, err := s.converters.Convert(inbounds, req.TargetCore)
	if err != nil {
		return nil, err
	}
	return &ConvertResult{ConfigJSON: converted}, nil
}

func normalizeRawConfigJSON(payload []byte) ([]byte, error) {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("config json is empty / 配置 JSON 为空")
	}
	if json.Valid(trimmed) && len(trimmed) > 0 && trimmed[0] != '"' {
		return trimmed, nil
	}
	var encoded string
	if err := json.Unmarshal(trimmed, &encoded); err != nil {
		return nil, fmt.Errorf("config json is invalid / 配置 JSON 无效")
	}
	normalized := bytes.TrimSpace([]byte(encoded))
	if !json.Valid(normalized) {
		return nil, fmt.Errorf("config json is invalid / 配置 JSON 无效")
	}
	return normalized, nil
}

func (s *agentCoreService) resolveConfigJSON(ctx context.Context, templateID int64, configJSON json.RawMessage) ([]byte, string, *int64, error) {
	var payload []byte
	var tplID *int64
	if len(configJSON) > 0 {
		payload = configJSON
	} else if templateID > 0 {
		if s.templates == nil {
			return nil, "", nil, fmt.Errorf("config template repository unavailable / 配置模板仓库不可用")
		}
		tpl, err := s.templates.FindByID(ctx, templateID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil, "", nil, ErrNotFound
			}
			return nil, "", nil, err
		}
		payload = []byte(tpl.Content)
		tplID = &tpl.ID
	} else {
		payload = json.RawMessage(`{}`)
	}
	normalized, err := normalizeRawConfigJSON(payload)
	if err != nil {
		return nil, "", nil, err
	}
	hash := md5.Sum(normalized)
	return normalized, hex.EncodeToString(hash[:]), tplID, nil
}
func normalizeLimit(limit int, fallback int) int {
	if limit <= 0 {
		return fallback
	}
	if limit > 200 {
		return 200
	}
	return limit
}

func normalizeOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

func listenPortsToInt32(ports []int) []int32 {
	if len(ports) == 0 {
		return nil
	}
	result := make([]int32, 0, len(ports))
	for _, port := range ports {
		if port <= 0 {
			continue
		}
		result = append(result, int32(port))
	}
	return result
}

func ptrToBool(value *bool) bool {
	if value == nil {
		return false
	}
	return *value
}

func (s *agentCoreService) findInstanceConfig(ctx context.Context, agentHostID int64, instanceID string) ([]byte, string, *int64, error) {
	if s.operations == nil {
		return nil, "", nil, fmt.Errorf("operations service unavailable")
	}
	ops, _, err := s.operations.List(ctx, ListCoreOperationsRequest{
		AgentHostID: &agentHostID,
		Statuses:    []string{coreOperationStatusCompleted},
		Limit:       50,
	})
	if err != nil {
		return nil, "", nil, err
	}

	for _, op := range ops {
		if op == nil {
			continue
		}
		if op.OperationType == coreOperationTypeCreate {
			var payload agentv1.CreateCoreInstancePayload
			if err := json.Unmarshal(op.RequestPayload, &payload); err == nil {
				if payload.GetInstanceId() == instanceID {
					var tplID *int64
					if payload.GetConfigTemplateId() > 0 {
						val := payload.GetConfigTemplateId()
						tplID = &val
					}
					return payload.GetConfigJson(), op.CoreType, tplID, nil
				}
			}
		} else if op.OperationType == coreOperationTypeSwitch {
			var result map[string]any
			if err := json.Unmarshal(op.ResultPayload, &result); err == nil {
				newInstanceID, ok := result["new_instance_id"].(string)
				if !ok {
					continue
				}
				if newInstanceID == instanceID {
					var payload agentv1.SwitchCorePayload
					if err := json.Unmarshal(op.RequestPayload, &payload); err == nil {
						var tplID *int64
						if payload.GetConfigTemplateId() > 0 {
							val := payload.GetConfigTemplateId()
							tplID = &val
						}
						return payload.GetConfigJson(), op.CoreType, tplID, nil
					}
				}
			}
		}
	}
	return nil, "", nil, repository.ErrNotFound
}
