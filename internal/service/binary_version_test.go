package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/creamcroissant/mgpanel/internal/repository"
	agentv1 "github.com/creamcroissant/mgpanel/pkg/pb/agent/v1"
)

type mockBinaryVersionRepo struct {
	states          map[string]*repository.BinaryVersionState
	upsertErr       error
	listErr         error
	findErr         error
	updateCheckErr  error
	lastCheckResult struct {
		agentHostID   int64
		component     string
		remoteVersion string
		status        string
		checkError    string
		checkedAt     int64
	}
}

func newMockBinaryVersionRepo() *mockBinaryVersionRepo {
	return &mockBinaryVersionRepo{states: make(map[string]*repository.BinaryVersionState)}
}

func (m *mockBinaryVersionRepo) key(agentHostID int64, component string) string {
	return string(rune(agentHostID)) + "\x00" + component
}

func (m *mockBinaryVersionRepo) Upsert(ctx context.Context, state *repository.BinaryVersionState) (*repository.BinaryVersionState, error) {
	if m.upsertErr != nil {
		return nil, m.upsertErr
	}
	if state == nil {
		return nil, errors.New("state is nil")
	}
	key := m.key(state.AgentHostID, state.Component)
	clone := *state
	if existing, ok := m.states[key]; ok {
		clone.ID = existing.ID
		clone.RemoteVersion = existing.RemoteVersion
		clone.LastCheckedAt = existing.LastCheckedAt
		clone.LastCheckError = existing.LastCheckError
	} else if clone.ID == 0 {
		clone.ID = int64(len(m.states) + 1)
	}
	m.states[key] = &clone
	return m.FindByHostComponent(ctx, state.AgentHostID, state.Component)
}

func (m *mockBinaryVersionRepo) FindByHostComponent(ctx context.Context, agentHostID int64, component string) (*repository.BinaryVersionState, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	if state, ok := m.states[m.key(agentHostID, component)]; ok {
		clone := *state
		return &clone, nil
	}
	return nil, repository.ErrNotFound
}

func (m *mockBinaryVersionRepo) List(ctx context.Context, filter repository.BinaryVersionFilter) ([]*repository.BinaryVersionState, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	result := make([]*repository.BinaryVersionState, 0, len(m.states))
	for _, state := range m.states {
		if filter.AgentHostID != nil && state.AgentHostID != *filter.AgentHostID {
			continue
		}
		if filter.Component != nil && state.Component != *filter.Component {
			continue
		}
		if filter.Status != nil && state.Status != *filter.Status {
			continue
		}
		clone := *state
		result = append(result, &clone)
	}
	return result, nil
}

func (m *mockBinaryVersionRepo) UpdateCheckResult(ctx context.Context, agentHostID int64, component, remoteVersion, status, checkError string, checkedAt int64) error {
	if m.updateCheckErr != nil {
		return m.updateCheckErr
	}
	m.lastCheckResult.agentHostID = agentHostID
	m.lastCheckResult.component = component
	m.lastCheckResult.remoteVersion = remoteVersion
	m.lastCheckResult.status = status
	m.lastCheckResult.checkError = checkError
	m.lastCheckResult.checkedAt = checkedAt
	state, ok := m.states[m.key(agentHostID, component)]
	if !ok {
		return repository.ErrNotFound
	}
	if checkError != "" {
		state.LastCheckError = checkError
		state.LastCheckedAt = checkedAt
		state.UpdatedAt = checkedAt
		return nil
	}
	state.RemoteVersion = remoteVersion
	state.Status = status
	state.LastCheckError = ""
	state.LastCheckedAt = checkedAt
	state.UpdatedAt = checkedAt
	return nil
}

type mockBinaryVersionHostRepo struct {
	repository.AgentHostRepository
	hosts   map[int64]*repository.AgentHost
	findErr error
}

func newMockBinaryVersionHostRepo() *mockBinaryVersionHostRepo {
	return &mockBinaryVersionHostRepo{hosts: make(map[int64]*repository.AgentHost)}
}

