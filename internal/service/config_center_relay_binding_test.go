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

// mockRelayPathRepo 中继链路仓库 mock。
type mockRelayPathRepo struct {
	items     map[int64]*repository.RelayPath
	getErr    error
	notFound  bool
	listErr   error
	nextID    int64
	deletedIDs []int64
}

func newMockRelayPathRepo() *mockRelayPathRepo {
	return &mockRelayPathRepo{items: map[int64]*repository.RelayPath{}, nextID: 1}
}

func (m *mockRelayPathRepo) Create(ctx context.Context, p *repository.RelayPath) (int64, error) {
	if m.getErr != nil && m.listErr == nil {
		// 复用 getErr 作为通用写入错误开关的场景不存在，保持独立
	}
	id := m.nextID
	m.nextID++
	p.ID = id
	clone := *p
	m.items[id] = &clone
	return id, nil
}

func (m *mockRelayPathRepo) Update(ctx context.Context, p *repository.RelayPath) error {
	if _, ok := m.items[p.ID]; !ok {
		return repository.ErrNotFound
	}
	clone := *p
	m.items[p.ID] = &clone
	return nil
}

func (m *mockRelayPathRepo) Delete(ctx context.Context, id int64) error {
	m.deletedIDs = append(m.deletedIDs, id)
	delete(m.items, id)
	return nil
}

