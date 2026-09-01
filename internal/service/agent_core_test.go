package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/creamcroissant/mgpanel/internal/repository"
	"github.com/creamcroissant/mgpanel/internal/template"
	agentv1 "github.com/creamcroissant/mgpanel/pkg/pb/agent/v1"
)

type mockAgentHostRepoForCore struct {
	repository.AgentHostRepository
	hosts   map[int64]*repository.AgentHost
	findErr error
}

func newMockAgentHostRepoForCore() *mockAgentHostRepoForCore {
	return &mockAgentHostRepoForCore{hosts: make(map[int64]*repository.AgentHost)}
}

func (m *mockAgentHostRepoForCore) FindByID(ctx context.Context, id int64) (*repository.AgentHost, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	if host, ok := m.hosts[id]; ok {
		return host, nil
	}
	return nil, repository.ErrNotFound
}

type mockAgentCoreInstanceRepo struct {
	instances map[int64]*repository.AgentCoreInstance
	nextID    int64
}

func newMockAgentCoreInstanceRepo() *mockAgentCoreInstanceRepo {
	return &mockAgentCoreInstanceRepo{instances: make(map[int64]*repository.AgentCoreInstance), nextID: 1}
}

func (m *mockAgentCoreInstanceRepo) Create(ctx context.Context, instance *repository.AgentCoreInstance) error {
	if instance == nil {
		return errors.New("instance is nil")
	}
	if instance.ID == 0 {
		instance.ID = m.nextID
		m.nextID++
	}
	clone := *instance
	m.instances[clone.ID] = &clone
	return nil
}

func (m *mockAgentCoreInstanceRepo) Update(ctx context.Context, instance *repository.AgentCoreInstance) error {
	if instance == nil {
		return errors.New("instance is nil")
	}
	if _, ok := m.instances[instance.ID]; !ok {
		return repository.ErrNotFound
	}
	clone := *instance
	m.instances[clone.ID] = &clone
	return nil
}

func (m *mockAgentCoreInstanceRepo) Delete(ctx context.Context, id int64) error {
	if _, ok := m.instances[id]; !ok {
		return repository.ErrNotFound
	}
	delete(m.instances, id)
	return nil
}

func (m *mockAgentCoreInstanceRepo) FindByID(ctx context.Context, id int64) (*repository.AgentCoreInstance, error) {
	if inst, ok := m.instances[id]; ok {
		clone := *inst
		return &clone, nil
	}
	return nil, repository.ErrNotFound
}