func (m *mockBinaryVersionHostRepo) FindByID(ctx context.Context, id int64) (*repository.AgentHost, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	if host, ok := m.hosts[id]; ok {
		clone := *host
		return &clone, nil
	}
	return nil, repository.ErrNotFound
}

type stubBinaryVersionRemoteProvider struct {
	versions map[string]string
	err      error
}

type mockBinaryVersionCoreOperationRepo struct {
	operations []*repository.CoreOperation
	listErr    error
}

func (m *mockBinaryVersionCoreOperationRepo) Create(ctx context.Context, operation *repository.CoreOperation) error {
	return errors.New("not implemented")
}

func (m *mockBinaryVersionCoreOperationRepo) UpdateStatus(ctx context.Context, id, status string, resultPayload json.RawMessage, errorMessage string, claimedBy string, claimedAt, startedAt, finishedAt *int64) error {
	return errors.New("not implemented")
}

func (m *mockBinaryVersionCoreOperationRepo) FindByID(ctx context.Context, id string) (*repository.CoreOperation, error) {
	return nil, repository.ErrNotFound
}

func (m *mockBinaryVersionCoreOperationRepo) List(ctx context.Context, filter repository.CoreOperationFilter) ([]*repository.CoreOperation, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	result := make([]*repository.CoreOperation, 0, len(m.operations))
	for _, operation := range m.operations {
		if operation == nil {
			continue
		}
		if filter.AgentHostID != nil && operation.AgentHostID != *filter.AgentHostID {
			continue
		}
		if filter.OperationType != nil && operation.OperationType != *filter.OperationType {
			continue
		}
		if filter.CoreType != nil && operation.CoreType != *filter.CoreType {
			continue
		}
		if filter.Status != nil && operation.Status != *filter.Status {
			continue
		}
		result = append(result, operation)
		if filter.Limit > 0 && len(result) >= filter.Limit {
			break
		}
	}
	return result, nil
}

func (m *mockBinaryVersionCoreOperationRepo) Count(ctx context.Context, filter repository.CoreOperationFilter) (int64, error) {
	return int64(len(m.operations)), nil
}

func (m *mockBinaryVersionCoreOperationRepo) ClaimNext(ctx context.Context, agentHostID int64, statuses []string, claimedBy string, claimedAt int64, reclaimBefore *int64) (*repository.CoreOperation, error) {
	return nil, repository.ErrNotFound
}

func (p stubBinaryVersionRemoteProvider) LatestVersion(ctx context.Context, component string) (string, error) {
	if p.err != nil {
		return "", p.err
	}
	return p.versions[component], nil
}

func newBinaryVersionTestService(repo *mockBinaryVersionRepo, hosts *mockBinaryVersionHostRepo, provider BinaryVersionRemoteProvider) BinaryVersionService {
	return NewBinaryVersionServiceWithOptions(repo, hosts, provider, BinaryVersionServiceOptions{Now: func() time.Time { return time.Unix(1710000000, 0) }})
}

func TestBinaryVersionServiceUpdateLocalVersionsUpsertsAgentAndCore(t *testing.T) {
	repo := newMockBinaryVersionRepo()
	hosts := newMockBinaryVersionHostRepo()
	hosts.hosts[7] = &repository.AgentHost{ID: 7}
	svc := newBinaryVersionTestService(repo, hosts, nil)

	err := svc.UpdateLocalVersions(context.Background(), UpdateLocalVersionsRequest{
		AgentHostID:     7,
		AgentVersion:    " v0.2.0 ",
		CurrentCoreType: "singbox",
		CoreVersion:     "1.11.0",
		Capabilities:    []string{"reality", "reality", " "},
		BuildTags:       []string{"with_gvisor"},
	})
	if err != nil {
		t.Fatalf("UpdateLocalVersions returned error: %v", err)
	}

	agent, err := repo.FindByHostComponent(context.Background(), 7, BinaryVersionComponentAgent)
	if err != nil {
		t.Fatalf("agent state missing: %v", err)
	}
	if agent.LocalVersion != "v0.2.0" || agent.Status != BinaryVersionStatusInstalled {
		t.Fatalf("unexpected agent state: %+v", agent)
	}
	core, err := repo.FindByHostComponent(context.Background(), 7, BinaryVersionComponentSingBox)
	if err != nil {
		t.Fatalf("core state missing: %v", err)
	}
	if core.LocalVersion != "1.11.0" || core.Status != BinaryVersionStatusInstalled {
		t.Fatalf("unexpected core state: %+v", core)
	}
	if core.CapabilitiesJSON != `["reality"]` || core.BuildTagsJSON != `["with_gvisor"]` {
		t.Fatalf("unexpected core metadata: %+v", core)
	}
}

