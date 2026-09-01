package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/creamcroissant/mgpanel/internal/repository"
)

type mockDriftDesiredArtifactRepo struct {
	latestRevision int64
	latestErr      error
	listErr        error
	listItems      []*repository.DesiredArtifact
	listCalls      []repository.DesiredArtifactFilter
	countCalls     []repository.DesiredArtifactFilter
	// findItems 提供按 filename 精确查找的产物；nil 时 FindByHostCoreRevisionFilename 返回 ErrNotFound。
	findItems map[string]*repository.DesiredArtifact
}

func (m *mockDriftDesiredArtifactRepo) PruneOldRevisions(ctx context.Context, keep int) (int64, error) {
	return 0, nil
}

func (m *mockDriftDesiredArtifactRepo) ReplaceRevision(ctx context.Context, agentHostID int64, coreType string, desiredRevision int64, artifacts []*repository.DesiredArtifact, sourceTags ...string) (int64, error) {
	return int64(len(artifacts)), nil
}

func (m *mockDriftDesiredArtifactRepo) CreateBatch(ctx context.Context, artifacts []*repository.DesiredArtifact) error {
	return nil
}

func (m *mockDriftDesiredArtifactRepo) DeleteByHostCoreRevision(ctx context.Context, agentHostID int64, coreType string, desiredRevision int64, sourceTags ...string) error {
	return nil
}

func (m *mockDriftDesiredArtifactRepo) List(ctx context.Context, filter repository.DesiredArtifactFilter) ([]*repository.DesiredArtifact, error) {
	m.listCalls = append(m.listCalls, filter)
	if m.listErr != nil {
		return nil, m.listErr
	}
	filtered := m.filterDesiredArtifacts(filter)
	paged := ddsPaginateDesiredArtifacts(filtered, filter.Limit, filter.Offset)
	if !filter.ExcludeContent {
		return paged, nil
	}
	for _, item := range paged {
		if item != nil {
			item.Content = nil
		}
	}
	return paged, nil
}

func (m *mockDriftDesiredArtifactRepo) Count(ctx context.Context, filter repository.DesiredArtifactFilter) (int64, error) {
	m.countCalls = append(m.countCalls, filter)
	if m.listErr != nil {
		return 0, m.listErr
	}
	return int64(len(m.filterDesiredArtifacts(filter))), nil
}

func (m *mockDriftDesiredArtifactRepo) GetLatestRevision(ctx context.Context, agentHostID int64, coreType string) (int64, error) {
	if m.latestErr != nil {
		return 0, m.latestErr
	}
	return m.latestRevision, nil
}

func (m *mockDriftDesiredArtifactRepo) filterDesiredArtifacts(filter repository.DesiredArtifactFilter) []*repository.DesiredArtifact {
	filtered := make([]*repository.DesiredArtifact, 0)
	for _, item := range m.listItems {
		if item == nil {
			continue
		}
		if filter.AgentHostID > 0 && item.AgentHostID != filter.AgentHostID {
			continue
		}
		if filter.CoreType != nil && strings.TrimSpace(item.CoreType) != strings.TrimSpace(*filter.CoreType) {
			continue
		}
		if filter.DesiredRevision != nil && item.DesiredRevision != *filter.DesiredRevision {
			continue
		}
		if filter.SourceTag != nil && strings.TrimSpace(item.SourceTag) != strings.TrimSpace(*filter.SourceTag) {
			continue
		}
		if filter.Filename != nil && strings.TrimSpace(item.Filename) != strings.TrimSpace(*filter.Filename) {
			continue
		}
		filtered = append(filtered, cloneDesiredArtifact(item))
	}
	return filtered
}

func (m *mockDriftDesiredArtifactRepo) FindByHostCoreRevisionFilename(ctx context.Context, agentHostID int64, coreType string, desiredRevision int64, filename string) (*repository.DesiredArtifact, error) {
	if m.findItems != nil {
		if item, ok := m.findItems[strings.TrimSpace(filename)]; ok {
			return cloneDesiredArtifact(item), nil
		}
	}
	return nil, repository.ErrNotFound
}

type mockDriftInventoryRepo struct {
	listItems []*repository.AgentConfigInventory
	listErr   error
	listCalls []repository.AgentConfigInventoryFilter
}

func (m *mockDriftInventoryRepo) UpsertBatch(ctx context.Context, inventories []*repository.AgentConfigInventory) error {
	return nil
}

func (m *mockDriftInventoryRepo) List(ctx context.Context, filter repository.AgentConfigInventoryFilter) ([]*repository.AgentConfigInventory, error) {
	m.listCalls = append(m.listCalls, filter)
	if m.listErr != nil {
		return nil, m.listErr
	}
	filtered := make([]*repository.AgentConfigInventory, 0)
	for _, item := range m.listItems {
		if item == nil {
			continue
		}
		if filter.AgentHostID != nil && item.AgentHostID != *filter.AgentHostID {
			continue
		}
		if filter.CoreType != nil && strings.TrimSpace(item.CoreType) != strings.TrimSpace(*filter.CoreType) {
			continue
		}
		if filter.Source != nil && strings.TrimSpace(item.Source) != strings.TrimSpace(*filter.Source) {
			continue
		}
		if filter.Filename != nil && strings.TrimSpace(item.Filename) != strings.TrimSpace(*filter.Filename) {
			continue
		}
		if filter.ParseStatus != nil && strings.TrimSpace(item.ParseStatus) != strings.TrimSpace(*filter.ParseStatus) {
			continue
		}
		clone := *item
		filtered = append(filtered, &clone)
	}
	return ddsPaginateInventory(filtered, filter.Limit, filter.Offset), nil
}

func (m *mockDriftInventoryRepo) DeleteStaleByHostCoreBefore(ctx context.Context, agentHostID int64, coreType string, beforeLastSeenAt int64) error {
	return nil
}

