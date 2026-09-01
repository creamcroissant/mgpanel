package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/creamcroissant/xboard/internal/repository"
)

type mockApplyDesiredArtifactRepo struct {
	replaced  [][]*repository.DesiredArtifact
	replaceErr error
	latestRevision int64
	latestErr      error
	listItems      []*repository.DesiredArtifact
	listErr        error
	lastListFilter repository.DesiredArtifactFilter
}

func (m *mockApplyDesiredArtifactRepo) PruneOldRevisions(ctx context.Context, keep int) (int64, error) {
	return 0, nil
}

func (m *mockApplyDesiredArtifactRepo) ReplaceRevision(ctx context.Context, agentHostID int64, coreType string, desiredRevision int64, artifacts []*repository.DesiredArtifact, sourceTags ...string) (int64, error) {
	if m.replaceErr != nil {
		return 0, m.replaceErr
	}
	m.replaced = append(m.replaced, artifacts)
	return int64(len(artifacts)), nil
}

func (m *mockApplyDesiredArtifactRepo) CreateBatch(ctx context.Context, artifacts []*repository.DesiredArtifact) error {
	return nil
}

func (m *mockApplyDesiredArtifactRepo) DeleteByHostCoreRevision(ctx context.Context, agentHostID int64, coreType string, desiredRevision int64, sourceTags ...string) error {
	return nil
}

func (m *mockApplyDesiredArtifactRepo) List(ctx context.Context, filter repository.DesiredArtifactFilter) ([]*repository.DesiredArtifact, error) {
	m.lastListFilter = filter
	if m.listErr != nil {
		return nil, m.listErr
	}
	result := make([]*repository.DesiredArtifact, 0, len(m.listItems))
	for _, item := range m.listItems {
		if item == nil {
			result = append(result, nil)
			continue
		}
		clone := *item
		if item.Content != nil {
			clone.Content = append([]byte(nil), item.Content...)
		}
		result = append(result, &clone)
	}
	return result, nil
}

func (m *mockApplyDesiredArtifactRepo) GetLatestRevision(ctx context.Context, agentHostID int64, coreType string) (int64, error) {
	if m.latestErr != nil {
		return 0, m.latestErr
	}
	return m.latestRevision, nil
}

func (m *mockApplyDesiredArtifactRepo) Count(ctx context.Context, filter repository.DesiredArtifactFilter) (int64, error) {
	return int64(len(m.listItems)), nil
}

func (m *mockApplyDesiredArtifactRepo) FindByHostCoreRevisionFilename(ctx context.Context, agentHostID int64, coreType string, desiredRevision int64, filename string) (*repository.DesiredArtifact, error) {
	return nil, repository.ErrNotFound
}

type mockApplyRunRepo struct {
	createErr     error
	findErr       error
	updateErr     error
	listErr       error
	countErr      error
	expireErr     error
	expireCount   int64
	createdRuns   []*repository.ApplyRun
	findRun       *repository.ApplyRun
	listRuns      []*repository.ApplyRun
	countTotal    int64
	updateCalls        []mockApplyRunUpdateCall
	lastFindRunID      string
	lastList           repository.ApplyRunFilter
	lastCount          repository.ApplyRunFilter
	lastExpireDeadline int64
	lastExpireMessage  string
}

type mockApplyRunUpdateCall struct {
	runID            string
	status           string
	errorMessage     string
	rollbackRevision int64
	finishedAt       int64
}

func (m *mockApplyRunRepo) Create(ctx context.Context, run *repository.ApplyRun) error {
	if m.createErr != nil {
		return m.createErr
	}
	if run != nil {
		clone := *run
		m.createdRuns = append(m.createdRuns, &clone)
	}
	return nil
}

func (m *mockApplyRunRepo) UpdateStatus(ctx context.Context, runID, status, errorMessage string, rollbackRevision int64, finishedAt int64) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.updateCalls = append(m.updateCalls, mockApplyRunUpdateCall{
		runID:            runID,
		status:           status,
		errorMessage:     errorMessage,
		rollbackRevision: rollbackRevision,
		finishedAt:       finishedAt,
	})
	return nil
}


func (m *mockApplyRunRepo) MarkStarted(ctx context.Context, runID, status string, startedAt int64) error {
	return nil
}
func (m *mockApplyRunRepo) FindByRunID(ctx context.Context, runID string) (*repository.ApplyRun, error) {
	m.lastFindRunID = runID
	if m.findErr != nil {
		return nil, m.findErr
	}
	if m.findRun == nil {
		return nil, repository.ErrNotFound
	}
	clone := *m.findRun
	return &clone, nil
}

func (m *mockApplyRunRepo) ExpireStale(ctx context.Context, deadline int64, errorMessage string) (int64, error) {
	m.lastExpireDeadline = deadline
	m.lastExpireMessage = errorMessage
	return m.expireCount, m.expireErr
}

func (m *mockApplyRunRepo) List(ctx context.Context, filter repository.ApplyRunFilter) ([]*repository.ApplyRun, error) {
	m.lastList = filter
	if m.listErr != nil {
		return nil, m.listErr
	}
	result := make([]*repository.ApplyRun, 0, len(m.listRuns))
	for _, run := range m.listRuns {
		if run == nil {
			result = append(result, nil)
			continue
		}
		clone := *run
		result = append(result, &clone)
	}
	return result, nil
}

func (m *mockApplyRunRepo) Count(ctx context.Context, filter repository.ApplyRunFilter) (int64, error) {
	m.lastCount = filter
	if m.countErr != nil {
		return 0, m.countErr
	}
	return m.countTotal, nil
}

type mockApplyDiagnosticsService struct {
	semanticResult *SemanticDiffResult
	semanticErr    error
	textResult     *TextDiffResult
	textErr        error

	lastSemanticReq GetSemanticDiffRequest
	lastTextReq     GetTextDiffRequest
	semanticCalls   int
	textCalls       int
}

func (m *mockApplyDiagnosticsService) EvaluateDrift(ctx context.Context, req EvaluateDriftRequest) (*EvaluateDriftResult, error) {
	return nil, errors.New("not implemented")
}