func TestBinaryVersionServiceUpdateLocalVersionsClearsExplicitlyMissingCore(t *testing.T) {
	repo := newMockBinaryVersionRepo()
	hosts := newMockBinaryVersionHostRepo()
	hosts.hosts[8] = &repository.AgentHost{ID: 8}
	repo.states[repo.key(8, BinaryVersionComponentXray)] = &repository.BinaryVersionState{ID: 2, AgentHostID: 8, Component: BinaryVersionComponentXray, LocalVersion: "25.1.1", RemoteVersion: "26.3.27", Status: BinaryVersionStatusOutdated, CapabilitiesJSON: "[]", BuildTagsJSON: "[]"}
	svc := newBinaryVersionTestService(repo, hosts, nil)
	installed := false

	err := svc.UpdateLocalVersions(context.Background(), UpdateLocalVersionsRequest{
		AgentHostID: 8,
		CoreStates:  []CoreVersionReport{{Component: "xray", Installed: &installed}},
	})
	if err != nil {
		t.Fatalf("UpdateLocalVersions returned error: %v", err)
	}

	state, err := repo.FindByHostComponent(context.Background(), 8, BinaryVersionComponentXray)
	if err != nil {
		t.Fatalf("xray state missing: %v", err)
	}
	if state.LocalVersion != "" || state.RemoteVersion != "26.3.27" || state.Status != BinaryVersionStatusMissing {
		t.Fatalf("unexpected xray state after uninstall: %+v", state)
	}
}

func TestBinaryVersionServiceListVersionStatesRepairsMissingCoreFromCompletedOperation(t *testing.T) {
	repo := newMockBinaryVersionRepo()
	hosts := newMockBinaryVersionHostRepo()
	hosts.hosts[8] = &repository.AgentHost{ID: 8, CurrentCoreType: "sing-box", CoreVersion: "1.13.14"}
	repo.states[repo.key(8, BinaryVersionComponentXray)] = &repository.BinaryVersionState{ID: 2, AgentHostID: 8, Component: BinaryVersionComponentXray, LocalVersion: "", RemoteVersion: "v26.3.27", Status: BinaryVersionStatusMissing, CapabilitiesJSON: "[]", BuildTagsJSON: "[]", UpdatedAt: 1710000200}
	payload, err := json.Marshal(&agentv1.InstallCorePayload{Action: "install"})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	result, err := json.Marshal(&agentv1.InstallCoreResponse{Success: true, CoreType: "xray", Version: "26.3.27"})
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	finishedAt := int64(1710000100)
	ops := &mockBinaryVersionCoreOperationRepo{operations: []*repository.CoreOperation{{
		ID: "op-xray-install", AgentHostID: 8, OperationType: "install", CoreType: "xray", Status: "completed",
		RequestPayload: payload, ResultPayload: result, FinishedAt: &finishedAt, UpdatedAt: finishedAt,
	}}}
	svc := NewBinaryVersionServiceWithOptions(repo, hosts, nil, BinaryVersionServiceOptions{Now: func() time.Time { return time.Unix(1710000300, 0) }, CoreOperations: ops})

	states, err := svc.ListVersionStates(context.Background(), ListVersionStatesRequest{AgentHostID: 8})
	if err != nil {
		t.Fatalf("ListVersionStates returned error: %v", err)
	}
	byComponent := make(map[string]BinaryVersionStateView, len(states))
	for _, state := range states {
		byComponent[state.Component] = state
	}
	xray := byComponent[BinaryVersionComponentXray]
	if xray.LocalVersion != "26.3.27" || xray.Status != BinaryVersionStatusUpToDate || xray.RemoteVersion != "v26.3.27" {
		t.Fatalf("expected repaired xray state, got %+v", xray)
	}
}