type mockDriftInboundIndexRepo struct {
	listItems []*repository.InboundIndex
	listErr   error
	listCalls []repository.InboundIndexFilter
}

func (m *mockDriftInboundIndexRepo) UpsertBatch(ctx context.Context, indexes []*repository.InboundIndex) error {
	return nil
}

func (m *mockDriftInboundIndexRepo) List(ctx context.Context, filter repository.InboundIndexFilter) ([]*repository.InboundIndex, error) {
	m.listCalls = append(m.listCalls, filter)
	if m.listErr != nil {
		return nil, m.listErr
	}
	filtered := make([]*repository.InboundIndex, 0)
	for _, item := range m.listItems {
		if item == nil {
			continue
		}
		if filter.AgentHostID != nil && item.AgentHostID != *filter.AgentHostID {
			continue
		}
		if filter.CoreType != nil && strings.TrimSpace(item.CoreType) != strings.TrimSpace(*filter.CoreType) {
			continue
		}
		if filter.Source != nil && strings.TrimSpace(item.Source) != strings.TrimSpace(*filter.Source) {
			continue
		}
		if filter.Tag != nil && strings.TrimSpace(item.Tag) != strings.TrimSpace(*filter.Tag) {
			continue
		}
		if filter.Protocol != nil && strings.TrimSpace(item.Protocol) != strings.TrimSpace(*filter.Protocol) {
			continue
		}
		if filter.Filename != nil && strings.TrimSpace(item.Filename) != strings.TrimSpace(*filter.Filename) {
			continue
		}
		clone := *item
		clone.TLS = append([]byte(nil), item.TLS...)
		clone.Transport = append([]byte(nil), item.Transport...)
		clone.Multiplex = append([]byte(nil), item.Multiplex...)
		filtered = append(filtered, &clone)
	}
	return ddsPaginateInboundIndex(filtered, filter.Limit, filter.Offset), nil
}

func (m *mockDriftInboundIndexRepo) DeleteStaleByHostCoreBefore(ctx context.Context, agentHostID int64, coreType string, beforeLastSeenAt int64) error {
	return nil
}

type mockDriftStateRepo struct {
	listItems          []*repository.DriftState
	listErr            error
	upsertErr          error
	markRecoveredErr   error
	upsertCalls        []*repository.DriftState
	markRecoveredCalls []mockDriftRecoveredCall
}

type mockDriftRecoveredCall struct {
	agentHostID int64
	coreType    string
	recoveredAt int64
}

func (m *mockDriftStateRepo) Upsert(ctx context.Context, drift *repository.DriftState) error {
	if m.upsertErr != nil {
		return m.upsertErr
	}
	if drift != nil {
		clone := *drift
		clone.Detail = append([]byte(nil), drift.Detail...)
		m.upsertCalls = append(m.upsertCalls, &clone)
	}
	return nil
}

func (m *mockDriftStateRepo) MarkRecoveredByHostCore(ctx context.Context, agentHostID int64, coreType string, recoveredAt int64) error {
	if m.markRecoveredErr != nil {
		return m.markRecoveredErr
	}
	m.markRecoveredCalls = append(m.markRecoveredCalls, mockDriftRecoveredCall{
		agentHostID: agentHostID,
		coreType:    coreType,
		recoveredAt: recoveredAt,
	})
	return nil
}

func (m *mockDriftStateRepo) List(ctx context.Context, filter repository.DriftStateFilter) ([]*repository.DriftState, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	filtered := make([]*repository.DriftState, 0)
	for _, item := range m.listItems {
		if item == nil {
			continue
		}
		if filter.AgentHostID != nil && item.AgentHostID != *filter.AgentHostID {
			continue
		}
		if filter.CoreType != nil && strings.TrimSpace(item.CoreType) != strings.TrimSpace(*filter.CoreType) {
			continue
		}
		if filter.Status != nil && strings.TrimSpace(item.Status) != strings.TrimSpace(*filter.Status) {
			continue
		}
		if filter.DriftType != nil && strings.TrimSpace(item.DriftType) != strings.TrimSpace(*filter.DriftType) {
			continue
		}
		if filter.Tag != nil && strings.TrimSpace(item.Tag) != strings.TrimSpace(*filter.Tag) {
			continue
		}
		if filter.Filename != nil && strings.TrimSpace(item.Filename) != strings.TrimSpace(*filter.Filename) {
			continue
		}
		clone := *item
		clone.Detail = append([]byte(nil), item.Detail...)
		filtered = append(filtered, &clone)
	}
	return ddsPaginateDriftStates(filtered, filter.Limit, filter.Offset), nil
}

func (m *mockDriftStateRepo) Count(ctx context.Context, filter repository.DriftStateFilter) (int64, error) {
	items, err := m.List(ctx, filter)
	if err != nil {
		return 0, err
	}
	return int64(len(items)), nil
}