func (m *mockApplyDiagnosticsService) ListAppliedSnapshot(ctx context.Context, req ListAppliedSnapshotRequest) (*ListAppliedSnapshotResult, error) {
	return nil, errors.New("not implemented")
}

func (m *mockApplyDiagnosticsService) ListDriftStates(ctx context.Context, req ListDriftStatesRequest) (*ListDriftStatesResult, error) {
	return nil, errors.New("not implemented")
}

func (m *mockApplyDiagnosticsService) ListArtifacts(ctx context.Context, req ListDesiredArtifactsRequest) (*ListDesiredArtifactsResult, error) {
	return nil, errors.New("not implemented")
}

func (m *mockApplyDiagnosticsService) GetTextDiff(ctx context.Context, req GetTextDiffRequest) (*TextDiffResult, error) {
	m.lastTextReq = req
	m.textCalls++
	if m.textErr != nil {
		return nil, m.textErr
	}
	if m.textResult == nil {
		return &TextDiffResult{DesiredRevision: req.DesiredRevision, Filename: req.Filename, Tag: req.Tag}, nil
	}
	return m.textResult, nil
}

func (m *mockApplyDiagnosticsService) GetSemanticDiff(ctx context.Context, req GetSemanticDiffRequest) (*SemanticDiffResult, error) {
	m.lastSemanticReq = req
	m.semanticCalls++
	if m.semanticErr != nil {
		return nil, m.semanticErr
	}
	if m.semanticResult == nil {
		return &SemanticDiffResult{DesiredRevision: req.DesiredRevision}, nil
	}
	return m.semanticResult, nil
}

func TestApplyOrchestratorService_GetApplyRunDetail_SuccessWithDiagnostics(t *testing.T) {
	repo := &mockApplyRunRepo{findRun: &repository.ApplyRun{
		RunID:          "run-1",
		AgentHostID:    11,
		CoreType:       "xray",
		TargetRevision: 9,
		Status:         "failed",
	}}
	diagnostics := &mockApplyDiagnosticsService{
		semanticResult: &SemanticDiffResult{
			DesiredRevision: 9,
			Items:           []SemanticDiffItem{{Tag: "in-a", DriftType: "hash_mismatch"}},
		},
		textResult: &TextDiffResult{
			DesiredRevision: 9,
			Filename:        "managed-a.json",
			Tag:             "in-a",
			Different:       true,
		},
	}
	svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, repo, diagnostics)

	result, err := svc.GetApplyRunDetail(context.Background(), GetApplyRunDetailRequest{
		RunID:       "run-1",
		IncludeText: true,
		TextTag:     "in-a",
		TextFile:    "managed-a.json",
	})
	if err != nil {
		t.Fatalf("GetApplyRunDetail returned error: %v", err)
	}
	if repo.lastFindRunID != "run-1" {
		t.Fatalf("expected find run id run-1, got %q", repo.lastFindRunID)
	}
	if result.Run == nil || result.Run.RunID != "run-1" {
		t.Fatalf("unexpected run payload: %+v", result.Run)
	}
	if result.SemanticDiff == nil || len(result.SemanticDiff.Items) != 1 {
		t.Fatalf("expected semantic diff, got %+v", result.SemanticDiff)
	}
	if result.TextDiff == nil || result.TextDiff.Tag != "in-a" {
		t.Fatalf("expected text diff, got %+v", result.TextDiff)
	}
	if len(result.Issues) != 0 {
		t.Fatalf("expected no issues, got %+v", result.Issues)
	}
	if diagnostics.semanticCalls != 1 {
		t.Fatalf("expected 1 semantic call, got %d", diagnostics.semanticCalls)
	}
	if diagnostics.lastSemanticReq.AgentHostID != 11 || diagnostics.lastSemanticReq.CoreType != "xray" || diagnostics.lastSemanticReq.DesiredRevision != 9 {
		t.Fatalf("unexpected semantic req: %+v", diagnostics.lastSemanticReq)
	}
	if diagnostics.textCalls != 1 {
		t.Fatalf("expected 1 text call, got %d", diagnostics.textCalls)
	}
	if diagnostics.lastTextReq.AgentHostID != 11 || diagnostics.lastTextReq.CoreType != "xray" || diagnostics.lastTextReq.DesiredRevision != 9 || diagnostics.lastTextReq.Tag != "in-a" || diagnostics.lastTextReq.Filename != "managed-a.json" {
		t.Fatalf("unexpected text req: %+v", diagnostics.lastTextReq)
	}
}

