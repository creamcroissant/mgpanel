package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

type mockDesiredArtifactRepo struct {
	createBatchErr error
	deleteErr      error
	createdBatches [][]*repository.DesiredArtifact
	deleteCalls    []struct {
		hostID   int64
		coreType string
		revision int64
	}
	replaceErr error

}

func (m *mockDesiredArtifactRepo) PruneOldRevisions(ctx context.Context, keep int) (int64, error) {
	return 0, nil
}

func (m *mockDesiredArtifactRepo) ReplaceRevision(ctx context.Context, agentHostID int64, coreType string, desiredRevision int64, artifacts []*repository.DesiredArtifact, sourceTags ...string) (int64, error) {
	if m.replaceErr != nil {
		return 0, m.replaceErr
	}
	m.createdBatches = append(m.createdBatches, artifacts)
	return int64(len(artifacts)), nil
}

func (m *mockDesiredArtifactRepo) CreateBatch(ctx context.Context, artifacts []*repository.DesiredArtifact) error {
	if m.createBatchErr != nil {
		return m.createBatchErr
	}
	clone := make([]*repository.DesiredArtifact, 0, len(artifacts))
	for _, item := range artifacts {
		if item == nil {
			clone = append(clone, nil)
			continue
		}
		copied := *item
		if item.Content != nil {
			copied.Content = append([]byte(nil), item.Content...)
		}
		clone = append(clone, &copied)
	}
	m.createdBatches = append(m.createdBatches, clone)
	return nil
}

func (m *mockDesiredArtifactRepo) DeleteByHostCoreRevision(ctx context.Context, agentHostID int64, coreType string, desiredRevision int64, sourceTags ...string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	m.deleteCalls = append(m.deleteCalls, struct {
		hostID   int64
		coreType string
		revision int64
	}{
		hostID:   agentHostID,
		coreType: coreType,
		revision: desiredRevision,
	})
	return nil
}

func (m *mockDesiredArtifactRepo) List(ctx context.Context, filter repository.DesiredArtifactFilter) ([]*repository.DesiredArtifact, error) {
	return nil, nil
}

func (m *mockDesiredArtifactRepo) GetLatestRevision(ctx context.Context, agentHostID int64, coreType string) (int64, error) {
	return 0, nil
}

func (m *mockDesiredArtifactRepo) Count(ctx context.Context, filter repository.DesiredArtifactFilter) (int64, error) {
	return 0, nil
}

func (m *mockDesiredArtifactRepo) FindByHostCoreRevisionFilename(ctx context.Context, agentHostID int64, coreType string, desiredRevision int64, filename string) (*repository.DesiredArtifact, error) {
	return nil, repository.ErrNotFound
}

func TestArtifactCompilerService_RenderArtifacts_SingBoxDeterministic(t *testing.T) {
	specRepo := newMockInboundSpecRepo()
	artifactRepo := &mockDesiredArtifactRepo{}
	service := NewArtifactCompilerService(specRepo, artifactRepo, nil, nil, nil, nil, nil)

	err := specRepo.Create(context.Background(), &repository.InboundSpec{
		AgentHostID: intPtr(11),
		CoreType:        "sing-box",
		Tag:             "sb-vless-1",
		Enabled:         true,
		SemanticSpec:    json.RawMessage(`{"protocol":"vless","port":443,"listen":"0.0.0.0","tag":"sb-vless-1","transport":{"type":"ws","path":"/ws","headers":{"Host":"example.com"}},"tls":{"enabled":true,"server_name":"example.com"}}`),
		CoreSpecific:    json.RawMessage(`{"core_type":"sing-box","sing-box":{"sniff":true}}`),
		DesiredRevision: 1,
	})
	if err != nil {
		t.Fatalf("seed spec error: %v", err)
	}

	resultA, err := service.RenderArtifacts(context.Background(), RenderArtifactsRequest{
		AgentHostID: 11,
		CoreType:        "sing-box",
		DesiredRevision: 7,
	})
	if err != nil {
		t.Fatalf("first render error: %v", err)
	}
	if resultA.ArtifactCount != 1 {
		t.Fatalf("expected artifact_count=1, got %d", resultA.ArtifactCount)
	}
	if len(artifactRepo.createdBatches) != 1 || len(artifactRepo.createdBatches[0]) != 1 {
		t.Fatalf("expected one artifact in first batch")
	}
	first := artifactRepo.createdBatches[0][0]
	if first == nil {
		t.Fatalf("first artifact is nil")
	}
	if first.SourceTag != "sb-vless-1" {
		t.Fatalf("unexpected source tag: %s", first.SourceTag)
	}
	if first.Filename == "" {
		t.Fatalf("filename should not be empty")
	}

	resultB, err := service.RenderArtifacts(context.Background(), RenderArtifactsRequest{
		AgentHostID: 11,
		CoreType:        "sing-box",
		DesiredRevision: 8,
	})
	if err != nil {
		t.Fatalf("second render error: %v", err)
	}
	if resultB.ArtifactCount != 1 {
		t.Fatalf("expected second artifact_count=1, got %d", resultB.ArtifactCount)
	}
	if len(artifactRepo.createdBatches) != 2 || len(artifactRepo.createdBatches[1]) != 1 {
		t.Fatalf("expected one artifact in second batch")
	}
	second := artifactRepo.createdBatches[1][0]
	if second == nil {
		t.Fatalf("second artifact is nil")
	}
	if first.ContentHash != second.ContentHash {
		t.Fatalf("expected deterministic content hash, got %s vs %s", first.ContentHash, second.ContentHash)
	}
	if string(first.Content) != string(second.Content) {
		t.Fatalf("expected deterministic content payload")
	}
	if resultA.Artifacts[0].ContentHash != first.ContentHash {
		t.Fatalf("result metadata content_hash mismatch")
	}
	if resultA.Artifacts[0].Filename != first.Filename {
		t.Fatalf("result metadata filename mismatch")
	}
	// ReplaceRevision（单事务原子替换）应被调用两次（两次渲染各一次），
	// 且不再有独立的 delete 调用。
	if len(artifactRepo.createdBatches) != 2 {
		t.Fatalf("expected ReplaceRevision called twice, got %d", len(artifactRepo.createdBatches))
	}
	if len(artifactRepo.deleteCalls) != 0 {
		t.Fatalf("expected no standalone deletes after tx replace, got %d", len(artifactRepo.deleteCalls))
	}
}