func TestDriftAndDiffService_EvaluateDrift_PersistsAndRecovers(t *testing.T) {
	desiredRepo := &mockDriftDesiredArtifactRepo{
		latestRevision: 10,
		listItems: []*repository.DesiredArtifact{
			{AgentHostID: 1, CoreType: "xray", DesiredRevision: 10, Filename: "managed-a.json", SourceTag: "in-a", ContentHash: "hash-a"},
			{AgentHostID: 1, CoreType: "xray", DesiredRevision: 10, Filename: "managed-b.json", SourceTag: "in-b", ContentHash: "hash-b"},
		},
	}
	inventoryRepo := &mockDriftInventoryRepo{listItems: []*repository.AgentConfigInventory{
		{AgentHostID: 1, CoreType: "xray", Source: "managed", Filename: "managed-a.json", HashApplied: "hash-a-applied", ParseStatus: "ok", LastSeenAt: 100},
		{AgentHostID: 1, CoreType: "xray", Source: "managed", Filename: "managed-b.json", HashApplied: "hash-b", ParseStatus: "parse_error", ParseError: "bad json", LastSeenAt: 101},
	}}
	indexRepo := &mockDriftInboundIndexRepo{listItems: []*repository.InboundIndex{
		{AgentHostID: 1, CoreType: "xray", Source: "managed", Filename: "managed-a.json", Tag: "in-a", Protocol: "vless", Listen: "127.0.0.1", Port: 443, TLS: json.RawMessage("{}"), Transport: json.RawMessage("{}"), Multiplex: json.RawMessage("{}"), LastSeenAt: 110},
	}}
	driftRepo := &mockDriftStateRepo{listItems: []*repository.DriftState{
		{AgentHostID: 1, CoreType: "xray", Filename: "legacy-old.json", Tag: "old", DriftType: driftTypeHashMismatch, Status: driftStatusDrift},
	}}
	service := NewDriftAndDiffService(desiredRepo, inventoryRepo, indexRepo, driftRepo)

	result, err := service.EvaluateDrift(context.Background(), EvaluateDriftRequest{
		AgentHostID: 1,
		CoreType:    "xray",
	})
	if err != nil {
		t.Fatalf("EvaluateDrift returned error: %v", err)
	}
	if result.DesiredRevision != 10 {
		t.Fatalf("expected desired revision 10, got %d", result.DesiredRevision)
	}
	if len(result.Items) != 3 {
		t.Fatalf("expected 3 drift items, got %d", len(result.Items))
	}
	if !ddsHasDrift(result.Items, "managed-a.json", "in-a", driftTypeHashMismatch) {
		t.Fatalf("expected hash_mismatch for managed-a.json/in-a")
	}
	if !ddsHasDrift(result.Items, "managed-b.json", "in-b", driftTypeMissingTag) {
		t.Fatalf("expected missing_tag for managed-b.json/in-b")
	}
	if !ddsHasDrift(result.Items, "managed-b.json", "in-b", driftTypeParseError) {
		t.Fatalf("expected parse_error for managed-b.json/in-b")
	}
	if len(driftRepo.markRecoveredCalls) != 1 {
		t.Fatalf("expected one mark recovered call, got %d", len(driftRepo.markRecoveredCalls))
	}
	if len(driftRepo.upsertCalls) != 3 {
		t.Fatalf("expected three upsert calls, got %d", len(driftRepo.upsertCalls))
	}
	for _, call := range driftRepo.upsertCalls {
		if call.DesiredRevision != 10 {
			t.Fatalf("expected upsert desired revision 10, got %d", call.DesiredRevision)
		}
		if call.Status != driftStatusDrift {
			t.Fatalf("expected upsert status drift, got %s", call.Status)
		}
	}
}

func TestDriftAndDiffService_EvaluateDrift_TagConflict(t *testing.T) {
	desiredRepo := &mockDriftDesiredArtifactRepo{
		latestRevision: 3,
		listItems: []*repository.DesiredArtifact{
			{AgentHostID: 7, CoreType: "xray", DesiredRevision: 3, Filename: "managed-main.json", SourceTag: "dup", ContentHash: "h1"},
		},
	}
	inventoryRepo := &mockDriftInventoryRepo{listItems: []*repository.AgentConfigInventory{
		{AgentHostID: 7, CoreType: "xray", Source: "managed", Filename: "managed-main.json", HashApplied: "h1", ParseStatus: "ok", LastSeenAt: 200},
	}}
	indexRepo := &mockDriftInboundIndexRepo{listItems: []*repository.InboundIndex{
		{ID: 21, AgentHostID: 7, CoreType: "xray", Source: "managed", Filename: "managed-main.json", Tag: "dup", Protocol: "vless", Listen: "127.0.0.1", Port: 443, TLS: json.RawMessage("{}"), Transport: json.RawMessage("{}"), Multiplex: json.RawMessage("{}"), LastSeenAt: 200},
		{ID: 22, AgentHostID: 7, CoreType: "xray", Source: "managed", Filename: "managed-shadow.json", Tag: "dup", Protocol: "vless", Listen: "127.0.0.1", Port: 444, TLS: json.RawMessage("{}"), Transport: json.RawMessage("{}"), Multiplex: json.RawMessage("{}"), LastSeenAt: 199},
	}}
	driftRepo := &mockDriftStateRepo{}
	service := NewDriftAndDiffService(desiredRepo, inventoryRepo, indexRepo, driftRepo)

	result, err := service.EvaluateDrift(context.Background(), EvaluateDriftRequest{
		AgentHostID: 7,
		CoreType:    "xray",
	})
	if err != nil {
		t.Fatalf("EvaluateDrift returned error: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 drift item, got %d", len(result.Items))
	}
	item := result.Items[0]
	if item.DriftType != driftTypeTagConflict {
		t.Fatalf("expected tag_conflict, got %s", item.DriftType)
	}
	if item.Tag != "dup" {
		t.Fatalf("expected tag dup, got %s", item.Tag)
	}
	if item.Filename != "managed-main.json" {
		t.Fatalf("expected desired filename managed-main.json, got %s", item.Filename)
	}
	if len(driftRepo.markRecoveredCalls) != 0 {
		t.Fatalf("expected no recovered call, got %d", len(driftRepo.markRecoveredCalls))
	}
	if len(driftRepo.upsertCalls) != 1 {
		t.Fatalf("expected one upsert call, got %d", len(driftRepo.upsertCalls))
	}
}