func TestApplyOrchestratorService_GetApplyRunDetail_DiagnosticsIssues(t *testing.T) {
	baseRun := &repository.ApplyRun{
		RunID:          "run-1",
		AgentHostID:    11,
		CoreType:       "xray",
		TargetRevision: 9,
		Status:         "failed",
	}

	t.Run("diagnostics service missing", func(t *testing.T) {
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, &mockApplyRunRepo{findRun: baseRun})
		result, err := svc.GetApplyRunDetail(context.Background(), GetApplyRunDetailRequest{RunID: "run-1"})
		if err != nil {
			t.Fatalf("GetApplyRunDetail returned error: %v", err)
		}
		if len(result.Issues) != 1 || result.Issues[0].Code != "diagnostics_unavailable" {
			t.Fatalf("expected diagnostics_unavailable issue, got %+v", result.Issues)
		}
	})

	t.Run("semantic diff error degrades", func(t *testing.T) {
		diagnostics := &mockApplyDiagnosticsService{semanticErr: errors.New("semantic boom")}
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, &mockApplyRunRepo{findRun: baseRun}, diagnostics)
		result, err := svc.GetApplyRunDetail(context.Background(), GetApplyRunDetailRequest{RunID: "run-1"})
		if err != nil {
			t.Fatalf("GetApplyRunDetail returned error: %v", err)
		}
		if result.SemanticDiff != nil {
			t.Fatalf("expected nil semantic diff, got %+v", result.SemanticDiff)
		}
		if len(result.Issues) != 1 || result.Issues[0].Code != "semantic_diff_unavailable" {
			t.Fatalf("expected semantic_diff_unavailable issue, got %+v", result.Issues)
		}
	})

	t.Run("text diff selector required", func(t *testing.T) {
		diagnostics := &mockApplyDiagnosticsService{semanticResult: &SemanticDiffResult{DesiredRevision: 9}}
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, &mockApplyRunRepo{findRun: baseRun}, diagnostics)
		result, err := svc.GetApplyRunDetail(context.Background(), GetApplyRunDetailRequest{RunID: "run-1", IncludeText: true})
		if err != nil {
			t.Fatalf("GetApplyRunDetail returned error: %v", err)
		}
		if diagnostics.textCalls != 0 {
			t.Fatalf("expected no text diff call, got %d", diagnostics.textCalls)
		}
		if len(result.Issues) != 1 || result.Issues[0].Code != "text_diff_selector_required" {
			t.Fatalf("expected text_diff_selector_required issue, got %+v", result.Issues)
		}
	})

	t.Run("text diff error degrades", func(t *testing.T) {
		diagnostics := &mockApplyDiagnosticsService{
			semanticResult: &SemanticDiffResult{DesiredRevision: 9},
			textErr:        errors.New("text boom"),
		}
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, &mockApplyRunRepo{findRun: baseRun}, diagnostics)
		result, err := svc.GetApplyRunDetail(context.Background(), GetApplyRunDetailRequest{
			RunID:       "run-1",
			IncludeText: true,
			TextTag:     "in-a",
		})
		if err != nil {
			t.Fatalf("GetApplyRunDetail returned error: %v", err)
		}
		if result.TextDiff != nil {
			t.Fatalf("expected nil text diff, got %+v", result.TextDiff)
		}
		if diagnostics.textCalls != 1 {
			t.Fatalf("expected 1 text diff call, got %d", diagnostics.textCalls)
		}
		if len(result.Issues) != 1 || result.Issues[0].Code != "text_diff_unavailable" {
			t.Fatalf("expected text_diff_unavailable issue, got %+v", result.Issues)
		}
	})
}

func TestApplyOrchestratorService_GetApplyRunDetail_InvalidCases(t *testing.T) {
	t.Run("invalid run id", func(t *testing.T) {
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, &mockApplyRunRepo{})
		_, err := svc.GetApplyRunDetail(context.Background(), GetApplyRunDetailRequest{})
		if !errors.Is(err, ErrApplyOrchestratorInvalidRequest) {
			t.Fatalf("expected invalid request error, got %v", err)
		}
	})

	t.Run("run not found", func(t *testing.T) {
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, &mockApplyRunRepo{findErr: repository.ErrNotFound})
		_, err := svc.GetApplyRunDetail(context.Background(), GetApplyRunDetailRequest{RunID: "missing"})
		if !errors.Is(err, ErrApplyOrchestratorNotFound) {
			t.Fatalf("expected not found error, got %v", err)
		}
	})
}

func TestApplyOrchestratorService_ListApplyRuns_Success(t *testing.T) {
	hostID := int64(11)
	runs := []*repository.ApplyRun{{RunID: "run-1", AgentHostID: 11, CoreType: "sing-box", Status: "failed"}}
	repo := &mockApplyRunRepo{listRuns: runs, countTotal: 1}
	svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, repo)

	result, err := svc.ListApplyRuns(context.Background(), ListApplyRunsRequest{
		AgentHostID: &hostID,
		CoreType:    "singbox",
		Status:      "failed",
		Limit:       15,
		Offset:      4,
	})
	if err != nil {
		t.Fatalf("ListApplyRuns returned error: %v", err)
	}
	if result.Total != 1 || len(result.Items) != 1 || result.Items[0].RunID != "run-1" {
		t.Fatalf("unexpected list result: %+v", result)
	}
	if repo.lastCount.AgentHostID == nil || *repo.lastCount.AgentHostID != 11 {
		t.Fatalf("unexpected count host filter: %+v", repo.lastCount.AgentHostID)
	}
	if repo.lastCount.CoreType == nil || *repo.lastCount.CoreType != "sing-box" {
		t.Fatalf("unexpected count core filter: %+v", repo.lastCount.CoreType)
	}
	if repo.lastCount.Status == nil || *repo.lastCount.Status != "failed" {
		t.Fatalf("unexpected count status filter: %+v", repo.lastCount.Status)
	}
	if repo.lastList.Limit != 15 || repo.lastList.Offset != 4 {
		t.Fatalf("unexpected list paging: %+v", repo.lastList)
	}
}

func TestApplyOrchestratorService_ListApplyRuns_Errors(t *testing.T) {
	t.Run("invalid host id", func(t *testing.T) {
		hostID := int64(0)
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, &mockApplyRunRepo{})
		_, err := svc.ListApplyRuns(context.Background(), ListApplyRunsRequest{AgentHostID: &hostID})
		if !errors.Is(err, ErrApplyOrchestratorInvalidRequest) {
			t.Fatalf("expected invalid request error, got %v", err)
		}
	})

	t.Run("invalid core type", func(t *testing.T) {
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, &mockApplyRunRepo{})
		_, err := svc.ListApplyRuns(context.Background(), ListApplyRunsRequest{CoreType: "bad-core"})
		if !errors.Is(err, ErrApplyOrchestratorInvalidRequest) {
			t.Fatalf("expected invalid request error, got %v", err)
		}
	})

	t.Run("invalid status", func(t *testing.T) {
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, &mockApplyRunRepo{})
		_, err := svc.ListApplyRuns(context.Background(), ListApplyRunsRequest{Status: "unknown"})
		if !errors.Is(err, ErrApplyOrchestratorInvalidRequest) {
			t.Fatalf("expected invalid request error, got %v", err)
		}
	})

	t.Run("count error", func(t *testing.T) {
		targetErr := errors.New("count failed")
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, &mockApplyRunRepo{countErr: targetErr})
		_, err := svc.ListApplyRuns(context.Background(), ListApplyRunsRequest{})
		if !errors.Is(err, targetErr) {
			t.Fatalf("expected propagated count error, got %v", err)
		}
	})

	t.Run("list error", func(t *testing.T) {
		targetErr := errors.New("list failed")
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, &mockApplyRunRepo{listErr: targetErr})
		_, err := svc.ListApplyRuns(context.Background(), ListApplyRunsRequest{})
		if !errors.Is(err, targetErr) {
			t.Fatalf("expected propagated list error, got %v", err)
		}
	})
}