func TestBinaryVersionServiceRefreshRemoteVersionRepairsLocalCoreBeforeChecking(t *testing.T) {
	repo := newMockBinaryVersionRepo()
	hosts := newMockBinaryVersionHostRepo()
	hosts.hosts[9] = &repository.AgentHost{ID: 9, CurrentCoreType: "sing-box", CoreVersion: "1.13.14"}
	repo.states[repo.key(9, BinaryVersionComponentXray)] = &repository.BinaryVersionState{ID: 3, AgentHostID: 9, Component: BinaryVersionComponentXray, LocalVersion: "", Status: BinaryVersionStatusMissing, CapabilitiesJSON: "[]", BuildTagsJSON: "[]", UpdatedAt: 1710000200}
	payload, err := json.Marshal(&agentv1.InstallCorePayload{Action: "install"})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	result, err := json.Marshal(&agentv1.InstallCoreResponse{Success: true, CoreType: "xray", Version: "26.3.27"})
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	finishedAt := int64(1710000100)
	ops := &mockBinaryVersionCoreOperationRepo{operations: []*repository.CoreOperation{{
		ID: "op-xray-install", AgentHostID: 9, OperationType: "install", CoreType: "xray", Status: "completed",
		RequestPayload: payload, ResultPayload: result, FinishedAt: &finishedAt, UpdatedAt: finishedAt,
	}}}
	svc := NewBinaryVersionServiceWithOptions(repo, hosts, stubBinaryVersionRemoteProvider{versions: map[string]string{BinaryVersionComponentXray: "v26.3.27"}}, BinaryVersionServiceOptions{Now: func() time.Time { return time.Unix(1710000300, 0) }, CoreOperations: ops})

	state, err := svc.RefreshRemoteVersion(context.Background(), RefreshRemoteVersionRequest{AgentHostID: 9, Component: "xray"})
	if err != nil {
		t.Fatalf("RefreshRemoteVersion returned error: %v", err)
	}
	if state.LocalVersion != "26.3.27" || state.RemoteVersion != "v26.3.27" || state.Status != BinaryVersionStatusUpToDate {
		t.Fatalf("expected refreshed repaired xray state, got %+v", state)
	}
}

func TestBinaryVersionServiceListVersionStatesIncludesHostFallbacks(t *testing.T) {
	repo := newMockBinaryVersionRepo()
	hosts := newMockBinaryVersionHostRepo()
	hosts.hosts[3] = &repository.AgentHost{ID: 3, AgentVersion: "0.1.0", CurrentCoreType: "xray", CoreVersion: "25.1.1", Capabilities: []string{"vision"}, BuildTags: []string{"official"}}
	svc := newBinaryVersionTestService(repo, hosts, nil)

	states, err := svc.ListVersionStates(context.Background(), ListVersionStatesRequest{AgentHostID: 3})
	if err != nil {
		t.Fatalf("ListVersionStates returned error: %v", err)
	}
	if len(states) != 3 {
		t.Fatalf("expected 3 states, got %d", len(states))
	}
	byComponent := make(map[string]BinaryVersionStateView, len(states))
	for _, state := range states {
		byComponent[state.Component] = state
	}
	if byComponent[BinaryVersionComponentAgent].LocalVersion != "0.1.0" {
		t.Fatalf("expected agent fallback version, got %+v", byComponent[BinaryVersionComponentAgent])
	}
	if byComponent[BinaryVersionComponentSingBox].Status != BinaryVersionStatusMissing {
		t.Fatalf("expected sing-box missing, got %+v", byComponent[BinaryVersionComponentSingBox])
	}
	xray := byComponent[BinaryVersionComponentXray]
	if xray.LocalVersion != "25.1.1" || xray.Status != BinaryVersionStatusInstalled || len(xray.Capabilities) != 1 || xray.Capabilities[0] != "vision" {
		t.Fatalf("unexpected xray fallback: %+v", xray)
	}
}