func TestDriftAndDiffService_ListAppliedSnapshot_FilterAndPaging(t *testing.T) {
	inventoryRepo := &mockDriftInventoryRepo{listItems: []*repository.AgentConfigInventory{
		{AgentHostID: 11, CoreType: "xray", Source: "managed", Filename: "a.json", HashApplied: "h1", ParseStatus: "ok", LastSeenAt: 100},
		{AgentHostID: 11, CoreType: "xray", Source: "legacy", Filename: "b.json", HashApplied: "h2", ParseStatus: "parse_error", LastSeenAt: 90},
	}}
	indexRepo := &mockDriftInboundIndexRepo{listItems: []*repository.InboundIndex{
		{AgentHostID: 11, CoreType: "xray", Source: "managed", Filename: "a.json", Tag: "in-a", Protocol: "vless", Listen: "127.0.0.1", Port: 443, LastSeenAt: 100},
		{AgentHostID: 11, CoreType: "xray", Source: "legacy", Filename: "b.json", Tag: "in-b", Protocol: "vmess", Listen: "0.0.0.0", Port: 8443, LastSeenAt: 90},
	}}
	service := NewDriftAndDiffService(&mockDriftDesiredArtifactRepo{}, inventoryRepo, indexRepo, &mockDriftStateRepo{})

	result, err := service.ListAppliedSnapshot(context.Background(), ListAppliedSnapshotRequest{
		AgentHostID: 11,
		CoreType:    "xray",
		Source:      "managed",
		Tag:         "in-a",
		Protocol:    "vless",
		ParseStatus: "ok",
		Limit:       20,
		Offset:      0,
	})
	if err != nil {
		t.Fatalf("ListAppliedSnapshot returned error: %v", err)
	}
	if len(result.Inventories) != 1 || result.Inventories[0].Filename != "a.json" {
		t.Fatalf("unexpected inventories: %+v", result.Inventories)
	}
	if len(result.InboundIndexes) != 1 || result.InboundIndexes[0].Tag != "in-a" {
		t.Fatalf("unexpected indexes: %+v", result.InboundIndexes)
	}
	if len(inventoryRepo.listCalls) == 0 {
		t.Fatalf("expected inventory list called")
	}
	if inventoryRepo.listCalls[0].Source == nil || *inventoryRepo.listCalls[0].Source != "managed" {
		t.Fatalf("unexpected inventory source filter: %+v", inventoryRepo.listCalls[0])
	}
	if inventoryRepo.listCalls[0].ParseStatus == nil || *inventoryRepo.listCalls[0].ParseStatus != "ok" {
		t.Fatalf("unexpected inventory parse_status filter: %+v", inventoryRepo.listCalls[0])
	}
	if len(indexRepo.listCalls) == 0 {
		t.Fatalf("expected index list called")
	}
	if indexRepo.listCalls[0].Tag == nil || *indexRepo.listCalls[0].Tag != "in-a" {
		t.Fatalf("unexpected index tag filter: %+v", indexRepo.listCalls[0])
	}
}

func TestDriftAndDiffService_ListAppliedSnapshot_InvalidFilters(t *testing.T) {
	service := NewDriftAndDiffService(&mockDriftDesiredArtifactRepo{}, &mockDriftInventoryRepo{}, &mockDriftInboundIndexRepo{}, &mockDriftStateRepo{})
	_, err := service.ListAppliedSnapshot(context.Background(), ListAppliedSnapshotRequest{AgentHostID: 1, CoreType: "xray", Source: "bad"})
	if !errors.Is(err, ErrDriftAndDiffInvalidRequest) {
		t.Fatalf("expected invalid request for source, got %v", err)
	}
	_, err = service.ListAppliedSnapshot(context.Background(), ListAppliedSnapshotRequest{AgentHostID: 1, CoreType: "xray", ParseStatus: "bad"})
	if !errors.Is(err, ErrDriftAndDiffInvalidRequest) {
		t.Fatalf("expected invalid request for parse_status, got %v", err)
	}
}

func TestDriftAndDiffService_ListDriftStates_FilterAndPaging(t *testing.T) {
	driftRepo := &mockDriftStateRepo{listItems: []*repository.DriftState{
		{AgentHostID: 11, CoreType: "xray", Filename: "a.json", Tag: "in-a", DriftType: driftTypeHashMismatch, Status: driftStatusDrift},
		{AgentHostID: 11, CoreType: "xray", Filename: "b.json", Tag: "in-b", DriftType: driftTypeMissingTag, Status: driftStatusRecovered},
	}}
	service := NewDriftAndDiffService(&mockDriftDesiredArtifactRepo{}, &mockDriftInventoryRepo{}, &mockDriftInboundIndexRepo{}, driftRepo)

	result, err := service.ListDriftStates(context.Background(), ListDriftStatesRequest{
		AgentHostID: 11,
		CoreType:    "xray",
		Status:      "drift",
		DriftType:   driftTypeHashMismatch,
		Tag:         "in-a",
		Filename:    "a.json",
		Limit:       10,
		Offset:      0,
	})
	if err != nil {
		t.Fatalf("ListDriftStates returned error: %v", err)
	}
	if result.Total != 1 || len(result.Items) != 1 {
		t.Fatalf("unexpected drift states result: %+v", result)
	}
	if result.Items[0].Filename != "a.json" || result.Items[0].Status != driftStatusDrift {
		t.Fatalf("unexpected drift item: %+v", result.Items[0])
	}
}

func TestDriftAndDiffService_ListDriftStates_InvalidFilters(t *testing.T) {
	service := NewDriftAndDiffService(&mockDriftDesiredArtifactRepo{}, &mockDriftInventoryRepo{}, &mockDriftInboundIndexRepo{}, &mockDriftStateRepo{})
	_, err := service.ListDriftStates(context.Background(), ListDriftStatesRequest{AgentHostID: 1, CoreType: "xray", Status: "bad"})
	if !errors.Is(err, ErrDriftAndDiffInvalidRequest) {
		t.Fatalf("expected invalid request for status, got %v", err)
	}
	_, err = service.ListDriftStates(context.Background(), ListDriftStatesRequest{AgentHostID: 1, CoreType: "xray", DriftType: "bad"})
	if !errors.Is(err, ErrDriftAndDiffInvalidRequest) {
		t.Fatalf("expected invalid request for drift_type, got %v", err)
	}
}