func TestApplyOrchestratorService_GetApplyBatch_FailsWhenNoDesiredArtifactsExist(t *testing.T) {
	artifacts := &mockApplyDesiredArtifactRepo{latestRevision: 0}
	applyRuns := &mockApplyRunRepo{}
	svc := NewApplyOrchestratorService(artifacts, applyRuns)

	result, err := svc.GetApplyBatch(context.Background(), GetApplyBatchRequest{
		AgentHostID:     11,
		CoreType:        "sing-box",
		CurrentRevision: 3,
	})
	if err != nil {
		t.Fatalf("expected no error when desired artifacts are missing, got %v", err)
	}
	if result == nil || !result.NotModified {
		t.Fatalf("expected NotModified=true result, got %+v", result)
	}
	if result.TargetRevision != 0 || result.RunID != "" || len(result.Artifacts) != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestApplyOrchestratorService_GetApplyBatch_ReusesOpenRunForSameTargetRevision(t *testing.T) {
	artifacts := &mockApplyDesiredArtifactRepo{
		latestRevision: 8,
		listItems: []*repository.DesiredArtifact{
			{Filename: "a.json", SourceTag: "tag-a", Content: []byte("A"), ContentHash: "ha"},
		},
	}
	applyRuns := &mockApplyRunRepo{
		listRuns: []*repository.ApplyRun{
			{
				RunID:            "run-existing",
				AgentHostID:      21,
				CoreType:         "sing-box",
				TargetRevision:   8,
				Status:           applyRunStatusApplying,
				PreviousRevision: 5,
			},
		},
	}
	svc := NewApplyOrchestratorService(artifacts, applyRuns)

	result, err := svc.GetApplyBatch(context.Background(), GetApplyBatchRequest{
		AgentHostID:     21,
		CoreType:        "sing-box",
		CurrentRevision: 6,
		RunOperatorID:   99,
	})
	if err != nil {
		t.Fatalf("GetApplyBatch returned error: %v", err)
	}
	if result.NotModified {
		t.Fatalf("expected NotModified=false")
	}
	if result.RunID != "run-existing" {
		t.Fatalf("expected reused run id, got %q", result.RunID)
	}
	if result.PreviousRevision != 5 {
		t.Fatalf("expected reused previous revision 5, got %d", result.PreviousRevision)
	}
	if len(applyRuns.createdRuns) != 0 {
		t.Fatalf("expected no new run created, got %d", len(applyRuns.createdRuns))
	}
	if applyRuns.lastList.AgentHostID == nil || *applyRuns.lastList.AgentHostID != 21 {
		t.Fatalf("unexpected list host filter: %+v", applyRuns.lastList.AgentHostID)
	}
	if applyRuns.lastList.CoreType == nil || *applyRuns.lastList.CoreType != "sing-box" {
		t.Fatalf("unexpected list core filter: %+v", applyRuns.lastList.CoreType)
	}
	if applyRuns.lastList.TargetRevision == nil || *applyRuns.lastList.TargetRevision != 8 {
		t.Fatalf("unexpected list target revision filter: %+v", applyRuns.lastList.TargetRevision)
	}
	if len(applyRuns.lastList.Statuses) != 2 || applyRuns.lastList.Statuses[0] != applyRunStatusPending || applyRuns.lastList.Statuses[1] != applyRunStatusApplying {
		t.Fatalf("unexpected open-run statuses filter: %+v", applyRuns.lastList.Statuses)
	}
}

func TestApplyOrchestratorService_GetApplyBatch_Errors(t *testing.T) {
	t.Run("invalid request", func(t *testing.T) {
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, &mockApplyRunRepo{})
		_, err := svc.GetApplyBatch(context.Background(), GetApplyBatchRequest{AgentHostID: 0, CoreType: "xray"})
		if !errors.Is(err, ErrApplyOrchestratorInvalidRequest) {
			t.Fatalf("expected invalid request error, got %v", err)
		}
	})

	t.Run("latest revision error", func(t *testing.T) {
		targetErr := errors.New("latest revision failed")
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{latestErr: targetErr}, &mockApplyRunRepo{})
		_, err := svc.GetApplyBatch(context.Background(), GetApplyBatchRequest{AgentHostID: 1, CoreType: "xray"})
		if !errors.Is(err, targetErr) {
			t.Fatalf("expected propagated latest revision error, got %v", err)
		}
	})

	t.Run("empty artifacts at latest revision", func(t *testing.T) {
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{latestRevision: 2}, &mockApplyRunRepo{})
		_, err := svc.GetApplyBatch(context.Background(), GetApplyBatchRequest{AgentHostID: 1, CoreType: "xray", CurrentRevision: 1})
		if !errors.Is(err, ErrApplyOrchestratorNoArtifacts) {
			t.Fatalf("expected no-artifacts error, got %v", err)
		}
	})

	t.Run("create run failed", func(t *testing.T) {
		targetErr := errors.New("create failed")
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{
			latestRevision: 2,
			listItems:      []*repository.DesiredArtifact{{Filename: "c.json", Content: []byte("{}")}},
		}, &mockApplyRunRepo{createErr: targetErr})
		_, err := svc.GetApplyBatch(context.Background(), GetApplyBatchRequest{AgentHostID: 1, CoreType: "xray", CurrentRevision: 1})
		if !errors.Is(err, targetErr) {
			t.Fatalf("expected propagated create error, got %v", err)
		}
	})
}

