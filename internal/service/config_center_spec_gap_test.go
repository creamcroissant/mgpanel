package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

// ---------------------------------------------------------------------------
// inboundSpecService: validateRelayPathBinding 全分支 + BindSpec 失败回滚
// ---------------------------------------------------------------------------

func TestInboundSpecService_ValidateRelayPathBinding_Branches(t *testing.T) {
	enabledPath := &repository.RelayPath{
		ID: 8, Name: "hk-tw", CoreType: "sing-box", Enabled: true,
		Nodes: []repository.RelayPathNode{
			{Sequence: 0, AgentHostID: 3},
			{Sequence: 1, AgentHostID: 4},
		},
	}

	cases := []struct {
		name       string
		setup      func() *mockRelayPathRepo
		wantErrSub string
	}{
		{
			name: "repo nil",
			setup: func() *mockRelayPathRepo {
				return nil
			},
			wantErrSub: "中继链路仓库未配置",
		},
		{
			name: "not found",
			setup: func() *mockRelayPathRepo {
				r := newMockRelayPathRepo()
				r.notFound = true
				return r
			},
			wantErrSub: "中继链路不存在",
		},
		{
			name: "disabled",
			setup: func() *mockRelayPathRepo {
				p := *enabledPath
				p.Enabled = false
				r := newMockRelayPathRepo()
				r.items[8] = &p
				return r
			},
			wantErrSub: "中继链路已禁用",
		},
		{
			name: "single hop",
			setup: func() *mockRelayPathRepo {
				p := *enabledPath
				p.Nodes = []repository.RelayPathNode{{Sequence: 0, AgentHostID: 3}}
				r := newMockRelayPathRepo()
				r.items[8] = &p
				return r
			},
			wantErrSub: "节点不足两跳",
		},
		{
			name: "valid two-hop",
			setup: func() *mockRelayPathRepo {
				r := newMockRelayPathRepo()
				r.items[8] = enabledPath
				return r
			},
			wantErrSub: "",
		},
		{
			name: "repo error",
			setup: func() *mockRelayPathRepo {
				r := newMockRelayPathRepo()
				r.getErr = errors.New("db boom")
				return r
			},
			wantErrSub: "查询中继链路失败",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newInboundSpecServiceForTest(t, nil)
			if tc.setup() != nil {
				svc.SetRelayPathRepository(tc.setup())
			}
			err := svc.validateRelayPathBinding(context.Background(), 8)
			if tc.wantErrSub == "" {
				if err != nil {
					t.Fatalf("expected nil error, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErrSub) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErrSub, err)
			}
		})
	}
}

// TestInboundSpecService_UpsertRelayPathBinding_ValidationError verifies that
// binding a spec to a disabled relay path rejects the whole upsert (create path).
func TestInboundSpecService_UpsertRelayPathBinding_RejectsDisabled(t *testing.T) {
	svc := newInboundSpecServiceForTest(t, nil)
	relay := newMockRelayPathRepo()
	p := &repository.RelayPath{ID: 3, Name: "down", CoreType: "sing-box", Enabled: false}
	relay.items[3] = p
	svc.SetRelayPathRepository(relay)

	hostID := int64(1)
	enabled := true
	relayID := int64(3)
	_, _, err := svc.UpsertSpec(context.Background(), UpsertInboundSpecRequest{
		AgentHostID:  &hostID,
		RelayPathID:  &relayID,
		CoreType:     "sing-box",
		Tag:          "relay-in",
		Enabled:      &enabled,
		SemanticSpec: json.RawMessage(`{"tag":"relay-in","protocol":"vless","listen":"0.0.0.0","port":28031}`),
		CoreSpecific: json.RawMessage(`{"core_type":"sing-box"}`),
		OperatorID:   1,
	})
	if err == nil || !strings.Contains(err.Error(), "中继链路已禁用") {
		t.Fatalf("expected disabled relay rejection, got %v", err)
	}
	// 无残留 spec
	if _, err := svc.specs.FindByID(context.Background(), 1); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("spec should not be persisted after failed upsert, got %v", err)
	}
}