func TestDriftAndDiffService_ListArtifacts_FilterAndPaging(t *testing.T) {
	desiredRepo := &mockDriftDesiredArtifactRepo{listItems: []*repository.DesiredArtifact{
		{AgentHostID: 11, CoreType: "xray", DesiredRevision: 5, Filename: "cfg-a.json", SourceTag: "in-a", Content: []byte(`{"inbounds":[]}`)},
		{AgentHostID: 11, CoreType: "xray", DesiredRevision: 5, Filename: "cfg-b.json", SourceTag: "in-b", Content: []byte(`{"inbounds":[]}`)},
		{AgentHostID: 11, CoreType: "xray", DesiredRevision: 4, Filename: "old.json", SourceTag: "old", Content: []byte(`{"inbounds":[]}`)},
	}}
	service := NewDriftAndDiffService(desiredRepo, &mockDriftInventoryRepo{}, &mockDriftInboundIndexRepo{}, &mockDriftStateRepo{})

	result, err := service.ListArtifacts(context.Background(), ListDesiredArtifactsRequest{
		AgentHostID:     11,
		CoreType:        "xray",
		DesiredRevision: 5,
		Tag:             "in-b",
		Limit:           10,
		Offset:          0,
	})
	if err != nil {
		t.Fatalf("ListArtifacts returned error: %v", err)
	}
	if result.DesiredRevision != 5 {
		t.Fatalf("expected desired revision 5, got %d", result.DesiredRevision)
	}
	if result.Total != 1 {
		t.Fatalf("expected total 1, got %d", result.Total)
	}
	if len(result.Items) != 1 || result.Items[0].Filename != "cfg-b.json" {
		t.Fatalf("unexpected listed artifacts: %+v", result.Items)
	}

	pageResult, err := service.ListArtifacts(context.Background(), ListDesiredArtifactsRequest{
		AgentHostID:     11,
		CoreType:        "xray",
		DesiredRevision: 5,
		Limit:           1,
		Offset:          1,
	})
	if err != nil {
		t.Fatalf("ListArtifacts paging returned error: %v", err)
	}
	if pageResult.Total != 2 {
		t.Fatalf("expected total 2 for revision 5, got %d", pageResult.Total)
	}
	if len(pageResult.Items) != 1 {
		t.Fatalf("expected one paged item, got %d", len(pageResult.Items))
	}
	if pageResult.Items[0].Filename != "cfg-b.json" {
		t.Fatalf("expected second page item cfg-b.json, got %s", pageResult.Items[0].Filename)
	}
}

func TestDriftAndDiffService_GetTextDiff_ByTag(t *testing.T) {
	desiredRepo := &mockDriftDesiredArtifactRepo{listItems: []*repository.DesiredArtifact{
		{
			AgentHostID:     11,
			CoreType:        "xray",
			DesiredRevision: 5,
			Filename:        "cfg.json",
			SourceTag:       "in-a",
			Content:         []byte(`{"inbounds":[{"transport":{},"tag":"in-a","protocol":"vless","listen":"127.0.0.1","port":443,"tls":{"enabled":true},"multiplex":{}}]}`),
		},
	}}
	indexRepo := &mockDriftInboundIndexRepo{listItems: []*repository.InboundIndex{
		{AgentHostID: 11, CoreType: "xray", Source: "managed", Filename: "cfg.json", Tag: "in-a", Protocol: "vless", Listen: "127.0.0.1", Port: 444, TLS: json.RawMessage(`{"enabled":false}`), Transport: json.RawMessage(`{}`), Multiplex: json.RawMessage(`{}`), LastSeenAt: 100},
	}}
	service := NewDriftAndDiffService(desiredRepo, &mockDriftInventoryRepo{}, indexRepo, &mockDriftStateRepo{})

	result, err := service.GetTextDiff(context.Background(), GetTextDiffRequest{
		AgentHostID:     11,
		CoreType:        "xray",
		DesiredRevision: 5,
		Tag:             "in-a",
	})
	if err != nil {
		t.Fatalf("GetTextDiff returned error: %v", err)
	}
	if !result.Different {
		t.Fatalf("expected different=true")
	}
	if result.DesiredRevision != 5 {
		t.Fatalf("expected desired revision 5, got %d", result.DesiredRevision)
	}
	if result.Filename != "cfg.json" {
		t.Fatalf("expected filename cfg.json, got %s", result.Filename)
	}
	if !strings.Contains(result.UnifiedDiff, "desired/cfg.json") {
		t.Fatalf("expected unified diff contains desired file header")
	}
	if !strings.Contains(result.DesiredText, `"port": 443`) {
		t.Fatalf("expected desired text contains port 443")
	}
	if !strings.Contains(result.AppliedText, `"port": 444`) {
		t.Fatalf("expected applied text contains port 444")
	}
}

func TestDriftAndDiffService_GetTextDiff_InvalidSelector(t *testing.T) {
	service := NewDriftAndDiffService(&mockDriftDesiredArtifactRepo{}, &mockDriftInventoryRepo{}, &mockDriftInboundIndexRepo{}, &mockDriftStateRepo{})
	_, err := service.GetTextDiff(context.Background(), GetTextDiffRequest{
		AgentHostID: 1,
		CoreType:    "xray",
	})
	if !errors.Is(err, ErrDriftAndDiffInvalidRequest) {
		t.Fatalf("expected invalid request error, got %v", err)
	}
}