func TestApplyOrchestratorService_ReportApplyResult_StateTransitionAndIdempotent(t *testing.T) {
	applyRuns := &mockApplyRunRepo{
		findRun: &repository.ApplyRun{
			RunID:          "run-1",
			AgentHostID:    12,
			CoreType:       "xray",
			TargetRevision: 9,
			Status:         applyRunStatusPending,
		},
	}
	svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, applyRuns)

	err := svc.ReportApplyResult(context.Background(), ReportApplyResultRequest{
		AgentHostID:    12,
		RunID:          "run-1",
		CoreType:       "xray",
		TargetRevision: 9,
		Status:         "applying",
		Success:        false,
	})
	if err != nil {
		t.Fatalf("ReportApplyResult applying returned error: %v", err)
	}
	if len(applyRuns.updateCalls) != 1 {
		t.Fatalf("expected one update call, got %d", len(applyRuns.updateCalls))
	}
	if applyRuns.updateCalls[0].status != applyRunStatusApplying {
		t.Fatalf("expected applying status update, got %s", applyRuns.updateCalls[0].status)
	}
	if applyRuns.updateCalls[0].finishedAt != 0 {
		t.Fatalf("expected non-terminal finished_at=0")
	}

	applyRuns.findRun.Status = applyRunStatusApplying
	err = svc.ReportApplyResult(context.Background(), ReportApplyResultRequest{
		AgentHostID:    12,
		RunID:          "run-1",
		CoreType:       "xray",
		TargetRevision: 9,
		Status:         "success",
		Success:        true,
	})
	if err != nil {
		t.Fatalf("ReportApplyResult success returned error: %v", err)
	}
	if len(applyRuns.updateCalls) != 2 {
		t.Fatalf("expected two update calls, got %d", len(applyRuns.updateCalls))
	}
	if applyRuns.updateCalls[1].status != applyRunStatusSuccess {
		t.Fatalf("expected success status update, got %s", applyRuns.updateCalls[1].status)
	}
	if applyRuns.updateCalls[1].finishedAt <= 0 {
		t.Fatalf("expected terminal finished_at set")
	}

	applyRuns.findRun.Status = applyRunStatusSuccess
	err = svc.ReportApplyResult(context.Background(), ReportApplyResultRequest{
		AgentHostID:    12,
		RunID:          "run-1",
		CoreType:       "xray",
		TargetRevision: 9,
		Status:         "success",
		Success:        true,
	})
	if err != nil {
		t.Fatalf("idempotent report should not fail: %v", err)
	}
	if len(applyRuns.updateCalls) != 2 {
		t.Fatalf("idempotent status should not trigger extra update")
	}
}

func TestApplyOrchestratorService_ReportApplyResult_InvalidCases(t *testing.T) {
	baseRun := &repository.ApplyRun{
		RunID:          "run-2",
		AgentHostID:    7,
		CoreType:       "sing-box",
		TargetRevision: 3,
		Status:         applyRunStatusPending,
	}

	t.Run("run not found", func(t *testing.T) {
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, &mockApplyRunRepo{findErr: repository.ErrNotFound})
		err := svc.ReportApplyResult(context.Background(), ReportApplyResultRequest{AgentHostID: 7, RunID: "missing"})
		if !errors.Is(err, ErrApplyOrchestratorNotFound) {
			t.Fatalf("expected not found, got %v", err)
		}
	})

	t.Run("permission denied", func(t *testing.T) {
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, &mockApplyRunRepo{findRun: baseRun})
		err := svc.ReportApplyResult(context.Background(), ReportApplyResultRequest{AgentHostID: 8, RunID: "run-2"})
		if !errors.Is(err, ErrApplyOrchestratorPermissionDenied) {
			t.Fatalf("expected permission denied, got %v", err)
		}
	})

	t.Run("core mismatch", func(t *testing.T) {
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, &mockApplyRunRepo{findRun: baseRun})
		err := svc.ReportApplyResult(context.Background(), ReportApplyResultRequest{AgentHostID: 7, RunID: "run-2", CoreType: "xray"})
		if !errors.Is(err, ErrApplyOrchestratorInvalidRequest) {
			t.Fatalf("expected invalid request, got %v", err)
		}
	})

	t.Run("target revision mismatch", func(t *testing.T) {
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, &mockApplyRunRepo{findRun: baseRun})
		err := svc.ReportApplyResult(context.Background(), ReportApplyResultRequest{AgentHostID: 7, RunID: "run-2", TargetRevision: 4})
		if !errors.Is(err, ErrApplyOrchestratorInvalidRequest) {
			t.Fatalf("expected invalid request, got %v", err)
		}
	})

	t.Run("stale pending after terminal is noop", func(t *testing.T) {
		run := *baseRun
		run.Status = applyRunStatusSuccess
		repo := &mockApplyRunRepo{findRun: &run}
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, repo)
		err := svc.ReportApplyResult(context.Background(), ReportApplyResultRequest{
			AgentHostID: 7,
			RunID:       "run-2",
			Status:      "pending",
			Success:     false,
		})
		if err != nil {
			t.Fatalf("expected noop for stale pending, got %v", err)
		}
		if len(repo.updateCalls) != 0 {
			t.Fatalf("expected no update calls for stale pending, got %d", len(repo.updateCalls))
		}
	})

	t.Run("late failed after success is noop", func(t *testing.T) {
		run := *baseRun
		run.Status = applyRunStatusSuccess
		repo := &mockApplyRunRepo{findRun: &run}
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, repo)
		err := svc.ReportApplyResult(context.Background(), ReportApplyResultRequest{
			AgentHostID: 7,
			RunID:       "run-2",
			Status:      "failed",
			Success:     false,
		})
		if err != nil {
			t.Fatalf("expected noop for late failed report, got %v", err)
		}
		if len(repo.updateCalls) != 0 {
			t.Fatalf("expected no update calls for late failed report, got %d", len(repo.updateCalls))
		}
	})

	t.Run("late success after failed is invalid transition", func(t *testing.T) {
		run := *baseRun
		run.Status = applyRunStatusFailed
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, &mockApplyRunRepo{findRun: &run})
		err := svc.ReportApplyResult(context.Background(), ReportApplyResultRequest{
			AgentHostID: 7,
			RunID:       "run-2",
			Status:      "success",
			Success:     true,
		})
		if !errors.Is(err, ErrApplyOrchestratorInvalidState) {
			t.Fatalf("expected invalid state, got %v", err)
		}
	})

	t.Run("update not found maps to orchestrator not found", func(t *testing.T) {
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, &mockApplyRunRepo{findRun: baseRun, updateErr: repository.ErrNotFound})
		err := svc.ReportApplyResult(context.Background(), ReportApplyResultRequest{AgentHostID: 7, RunID: "run-2", Status: "applying"})
		if !errors.Is(err, ErrApplyOrchestratorNotFound) {
			t.Fatalf("expected orchestrator not found, got %v", err)
		}
	})

	t.Run("invalid status", func(t *testing.T) {
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, &mockApplyRunRepo{findRun: baseRun})
		err := svc.ReportApplyResult(context.Background(), ReportApplyResultRequest{AgentHostID: 7, RunID: "run-2", Status: "unknown"})
		if !errors.Is(err, ErrApplyOrchestratorInvalidRequest) {
			t.Fatalf("expected invalid request, got %v", err)
		}
	})

	t.Run("status and success inconsistent", func(t *testing.T) {
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, &mockApplyRunRepo{findRun: baseRun})
		err := svc.ReportApplyResult(context.Background(), ReportApplyResultRequest{
			AgentHostID: 7,
			RunID:       "run-2",
			Status:      "success",
			Success:     false,
		})
		if !errors.Is(err, ErrApplyOrchestratorInvalidRequest) {
			t.Fatalf("expected invalid request for inconsistent status/success, got %v", err)
		}
	})
}