func TestArtifactCompilerService_RenderArtifacts_XrayCoreSpecificFilenameAndWarnings(t *testing.T) {
	specRepo := newMockInboundSpecRepo()
	artifactRepo := &mockDesiredArtifactRepo{}
	service := NewArtifactCompilerService(specRepo, artifactRepo, nil, nil, nil, nil, nil)

	err := specRepo.Create(context.Background(), &repository.InboundSpec{
		AgentHostID: intPtr(22),
		CoreType:    "xray",
		Tag:         "xr-vmess",
		Enabled:     true,
		SemanticSpec: json.RawMessage(`{
			"tag":"xr-vmess",
			"protocol":"vmess",
			"listen":"127.0.0.1",
			"port":10086,
			"unknown_semantic":"keep-warning"
		}`),
		CoreSpecific:    json.RawMessage(`{"core_type":"xray","xray":{"filename":"managed-xray.json","sniffing":{"enabled":true}}}`),
		DesiredRevision: 1,
	})
	if err != nil {
		t.Fatalf("seed spec error: %v", err)
	}

	result, err := service.RenderArtifacts(context.Background(), RenderArtifactsRequest{
		AgentHostID: 22,
		CoreType:        "xray",
		DesiredRevision: 3,
	})
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	if result.ArtifactCount != 1 {
		t.Fatalf("expected artifact_count=1, got %d", result.ArtifactCount)
	}
	if len(result.Warnings) == 0 {
		t.Fatalf("expected unsupported semantic field warning")
	}
	foundSemanticWarning := false
	for _, warning := range result.Warnings {
		if warning.Field == "semantic_spec.unknown_semantic" {
			foundSemanticWarning = true
			break
		}
	}
	if !foundSemanticWarning {
		t.Fatalf("expected semantic_spec.unknown_semantic warning")
	}
	if len(artifactRepo.createdBatches) != 1 || len(artifactRepo.createdBatches[0]) != 1 {
		t.Fatalf("expected one persisted artifact")
	}
	artifact := artifactRepo.createdBatches[0][0]
	if artifact.Filename != "managed-xray.json" {
		t.Fatalf("expected custom filename managed-xray.json, got %s", artifact.Filename)
	}
}

func TestArtifactCompilerService_RenderArtifacts_CoreSpecificReservedFieldFails(t *testing.T) {
	specRepo := newMockInboundSpecRepo()
	artifactRepo := &mockDesiredArtifactRepo{}
	service := NewArtifactCompilerService(specRepo, artifactRepo, nil, nil, nil, nil, nil)

	err := specRepo.Create(context.Background(), &repository.InboundSpec{
		AgentHostID: intPtr(31),
		CoreType:    "xray",
		Tag:         "bad-override",
		Enabled:     true,
		SemanticSpec: json.RawMessage(`{
			"tag":"bad-override",
			"protocol":"vless",
			"listen":"0.0.0.0",
			"port":443
		}`),
		CoreSpecific:    json.RawMessage(`{"core_type":"xray","xray":{"protocol":"trojan"}}`),
		DesiredRevision: 1,
	})
	if err != nil {
		t.Fatalf("seed spec error: %v", err)
	}

	_, err = service.RenderArtifacts(context.Background(), RenderArtifactsRequest{
		AgentHostID: 31,
		CoreType:        "xray",
		DesiredRevision: 5,
	})
	if err == nil {
		t.Fatalf("expected unsupported field error")
	}
	if !errors.Is(err, ErrArtifactCompileUnsupportedField) {
		t.Fatalf("expected ErrArtifactCompileUnsupportedField, got %v", err)
	}
	var fieldErr *ArtifactUnsupportedFieldError
	if !errors.As(err, &fieldErr) {
		t.Fatalf("expected ArtifactUnsupportedFieldError, got %T", err)
	}
	if fieldErr.Field != "core_specific.xray.protocol" {
		t.Fatalf("unexpected field: %s", fieldErr.Field)
	}
	if len(artifactRepo.createdBatches) != 0 {
		t.Fatalf("should not persist artifacts when render fails")
	}
}