func TestDriftAndDiffService_GetTextDiff_AmbiguousSelector(t *testing.T) {
	desiredRepo := &mockDriftDesiredArtifactRepo{listItems: []*repository.DesiredArtifact{
		{AgentHostID: 1, CoreType: "xray", DesiredRevision: 2, Filename: "a.json", SourceTag: "dup", Content: []byte(`{"inbounds":[]}`)},
		{AgentHostID: 1, CoreType: "xray", DesiredRevision: 2, Filename: "b.json", SourceTag: "dup", Content: []byte(`{"inbounds":[]}`)},
	}}
	service := NewDriftAndDiffService(desiredRepo, &mockDriftInventoryRepo{}, &mockDriftInboundIndexRepo{}, &mockDriftStateRepo{})
	_, err := service.GetTextDiff(context.Background(), GetTextDiffRequest{
		AgentHostID:     1,
		CoreType:        "xray",
		DesiredRevision: 2,
		Tag:             "dup",
	})
	if !errors.Is(err, ErrDriftAndDiffInvalidRequest) {
		t.Fatalf("expected ambiguous selector invalid request error, got %v", err)
	}
}

func TestDriftAndDiffService_GetSemanticDiff_FieldMismatchAndMissingTag(t *testing.T) {
	desiredRepo := &mockDriftDesiredArtifactRepo{
		latestRevision: 6,
		listItems: []*repository.DesiredArtifact{
			{
				AgentHostID:     19,
				CoreType:        "xray",
				DesiredRevision: 6,
				Filename:        "a.json",
				SourceTag:       "in-a",
				Content: []byte(`{
					"inbounds": [
						{
							"protocol": "vless",
							"tag": "in-a",
							"listen": "127.0.0.1",
							"port": 443,
							"streamSettings": {
								"network": "ws",
								"wsSettings": {"path": "/ws"},
								"security": "tls",
								"tlsSettings": {"serverName": "example.com"}
							}
						}
					]
				}`),
			},
			{
				AgentHostID:     19,
				CoreType:        "xray",
				DesiredRevision: 6,
				Filename:        "b.json",
				SourceTag:       "in-b",
				Content: []byte(`{
					"inbounds": [
						{
							"protocol": "vmess",
							"tag": "in-b",
							"listen": "0.0.0.0",
							"port": 8443
						}
					]
				}`),
			},
		},
	}
	indexRepo := &mockDriftInboundIndexRepo{listItems: []*repository.InboundIndex{
		{AgentHostID: 19, CoreType: "xray", Source: "managed", Filename: "a.json", Tag: "in-a", Protocol: "vmess", Listen: "127.0.0.1", Port: 444, TLS: json.RawMessage(`{}`), Transport: json.RawMessage(`{}`), Multiplex: json.RawMessage(`{}`), LastSeenAt: 300},
	}}
	service := NewDriftAndDiffService(desiredRepo, &mockDriftInventoryRepo{}, indexRepo, &mockDriftStateRepo{})

	result, err := service.GetSemanticDiff(context.Background(), GetSemanticDiffRequest{
		AgentHostID: 19,
		CoreType:    "xray",
	})
	if err != nil {
		t.Fatalf("GetSemanticDiff returned error: %v", err)
	}
	if result.DesiredRevision != 6 {
		t.Fatalf("expected desired revision 6, got %d", result.DesiredRevision)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 semantic diff items, got %d", len(result.Items))
	}

	itemA := ddsFindSemanticDiffItem(result.Items, "in-a")
	if itemA == nil {
		t.Fatalf("expected semantic diff item for tag in-a")
	}
	if itemA.DriftType != driftTypeHashMismatch {
		t.Fatalf("expected in-a drift type hash_mismatch, got %s", itemA.DriftType)
	}
	if !ddsHasSemanticFieldDiff(itemA.FieldDiffs, "protocol") {
		t.Fatalf("expected protocol field diff for in-a")
	}

	itemB := ddsFindSemanticDiffItem(result.Items, "in-b")
	if itemB == nil {
		t.Fatalf("expected semantic diff item for tag in-b")
	}
	if itemB.DriftType != driftTypeMissingTag {
		t.Fatalf("expected in-b drift type missing_tag, got %s", itemB.DriftType)
	}
}

func TestDriftAndDiffService_GetSemanticDiff_TargetTagNotFound(t *testing.T) {
	desiredRepo := &mockDriftDesiredArtifactRepo{
		latestRevision: 4,
		listItems: []*repository.DesiredArtifact{
			{AgentHostID: 2, CoreType: "xray", DesiredRevision: 4, Filename: "a.json", SourceTag: "in-a", Content: []byte(`{"inbounds":[{"protocol":"vless","tag":"in-a","listen":"127.0.0.1","port":443}]}`)},
		},
	}
	service := NewDriftAndDiffService(desiredRepo, &mockDriftInventoryRepo{}, &mockDriftInboundIndexRepo{}, &mockDriftStateRepo{})

	_, err := service.GetSemanticDiff(context.Background(), GetSemanticDiffRequest{
		AgentHostID: 2,
		CoreType:    "xray",
		Tag:         "not-exist",
	})
	if !errors.Is(err, ErrDriftAndDiffTagMissing) {
		t.Fatalf("expected tag missing error, got %v", err)
	}
}

func TestDriftAndDiffService_GetSemanticDiff_DesiredMissingStillUsesDesiredMissing(t *testing.T) {
	desiredRepo := &mockDriftDesiredArtifactRepo{latestRevision: 0}
	service := NewDriftAndDiffService(desiredRepo, &mockDriftInventoryRepo{}, &mockDriftInboundIndexRepo{}, &mockDriftStateRepo{})

	_, err := service.GetSemanticDiff(context.Background(), GetSemanticDiffRequest{
		AgentHostID: 2,
		CoreType:    "xray",
	})
	if !errors.Is(err, ErrDriftAndDiffDesiredMissing) {
		t.Fatalf("expected desired missing error, got %v", err)
	}
}