func TestApplyOrchestratorService_CleanupExpiredApplyRuns(t *testing.T) {
	t.Run("expires stale runs with deadline and timeout message", func(t *testing.T) {
		repo := &mockApplyRunRepo{expireCount: 2}
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, repo)
		before := time.Now().Unix()
		n, err := svc.CleanupExpiredApplyRuns(context.Background(), 30*time.Minute)
		if err != nil {
			t.Fatalf("CleanupExpiredApplyRuns returned error: %v", err)
		}
		if n != 2 {
			t.Fatalf("expected 2 expired, got %d", n)
		}
		if repo.lastExpireDeadline <= before-1801 || repo.lastExpireDeadline > before {
			t.Fatalf("expected deadline ~now-30m, got %d (before=%d)", repo.lastExpireDeadline, before)
		}
		if repo.lastExpireMessage == "" || !strings.Contains(repo.lastExpireMessage, "timed out") {
			t.Fatalf("expected timeout message, got %q", repo.lastExpireMessage)
		}
	})

	t.Run("rejects non-positive max age", func(t *testing.T) {
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, &mockApplyRunRepo{})
		if _, err := svc.CleanupExpiredApplyRuns(context.Background(), 0); !errors.Is(err, ErrApplyOrchestratorInvalidRequest) {
			t.Fatalf("expected invalid request, got %v", err)
		}
		if _, err := svc.CleanupExpiredApplyRuns(context.Background(), -time.Minute); !errors.Is(err, ErrApplyOrchestratorInvalidRequest) {
			t.Fatalf("expected invalid request, got %v", err)
		}
	})

	t.Run("not configured", func(t *testing.T) {
		var svc *applyOrchestratorService
		if _, err := svc.CleanupExpiredApplyRuns(context.Background(), time.Minute); !errors.Is(err, ErrApplyOrchestratorNotConfigured) {
			t.Fatalf("expected not configured, got %v", err)
		}
	})
}

func TestApplyOrchestratorService_CancelApplyRun_Paths(t *testing.T) {
	pendingRun := &repository.ApplyRun{RunID: "run-cancel", AgentHostID: 7, CoreType: "sing-box", TargetRevision: 5, Status: applyRunStatusPending}

	t.Run("success cancels pending run", func(t *testing.T) {
		repo := &mockApplyRunRepo{findRun: pendingRun}
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, repo)
		if err := svc.CancelApplyRun(context.Background(), CancelApplyRunRequest{AgentHostID: 7, RunID: "run-cancel", OperatorID: 1}); err != nil {
			t.Fatalf("cancel: %v", err)
		}
		if len(repo.updateCalls) != 1 {
			t.Fatalf("expected one update call, got %d", len(repo.updateCalls))
		}
		if repo.updateCalls[0].runID != "run-cancel" || repo.updateCalls[0].status != applyRunStatusFailed {
			t.Fatalf("unexpected update: %+v", repo.updateCalls[0])
		}
	})

	t.Run("not configured", func(t *testing.T) {
		var svc *applyOrchestratorService
		if err := svc.CancelApplyRun(context.Background(), CancelApplyRunRequest{AgentHostID: 7, RunID: "r"}); !errors.Is(err, ErrApplyOrchestratorNotConfigured) {
			t.Fatalf("expected not configured, got %v", err)
		}
	})

	t.Run("invalid host", func(t *testing.T) {
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, &mockApplyRunRepo{})
		if err := svc.CancelApplyRun(context.Background(), CancelApplyRunRequest{AgentHostID: 0, RunID: "r"}); !errors.Is(err, ErrApplyOrchestratorInvalidRequest) {
			t.Fatalf("expected invalid request, got %v", err)
		}
	})

	t.Run("missing run id", func(t *testing.T) {
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, &mockApplyRunRepo{})
		if err := svc.CancelApplyRun(context.Background(), CancelApplyRunRequest{AgentHostID: 7, RunID: "  "}); !errors.Is(err, ErrApplyOrchestratorInvalidRequest) {
			t.Fatalf("expected invalid request, got %v", err)
		}
	})

	t.Run("run not found", func(t *testing.T) {
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, &mockApplyRunRepo{})
		if err := svc.CancelApplyRun(context.Background(), CancelApplyRunRequest{AgentHostID: 7, RunID: "nope"}); !errors.Is(err, ErrApplyOrchestratorNotFound) {
			t.Fatalf("expected not found, got %v", err)
		}
	})

	t.Run("permission denied", func(t *testing.T) {
		repo := &mockApplyRunRepo{findRun: pendingRun}
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, repo)
		if err := svc.CancelApplyRun(context.Background(), CancelApplyRunRequest{AgentHostID: 99, RunID: "run-cancel"}); !errors.Is(err, ErrApplyOrchestratorPermissionDenied) {
			t.Fatalf("expected permission denied, got %v", err)
		}
	})

	t.Run("terminal run cannot be cancelled", func(t *testing.T) {
		done := *pendingRun
		done.Status = applyRunStatusSuccess
		repo := &mockApplyRunRepo{findRun: &done}
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, repo)
		if err := svc.CancelApplyRun(context.Background(), CancelApplyRunRequest{AgentHostID: 7, RunID: "run-cancel"}); !errors.Is(err, ErrApplyOrchestratorInvalidState) {
			t.Fatalf("expected invalid state, got %v", err)
		}
	})

	t.Run("update failure propagated", func(t *testing.T) {
		targetErr := errors.New("update failed")
		repo := &mockApplyRunRepo{findRun: pendingRun, updateErr: targetErr}
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, repo)
		if err := svc.CancelApplyRun(context.Background(), CancelApplyRunRequest{AgentHostID: 7, RunID: "run-cancel"}); !errors.Is(err, targetErr) {
			t.Fatalf("expected propagated update error, got %v", err)
		}
	})
}