func TestArtifactCompilerService_RenderArtifacts_InvalidRequest(t *testing.T) {
	service := NewArtifactCompilerService(newMockInboundSpecRepo(), &mockDesiredArtifactRepo{}, nil, nil, nil, nil, nil)

	_, err := service.RenderArtifacts(context.Background(), RenderArtifactsRequest{
		AgentHostID: 0,
		CoreType:        "",
		DesiredRevision: 0,
	})
	if err == nil {
		t.Fatalf("expected invalid request error")
	}
	if !errors.Is(err, ErrArtifactCompileInvalidRequest) {
		t.Fatalf("expected ErrArtifactCompileInvalidRequest, got %v", err)
	}
}

func TestArtifactCompilerService_RenderArtifacts_RepositoryError(t *testing.T) {
	specRepo := newMockInboundSpecRepo()
	targetErr := errors.New("repo list failed")
	specRepo.listErr = targetErr
	service := NewArtifactCompilerService(specRepo, &mockDesiredArtifactRepo{}, nil, nil, nil, nil, nil)

	_, err := service.RenderArtifacts(context.Background(), RenderArtifactsRequest{
		AgentHostID: 1,
		CoreType:        "sing-box",
		DesiredRevision: 1,
	})
	if !errors.Is(err, targetErr) {
		t.Fatalf("expected propagated repository error, got %v", err)
	}
}


func intPtr(v int64) *int64 { return &v }

type mockSpecHostBindingRepo struct{}

func (m *mockSpecHostBindingRepo) Bind(ctx context.Context, specID, agentHostID int64) error { return nil }
func (m *mockSpecHostBindingRepo) Unbind(ctx context.Context, specID, agentHostID int64) error { return nil }
func (m *mockSpecHostBindingRepo) UnbindAll(ctx context.Context, specID int64) error { return nil }
func (m *mockSpecHostBindingRepo) ListBySpec(ctx context.Context, specID int64) ([]int64, error) { return nil, nil }
func (m *mockSpecHostBindingRepo) ListByHost(ctx context.Context, agentHostID int64) ([]int64, error) { return nil, nil }

// ---------------------------------------------------------------------------
// relay path renderer tests
// ---------------------------------------------------------------------------

type fakeRelayPathRepo struct {
	paths []*repository.RelayPath
	err   error
}