func TestBinaryVersionServiceRefreshRemoteVersionUpdatesStatus(t *testing.T) {
	repo := newMockBinaryVersionRepo()
	hosts := newMockBinaryVersionHostRepo()
	hosts.hosts[4] = &repository.AgentHost{ID: 4}
	repo.states[repo.key(4, BinaryVersionComponentSingBox)] = &repository.BinaryVersionState{ID: 1, AgentHostID: 4, Component: BinaryVersionComponentSingBox, LocalVersion: "1.10.0", Status: BinaryVersionStatusInstalled, CapabilitiesJSON: "[]", BuildTagsJSON: "[]"}
	svc := newBinaryVersionTestService(repo, hosts, stubBinaryVersionRemoteProvider{versions: map[string]string{BinaryVersionComponentSingBox: "1.12.0"}})

	state, err := svc.RefreshRemoteVersion(context.Background(), RefreshRemoteVersionRequest{AgentHostID: 4, Component: "sing-box"})
	if err != nil {
		t.Fatalf("RefreshRemoteVersion returned error: %v", err)
	}
	if state.RemoteVersion != "1.12.0" || state.Status != BinaryVersionStatusOutdated || state.LastCheckedAt != 1710000000 || state.LastCheckError != "" {
		t.Fatalf("unexpected refreshed state: %+v", state)
	}
}

func TestBinaryVersionServiceRefreshFailurePreservesExistingVersions(t *testing.T) {
	repo := newMockBinaryVersionRepo()
	hosts := newMockBinaryVersionHostRepo()
	hosts.hosts[5] = &repository.AgentHost{ID: 5}
	repo.states[repo.key(5, BinaryVersionComponentXray)] = &repository.BinaryVersionState{ID: 9, AgentHostID: 5, Component: BinaryVersionComponentXray, LocalVersion: "25.1.1", RemoteVersion: "25.2.0", Status: BinaryVersionStatusOutdated, CapabilitiesJSON: "[]", BuildTagsJSON: "[]", LastCheckedAt: 1700000000}
	svc := newBinaryVersionTestService(repo, hosts, stubBinaryVersionRemoteProvider{err: errors.New("remote down")})

	state, err := svc.RefreshRemoteVersion(context.Background(), RefreshRemoteVersionRequest{AgentHostID: 5, Component: "xray"})
	if err != nil {
		t.Fatalf("RefreshRemoteVersion returned error: %v", err)
	}
	if state.LocalVersion != "25.1.1" || state.RemoteVersion != "25.2.0" || state.Status != BinaryVersionStatusOutdated {
		t.Fatalf("refresh failure should preserve versions/status: %+v", state)
	}
	if state.LastCheckError != "remote down" || state.LastCheckedAt != 1710000000 {
		t.Fatalf("unexpected refresh error fields: %+v", state)
	}
}

func TestBinaryVersionServiceRejectsInvalidRequests(t *testing.T) {
	repo := newMockBinaryVersionRepo()
	hosts := newMockBinaryVersionHostRepo()
	hosts.hosts[1] = &repository.AgentHost{ID: 1}
	svc := newBinaryVersionTestService(repo, hosts, nil)

	if err := svc.UpdateLocalVersions(context.Background(), UpdateLocalVersionsRequest{AgentHostID: 0}); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("expected bad request for missing host id, got %v", err)
	}
	if _, err := svc.RefreshRemoteVersion(context.Background(), RefreshRemoteVersionRequest{AgentHostID: 1, Component: "bad"}); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("expected bad request for invalid component, got %v", err)
	}
	if _, err := svc.ListVersionStates(context.Background(), ListVersionStatesRequest{AgentHostID: 99}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found for missing host, got %v", err)
	}
}