func TestApplyOrchestratorService_CreateApplyRun_Validation(t *testing.T) {
	t.Run("not configured", func(t *testing.T) {
		var svc *applyOrchestratorService
		if _, err := svc.CreateApplyRun(context.Background(), CreateApplyRunRequest{}); !errors.Is(err, ErrApplyOrchestratorNotConfigured) {
			t.Fatalf("expected not configured, got %v", err)
		}
	})
	t.Run("invalid host", func(t *testing.T) {
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, &mockApplyRunRepo{})
		if _, err := svc.CreateApplyRun(context.Background(), CreateApplyRunRequest{CoreType: "xray", TargetRevision: 1}); !errors.Is(err, ErrApplyOrchestratorInvalidRequest) {
			t.Fatalf("expected invalid request, got %v", err)
		}
	})
	t.Run("invalid revision", func(t *testing.T) {
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, &mockApplyRunRepo{})
		if _, err := svc.CreateApplyRun(context.Background(), CreateApplyRunRequest{AgentHostID: 1, CoreType: "xray"}); !errors.Is(err, ErrApplyOrchestratorInvalidRequest) {
			t.Fatalf("expected invalid request, got %v", err)
		}
	})
	t.Run("invalid core type", func(t *testing.T) {
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, &mockApplyRunRepo{})
		if _, err := svc.CreateApplyRun(context.Background(), CreateApplyRunRequest{AgentHostID: 1, CoreType: "bogus", TargetRevision: 1}); !errors.Is(err, ErrApplyOrchestratorInvalidRequest) {
			t.Fatalf("expected invalid request, got %v", err)
		}
	})
	t.Run("creates pending run", func(t *testing.T) {
		repo := &mockApplyRunRepo{}
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, repo)
		run, err := svc.CreateApplyRun(context.Background(), CreateApplyRunRequest{AgentHostID: 5, CoreType: "sing-box", TargetRevision: 9, OperatorID: 3})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if run.Status != applyRunStatusPending || run.AgentHostID != 5 || run.TargetRevision != 9 || run.OperatorID != 3 {
			t.Fatalf("unexpected run: %+v", run)
		}
		if len(repo.createdRuns) != 1 {
			t.Fatalf("expected one created run, got %d", len(repo.createdRuns))
		}
	})
}

func TestApplyOrchestratorService_PrepareApplyRun_Validation(t *testing.T) {
	t.Run("not configured", func(t *testing.T) {
		var svc *applyOrchestratorService
		if _, err := svc.PrepareApplyRun(context.Background(), PrepareApplyRunRequest{}); !errors.Is(err, ErrApplyOrchestratorNotConfigured) {
			t.Fatalf("expected not configured, got %v", err)
		}
	})
	t.Run("invalid host", func(t *testing.T) {
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, &mockApplyRunRepo{})
		if _, err := svc.PrepareApplyRun(context.Background(), PrepareApplyRunRequest{CoreType: "xray", TargetRevision: 1}); !errors.Is(err, ErrApplyOrchestratorInvalidRequest) {
			t.Fatalf("expected invalid request, got %v", err)
		}
	})
	t.Run("invalid revision", func(t *testing.T) {
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, &mockApplyRunRepo{})
		if _, err := svc.PrepareApplyRun(context.Background(), PrepareApplyRunRequest{AgentHostID: 1, CoreType: "xray"}); !errors.Is(err, ErrApplyOrchestratorInvalidRequest) {
			t.Fatalf("expected invalid request, got %v", err)
		}
	})
	t.Run("invalid core type", func(t *testing.T) {
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, &mockApplyRunRepo{})
		if _, err := svc.PrepareApplyRun(context.Background(), PrepareApplyRunRequest{AgentHostID: 1, CoreType: "bogus", TargetRevision: 1}); !errors.Is(err, ErrApplyOrchestratorInvalidRequest) {
			t.Fatalf("expected invalid request, got %v", err)
		}
	})
	t.Run("no artifacts", func(t *testing.T) {
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, &mockApplyRunRepo{})
		if _, err := svc.PrepareApplyRun(context.Background(), PrepareApplyRunRequest{AgentHostID: 1, CoreType: "xray", TargetRevision: 3}); !errors.Is(err, ErrApplyOrchestratorNoArtifacts) {
			t.Fatalf("expected no artifacts, got %v", err)
		}
	})
	t.Run("reuses open run", func(t *testing.T) {
		open := &repository.ApplyRun{RunID: "run-open", AgentHostID: 8, CoreType: "xray", TargetRevision: 4, Status: applyRunStatusPending}
		repo := &mockApplyRunRepo{listRuns: []*repository.ApplyRun{open}}
		svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{
			latestRevision: 4,
			listItems:      []*repository.DesiredArtifact{{Filename: "a.json", Content: []byte("{}")}},
		}, repo)
		run, err := svc.PrepareApplyRun(context.Background(), PrepareApplyRunRequest{AgentHostID: 8, CoreType: "xray", TargetRevision: 4})
		if err != nil {
			t.Fatalf("prepare: %v", err)
		}
		if run.RunID != "run-open" {
			t.Fatalf("expected reused run, got %q", run.RunID)
		}
		if len(repo.createdRuns) != 0 {
			t.Fatalf("expected no new run created, got %d", len(repo.createdRuns))
		}
	})
}