func cloneDesiredArtifact(item *repository.DesiredArtifact) *repository.DesiredArtifact {
	if item == nil {
		return nil
	}
	clone := *item
	clone.Content = append([]byte(nil), item.Content...)
	return &clone
}

func ddsPaginateDesiredArtifacts(items []*repository.DesiredArtifact, limit, offset int) []*repository.DesiredArtifact {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return []*repository.DesiredArtifact{}
	}
	end := len(items)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return items[offset:end]
}

func ddsPaginateInventory(items []*repository.AgentConfigInventory, limit, offset int) []*repository.AgentConfigInventory {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return []*repository.AgentConfigInventory{}
	}
	end := len(items)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return items[offset:end]
}

func ddsPaginateInboundIndex(items []*repository.InboundIndex, limit, offset int) []*repository.InboundIndex {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return []*repository.InboundIndex{}
	}
	end := len(items)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return items[offset:end]
}

func ddsPaginateDriftStates(items []*repository.DriftState, limit, offset int) []*repository.DriftState {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return []*repository.DriftState{}
	}
	end := len(items)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return items[offset:end]
}

func ddsHasDrift(items []DriftItem, filename, tag, driftType string) bool {
	for _, item := range items {
		if item.Filename == filename && item.Tag == tag && item.DriftType == driftType {
			return true
		}
	}
	return false
}

func ddsFindSemanticDiffItem(items []SemanticDiffItem, tag string) *SemanticDiffItem {
	for i := range items {
		if items[i].Tag == tag {
			return &items[i]
		}
	}
	return nil
}

func ddsHasSemanticFieldDiff(items []SemanticFieldDiff, field string) bool {
	for _, item := range items {
		if item.Field == field {
			return true
		}
	}
	return false
}

func TestDriftAndDiffService_GetTextDiff_ByFilename(t *testing.T) {
	desiredRepo := &mockDriftDesiredArtifactRepo{
		listItems: []*repository.DesiredArtifact{
			{AgentHostID: 11, CoreType: "xray", DesiredRevision: 5, Filename: "cfg.json", SourceTag: "in-a", Content: []byte(`{"inbounds":[{"protocol":"vless","tag":"in-a","listen":"127.0.0.1","port":443}]}`)},
		},
		findItems: map[string]*repository.DesiredArtifact{
			"cfg.json": {AgentHostID: 11, CoreType: "xray", DesiredRevision: 5, Filename: "cfg.json", SourceTag: "in-a", Content: []byte(`{"inbounds":[{"protocol":"vless","tag":"in-a","listen":"127.0.0.1","port":443}]}`)},
		},
	}
	indexRepo := &mockDriftInboundIndexRepo{listItems: []*repository.InboundIndex{
		{AgentHostID: 11, CoreType: "xray", Source: "managed", Filename: "cfg.json", Tag: "in-a", Protocol: "vless", Listen: "127.0.0.1", Port: 444, TLS: json.RawMessage(`{}`), Transport: json.RawMessage(`{}`), Multiplex: json.RawMessage(`{}`), LastSeenAt: 100},
	}}
	service := NewDriftAndDiffService(desiredRepo, &mockDriftInventoryRepo{}, indexRepo, &mockDriftStateRepo{})

	result, err := service.GetTextDiff(context.Background(), GetTextDiffRequest{
		AgentHostID: 11, CoreType: "xray", DesiredRevision: 5, Filename: "cfg.json",
	})
	if err != nil {
		t.Fatalf("GetTextDiff by filename: %v", err)
	}
	if result.Filename != "cfg.json" || result.Tag != "in-a" || result.DesiredRevision != 5 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !result.Different {
		t.Fatal("expected different=true (desired port 443 vs applied 444)")
	}
}

func TestDriftAndDiffService_GetTextDiff_AmbiguousSelector_ByTag(t *testing.T) {
	desiredRepo := &mockDriftDesiredArtifactRepo{
		listItems: []*repository.DesiredArtifact{
			{AgentHostID: 3, CoreType: "xray", DesiredRevision: 2, Filename: "a.json", SourceTag: "dup", Content: []byte(`{"inbounds":[{"protocol":"vless","tag":"dup","listen":"0.0.0.0","port":443}]}`)},
			{AgentHostID: 3, CoreType: "xray", DesiredRevision: 2, Filename: "b.json", SourceTag: "dup", Content: []byte(`{"inbounds":[{"protocol":"vless","tag":"dup","listen":"0.0.0.0","port":8443}]}`)},
		},
	}
	service := NewDriftAndDiffService(desiredRepo, &mockDriftInventoryRepo{}, &mockDriftInboundIndexRepo{}, &mockDriftStateRepo{})

	_, err := service.GetTextDiff(context.Background(), GetTextDiffRequest{
		AgentHostID: 3, CoreType: "xray", DesiredRevision: 2, Tag: "dup",
	})
	if !errors.Is(err, ErrDriftAndDiffInvalidRequest) {
		t.Fatalf("expected ambiguous selector error, got %v", err)
	}
}

func TestDriftAndDiffService_EvaluateDrift_TagConflict_Persists(t *testing.T) {
	desiredRepo := &mockDriftDesiredArtifactRepo{
		latestRevision: 1,
		listItems: []*repository.DesiredArtifact{
			{AgentHostID: 5, CoreType: "xray", DesiredRevision: 1, Filename: "a.json", SourceTag: "dup", ContentHash: "h"},
		},
	}
	indexRepo := &mockDriftInboundIndexRepo{listItems: []*repository.InboundIndex{
		{AgentHostID: 5, CoreType: "xray", Source: "managed", Filename: "a.json", Tag: "dup", Protocol: "vless", Listen: "0.0.0.0", Port: 443, LastSeenAt: 10},
		{AgentHostID: 5, CoreType: "xray", Source: "managed", Filename: "b.json", Tag: "dup", Protocol: "vless", Listen: "0.0.0.0", Port: 443, LastSeenAt: 11},
	}}
	driftRepo := &mockDriftStateRepo{}
	service := NewDriftAndDiffService(desiredRepo, &mockDriftInventoryRepo{}, indexRepo, driftRepo)

	result, err := service.EvaluateDrift(context.Background(), EvaluateDriftRequest{AgentHostID: 5, CoreType: "xray"})
	if err != nil {
		t.Fatalf("EvaluateDrift: %v", err)
	}
	if !ddsHasDrift(result.Items, "a.json", "dup", driftTypeTagConflict) {
		t.Fatalf("expected tag_conflict drift, got %+v", result.Items)
	}
	if len(driftRepo.upsertCalls) == 0 {
		t.Fatal("expected drift states persisted")
	}
}