func (m *mockAgentCoreInstanceRepo) FindByInstanceID(ctx context.Context, agentHostID int64, instanceID string) (*repository.AgentCoreInstance, error) {
	for _, inst := range m.instances {
		if inst.AgentHostID == agentHostID && inst.InstanceID == instanceID {
			clone := *inst
			return &clone, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (m *mockAgentCoreInstanceRepo) ListByAgentHostID(ctx context.Context, agentHostID int64) ([]*repository.AgentCoreInstance, error) {
	result := make([]*repository.AgentCoreInstance, 0)
	for _, inst := range m.instances {
		if inst.AgentHostID == agentHostID {
			clone := *inst
			result = append(result, &clone)
		}
	}
	return result, nil
}

func (m *mockAgentCoreInstanceRepo) ReplaceSnapshot(ctx context.Context, agentHostID int64, instances []*repository.AgentCoreInstance) error {
	for _, instance := range instances {
		if instance == nil {
			continue
		}
		if existing, err := m.FindByInstanceID(ctx, agentHostID, instance.InstanceID); err == nil {
			instance.ID = existing.ID
			clone := *instance
			m.instances[clone.ID] = &clone
			continue
		}
		if err := m.Create(ctx, instance); err != nil {
			return err
		}
	}
	return nil
}

func (m *mockAgentCoreInstanceRepo) UpdateHeartbeat(ctx context.Context, agentHostID int64, instanceID string, heartbeatAt int64) error {
	return nil
}

type switchLogUpdate struct {
	id          int64
	status      string
	detail      string
	completedAt *int64
}

type mockAgentCoreSwitchLogRepo struct {
	logs            map[int64]*repository.AgentCoreSwitchLog
	nextID          int64
	updates         []switchLogUpdate
	lastListFilter  repository.AgentCoreSwitchLogFilter
	lastCountFilter repository.AgentCoreSwitchLogFilter
	listResp        []*repository.AgentCoreSwitchLog
	listErr         error
	countResp       *int64
	countErr        error
}

func newMockAgentCoreSwitchLogRepo() *mockAgentCoreSwitchLogRepo {
	return &mockAgentCoreSwitchLogRepo{logs: make(map[int64]*repository.AgentCoreSwitchLog), nextID: 1}
}

func (m *mockAgentCoreSwitchLogRepo) Create(ctx context.Context, log *repository.AgentCoreSwitchLog) error {
	if log == nil {
		return errors.New("log is nil")
	}
	if log.ID == 0 {
		log.ID = m.nextID
		m.nextID++
	}
	clone := *log
	m.logs[clone.ID] = &clone
	return nil
}

func (m *mockAgentCoreSwitchLogRepo) UpdateStatus(ctx context.Context, id int64, status string, detail string, completedAt *int64) error {
	m.updates = append(m.updates, switchLogUpdate{id: id, status: status, detail: detail, completedAt: completedAt})
	if log, ok := m.logs[id]; ok {
		log.Status = status
		log.Detail = detail
		log.CompletedAt = completedAt
	}
	return nil
}

func (m *mockAgentCoreSwitchLogRepo) List(ctx context.Context, filter repository.AgentCoreSwitchLogFilter) ([]*repository.AgentCoreSwitchLog, error) {
	m.lastListFilter = filter
	if m.listErr != nil {
		return nil, m.listErr
	}
	if m.listResp != nil {
		return m.listResp, nil
	}
	return nil, nil
}

func (m *mockAgentCoreSwitchLogRepo) Count(ctx context.Context, filter repository.AgentCoreSwitchLogFilter) (int64, error) {
	m.lastCountFilter = filter
	if m.countErr != nil {
		return 0, m.countErr
	}
	if m.countResp != nil {
		return *m.countResp, nil
	}
	return int64(len(m.logs)), nil
}

type mockCoreOperationRepo struct {
	operations  map[string]*repository.CoreOperation
	lastCreated *repository.CoreOperation
	listResp    []*repository.CoreOperation
	listErr     error
	countResp   int64
	countErr    error
}

func newMockCoreOperationRepo() *mockCoreOperationRepo {
	return &mockCoreOperationRepo{operations: make(map[string]*repository.CoreOperation)}
}

func (m *mockCoreOperationRepo) Create(ctx context.Context, operation *repository.CoreOperation) error {
	clone := *operation
	m.operations[clone.ID] = &clone
	m.lastCreated = &clone
	return nil
}

func (m *mockCoreOperationRepo) UpdateStatus(ctx context.Context, id, status string, resultPayload json.RawMessage, errorMessage string, claimedBy string, claimedAt, startedAt, finishedAt *int64) error {
	if op, ok := m.operations[id]; ok {
		op.Status = status
		op.ResultPayload = append(json.RawMessage(nil), resultPayload...)
		op.ErrorMessage = errorMessage
		op.ClaimedBy = claimedBy
		op.ClaimedAt = claimedAt
		op.StartedAt = startedAt
		op.FinishedAt = finishedAt
		return nil
	}
	return repository.ErrNotFound
}

func (m *mockCoreOperationRepo) FindByID(ctx context.Context, id string) (*repository.CoreOperation, error) {
	if op, ok := m.operations[id]; ok {
		clone := *op
		return &clone, nil
	}
	return nil, repository.ErrNotFound
}

func (m *mockCoreOperationRepo) List(ctx context.Context, filter repository.CoreOperationFilter) ([]*repository.CoreOperation, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	if m.listResp != nil {
		return m.listResp, nil
	}
	items := make([]*repository.CoreOperation, 0, len(m.operations))
	for _, op := range m.operations {
		clone := *op
		items = append(items, &clone)
	}
	return items, nil
}

func (m *mockCoreOperationRepo) Count(ctx context.Context, filter repository.CoreOperationFilter) (int64, error) {
	if m.countErr != nil {
		return 0, m.countErr
	}
	if m.countResp != 0 {
		return m.countResp, nil
	}
	return int64(len(m.operations)), nil
}

func (m *mockCoreOperationRepo) ClaimNext(ctx context.Context, agentHostID int64, statuses []string, claimedBy string, claimedAt int64, reclaimBefore *int64) (*repository.CoreOperation, error) {
	for _, op := range m.operations {
		if op.AgentHostID != agentHostID {
			continue
		}
		allowed := len(statuses) == 0
		for _, status := range statuses {
			if op.Status == status {
				if status == coreOperationStatusClaimed && reclaimBefore != nil && op.ClaimedAt != nil && *op.ClaimedAt > *reclaimBefore {
					continue
				}
				allowed = true
				break
			}
		}
		if !allowed {
			continue
		}
		startedAt := claimedAt
		op.Status = coreOperationStatusClaimed
		op.ClaimedBy = claimedBy
		op.ClaimedAt = &claimedAt
		op.StartedAt = &startedAt
		clone := *op
		return &clone, nil
	}
	return nil, repository.ErrNotFound
}

type mockConfigTemplateRepoForCore struct {
	repository.ConfigTemplateRepository
	templates map[int64]*repository.ConfigTemplate
}

func (m *mockConfigTemplateRepoForCore) FindByID(ctx context.Context, id int64) (*repository.ConfigTemplate, error) {
	if tpl, ok := m.templates[id]; ok {
		return tpl, nil
	}
	return nil, repository.ErrNotFound
}

type fakeConverter struct {
	target   string
	inbounds []template.UnifiedInbound
	output   []byte
}

func (f *fakeConverter) TargetCore() string { return f.target }
func (f *fakeConverter) ToUnified(configJSON []byte) ([]template.UnifiedInbound, error) {
	return f.inbounds, nil
}
func (f *fakeConverter) FromUnified(inbounds []template.UnifiedInbound) ([]byte, error) {
	return f.output, nil
}

func TestAgentCoreService_GetInstances_NotFound(t *testing.T) {
	hosts := newMockAgentHostRepoForCore()
	instances := newMockAgentCoreInstanceRepo()
	svc := NewAgentCoreService(hosts, instances, nil, nil, nil, nil)
	_, err := svc.GetInstances(context.Background(), 1)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAgentCoreService_GetCores_FromSnapshots(t *testing.T) {
	hosts := newMockAgentHostRepoForCore()
	hosts.hosts[1] = &repository.AgentHost{ID: 1, CoreVersion: "1.12.0", Capabilities: []string{"reality"}}
	instances := newMockAgentCoreInstanceRepo()
	_ = instances.Create(context.Background(), &repository.AgentCoreInstance{
		AgentHostID: 1,
		InstanceID:  "inst-1",
		CoreType:    "sing-box",
		Status:      "running",
		CoreSnapshot: &repository.CoreStatusSnapshot{
			Type:         "sing-box",
			Version:      "1.12.0",
			Installed:    true,
			Capabilities: []string{"reality"},
		},
	})
	svc := NewAgentCoreServiceWithOptions(hosts, instances, nil, nil, nil, nil, AgentCoreServiceOptions{})
	cores, err := svc.GetCores(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetCores error: %v", err)
	}
	if len(cores) != 1 || cores[0].Type != "sing-box" || cores[0].Version != "1.12.0" || !cores[0].Installed {
		t.Fatalf("unexpected cores: %+v", cores)
	}
}

func TestAgentCoreService_GetCores_MergesInstalledBinaryStatesWithSnapshots(t *testing.T) {
	hosts := newMockAgentHostRepoForCore()
	hosts.hosts[1] = &repository.AgentHost{ID: 1, CoreVersion: "1.13.14", CurrentCoreType: "sing-box", Capabilities: []string{"reality"}}
	instances := newMockAgentCoreInstanceRepo()
	_ = instances.Create(context.Background(), &repository.AgentCoreInstance{
		AgentHostID: 1,
		InstanceID:  "inst-1",
		CoreType:    "sing-box",
		Status:      "running",
		CoreSnapshot: &repository.CoreStatusSnapshot{
			Type:         "sing-box",
			Version:      "1.13.14",
			Installed:    true,
			Capabilities: []string{"reality"},
		},
	})
	versions := newMockBinaryVersionRepo()
	versions.states[versions.key(1, BinaryVersionComponentXray)] = &repository.BinaryVersionState{AgentHostID: 1, Component: BinaryVersionComponentXray, LocalVersion: "26.3.27", Status: BinaryVersionStatusUpToDate}
	svc := NewAgentCoreServiceWithOptions(hosts, instances, nil, nil, nil, nil, AgentCoreServiceOptions{BinaryVersionStates: versions})

	cores, err := svc.GetCores(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetCores error: %v", err)
	}
	byType := make(map[string]*agentv1.CoreInfo, len(cores))
	for _, core := range cores {
		byType[core.Type] = core
	}
	if len(byType) != 2 {
		t.Fatalf("expected sing-box and xray, got %+v", cores)
	}
	if core := byType[BinaryVersionComponentSingBox]; core == nil || core.Version != "1.13.14" || !core.Installed || len(core.Capabilities) != 1 || core.Capabilities[0] != "reality" {
		t.Fatalf("unexpected sing-box core: %+v", core)
	}
	if core := byType[BinaryVersionComponentXray]; core == nil || core.Version != "26.3.27" || !core.Installed {
		t.Fatalf("unexpected xray core: %+v", core)
	}
}

func TestAgentCoreService_CreateInstance_CreatesOperation(t *testing.T) {
	hosts := newMockAgentHostRepoForCore()
	hosts.hosts[1] = &repository.AgentHost{ID: 1}
	ops := newMockCoreOperationRepo()
	svc := NewAgentCoreServiceWithOptions(hosts, newMockAgentCoreInstanceRepo(), nil, nil, nil, nil, AgentCoreServiceOptions{Operations: ops})
	configJSON := json.RawMessage(`{"log":{"level":"info"}}`)
	result, err := svc.CreateInstance(context.Background(), CreateInstanceRequest{AgentHostID: 1, CoreType: "sing-box", InstanceID: "inst-1", ConfigJSON: configJSON})
	if err != nil {
		t.Fatalf("CreateInstance error: %v", err)
	}
	if result.OperationType != coreOperationTypeCreate || result.Status != coreOperationStatusPending {
		t.Fatalf("unexpected operation: %+v", result)
	}
	var payload agentv1.CreateCoreInstancePayload
	if err := json.Unmarshal(result.RequestPayload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.InstanceId != "inst-1" || string(payload.ConfigJson) != string(configJSON) {
		t.Fatalf("unexpected payload: instance_id=%q config_json=%s", payload.InstanceId, string(payload.ConfigJson))
	}
}

func TestAgentCoreService_CreateInstance_TemplatePayload(t *testing.T) {
	hosts := newMockAgentHostRepoForCore()
	hosts.hosts[1] = &repository.AgentHost{ID: 1}
	templates := &mockConfigTemplateRepoForCore{templates: map[int64]*repository.ConfigTemplate{1: {ID: 1, Content: `{"inbounds":[]}`}}}
	ops := newMockCoreOperationRepo()
	svc := NewAgentCoreServiceWithOptions(hosts, newMockAgentCoreInstanceRepo(), nil, templates, nil, nil, AgentCoreServiceOptions{Operations: ops})
	result, err := svc.CreateInstance(context.Background(), CreateInstanceRequest{AgentHostID: 1, CoreType: "sing-box", InstanceID: "inst-2", ConfigTemplateID: 1})
	if err != nil {
		t.Fatalf("CreateInstance error: %v", err)
	}
	var payload agentv1.CreateCoreInstancePayload
	if err := json.Unmarshal(result.RequestPayload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if string(payload.ConfigJson) != `{"inbounds":[]}` {
		t.Fatalf("unexpected config payload: %s", string(payload.ConfigJson))
	}
}

func TestAgentCoreService_DeleteInstance(t *testing.T) {
	instances := newMockAgentCoreInstanceRepo()
	hosts := newMockAgentHostRepoForCore()
	svc := NewAgentCoreService(hosts, instances, nil, nil, nil, nil)
	if err := svc.DeleteInstance(context.Background(), 1, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	_ = instances.Create(context.Background(), &repository.AgentCoreInstance{AgentHostID: 1, InstanceID: "core-1", CoreType: "xray"})
	if err := svc.DeleteInstance(context.Background(), 1, "core-1"); err != nil {
		t.Fatalf("DeleteInstance error: %v", err)
	}
}

func TestAgentCoreService_SwitchCore_CreatesOperation(t *testing.T) {
	hosts := newMockAgentHostRepoForCore()
	hosts.hosts[1] = &repository.AgentHost{ID: 1}
	ops := newMockCoreOperationRepo()
	svc := NewAgentCoreServiceWithOptions(hosts, newMockAgentCoreInstanceRepo(), nil, nil, nil, nil, AgentCoreServiceOptions{Operations: ops})
	result, err := svc.SwitchCore(context.Background(), SwitchCoreRequest{AgentHostID: 1, FromInstanceID: "core-1", ToCoreType: "xray", ConfigJSON: json.RawMessage(`{"inbounds":[]}`), ListenPorts: []int{443}, SwitchID: "sw-1"})
	if err != nil {
		t.Fatalf("SwitchCore error: %v", err)
	}
	if result.OperationType != coreOperationTypeSwitch || result.CoreType != "xray" {
		t.Fatalf("unexpected operation: %+v", result)
	}
	var payload agentv1.SwitchCorePayload
	if err := json.Unmarshal(result.RequestPayload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.FromInstanceId != "core-1" || payload.ToCoreType != "xray" || payload.SwitchId != "sw-1" {
		t.Fatalf("unexpected payload: from_instance_id=%q to_core_type=%q switch_id=%q", payload.FromInstanceId, payload.ToCoreType, payload.SwitchId)
	}
}

func TestAgentCoreService_InstallCore_CreatesOperation(t *testing.T) {
	hosts := newMockAgentHostRepoForCore()
	hosts.hosts[1] = &repository.AgentHost{ID: 1}
	ops := newMockCoreOperationRepo()
	svc := NewAgentCoreServiceWithOptions(hosts, nil, nil, nil, nil, nil, AgentCoreServiceOptions{Operations: ops})
	result, err := svc.InstallCore(context.Background(), InstallCoreRequest{AgentHostID: 1, CoreType: "sing-box", Action: "upgrade", Version: "1.12.0", Channel: "stable", Flavor: "official", Activate: true, RequestID: "req-1"})
	if err != nil {
		t.Fatalf("InstallCore error: %v", err)
	}
	if result.OperationType != coreOperationTypeInstall || result.CoreType != "sing-box" {
		t.Fatalf("unexpected operation: %+v", result)
	}
	var payload agentv1.InstallCorePayload
	if err := json.Unmarshal(result.RequestPayload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.Action != "upgrade" || payload.Version != "1.12.0" || payload.RequestId != "req-1" || !payload.Activate {
		t.Fatalf("unexpected payload: action=%q version=%q request_id=%q activate=%t", payload.Action, payload.Version, payload.RequestId, payload.Activate)
	}
}

func TestAgentCoreService_InstallCore_BadRequest(t *testing.T) {
	svc := NewAgentCoreService(nil, nil, nil, nil, nil, nil)
	if _, err := svc.InstallCore(context.Background(), InstallCoreRequest{}); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("expected ErrBadRequest, got %v", err)
	}
}

func TestAgentCoreService_GetSwitchLogs_Normalize(t *testing.T) {
	switchLogs := newMockAgentCoreSwitchLogRepo()
	switchLogs.listResp = []*repository.AgentCoreSwitchLog{{ID: 1, AgentHostID: 1}}
	countVal := int64(1)
	switchLogs.countResp = &countVal
	svc := NewAgentCoreService(nil, nil, switchLogs, nil, nil, nil)
	statusFilter := switchStatusCompleted
	_, _, err := svc.GetSwitchLogs(context.Background(), SwitchLogFilter{AgentHostID: 1, Status: &statusFilter, Limit: 500, Offset: -5})
	if err != nil {
		t.Fatalf("GetSwitchLogs error: %v", err)
	}
	if switchLogs.lastListFilter.Limit != 200 || switchLogs.lastListFilter.Offset != 0 {
		t.Fatalf("unexpected filter: %+v", switchLogs.lastListFilter)
	}
}

func TestAgentCoreService_ConvertConfig(t *testing.T) {
	conv := &fakeConverter{target: "sing-box", inbounds: []template.UnifiedInbound{{Protocol: "vless"}}, output: []byte(`{"converted":true}`)}
	registry := template.NewConverterRegistry(conv)
	svc := NewAgentCoreService(nil, nil, nil, nil, registry, nil)
	res, err := svc.ConvertConfig(context.Background(), ConvertRequest{SourceCore: "sing-box", TargetCore: "sing-box", ConfigJSON: json.RawMessage(`{"inbounds":[]}`)})
	if err != nil {
		t.Fatalf("ConvertConfig error: %v", err)
	}
	if string(res.ConfigJSON) != `{"converted":true}` {
		t.Fatalf("unexpected output: %s", string(res.ConfigJSON))
	}
}

func TestAgentCoreService_SwitchCore_InheritConfig(t *testing.T) {
	hosts := newMockAgentHostRepoForCore()
	hosts.hosts[1] = &repository.AgentHost{ID: 1}
	ops := newMockCoreOperationRepo()

	// 模拟已存在一个已完成的创建实例操作，带配置和模板 ID
	createPayload, _ := json.Marshal(&agentv1.CreateCoreInstancePayload{
		InstanceId:       "core-old",
		ConfigJson:       []byte(`{"port": 8080}`),
		ConfigTemplateId: 5,
	})
	ops.operations["op-old"] = &repository.CoreOperation{
		ID:             "op-old",
		AgentHostID:    1,
		OperationType:  coreOperationTypeCreate,
		CoreType:       "xray",
		Status:         coreOperationStatusCompleted,
		RequestPayload: createPayload,
	}

	svc := NewAgentCoreServiceWithOptions(hosts, newMockAgentCoreInstanceRepo(), nil, nil, nil, nil, AgentCoreServiceOptions{Operations: ops})

	// 进行同核心切换（不提供配置 JSON 和模板 ID）
	result, err := svc.SwitchCore(context.Background(), SwitchCoreRequest{
		AgentHostID:    1,
		FromInstanceID: "core-old",
		ToCoreType:     "xray",
		SwitchID:       "sw-1",
	})
	if err != nil {
		t.Fatalf("SwitchCore error: %v", err)
	}

	var payload agentv1.SwitchCorePayload
	if err := json.Unmarshal(result.RequestPayload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	if string(payload.ConfigJson) != `{"port": 8080}` {
		t.Fatalf("expected inherited config, got %s", string(payload.ConfigJson))
	}
	if payload.ConfigTemplateId != 5 {
		t.Fatalf("expected inherited template ID 5, got %d", payload.ConfigTemplateId)
	}
}

func TestAgentCoreService_SwitchCore_ConvertConfig(t *testing.T) {
	hosts := newMockAgentHostRepoForCore()
	hosts.hosts[1] = &repository.AgentHost{ID: 1}
	ops := newMockCoreOperationRepo()

	// 模拟已完成的 sing-box 创建操作
	createPayload, _ := json.Marshal(&agentv1.CreateCoreInstancePayload{
		InstanceId:       "core-old",
		ConfigJson:       []byte(`{"inbounds": []}`),
		ConfigTemplateId: 5,
	})
	ops.operations["op-old"] = &repository.CoreOperation{
		ID:             "op-old",
		AgentHostID:    1,
		OperationType:  coreOperationTypeCreate,
		CoreType:       "sing-box",
		Status:         coreOperationStatusCompleted,
		RequestPayload: createPayload,
	}

	// 注册转换器
	convSingBox := &fakeConverter{target: "sing-box", inbounds: []template.UnifiedInbound{{Protocol: "vless"}}, output: []byte(`{"inbounds":[]}`)}
	convXray := &fakeConverter{target: "xray", inbounds: []template.UnifiedInbound{{Protocol: "vless"}}, output: []byte(`{"xray_converted":true}`)}
	registry := template.NewConverterRegistry(convSingBox, convXray)

	svc := NewAgentCoreServiceWithOptions(hosts, newMockAgentCoreInstanceRepo(), nil, nil, registry, nil, AgentCoreServiceOptions{Operations: ops})

	// 切换为 xray（不同核心，进行配置转换）
	result, err := svc.SwitchCore(context.Background(), SwitchCoreRequest{
		AgentHostID:    1,
		FromInstanceID: "core-old",
		ToCoreType:     "xray",
		SwitchID:       "sw-1",
	})
	if err != nil {
		t.Fatalf("SwitchCore error: %v", err)
	}

	var payload agentv1.SwitchCorePayload
	if err := json.Unmarshal(result.RequestPayload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	if string(payload.ConfigJson) != `{"xray_converted":true}` {
		t.Fatalf("expected converted config, got %s", string(payload.ConfigJson))
	}
	if payload.ConfigTemplateId != 0 {
		t.Fatalf("expected template ID 0 after conversion, got %d", payload.ConfigTemplateId)
	}
}

func TestCoreOperationService_ReportResult_Forbidden(t *testing.T) {
	ops := newMockCoreOperationRepo()
	ops.operations["op-1"] = &repository.CoreOperation{ID: "op-1", AgentHostID: 1, Status: coreOperationStatusClaimed}
	svc := NewCoreOperationService(ops)
	if err := svc.ReportResult(context.Background(), ReportCoreOperationResultRequest{AgentHostID: 2, OperationID: "op-1", Status: coreOperationStatusCompleted}); !errors.Is(err, ErrCoreOperationForbidden) {
		t.Fatalf("expected ErrCoreOperationForbidden, got %v", err)
	}
}

func TestCoreOperationService_ReportResult_InvalidTransition(t *testing.T) {
	ops := newMockCoreOperationRepo()
	ops.operations["op-1"] = &repository.CoreOperation{ID: "op-1", AgentHostID: 1, Status: coreOperationStatusCompleted}
	svc := NewCoreOperationService(ops)
	if err := svc.ReportResult(context.Background(), ReportCoreOperationResultRequest{AgentHostID: 1, OperationID: "op-1", Status: coreOperationStatusFailed}); !errors.Is(err, ErrCoreOperationInvalidRequest) {
		t.Fatalf("expected ErrCoreOperationInvalidRequest, got %v", err)
	}
}