func TestApplyOrchestratorService_ReportApplyResult_StatusNormalization(t *testing.T) {
	// statusValue 为空时按 success 推断
	t.Run("empty status inferred from success", func(t *testing.T) {
		status, err := normalizeApplyRunStatus(true, "")
		if err != nil || status != applyRunStatusSuccess {
			t.Fatalf("expected success, got %q err=%v", status, err)
		}
		status, err = normalizeApplyRunStatus(false, "")
		if err != nil || status != applyRunStatusFailed {
			t.Fatalf("expected failed, got %q err=%v", status, err)
		}
	})
	t.Run("normalizes case and whitespace", func(t *testing.T) {
		status, err := normalizeApplyRunStatus(false, " Success ")
		if err != nil || status != applyRunStatusSuccess {
			t.Fatalf("expected success, got %q err=%v", status, err)
		}
	})
	t.Run("invalid status rejected", func(t *testing.T) {
		if _, err := normalizeApplyRunStatus(false, "exploded"); !errors.Is(err, ErrApplyOrchestratorInvalidRequest) {
			t.Fatalf("expected invalid request, got %v", err)
		}
	})
}

func TestApplyOrchestratorService_ReportApplyResult_TransitionMatrix(t *testing.T) {
	allowed := map[string][]string{
		applyRunStatusPending:  {applyRunStatusApplying, applyRunStatusSuccess, applyRunStatusFailed, applyRunStatusRolledBack},
		applyRunStatusApplying: {applyRunStatusSuccess, applyRunStatusFailed, applyRunStatusRolledBack},
		applyRunStatusFailed:   {applyRunStatusRolledBack},
	}
	for from, nexts := range allowed {
		for _, to := range nexts {
			if !isApplyRunTransitionAllowed(from, to) {
				t.Fatalf("expected %s -> %s allowed", from, to)
			}
		}
	}
	denied := [][2]string{
		{applyRunStatusPending, applyRunStatusPending},
		{applyRunStatusApplying, applyRunStatusApplying},
		{applyRunStatusSuccess, applyRunStatusFailed},
		{applyRunStatusSuccess, applyRunStatusSuccess},
		{applyRunStatusFailed, applyRunStatusFailed},
		{applyRunStatusRolledBack, applyRunStatusSuccess},
		{applyRunStatusRolledBack, applyRunStatusFailed},
		{applyRunStatusRolledBack, applyRunStatusRolledBack},
	}
	for _, pair := range denied {
		if isApplyRunTransitionAllowed(pair[0], pair[1]) {
			t.Fatalf("expected %s -> %s denied", pair[0], pair[1])
		}
	}
}

func TestApplyOrchestratorService_ReportApplyResult_SuccessAndRollbackFlow(t *testing.T) {
	// pending -> success（无 statusValue 推断）
	run := &repository.ApplyRun{RunID: "r1", AgentHostID: 7, CoreType: "xray", TargetRevision: 2, Status: applyRunStatusPending}
	repo := &mockApplyRunRepo{findRun: run}
	svc := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, repo)
	if err := svc.ReportApplyResult(context.Background(), ReportApplyResultRequest{
		AgentHostID: 7, RunID: "r1", CoreType: "xray", TargetRevision: 2, Success: true,
	}); err != nil {
		t.Fatalf("report success: %v", err)
	}
	if len(repo.updateCalls) != 1 || repo.updateCalls[0].status != applyRunStatusSuccess {
		t.Fatalf("expected success update, got %+v", repo.updateCalls)
	}

	// failed -> rolled_back（agent 侧始终传 success=false + status）
	failedRun := &repository.ApplyRun{RunID: "r2", AgentHostID: 7, CoreType: "xray", TargetRevision: 2, Status: applyRunStatusFailed}
	repo2 := &mockApplyRunRepo{findRun: failedRun}
	svc2 := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, repo2)
	if err := svc2.ReportApplyResult(context.Background(), ReportApplyResultRequest{
		AgentHostID: 7, RunID: "r2", CoreType: "xray", TargetRevision: 2,
		Status: "rolled_back", Success: false,
	}); err != nil {
		t.Fatalf("report rolled_back: %v", err)
	}
	if len(repo2.updateCalls) != 1 || repo2.updateCalls[0].status != applyRunStatusRolledBack {
		t.Fatalf("expected rolled_back update, got %+v", repo2.updateCalls)
	}

	// success 推断（statusValue 为空 + success=true）
	run3 := &repository.ApplyRun{RunID: "r3", AgentHostID: 7, CoreType: "xray", TargetRevision: 2, Status: applyRunStatusPending}
	repo3 := &mockApplyRunRepo{findRun: run3}
	svc3 := NewApplyOrchestratorService(&mockApplyDesiredArtifactRepo{}, repo3)
	if err := svc3.ReportApplyResult(context.Background(), ReportApplyResultRequest{
		AgentHostID: 7, RunID: "r3", CoreType: "xray", TargetRevision: 2, Success: true,
	}); err != nil {
		t.Fatalf("report inferred success: %v", err)
	}
	if len(repo3.updateCalls) != 1 || repo3.updateCalls[0].status != applyRunStatusSuccess {
		t.Fatalf("expected inferred success update, got %+v", repo3.updateCalls)
	}
}