func (f *fakeRelayPathRepo) Create(ctx context.Context, p *repository.RelayPath) (int64, error) {
	return 0, nil
}
func (f *fakeRelayPathRepo) Update(ctx context.Context, p *repository.RelayPath) error   { return nil }
func (f *fakeRelayPathRepo) Delete(ctx context.Context, id int64) error                  { return nil }
func (f *fakeRelayPathRepo) GetByID(ctx context.Context, id int64) (*repository.RelayPath, error) {
	for _, p := range f.paths {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, repository.ErrNotFound
}
func (f *fakeRelayPathRepo) List(ctx context.Context, coreType string) ([]*repository.RelayPath, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []*repository.RelayPath
	for _, p := range f.paths {
		if p.CoreType == coreType || coreType == "" {
			out = append(out, p)
		}
	}
	return out, nil
}

func newMeshPeersForRelay(ids ...int64) *fakeMeshPeerRepo {
	f := &fakeMeshPeerRepo{peers: map[int64]*repository.AgentMeshPeer{}}
	for _, id := range ids {
		ip := fmt.Sprintf("10.144.0.%d", id)
		f.peers[id] = &repository.AgentMeshPeer{AgentHostID: id, WGIP: ip, NetworkID: "default"}
	}
	return f
}

// TestArtifactCompilerService_RenderArtifacts_RelayPathMarkBinding 表驱动：
// spec 绑定链路后，入口 agent 产出 mark 出站(relay-{id}-out.json)+路由规则(mesh-*-relay-{id}.json)，
// 双核断言；不再产出逐跳 socks hop 文件。
func TestArtifactCompilerService_RenderArtifacts_RelayPathMarkBinding(t *testing.T) {
	tests := []struct {
		name      string
		core      string
		pathID    int64
		path      *repository.RelayPath
		renderFor int64 // 入口 agent（nodes[0] 宿主）
		wantMark  string
	}{
		{
			name: "2-hop sing-box: entry renders rule+mark outbound",
			core: "sing-box", pathID: 5, renderFor: 1, wantMark: "40005",
			path: &repository.RelayPath{
				ID: 5, Name: "two-hop", CoreType: "sing-box", Enabled: true,
				Nodes: []repository.RelayPathNode{
					{Sequence: 0, AgentHostID: 1},
					{Sequence: 1, AgentHostID: 2},
				},
			},
		},
		{
			name: "3-hop xray: unsorted nodes normalized",
			core: "xray", pathID: 7, renderFor: 1, wantMark: "40007",
			path: &repository.RelayPath{
				ID: 7, Name: "three-hop", CoreType: "xray", Enabled: true,
				Nodes: []repository.RelayPathNode{
					{Sequence: 2, AgentHostID: 3},
					{Sequence: 0, AgentHostID: 1}, // 乱序入库，渲染器必须按 sequence 排序
					{Sequence: 1, AgentHostID: 2},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mesh := newMeshPeersForRelay(1, 2, 3)
			relayRepo := &fakeRelayPathRepo{paths: []*repository.RelayPath{tc.path}}
			specRepo := newMockInboundSpecRepo()
			artifactRepo := &mockDesiredArtifactRepo{}
			service := NewArtifactCompilerService(specRepo, artifactRepo, nil, mesh, nil, nil, relayRepo)

			// 种子绑定链路的入站 spec（宿主 = 入口 agent；core 跟随用例）
			pid := tc.pathID
			_ = specRepo.Create(context.Background(), &repository.InboundSpec{
				AgentHostID: intPtr(tc.renderFor), CoreType: tc.core, Tag: fmt.Sprintf("seed-%d", tc.renderFor),
				Enabled: true, SemanticSpec: json.RawMessage(fmt.Sprintf(`{"protocol":"vless","port":443,"listen":"0.0.0.0","tag":"seed-%d"}`, tc.renderFor)),
				CoreSpecific: json.RawMessage(fmt.Sprintf(`{"core_type":"%s"}`, tc.core)), DesiredRevision: 1,
				RelayPathID: &pid,
			})

			if _, err := service.RenderArtifacts(context.Background(), RenderArtifactsRequest{
				AgentHostID: tc.renderFor, CoreType: tc.core, DesiredRevision: 1,
			}); err != nil {
				t.Fatalf("render error: %v", err)
			}
			if len(artifactRepo.createdBatches) != 1 {
				t.Fatalf("expected one ReplaceRevision batch")
			}

			var ruleContent, outContent string
			for _, a := range artifactRepo.createdBatches[0] {
				if a == nil || len(a.Filename) < 6 || a.Filename[:6] != "relay-" {
					continue
				}
				if strings.Contains(a.Filename, "-hop-") {
					t.Fatalf("legacy hop file %s must not be produced anymore", a.Filename)
				}
				if a.Filename == fmt.Sprintf("relay-%d-out.json", tc.pathID) {
					outContent = string(a.Content)
				}
			}
			ruleFile := findArtifactByFilename(artifactRepo.createdBatches[0],
				fmt.Sprintf("mesh-seed-%d-relay-%d.json", tc.renderFor, tc.pathID))
			if ruleFile == nil {
				t.Fatalf("missing relay routing rule artifact")
			}
			ruleContent = string(ruleFile.Content)
			if !strings.Contains(ruleContent, fmt.Sprintf("relay-mk-%d", tc.pathID)) {
				t.Fatalf("rule should target relay-mk tag, content=%s", ruleContent)
			}

			switch tc.core {
			case "sing-box":
				for _, want := range []string{`"type":"direct"`, `"routing_mark":` + tc.wantMark} {
					if !strings.Contains(outContent, want) {
						t.Fatalf("sing-box mark outbound missing %s: %s", want, outContent)
					}
				}
			case "xray":
				for _, want := range []string{`"protocol":"freedom"`, `"sockopt"`, `"mark":` + tc.wantMark} {
					if !strings.Contains(outContent, want) {
						t.Fatalf("xray mark outbound missing %s: %s", want, outContent)
					}
				}
			}
		})
	}
}

// TestArtifactCompilerService_RenderArtifacts_RelayPathDisabledAndGuards
// disabled path / 单节点 path / 下一跳无 mesh IP 三种情况均零产物。
func TestArtifactCompilerService_RenderArtifacts_RelayPathDisabledAndGuards(t *testing.T) {
	cases := []struct {
		name string
		path *repository.RelayPath
	}{
		{"disabled", &repository.RelayPath{ID: 9, Name: "off", CoreType: "sing-box", Enabled: false,
			Nodes: []repository.RelayPathNode{{Sequence: 0, AgentHostID: 1}, {Sequence: 1, AgentHostID: 2}}}},
		{"single-node", &repository.RelayPath{ID: 10, Name: "solo", CoreType: "sing-box", Enabled: true,
			Nodes: []repository.RelayPathNode{{Sequence: 0, AgentHostID: 1}}}},
		{"next-hop-no-mesh-ip", &repository.RelayPath{ID: 11, Name: "ghost", CoreType: "sing-box", Enabled: true,
			Nodes: []repository.RelayPathNode{{Sequence: 0, AgentHostID: 1}, {Sequence: 1, AgentHostID: 99}}}},
		{"wrong-core", &repository.RelayPath{ID: 12, Name: "xray-only", CoreType: "xray", Enabled: true,
			Nodes: []repository.RelayPathNode{{Sequence: 0, AgentHostID: 1}, {Sequence: 1, AgentHostID: 2}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mesh := newMeshPeersForRelay(1, 2)
			relayRepo := &fakeRelayPathRepo{paths: []*repository.RelayPath{tc.path}}
			specRepo := newMockInboundSpecRepo()
			service := NewArtifactCompilerService(specRepo, &mockDesiredArtifactRepo{}, nil, mesh, nil, nil, relayRepo)
			_ = specRepo.Create(context.Background(), &repository.InboundSpec{
				AgentHostID: intPtr(1), CoreType: "sing-box", Tag: "seed-1",
				Enabled: true, SemanticSpec: json.RawMessage(`{"protocol":"vless","port":443,"listen":"0.0.0.0","tag":"seed-1"}`),
				CoreSpecific: json.RawMessage(`{"core_type":"sing-box"}`), DesiredRevision: 1,
			})
			res, err := service.RenderArtifacts(context.Background(), RenderArtifactsRequest{
				AgentHostID: 1, CoreType: "sing-box", DesiredRevision: 3,
			})
			if err != nil {
				t.Fatalf("render error: %v", err)
			}
			for _, m := range res.Artifacts {
				if len(m.Filename) >= 6 && m.Filename[:6] == "relay-" {
					t.Fatalf("unexpected relay artifact %s for case %s", m.Filename, tc.name)
				}
			}
		})
	}
}
func TestArtifactCompilerService_RenderArtifacts_EmptyRealityKeyOmitted(t *testing.T) {
	specRepo := newMockInboundSpecRepo()
	artifactRepo := &mockDesiredArtifactRepo{}
	svc := NewArtifactCompilerService(specRepo, artifactRepo, nil, nil, nil, nil, nil)

	sbSpec := `{
		"protocol":"vless","port":443,"listen":"0.0.0.0","tag":"sb-vless-reality",
		"tls":{"enabled":true,"server_name":"example.com",
			"reality":{"enabled":true,"handshake_server":"example.com","handshake_port":443,"server_names":["example.com"]}}
	}`
	if err := specRepo.Create(context.Background(), &repository.InboundSpec{
		AgentHostID: intPtr(11), CoreType: "sing-box", Tag: "sb-vless-reality",
		Enabled: true, DesiredRevision: 1,
		SemanticSpec: json.RawMessage(sbSpec),
		CoreSpecific: json.RawMessage(`{"core_type":"sing-box","sing-box":{}}`),
	}); err != nil {
		t.Fatalf("seed singbox spec: %v", err)
	}
	res, err := svc.RenderArtifacts(context.Background(), RenderArtifactsRequest{
		AgentHostID: 11, CoreType: "sing-box", DesiredRevision: 2,
	})
	if err != nil {
		t.Fatalf("singbox render with empty reality key: %v", err)
	}
	if res.ArtifactCount != 1 {
		t.Fatalf("singbox artifact_count=1 got %d", res.ArtifactCount)
	}
	sbReality := firstRealityFromSS(t, artifactRepo)
	for _, k := range []string{"private_key", "public_key", "short_id"} {
		if _, ok := sbReality[k]; ok {
			t.Fatalf("singbox reality MUST NOT contain %s=%v (empty key contract violated)", k, sbReality[k])
		}
	}
	if sbReality["handshake"] == nil {
		t.Fatalf("singbox reality handshake should still render")
	}

	// xray
	specRepo2 := newMockInboundSpecRepo()
	artifactRepo2 := &mockDesiredArtifactRepo{}
	svc2 := NewArtifactCompilerService(specRepo2, artifactRepo2, nil, nil, nil, nil, nil)
	xrSpec := `{
		"protocol":"vless","port":443,"listen":"0.0.0.0","tag":"xr-vless-reality",
		"tls":{"server_name":"example.com",
			"reality":{"enabled":true,"handshake_server":"example.com","handshake_port":443,"server_names":["example.com"]}}
	}`
	if err := specRepo2.Create(context.Background(), &repository.InboundSpec{
		AgentHostID: intPtr(12), CoreType: "xray", Tag: "xr-vless-reality",
		Enabled: true, DesiredRevision: 1,
		SemanticSpec: json.RawMessage(xrSpec),
		CoreSpecific: json.RawMessage(`{"core_type":"xray","xray":{}}`),
	}); err != nil {
		t.Fatalf("seed xray spec: %v", err)
	}
	res2, err := svc2.RenderArtifacts(context.Background(), RenderArtifactsRequest{
		AgentHostID: 12, CoreType: "xray", DesiredRevision: 2,
	})
	if err != nil {
		t.Fatalf("xray render with empty reality key: %v", err)
	}
	if res2.ArtifactCount != 1 {
		t.Fatalf("xray artifact_count=1 got %d", res2.ArtifactCount)
	}
	xrReality := firstRealityFromXray(t, artifactRepo2)
	for _, k := range []string{"privateKey", "publicKey", "shortIds"} {
		if _, ok := xrReality[k]; ok {
			t.Fatalf("xray realitySettings MUST NOT contain %s (empty key contract violated)", k)
		}
	}
	t.Logf("empty-key reality renders (sing-box + xray), key fields omitted")
}


func firstRealityFromSS(t *testing.T, repo *mockDesiredArtifactRepo) map[string]any {
	t.Helper()
	art := repo.createdBatches[0][0]
	var doc struct {
		Inbounds []json.RawMessage `json:"inbounds"`
	}
	if err := json.Unmarshal(art.Content, &doc); err != nil {
		t.Fatalf("unmarshal singbox inbound: %v", err)
	}
	if len(doc.Inbounds) != 1 {
		t.Fatalf("expected 1 inbound, got %d", len(doc.Inbounds))
	}
	var inb map[string]any
	if err := json.Unmarshal(doc.Inbounds[0], &inb); err != nil {
		t.Fatalf("unmarshal inbound body: %v", err)
	}
	tls, _ := inb["tls"].(map[string]any)
	reality, _ := tls["reality"].(map[string]any)
	if reality == nil {
		t.Fatalf("singbox reality block missing")
	}
	return reality
}

func TestArtifactCompilerService_RenderArtifacts_ExistingRealityKeyPreserved(t *testing.T) {
	// 存量兼容：含 private_key/public_key/short_ids 的 spec 仍按非空照旧透传（不可被误删）
	specRepo := newMockInboundSpecRepo()
	artifactRepo := &mockDesiredArtifactRepo{}
	svc := NewArtifactCompilerService(specRepo, artifactRepo, nil, nil, nil, nil, nil)

	sbSpec := `{
		"protocol":"vless","port":443,"listen":"0.0.0.0","tag":"sb-vless-reality",
		"tls":{"enabled":true,"server_name":"example.com",
			"reality":{"enabled":true,"private_key":"abcd_private","public_key":"abcd_public","short_ids":["1122"],"handshake_server":"example.com","handshake_port":443,"server_names":["example.com"]}}
	}`
	if err := specRepo.Create(context.Background(), &repository.InboundSpec{
		AgentHostID: intPtr(21), CoreType: "sing-box", Tag: "sb-vless-reality",
		Enabled: true, DesiredRevision: 1,
		SemanticSpec: json.RawMessage(sbSpec),
		CoreSpecific: json.RawMessage(`{"core_type":"sing-box","sing-box":{}}`),
	}); err != nil {
		t.Fatalf("seed singbox spec: %v", err)
	}
	if _, err := svc.RenderArtifacts(context.Background(), RenderArtifactsRequest{
		AgentHostID: 21, CoreType: "sing-box", DesiredRevision: 2,
	}); err != nil {
		t.Fatalf("singbox render with existing reality key: %v", err)
	}
	r := firstRealityFromSS(t, artifactRepo)
	if r["private_key"] != "abcd_private" || r["public_key"] != "abcd_public" {
		t.Fatalf("singbox existing reality keys not preserved: %#v", r)
	}
	short, _ := r["short_id"].([]any)
	if len(short) != 1 || short[0] != "1122" {
		t.Fatalf("singbox existing short_id not preserved: %#v", r["short_id"])
	}

	// xray 非空照旧透传
	specRepo2 := newMockInboundSpecRepo()
	artifactRepo2 := &mockDesiredArtifactRepo{}
	svc2 := NewArtifactCompilerService(specRepo2, artifactRepo2, nil, nil, nil, nil, nil)
	xrSpec := `{
		"protocol":"vless","port":443,"listen":"0.0.0.0","tag":"xr-vless-reality",
		"tls":{"server_name":"example.com",
			"reality":{"enabled":true,"private_key":"xyz_private","public_key":"xyz_public","short_ids":["3344"],"handshake_server":"example.com","handshake_port":443,"server_names":["example.com"]}}
	}`
	if err := specRepo2.Create(context.Background(), &repository.InboundSpec{
		AgentHostID: intPtr(22), CoreType: "xray", Tag: "xr-vless-reality",
		Enabled: true, DesiredRevision: 1,
		SemanticSpec: json.RawMessage(xrSpec),
		CoreSpecific: json.RawMessage(`{"core_type":"xray","xray":{}}`),
	}); err != nil {
		t.Fatalf("seed xray spec: %v", err)
	}
	if _, err := svc2.RenderArtifacts(context.Background(), RenderArtifactsRequest{
		AgentHostID: 22, CoreType: "xray", DesiredRevision: 2,
	}); err != nil {
		t.Fatalf("xray render with existing reality key: %v", err)
	}
	xr := firstRealityFromXray(t, artifactRepo2)
	if xr["privateKey"] != "xyz_private" || xr["publicKey"] != "xyz_public" {
		t.Fatalf("xray existing reality keys not preserved: %#v", xr)
	}
}

func firstRealityFromXray(t *testing.T, repo *mockDesiredArtifactRepo) map[string]any {
	t.Helper()
	art := repo.createdBatches[0][0]
	var doc struct {
		Inbounds []json.RawMessage `json:"inbounds"`
	}
	if err := json.Unmarshal(art.Content, &doc); err != nil {
		t.Fatalf("unmarshal xray inbound: %v", err)
	}
	var inb map[string]any
	if err := json.Unmarshal(doc.Inbounds[0], &inb); err != nil {
		t.Fatalf("unmarshal inbound body: %v", err)
	}
	stream, _ := inb["streamSettings"].(map[string]any)
	rs, _ := stream["realitySettings"].(map[string]any)
	if rs == nil {
		t.Fatalf("xray realitySettings missing")
	}
	return rs
}

func TestArtifactCompilerService_RenderArtifacts_V2RayAPIFragmentEnabled(t *testing.T) {
	specRepo := newMockInboundSpecRepo()
	artifactRepo := &mockDesiredArtifactRepo{}
	svc := NewArtifactCompilerService(specRepo, artifactRepo, nil, nil, nil, nil, nil)

	sbSpec := `{
		"protocol":"vless","port":443,"listen":"0.0.0.0","tag":"sb-vless-v2rayapi"
	}`
	if err := specRepo.Create(context.Background(), &repository.InboundSpec{
		AgentHostID: intPtr(11), CoreType: "sing-box", Tag: "sb-vless-v2rayapi",
		Enabled: true, DesiredRevision: 1,
		SemanticSpec: json.RawMessage(sbSpec),
		CoreSpecific: json.RawMessage(`{"core_type":"sing-box","sing-box":{"v2ray_api":{"enabled":true,"listen":"127.0.0.1:20001"}}}`),
	}); err != nil {
		t.Fatalf("seed spec: %v", err)
	}
	res, err := svc.RenderArtifacts(context.Background(), RenderArtifactsRequest{
		AgentHostID: 11, CoreType: "sing-box", DesiredRevision: 2,
	})
	if err != nil {
		t.Fatalf("render with v2ray_api enabled: %v", err)
	}
	// 期望: 1 inbound artifact + 1 experimental-v2ray-api.json fragment = 2
	_ = res
	if batch := artifactRepo.createdBatches[0]; len(batch) != 2 {
		t.Fatalf("expected 2 artifacts (inbound + v2ray fragment), got %d", len(batch))
	}
	// 找 fragment
	var frag *repository.DesiredArtifact
	for _, a := range artifactRepo.createdBatches[0] {
		if a.Filename == "experimental-v2ray-api.json" {
			frag = a
		}
	}
	if frag == nil {
		t.Fatalf("experimental-v2ray-api.json fragment missing")
	}
	var doc struct {
		Experimental struct {
			V2RayAPI struct {
				Listen string `json:"listen"`
				Stats  struct {
					Enabled bool `json:"enabled"`
				} `json:"stats"`
			} `json:"v2ray_api"`
		} `json:"experimental"`
	}
	if err := json.Unmarshal(frag.Content, &doc); err != nil {
		t.Fatalf("unmarshal fragment: %v", err)
	}
	if doc.Experimental.V2RayAPI.Listen != "127.0.0.1:20001" {
		t.Fatalf("fragment listen = %q, want 127.0.0.1:20001", doc.Experimental.V2RayAPI.Listen)
	}
	if !doc.Experimental.V2RayAPI.Stats.Enabled {
		t.Fatalf("fragment stats.enabled should be true")
	}
	// 契约：inbound artifact 不得携带 v2ray_api 字段（它属于顶层 experimental，
	// 由独立 fragment 承载；合并进 inbound 会被 sing-box 解析为 unknown field）。
	for _, a := range artifactRepo.createdBatches[0] {
		if a.Filename == "experimental-v2ray-api.json" {
			continue
		}
		var inboundDoc struct {
			Inbounds []map[string]any `json:"inbounds"`
		}
		if err := json.Unmarshal(a.Content, &inboundDoc); err != nil {
			t.Fatalf("unmarshal inbound artifact %s: %v", a.Filename, err)
		}
		for i, inb := range inboundDoc.Inbounds {
			if _, has := inb["v2ray_api"]; has {
				t.Fatalf("inbound artifact %s inbounds[%d] must NOT carry v2ray_api", a.Filename, i)
			}
		}
	}
}

func TestArtifactCompilerService_RenderArtifacts_V2RayAPIFragmentDisabledOrMissing(t *testing.T) {
	// enabled=false → 不渲染 fragment
	specRepo := newMockInboundSpecRepo()
	artifactRepo := &mockDesiredArtifactRepo{}
	svc := NewArtifactCompilerService(specRepo, artifactRepo, nil, nil, nil, nil, nil)
	if err := specRepo.Create(context.Background(), &repository.InboundSpec{
		AgentHostID: intPtr(11), CoreType: "sing-box", Tag: "sb-vless-disabled",
		Enabled: true, DesiredRevision: 1,
		SemanticSpec: json.RawMessage(`{"protocol":"vless","port":443,"listen":"0.0.0.0","tag":"sb-vless-disabled"}`),
		CoreSpecific: json.RawMessage(`{"core_type":"sing-box","sing-box":{"v2ray_api":{"enabled":false}}}`),
	}); err != nil {
		t.Fatalf("seed disabled spec: %v", err)
	}
	if _, err := svc.RenderArtifacts(context.Background(), RenderArtifactsRequest{
		AgentHostID: 11, CoreType: "sing-box", DesiredRevision: 2,
	}); err != nil {
		t.Fatalf("render disabled: %v", err)
	}
	if batch := artifactRepo.createdBatches[0]; len(batch) != 1 {
		t.Fatalf("disabled: expected 1 artifact, got %d", len(batch))
	}

	// 缺失 → 不渲染
	specRepo2 := newMockInboundSpecRepo()
	artifactRepo2 := &mockDesiredArtifactRepo{}
	svc2 := NewArtifactCompilerService(specRepo2, artifactRepo2, nil, nil, nil, nil, nil)
	if err := specRepo2.Create(context.Background(), &repository.InboundSpec{
		AgentHostID: intPtr(12), CoreType: "sing-box", Tag: "sb-vless-nov2ray",
		Enabled: true, DesiredRevision: 1,
		SemanticSpec: json.RawMessage(`{"protocol":"vless","port":443,"listen":"0.0.0.0","tag":"sb-vless-nov2ray"}`),
		CoreSpecific: json.RawMessage(`{"core_type":"sing-box","sing-box":{}}`),
	}); err != nil {
		t.Fatalf("seed no-v2ray spec: %v", err)
	}
	if _, err := svc2.RenderArtifacts(context.Background(), RenderArtifactsRequest{
		AgentHostID: 12, CoreType: "sing-box", DesiredRevision: 2,
	}); err != nil {
		t.Fatalf("render no-v2ray: %v", err)
	}
	if batch := artifactRepo2.createdBatches[0]; len(batch) != 1 {
		t.Fatalf("missing: expected 1 artifact, got %d", len(batch))
	}
}

func TestArtifactCompilerService_RenderArtifacts_V2RayAPIFragmentDefaultListenAndDedup(t *testing.T) {
	// 未指定 listen → 默认 127.0.0.1:19194
	out, err := buildV2RayAPIFragmentArtifacts(context.Background(), RenderArtifactsRequest{
		AgentHostID: 11, CoreType: "sing-box", DesiredRevision: 2,
	}, "sing-box", []*repository.InboundSpec{
		{Tag: "a", CoreSpecific: json.RawMessage(`{"core_type":"sing-box","sing-box":{"v2ray_api":{"enabled":true}}}`)},
	})
	if err != nil {
		t.Fatalf("build fragment (default listen): %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("default listen: expected 1 fragment, got %d", len(out))
	}
	if out[0].Filename != "experimental-v2ray-api.json" {
		t.Fatalf("fragment filename = %q", out[0].Filename)
	}
	var doc struct {
		Experimental struct {
			V2RayAPI struct {
				Listen string `json:"listen"`
			} `json:"v2ray_api"`
		} `json:"experimental"`
	}
	if err := json.Unmarshal(out[0].Content, &doc); err != nil {
		t.Fatalf("unmarshal default fragment: %v", err)
	}
	if doc.Experimental.V2RayAPI.Listen != "127.0.0.1:19194" {
		t.Fatalf("default listen = %q, want 127.0.0.1:19194", doc.Experimental.V2RayAPI.Listen)
	}

	// 多 spec 啟用 → 只生成一次（去重）
	out2, err := buildV2RayAPIFragmentArtifacts(context.Background(), RenderArtifactsRequest{
		AgentHostID: 11, CoreType: "sing-box", DesiredRevision: 2,
	}, "sing-box", []*repository.InboundSpec{
		{Tag: "a", CoreSpecific: json.RawMessage(`{"core_type":"sing-box","sing-box":{"v2ray_api":{"enabled":true}}}`)},
		{Tag: "b", CoreSpecific: json.RawMessage(`{"core_type":"sing-box","sing-box":{"v2ray_api":{"enabled":true,"listen":"127.0.0.1:20002"}}}`)},
	})
	if err != nil {
		t.Fatalf("build fragment (multi): %v", err)
	}
	if len(out2) != 1 {
		t.Fatalf("multi-spec: expected 1 fragment (deduped), got %d", len(out2))
	}

	// xray core type → 不渲染
	out3, _ := buildV2RayAPIFragmentArtifacts(context.Background(), RenderArtifactsRequest{
		AgentHostID: 11, CoreType: "xray", DesiredRevision: 2,
	}, "xray", []*repository.InboundSpec{
		{Tag: "x", CoreSpecific: json.RawMessage(`{"core_type":"xray","xray":{"v2ray_api":{"enabled":true}}}`)},
	})
	if len(out3) != 0 {
		t.Fatalf("xray: expected no fragment, got %d", len(out3))
	}
}

// TestArtifactCompilerService_RenderArtifacts_NoSpecMeshMember verifies the
// queue-mode-era fix: a host with NO inbound spec bindings but present in the
// mesh (has WG IP) must still get its mesh baseline artifacts rendered
// (mesh-direct.json / mesh-inbound.json). Previously an empty enabledSpecs
// list aborted rendering entirely, leaving such hosts with zero artifacts and
// apply-runs failing with 422 no-artifacts.
func TestArtifactCompilerService_RenderArtifacts_NoSpecMeshMember(t *testing.T) {
	mesh := newMeshPeersForRelay(4, 6)
	specRepo := newMockInboundSpecRepo() // 无任何 spec 绑定
	artifactRepo := &mockDesiredArtifactRepo{}
	service := NewArtifactCompilerService(specRepo, artifactRepo, nil, mesh, nil, nil, nil)

	result, err := service.RenderArtifacts(context.Background(), RenderArtifactsRequest{
		AgentHostID: 4, CoreType: "sing-box", DesiredRevision: 21,
	})
	if err != nil {
		t.Fatalf("render for mesh member without specs: %v", err)
	}
	if result == nil || result.ArtifactCount == 0 {
		t.Fatalf("expected mesh baseline artifacts for spec-less mesh member, got count=%d", result.ArtifactCount)
	}

	if len(artifactRepo.createdBatches) == 0 || len(artifactRepo.createdBatches[0]) == 0 {
		t.Fatalf("expected artifacts to be replaced, got %d batches", len(artifactRepo.createdBatches))
	}
	got := map[string]bool{}
	for _, a := range artifactRepo.createdBatches[0] {
		got[a.Filename] = true
	}
	if !got["mesh-direct.json"] {
		t.Fatalf("expected mesh-direct.json, got %v", got)
	}
	if !got["mesh-inbound.json"] {
		t.Fatalf("expected mesh-inbound.json, got %v", got)
	}
}

// TestArtifactCompilerService_RenderArtifacts_NoSpecNonMeshMemberStillFails
// verifies a host that is neither spec-bound nor a mesh member still renders
// zero artifacts (upper layer rejects with 422 no-artifacts).
func TestArtifactCompilerService_RenderArtifacts_NoSpecNonMeshMemberStillFails(t *testing.T) {
	specRepo := newMockInboundSpecRepo() // 无 spec
	artifactRepo := &mockDesiredArtifactRepo{}
	service := NewArtifactCompilerService(specRepo, artifactRepo, nil, nil, nil, nil, nil)

	result, err := service.RenderArtifacts(context.Background(), RenderArtifactsRequest{
		AgentHostID: 99, CoreType: "sing-box", DesiredRevision: 1,
	})
	if err != nil {
		t.Fatalf("expected no error for spec-less non-mesh render, got %v", err)
	}
	if result != nil && result.ArtifactCount != 0 {
		t.Fatalf("expected zero artifacts for non-mesh non-spec host, got %d", result.ArtifactCount)
	}
}