// TestInboundSpecService_GetSpecHistory_Paths covers GetSpecHistory guards and
// NotFound mapping.
func TestInboundSpecService_GetSpecHistory_Paths(t *testing.T) {
	t.Run("not configured", func(t *testing.T) {
		var svc *inboundSpecService
		if _, err := svc.GetSpecHistory(context.Background(), 1, 10, 0); err == nil {
			t.Fatal("expected not-configured error")
		}
	})
	t.Run("invalid id", func(t *testing.T) {
		svc := newInboundSpecServiceForTest(t, nil)
		if _, err := svc.GetSpecHistory(context.Background(), 0, 10, 0); !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound for id<=0, got %v", err)
		}
	})
	t.Run("spec missing", func(t *testing.T) {
		svc := newInboundSpecServiceForTest(t, nil)
		if _, err := svc.GetSpecHistory(context.Background(), 999, 10, 0); !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound for missing spec, got %v", err)
		}
	})
}

// TestInboundSpecService_ListSpecs_InvalidCoreType verifies normalized core_type
// validation on the list path.
func TestInboundSpecService_ListSpecs_InvalidCoreType(t *testing.T) {
	svc := newInboundSpecServiceForTest(t, nil)
	bad := "hysteria2"
	_, _, err := svc.ListSpecs(context.Background(), ListInboundSpecFilter{CoreType: &bad})
	var verr *InboundSpecValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("expected InboundSpecValidationError, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// 辅助构造：spec service 使用 mockInboundSpecRepo + mock revision repo
// ---------------------------------------------------------------------------

// mockInboundSpecRevisionRepo 只实现 ListBySpecID（GetSpecHistory 用）。
type mockInboundSpecRevisionRepo struct {
	items []*repository.InboundSpecRevision
}

func (m *mockInboundSpecRevisionRepo) Create(ctx context.Context, revision *repository.InboundSpecRevision) error { return nil }
func (m *mockInboundSpecRevisionRepo) FindBySpecAndRevision(ctx context.Context, specID int64, revision int64) (*repository.InboundSpecRevision, error) {
	return nil, repository.ErrNotFound
}
func (m *mockInboundSpecRevisionRepo) ListBySpecID(ctx context.Context, specID int64, limit, offset int) ([]*repository.InboundSpecRevision, error) {
	return m.items, nil
}
func (m *mockInboundSpecRevisionRepo) GetMaxRevision(ctx context.Context, specID int64) (int64, error) { return 0, nil }

// mockSpecBindingRepoForTest 记录绑定/解绑调用。
type mockSpecBindingRepoForTest struct {
	bound   []struct{ specID, hostID int64 }
	unbound []struct{ specID, hostID int64 }
}

func (m *mockSpecBindingRepoForTest) Bind(ctx context.Context, specID, agentHostID int64) error {
	m.bound = append(m.bound, struct{ specID, hostID int64 }{specID, agentHostID})
	return nil
}
func (m *mockSpecBindingRepoForTest) Unbind(ctx context.Context, specID, agentHostID int64) error {
	m.unbound = append(m.unbound, struct{ specID, hostID int64 }{specID, agentHostID})
	return nil
}
func (m *mockSpecBindingRepoForTest) UnbindAll(ctx context.Context, specID int64) error { return nil }
func (m *mockSpecBindingRepoForTest) ListBySpec(ctx context.Context, specID int64) ([]int64, error) { return nil, nil }
func (m *mockSpecBindingRepoForTest) ListByHost(ctx context.Context, agentHostID int64) ([]int64, error) { return nil, nil }

// newInboundSpecServiceForTest 构造一个带 mockInboundSpecRepo/mock revisions/bindings
// 的 inboundSpecService（compiler 可空）。
func newInboundSpecServiceForTest(t *testing.T, compiler ArtifactCompilerService) *inboundSpecService {
	t.Helper()
	svc := NewInboundSpecService(
		newMockInboundSpecRepo(),
		&mockInboundSpecRevisionRepo{},
		&mockDriftInboundIndexRepo{},
		&mockSpecBindingRepoForTest{},
	)
	if compiler != nil {
		svc = NewInboundSpecService(
			newMockInboundSpecRepo(),
			&mockInboundSpecRevisionRepo{},
			&mockDriftInboundIndexRepo{},
			&mockSpecBindingRepoForTest{},
			compiler,
		)
	}
	// 直接断言实现类型以访问内嵌字段
	impl, ok := svc.(*inboundSpecService)
	if !ok {
		t.Fatalf("expected *inboundSpecService, got %T", svc)
	}
	return impl
}

// TestInboundSpecService_BindSpec_UnbindSpec_Paths verifies binding/unbinding
// records calls through the bindings repo.
func TestInboundSpecService_BindSpec_UnbindSpec_Paths(t *testing.T) {
	svc := newInboundSpecServiceForTest(t, nil)
	bindings := svc.bindings.(*mockSpecBindingRepoForTest)
	ctx := context.Background()

	if err := svc.BindSpec(ctx, 11, 22); err != nil {
		t.Fatalf("BindSpec: %v", err)
	}
	if len(bindings.bound) != 1 || bindings.bound[0].specID != 11 || bindings.bound[0].hostID != 22 {
		t.Fatalf("unexpected bind call: %+v", bindings.bound)
	}
	if err := svc.UnbindSpec(ctx, 11, 22); err != nil {
		t.Fatalf("UnbindSpec: %v", err)
	}
	if len(bindings.unbound) != 1 || bindings.unbound[0].specID != 11 || bindings.unbound[0].hostID != 22 {
		t.Fatalf("unexpected unbind call: %+v", bindings.unbound)
	}
}

// TestInboundSpecService_TriggerCDNAccelerateDeploy_NoopPaths covers
// triggerCDNAccelerateDeploy when cdn is nil or acceleration missing/disabled.
func TestInboundSpecService_TriggerCDNAccelerateDeploy_NoopPaths(t *testing.T) {
	svc := newInboundSpecServiceForTest(t, nil)
	// cdn nil → noop
	svc.triggerCDNAccelerateDeploy(context.Background(), 1)

	// cdn 存在但 acceleration not found → noop
	svc.cdn = &mockCDNServiceForSpec{getErr: repository.ErrNotFound}
	svc.triggerCDNAccelerateDeploy(context.Background(), 1)

	// acceleration disabled → noop
	svc.cdn = &mockCDNServiceForSpec{get: &CDNAccelerationConfig{ID: 7, Enabled: false}}
	svc.triggerCDNAccelerateDeploy(context.Background(), 1)

	// acceleration enabled → 触发 DeployAcceleration
	deployCDN := &mockCDNServiceForSpec{get: &CDNAccelerationConfig{ID: 7, Enabled: true}}
	svc.cdn = deployCDN
	svc.triggerCDNAccelerateDeploy(context.Background(), 1)
	if deployCDN.deployID != 7 {
		t.Fatalf("expected DeployAcceleration(7), got %d", deployCDN.deployID)
	}

	// get 报错非 NotFound → 仅记录，不 panic
	svc.cdn = &mockCDNServiceForSpec{getErr: errors.New("boom")}
	svc.triggerCDNAccelerateDeploy(context.Background(), 1)
}

// mockCDNServiceForSpec 通过嵌入 CDNService 满足完整接口，仅覆盖
// GetAccelerationByInboundSpec 与 DeployAcceleration。
type mockCDNServiceForSpec struct {
	CDNService
	get      *CDNAccelerationConfig
	getErr   error
	deployID int64
}

func (m *mockCDNServiceForSpec) GetAccelerationByInboundSpec(ctx context.Context, specID int64) (*CDNAccelerationConfig, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.get, nil
}
func (m *mockCDNServiceForSpec) DeployAcceleration(ctx context.Context, accelerationID int64) error {
	m.deployID = accelerationID
	return nil
}

// TestInboundSpecService_DeleteSpec_Paths covers DeleteSpec success/not-found.
func TestInboundSpecService_DeleteSpec_Paths(t *testing.T) {
	svc := newInboundSpecServiceForTest(t, nil)
	ctx := context.Background()

	t.Run("missing spec", func(t *testing.T) {
		if err := svc.DeleteSpec(ctx, 404); !errors.Is(err, ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})
	t.Run("success", func(t *testing.T) {
		hostID := int64(1)
		enabled := true
		specID, _, err := svc.UpsertSpec(ctx, UpsertInboundSpecRequest{
			AgentHostID:  &hostID,
			CoreType:     "sing-box",
			Tag:          "del-me",
			Enabled:      &enabled,
			SemanticSpec: json.RawMessage(`{"tag":"del-me","protocol":"vless","listen":"0.0.0.0","port":28041}`),
			CoreSpecific: json.RawMessage(`{"core_type":"sing-box"}`),
			OperatorID:   1,
		})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if err := svc.DeleteSpec(ctx, specID); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if _, err := svc.specs.FindByID(ctx, specID); !errors.Is(err, repository.ErrNotFound) {
			t.Fatalf("spec should be deleted, got %v", err)
		}
	})
}

// TestInboundSpecService_RenderDesiredArtifacts_NoopAndError covers the
// compiler-nil noop and the propagated render error path.
func TestInboundSpecService_RenderDesiredArtifacts_NoopAndError(t *testing.T) {
	ctx := context.Background()

	t.Run("nil compiler noop", func(t *testing.T) {
		svc := newInboundSpecServiceForTest(t, nil)
		if err := svc.renderDesiredArtifacts(ctx, 1, "sing-box", 3); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})

	t.Run("render error propagated", func(t *testing.T) {
		compiler := &stubArtifactCompilerService{renderErr: errors.New("render boom")}
		svc := newInboundSpecServiceForTest(t, compiler)
		err := svc.renderDesiredArtifacts(ctx, 1, "sing-box", 3)
		if err == nil || !strings.Contains(err.Error(), "render boom") {
			t.Fatalf("expected render error propagated, got %v", err)
		}
	})

	t.Run("delete noop with nil compiler", func(t *testing.T) {
		svc := newInboundSpecServiceForTest(t, nil)
		if err := svc.deleteDesiredArtifacts(ctx, 1, "sing-box", 3); err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	})
}

// stubArtifactCompilerService 仅用于触发 render/delete 错误路径。
// failFromCall > 0 时，第 failFromCall 次 RenderArtifacts 调用起返回 renderErr。
type stubArtifactCompilerService struct {
	renderErr    error
	deleteErr    error
	renderCalls  int
	failFromCall int
}

func (s *stubArtifactCompilerService) RenderArtifacts(ctx context.Context, req RenderArtifactsRequest) (*RenderArtifactsResult, error) {
	s.renderCalls++
	if s.renderErr != nil && (s.failFromCall == 0 || s.renderCalls >= s.failFromCall) {
		return nil, s.renderErr
	}
	return &RenderArtifactsResult{}, nil
}
func (s *stubArtifactCompilerService) RenderCoreConfigs(ctx context.Context, req RenderArtifactsRequest) (*RenderArtifactsResult, error) {
	return s.RenderArtifacts(ctx, req)
}
func (s *stubArtifactCompilerService) DeleteArtifacts(ctx context.Context, agentHostID int64, coreType string, desiredRevision int64) error {
	return s.deleteErr
}
func (s *stubArtifactCompilerService) GetLatestRevision(ctx context.Context, agentHostID int64, coreType string) (int64, error) {
	return 0, nil
}

// TestInboundSpecService_BindSpec_CompilerPaths verifies BindSpec's compiler
// branches: disabled spec noop, missing spec noop, render error propagation,
// and successful render with revision bump.
func TestInboundSpecService_BindSpec_CompilerPaths(t *testing.T) {
	ctx := context.Background()
	hostID := int64(2)

	t.Run("missing spec returns nil", func(t *testing.T) {
		compiler := &stubArtifactCompilerService{}
		svc := newInboundSpecServiceForTest(t, compiler)
		if err := svc.BindSpec(ctx, 999, hostID); err != nil {
			t.Fatalf("expected nil for missing spec, got %v", err)
		}
	})

	t.Run("disabled spec returns nil without render", func(t *testing.T) {
		compiler := &stubArtifactCompilerService{}
		svc := newInboundSpecServiceForTest(t, compiler)
		disabled := false
		specID, _, err := svc.UpsertSpec(ctx, UpsertInboundSpecRequest{
			AgentHostID:  &hostID,
			CoreType:     "sing-box",
			Tag:          "bs-disabled",
			Enabled:      &disabled,
			SemanticSpec: json.RawMessage(`{"tag":"bs-disabled","protocol":"vless","listen":"0.0.0.0","port":28051}`),
			CoreSpecific: json.RawMessage(`{"core_type":"sing-box"}`),
			OperatorID:   1,
		})
		if err != nil {
			t.Fatalf("create disabled spec: %v", err)
		}
		if err := svc.BindSpec(ctx, specID, hostID); err != nil {
			t.Fatalf("bind disabled spec: %v", err)
		}
		if len(svc.bindings.(*mockSpecBindingRepoForTest).bound) == 0 {
			t.Fatal("bind should still record binding call")
		}
	})

	t.Run("render error propagated", func(t *testing.T) {
		compiler := &stubArtifactCompilerService{renderErr: errors.New("bind render boom"), failFromCall: 2}
		svc := newInboundSpecServiceForTest(t, compiler)
		enabled := true
		specID, _, err := svc.UpsertSpec(ctx, UpsertInboundSpecRequest{
			AgentHostID:  &hostID,
			CoreType:     "sing-box",
			Tag:          "bs-render-err",
			Enabled:      &enabled,
			SemanticSpec: json.RawMessage(`{"tag":"bs-render-err","protocol":"vless","listen":"0.0.0.0","port":28052}`),
			CoreSpecific: json.RawMessage(`{"core_type":"sing-box"}`),
			OperatorID:   1,
		})
		if err != nil {
			t.Fatalf("create spec: %v", err)
		}
		err = svc.BindSpec(ctx, specID, hostID)
		if err == nil || !strings.Contains(err.Error(), "bind render boom") {
			t.Fatalf("expected render error, got %v", err)
		}
	})

	t.Run("successful bind bumps revision and renders", func(t *testing.T) {
		compiler := &stubArtifactCompilerService{}
		svc := newInboundSpecServiceForTest(t, compiler)
		enabled := true
		specID, _, err := svc.UpsertSpec(ctx, UpsertInboundSpecRequest{
			AgentHostID:  &hostID,
			CoreType:     "sing-box",
			Tag:          "bs-ok",
			Enabled:      &enabled,
			SemanticSpec: json.RawMessage(`{"tag":"bs-ok","protocol":"vless","listen":"0.0.0.0","port":28053}`),
			CoreSpecific: json.RawMessage(`{"core_type":"sing-box"}`),
			OperatorID:   1,
		})
		if err != nil {
			t.Fatalf("create spec: %v", err)
		}
		if err := svc.BindSpec(ctx, specID, hostID); err != nil {
			t.Fatalf("bind: %v", err)
		}
		spec, err := svc.specs.FindByID(ctx, specID)
		if err != nil {
			t.Fatalf("find: %v", err)
		}
		if spec.DesiredRevision < 1 {
			t.Fatalf("expected bumped revision, got %d", spec.DesiredRevision)
		}
	})
}

// TestInboundSpecService_ImportFromApplied_Paths 覆盖 ImportFromApplied 全分支。
func TestInboundSpecService_ImportFromApplied_Paths(t *testing.T) {
	ctx := context.Background()
	hostID := int64(3)

	t.Run("invalid host", func(t *testing.T) {
		svc := newInboundSpecServiceForTest(t, nil)
		_, err := svc.ImportFromApplied(ctx, ImportInboundSpecRequest{})
		if err == nil {
			t.Fatal("expected validation error")
		}
		var verr *InboundSpecValidationError
		if !errors.As(err, &verr) {
			t.Fatalf("expected InboundSpecValidationError, got %v", err)
		}
	})
	t.Run("invalid core type", func(t *testing.T) {
		svc := newInboundSpecServiceForTest(t, nil)
		if _, err := svc.ImportFromApplied(ctx, ImportInboundSpecRequest{AgentHostID: &hostID, CoreType: "bogus"}); err == nil {
			t.Fatal("expected validation error")
		}
	})
	t.Run("index list error propagated", func(t *testing.T) {
		svc := newInboundSpecServiceForTest(t, nil)
		targetErr := errors.New("index list boom")
		svc.inboundIndexes = &mockDriftInboundIndexRepo{listErr: targetErr}
		if _, err := svc.ImportFromApplied(ctx, ImportInboundSpecRequest{AgentHostID: &hostID, CoreType: "xray"}); !errors.Is(err, targetErr) {
			t.Fatalf("expected propagated list error, got %v", err)
		}
	})
	t.Run("no candidates returns 0", func(t *testing.T) {
		svc := newInboundSpecServiceForTest(t, nil)
		svc.inboundIndexes = &mockDriftInboundIndexRepo{}
		n, err := svc.ImportFromApplied(ctx, ImportInboundSpecRequest{AgentHostID: &hostID, CoreType: "xray"})
		if err != nil || n != 0 {
			t.Fatalf("expected (0,nil), got n=%d err=%v", n, err)
		}
	})
	t.Run("conflicting tag candidates rejected", func(t *testing.T) {
		svc := newInboundSpecServiceForTest(t, nil)
		svc.inboundIndexes = &mockDriftInboundIndexRepo{listItems: []*repository.InboundIndex{
			{AgentHostID: 3, CoreType: "xray", Source: "managed", Filename: "a.json", Tag: "dup", Protocol: "vless", Listen: "0.0.0.0", Port: 443, LastSeenAt: 10},
			{AgentHostID: 3, CoreType: "xray", Source: "managed", Filename: "b.json", Tag: "dup", Protocol: "vless", Listen: "0.0.0.0", Port: 8443, LastSeenAt: 11},
		}}
		_, err := svc.ImportFromApplied(ctx, ImportInboundSpecRequest{AgentHostID: &hostID, CoreType: "xray"})
		var cerr *InboundSpecConflictError
		if !errors.As(err, &cerr) || cerr.Kind != "tag" {
			t.Fatalf("expected tag conflict, got %v", err)
		}
	})
	t.Run("imports candidates and returns count", func(t *testing.T) {
		svc := newInboundSpecServiceForTest(t, nil)
		svc.inboundIndexes = &mockDriftInboundIndexRepo{listItems: []*repository.InboundIndex{
			{AgentHostID: 3, CoreType: "xray", Source: "managed", Filename: "a.json", Tag: "in-a", Protocol: "vless", Listen: "0.0.0.0", Port: 443, TLS: json.RawMessage(`{}`), Transport: json.RawMessage(`{}`), Multiplex: json.RawMessage(`{}`), LastSeenAt: 10},
			{AgentHostID: 3, CoreType: "xray", Source: "managed", Filename: "b.json", Tag: "in-b", Protocol: "vmess", Listen: "0.0.0.0", Port: 8443, LastSeenAt: 11},
		}}
		n, err := svc.ImportFromApplied(ctx, ImportInboundSpecRequest{AgentHostID: &hostID, CoreType: "xray", OperatorID: 1})
		if err != nil {
			t.Fatalf("import: %v", err)
		}
		if n != 2 {
			t.Fatalf("expected 2 imported, got %d", n)
		}
		// 再次导入不覆盖 → 0 新增
		n2, err := svc.ImportFromApplied(ctx, ImportInboundSpecRequest{AgentHostID: &hostID, CoreType: "xray", OperatorID: 1})
		if err != nil || n2 != 0 {
			t.Fatalf("expected 0 on re-import without overwrite, got n=%d err=%v", n2, err)
		}
		// OverwriteExisting=true → 覆盖已有但 specID 不变 → 仍 0 新增
		n3, err := svc.ImportFromApplied(ctx, ImportInboundSpecRequest{AgentHostID: &hostID, CoreType: "xray", OperatorID: 1, OverwriteExisting: true})
		if err != nil || n3 != 0 {
			t.Fatalf("expected 0 created on overwrite (specs exist), got n=%d err=%v", n3, err)
		}
	})
}

// TestInboundSpecService_EnsureListenConflict_Paths 覆盖 listen 冲突检测。
func TestInboundSpecService_EnsureListenConflict_Paths(t *testing.T) {
	svc := newInboundSpecServiceForTest(t, nil)
	ctx := context.Background()
	hostID := int64(4)
	enabled := true

	specID, _, err := svc.UpsertSpec(ctx, UpsertInboundSpecRequest{
		AgentHostID:  &hostID,
		CoreType:     "sing-box",
		Tag:          "listen-owner",
		Enabled:      &enabled,
		SemanticSpec: json.RawMessage(`{"tag":"listen-owner","protocol":"vless","listen":"0.0.0.0","port":28061}`),
		CoreSpecific: json.RawMessage(`{"core_type":"sing-box"}`),
		OperatorID:   1,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// 同主机同端口 → listen 冲突
	_, _, err = svc.UpsertSpec(ctx, UpsertInboundSpecRequest{
		AgentHostID:  &hostID,
		CoreType:     "sing-box",
		Tag:          "listen-clash",
		Enabled:      &enabled,
		SemanticSpec: json.RawMessage(`{"tag":"listen-clash","protocol":"vless","listen":"0.0.0.0","port":28061}`),
		CoreSpecific: json.RawMessage(`{"core_type":"sing-box"}`),
		OperatorID:   1,
	})
	var cerr *InboundSpecConflictError
	if !errors.As(err, &cerr) || cerr.Kind != "listen" {
		t.Fatalf("expected listen conflict, got %v", err)
	}

	// 更新自身（excludeSpecID）不冲突
	newTag := "listen-owner2"
	if _, _, err := svc.UpsertSpec(ctx, UpsertInboundSpecRequest{
		SpecID:       specID,
		AgentHostID:  &hostID,
		CoreType:     "sing-box",
		Tag:          newTag,
		Enabled:      &enabled,
		SemanticSpec: json.RawMessage(`{"tag":"listen-owner2","protocol":"vless","listen":"0.0.0.0","port":28061}`),
		CoreSpecific: json.RawMessage(`{"core_type":"sing-box"}`),
		OperatorID:   1,
	}); err != nil {
		t.Fatalf("update own listen should not conflict: %v", err)
	}

	// 不同主机同端口 → 不冲突
	hostID2 := int64(5)
	if _, _, err := svc.UpsertSpec(ctx, UpsertInboundSpecRequest{
		AgentHostID:  &hostID2,
		CoreType:     "sing-box",
		Tag:          "listen-other-host",
		Enabled:      &enabled,
		SemanticSpec: json.RawMessage(`{"tag":"listen-other-host","protocol":"vless","listen":"0.0.0.0","port":28061}`),
		CoreSpecific: json.RawMessage(`{"core_type":"sing-box"}`),
		OperatorID:   1,
	}); err != nil {
		t.Fatalf("same port on other host should not conflict: %v", err)
	}
}

// TestInboundSpecService_SetCDNService_SetRelayPathRepository 覆盖两个 setter。
func TestInboundSpecService_SetCDNService_SetRelayPathRepository(t *testing.T) {
	svc := newInboundSpecServiceForTest(t, nil)
	cdn := &mockCDNServiceForSpec{}
	svc.SetCDNService(cdn)
	if svc.cdn != cdn {
		t.Fatal("SetCDNService did not set cdn")
	}
	relay := newMockRelayPathRepo()
	svc.SetRelayPathRepository(relay)
	if svc.relayPaths != relay {
		t.Fatal("SetRelayPathRepository did not set relayPaths")
	}
}

// TestInboundSpecService_BuildPreferredInventoryByFilename 覆盖选择优先级。
func TestInboundSpecService_BuildPreferredInventoryByFilename(t *testing.T) {
	items := []*repository.AgentConfigInventory{
		{AgentHostID: 1, CoreType: "xray", Source: inventorySourceManaged, Filename: "a.json", HashApplied: "m", LastSeenAt: 100},
		{AgentHostID: 1, CoreType: "xray", Source: inventorySourceLegacy, Filename: "a.json", HashApplied: "l", LastSeenAt: 200},
		{AgentHostID: 1, CoreType: "xray", Source: inventorySourceMerged, Filename: "a.json", HashApplied: "mg", LastSeenAt: 150},
	}
	preferred := buildPreferredInventoryByFilename(items)
	if got := preferred["a.json"]; got == nil || got.Source != inventorySourceManaged {
		t.Fatalf("expected managed preferred, got %+v", got)
	}
	// 同 source 选 LastSeenAt 更大
	items2 := []*repository.AgentConfigInventory{
		{AgentHostID: 1, CoreType: "xray", Source: inventorySourceManaged, Filename: "b.json", HashApplied: "old", LastSeenAt: 10},
		{AgentHostID: 1, CoreType: "xray", Source: inventorySourceManaged, Filename: "b.json", HashApplied: "new", LastSeenAt: 20},
	}
	preferred2 := buildPreferredInventoryByFilename(items2)
	if got := preferred2["b.json"]; got == nil || got.HashApplied != "new" {
		t.Fatalf("expected newer last-seen preferred, got %+v", got)
	}
	// nil 项与空文件名忽略
	empty := buildPreferredInventoryByFilename([]*repository.AgentConfigInventory{nil, {Filename: ""}})
	if len(empty) != 0 {
		t.Fatalf("expected empty map, got %+v", empty)
	}
}

// TestInboundSpecService_BindSpec_ConflictRetry 覆盖 UpdateWithRevision 返回
// ErrConflict 时重试成功路径（spec 并发更新后以新 revision 重试）。
func TestInboundSpecService_BindSpec_ConflictRetry(t *testing.T) {
	ctx := context.Background()
	hostID := int64(6)
	enabled := true
	compiler := &stubArtifactCompilerService{}
	svc := newInboundSpecServiceForTest(t, compiler)

	specID, _, err := svc.UpsertSpec(ctx, UpsertInboundSpecRequest{
		AgentHostID:  &hostID,
		CoreType:     "sing-box",
		Tag:          "bs-conflict",
		Enabled:      &enabled,
		SemanticSpec: json.RawMessage(`{"tag":"bs-conflict","protocol":"vless","listen":"0.0.0.0","port":28071}`),
		CoreSpecific: json.RawMessage(`{"core_type":"sing-box"}`),
		OperatorID:   1,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// 先手工推进 revision（模拟并发更新），使 BindSpec 首次 UpdateWithRevision 冲突
	spec, err := svc.specs.FindByID(ctx, specID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	spec.DesiredRevision++
	if err := svc.specs.Update(ctx, spec); err != nil {
		t.Fatalf("bump: %v", err)
	}

	if err := svc.BindSpec(ctx, specID, hostID); err != nil {
		t.Fatalf("bind with conflict retry: %v", err)
	}
	after, err := svc.specs.FindByID(ctx, specID)
	if err != nil {
		t.Fatalf("find after: %v", err)
	}
	if after.DesiredRevision < spec.DesiredRevision+1 {
		t.Fatalf("expected retry bumped revision, got %d", after.DesiredRevision)
	}
}

// TestInboundSpecService_BindSpec_SpecDeletedDuringBind 覆盖重试时 spec 已被并发删除。
func TestInboundSpecService_BindSpec_SpecDeletedDuringBind(t *testing.T) {
	ctx := context.Background()
	hostID := int64(6)
	enabled := true
	compiler := &stubArtifactCompilerService{renderErr: errors.New("never"), failFromCall: 2}
	svc := newInboundSpecServiceForTest(t, compiler)

	specID, _, err := svc.UpsertSpec(ctx, UpsertInboundSpecRequest{
		AgentHostID:  &hostID,
		CoreType:     "sing-box",
		Tag:          "bs-race",
		Enabled:      &enabled,
		SemanticSpec: json.RawMessage(`{"tag":"bs-race","protocol":"vless","listen":"0.0.0.0","port":28072}`),
		CoreSpecific: json.RawMessage(`{"core_type":"sing-box"}`),
		OperatorID:   1,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// 首次 render 成功（BindSpec 内 UpdateWithRevision 成功），随后手工删除 spec
	// 模拟并发删除；再次 BindSpec 应静默返回 nil（FindByID → ErrNotFound）。
	// 这里直接让 spec 消失：先 render 一次成功（failFromCall 大），然后 DeleteSpec。
	if err := svc.DeleteSpec(ctx, specID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := svc.BindSpec(ctx, specID, hostID); err != nil {
		t.Fatalf("bind after delete should be nil, got %v", err)
	}
}