func TestDriftAndDiffService_ShouldUpsertDrift_Matrix(t *testing.T) {
	base := &repository.DriftState{
		AgentHostID: 1, CoreType: "xray", Filename: "a.json", Tag: "in-a",
		DesiredRevision: 3, DesiredHash: "dh", AppliedHash: "ah",
		DriftType: driftTypeHashMismatch, Status: driftStatusDrift,
		Detail: json.RawMessage(`{"k":"v"}`),
	}
	item := DriftItem{
		Filename: "a.json", Tag: "in-a", DesiredRevision: 3,
		DesiredHash: "dh", AppliedHash: "ah", DriftType: driftTypeHashMismatch,
		Detail: json.RawMessage(`{"k":"v"}`),
	}
	if shouldUpsertDrift(nil, item, false) != true {
		t.Fatal("nil existing must upsert")
	}
	if shouldUpsertDrift(base, item, false) != false {
		t.Fatal("identical drift must be skipped")
	}
	recovered := *base
	recovered.Status = driftStatusRecovered
	if shouldUpsertDrift(&recovered, item, false) != true {
		t.Fatal("recovered existing must upsert")
	}
	if shouldUpsertDrift(base, item, true) != true {
		t.Fatal("staleRecovered flag must force upsert")
	}
	diffRev := *base
	diffRev.DesiredRevision = 4
	if shouldUpsertDrift(&diffRev, item, false) != true {
		t.Fatal("revision change must upsert")
	}
	diffHash := *base
	diffHash.DesiredHash = "dh2"
	if shouldUpsertDrift(&diffHash, item, false) != true {
		t.Fatal("hash change must upsert")
	}
	diffDetail := *base
	diffDetail.Detail = json.RawMessage(`{"k":"v2"}`)
	if shouldUpsertDrift(&diffDetail, item, false) != true {
		t.Fatal("detail change must upsert")
	}
	// jsonRawEqual: 语义相同但字节序不同 → 视为相等（不 upsert）
	reordered := *base
	reordered.Detail = json.RawMessage(`{"k":"v","extra":1}`)
	if shouldUpsertDrift(&reordered, item, false) != true {
		t.Fatal("extra key must upsert")
	}
	if !jsonRawEqual(json.RawMessage(`{"a":1,"b":[1,2]}`), json.RawMessage(`{"b":[1,2],"a":1}`)) {
		t.Fatal("jsonRawEqual must be order-insensitive")
	}
	if jsonRawEqual(json.RawMessage(`{"a":1}`), json.RawMessage(`{"a":2}`)) {
		t.Fatal("jsonRawEqual must detect value change")
	}
	if !jsonRawEqual(json.RawMessage(``), json.RawMessage(``)) {
		t.Fatal("empty raw equal")
	}
}

func TestDriftAndDiffService_GetSemanticDiff_TagConflict_Persists(t *testing.T) {
	// desired 中同一 tag 出现在两个不同文件名 → 期望 tag_conflict（desired 侧）
	desiredRepo := &mockDriftDesiredArtifactRepo{
		latestRevision: 7,
		listItems: []*repository.DesiredArtifact{
			{AgentHostID: 31, CoreType: "xray", DesiredRevision: 7, Filename: "x.json", SourceTag: "dup", Content: []byte(`{"inbounds":[{"protocol":"vless","tag":"dup","listen":"0.0.0.0","port":443}]}`)},
			{AgentHostID: 31, CoreType: "xray", DesiredRevision: 7, Filename: "y.json", SourceTag: "dup", Content: []byte(`{"inbounds":[{"protocol":"vless","tag":"dup","listen":"0.0.0.0","port":8443}]}`)},
		},
	}
	service := NewDriftAndDiffService(desiredRepo, &mockDriftInventoryRepo{}, &mockDriftInboundIndexRepo{}, &mockDriftStateRepo{})

	result, err := service.GetSemanticDiff(context.Background(), GetSemanticDiffRequest{AgentHostID: 31, CoreType: "xray"})
	if err != nil {
		t.Fatalf("GetSemanticDiff: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].DriftType != driftTypeTagConflict || result.Items[0].Tag != "dup" {
		t.Fatalf("expected desired-side tag_conflict, got %+v", result.Items)
	}
}

func TestDriftAndDiffService_SourcePriority_Ordering(t *testing.T) {
	// sourcePriority: managed(3) > merged(2) > legacy(1) > other(0)
	if p := sourcePriority(inventorySourceManaged); p != 3 {
		t.Fatalf("managed priority = %d", p)
	}
	if p := sourcePriority(inventorySourceMerged); p != 2 {
		t.Fatalf("merged priority = %d", p)
	}
	if p := sourcePriority(inventorySourceLegacy); p != 1 {
		t.Fatalf("legacy priority = %d", p)
	}
	if p := sourcePriority("manual"); p != 0 {
		t.Fatalf("manual priority = %d", p)
	}
	if p := sourcePriority(" MANAGED "); p != 3 {
		t.Fatalf("case/space-insensitive priority = %d", p)
	}
}