func (m *mockRelayPathRepo) GetByID(ctx context.Context, id int64) (*repository.RelayPath, error) {
	if m.notFound {
		return nil, repository.ErrNotFound
	}
	if m.getErr != nil {
		return nil, m.getErr
	}
	p, ok := m.items[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	clone := *p
	return &clone, nil
}

func (m *mockRelayPathRepo) List(ctx context.Context, coreType string) ([]*repository.RelayPath, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	result := make([]*repository.RelayPath, 0)
	for _, p := range m.items {
		if coreType != "" && p.CoreType != coreType {
			continue
		}
		clone := *p
		result = append(result, &clone)
	}
	return result, nil
}

// mockAgentMeshPeerRepo mesh peer 仓库 mock。
type mockAgentMeshPeerRepo struct {
	peers   []*repository.AgentMeshPeer
	upserts int
}

func (m *mockAgentMeshPeerRepo) Upsert(ctx context.Context, peer *repository.AgentMeshPeer) error {
	m.upserts++
	return nil
}

func (m *mockAgentMeshPeerRepo) FindByAgentHostID(ctx context.Context, agentHostID int64) (*repository.AgentMeshPeer, error) {
	for _, p := range m.peers {
		if p.AgentHostID == agentHostID {
			clone := *p
			return &clone, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (m *mockAgentMeshPeerRepo) ListByNetworkID(ctx context.Context, networkID string) ([]*repository.AgentMeshPeer, error) {
	result := make([]*repository.AgentMeshPeer, 0)
	for _, p := range m.peers {
		if networkID == "" || p.NetworkID == networkID || p.NetworkID == "default" {
			clone := *p
			result = append(result, &clone)
		}
	}
	return result, nil
}

func (m *mockAgentMeshPeerRepo) Delete(ctx context.Context, agentHostID int64) error {
	return nil
}

func seedRelaySpec(t *testing.T, repo *mockInboundSpecRepo, tag string, hostID int64, relayPathID *int64) {
	t.Helper()
	err := repo.Create(context.Background(), &repository.InboundSpec{
		AgentHostID:  &hostID,
		CoreType:     "sing-box",
		Tag:          tag,
		Enabled:      true,
		SemanticSpec: json.RawMessage(fmt.Sprintf(`{"protocol":"vless","port":28001,"listen":"0.0.0.0","tag":"%s"}`, tag)),
		CoreSpecific: json.RawMessage(`{"core_type":"sing-box","sing-box":{"sniff":true}}`),
		RelayPathID:  relayPathID,
	})
	if err != nil {
		t.Fatalf("seed spec %s error: %v", tag, err)
	}
}

func findArtifactByFilename(artifacts []*repository.DesiredArtifact, filename string) *repository.DesiredArtifact {
	for _, a := range artifacts {
		if a != nil && a.Filename == filename {
			return a
		}
	}
	return nil
}

// TestRenderArtifacts_RelayPathBinding 表驱动：spec.relay_path_id 绑定的路由优先级与回落行为。
func TestRenderArtifacts_RelayPathBinding(t *testing.T) {
	enabledPath := func() *repository.RelayPath {
		return &repository.RelayPath{
			ID: 5, Name: "s2-s1", CoreType: "sing-box", Enabled: true,
			Nodes: []repository.RelayPathNode{
				{Sequence: 0, AgentHostID: 11},
				{Sequence: 1, AgentHostID: 12},
			},
		}
	}
	disabledPath := func() *repository.RelayPath {
		p := enabledPath()
		p.Enabled = false
		return p
	}

	cases := []struct {
		name string
		// 输入
		relayPathID *int64
		path        *repository.RelayPath // nil = 链路仓库中无此链路（not found）
		// 断言
		wantRelayRule    bool   // 是否期望生成 mesh-{tag}-relay-5.json
		wantOutboundTag  bool   // 是否校验出站 tag 指向 hop-0（ID 动态）
		wantLegacyExit   bool   // 是否期望旧出口文件 mesh-{tag}-exit.json
	}{
		{
			name:            "bound_enabled_path_routes_to_relay_hop0",
			relayPathID:     intPtr(5),
			path:            enabledPath(),
			wantRelayRule:   true,
			wantOutboundTag: true,
			wantLegacyExit:  false,
		},
		{
			name:            "no_binding_keeps_legacy_behavior",
			relayPathID:     nil,
			path:            enabledPath(),
			wantRelayRule:   false,
			wantLegacyExit:  false,
		},
		{
			name:            "disabled_path_falls_back",
			relayPathID:     intPtr(5),
			path:            disabledPath(),
			wantRelayRule:   false,
			wantLegacyExit:  false,
		},
		{
			name:            "missing_path_falls_back",
			relayPathID:     intPtr(5),
			path:            nil,
			wantRelayRule:   false,
			wantLegacyExit:  false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			specRepo := newMockInboundSpecRepo()
			artifactRepo := &mockDesiredArtifactRepo{}
			relayRepo := newMockRelayPathRepo()
			peerRepo := &mockAgentMeshPeerRepo{
				peers: []*repository.AgentMeshPeer{
					{AgentHostID: 11, WGIP: "10.144.0.1", NetworkID: "default"},
					{AgentHostID: 12, WGIP: "10.144.0.2", NetworkID: "default"},
				},
			}

			if tc.path != nil {
				relayRepo.nextID = 5 // 对齐用例中 spec 绑定的 relay_path_id=5
				if _, err := relayRepo.Create(context.Background(), tc.path); err != nil {
					t.Fatalf("seed relay path: %v", err)
				}
			}
			seedRelaySpec(t, specRepo, "sb-relay-spec", 11, tc.relayPathID)

			svc := NewArtifactCompilerService(specRepo, artifactRepo, nil, peerRepo, nil, nil, relayRepo)

			res, err := svc.RenderArtifacts(context.Background(), RenderArtifactsRequest{
				AgentHostID:     11,
				CoreType:        "sing-box",
				DesiredRevision: 3,
			})
			if err != nil {
				t.Fatalf("render error: %v", err)
			}
			batch := artifactRepo.createdBatches[len(artifactRepo.createdBatches)-1]
			_ = res

			ruleFile := fmt.Sprintf("mesh-sb-relay-spec-relay-%d.json", 5)
			gotRule := findArtifactByFilename(batch, ruleFile)
			if tc.wantRelayRule {
				if gotRule == nil {
					t.Fatalf("expected relay rule artifact %s, got filenames=%v", ruleFile, filenamesOf(batch))
				}
				content := string(gotRule.Content)
				wantTag := fmt.Sprintf("relay-mk-%d", 5)
				if !strings.Contains(content, wantTag) {
					t.Fatalf("relay rule should route to %s, content=%s", wantTag, content)
				}
				if !strings.Contains(content, "sb-relay-spec") {
					t.Fatalf("relay rule should match spec inbound tag, content=%s", content)
				}
			}

			// 成功路径同时应产出 mark 出站 artifact（双核字段断言在渲染函数单测中）
			outFile := findArtifactByFilename(batch, "relay-5-out.json")
			if tc.wantRelayRule {
				if outFile == nil {
					t.Fatalf("expected mark outbound artifact relay-5-out.json, got filenames=%v", filenamesOf(batch))
				}
				if c := string(outFile.Content); !strings.Contains(c, "relay-mk-5") || !strings.Contains(c, "40005") {
					t.Fatalf("mark outbound should contain tag+fwmark(40005), content=%s", c)
				}
			} else if outFile != nil {
				t.Fatalf("unexpected mark outbound artifact relay-5-out.json")
			} else if gotRule != nil {
				t.Fatalf("unexpected relay rule artifact %s", ruleFile)
			}

			legacyExit := fmt.Sprintf("mesh-%s-exit.json", "sb-relay-spec")
			gotLegacy := findArtifactByFilename(batch, legacyExit)
			if tc.wantLegacyExit && gotLegacy == nil {
				t.Fatalf("expected legacy exit artifact %s", legacyExit)
			}
			if !tc.wantLegacyExit && gotLegacy != nil {
				t.Fatalf("unexpected legacy exit artifact %s", legacyExit)
			}
		})
	}
}

// TestRenderArtifacts_RelayPathGetByIDError 仓库错误向上传播（不静默回落）。
func TestRenderArtifacts_RelayPathGetByIDError(t *testing.T) {
	specRepo := newMockInboundSpecRepo()
	artifactRepo := &mockDesiredArtifactRepo{}
	relayRepo := newMockRelayPathRepo()
	peerRepo := &mockAgentMeshPeerRepo{
		peers: []*repository.AgentMeshPeer{{AgentHostID: 11, WGIP: "10.144.0.1", NetworkID: "default"}},
	}
	five := int64(5)
	seedRelaySpec(t, specRepo, "sb-relay-spec", 11, &five)
	relayRepo.getErr = errors.New("db boom")

	svc := NewArtifactCompilerService(specRepo, artifactRepo, nil, peerRepo, nil, nil, relayRepo)
	_, err := svc.RenderArtifacts(context.Background(), RenderArtifactsRequest{
		AgentHostID:     11,
		CoreType:        "sing-box",
		DesiredRevision: 3,
	})
	if err == nil {
		t.Fatalf("expected error to propagate")
	}
	if !strings.Contains(err.Error(), "build relay routing") {
		t.Fatalf("error should be wrapped by relay routing context, got: %v", err)
	}
}

func filenamesOf(artifacts []*repository.DesiredArtifact) []string {
	names := make([]string, 0, len(artifacts))
	for _, a := range artifacts {
		if a != nil {
			names = append(names, a.Filename)
		}
	}
	return names
}
