package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/creamcroissant/mgpanel/internal/api/requestctx"
	"github.com/creamcroissant/mgpanel/internal/repository"
	"github.com/creamcroissant/mgpanel/internal/service"
)

type driftAndDiffServiceStub struct {
	listReq    service.ListDesiredArtifactsRequest
	listResult *service.ListDesiredArtifactsResult
	listErr    error

	textReq    service.GetTextDiffRequest
	textResult *service.TextDiffResult
	textErr    error

	semanticReq    service.GetSemanticDiffRequest
	semanticResult *service.SemanticDiffResult
	semanticErr    error
}

func (s *driftAndDiffServiceStub) EvaluateDrift(ctx context.Context, req service.EvaluateDriftRequest) (*service.EvaluateDriftResult, error) {
	return nil, errors.New("not implemented")
}

func (s *driftAndDiffServiceStub) ListAppliedSnapshot(ctx context.Context, req service.ListAppliedSnapshotRequest) (*service.ListAppliedSnapshotResult, error) {
	return nil, errors.New("not implemented")
}

func (s *driftAndDiffServiceStub) ListDriftStates(ctx context.Context, req service.ListDriftStatesRequest) (*service.ListDriftStatesResult, error) {
	return nil, errors.New("not implemented")
}

func (s *driftAndDiffServiceStub) ListArtifacts(ctx context.Context, req service.ListDesiredArtifactsRequest) (*service.ListDesiredArtifactsResult, error) {
	s.listReq = req
	if s.listErr != nil {
		return nil, s.listErr
	}
	if s.listResult == nil {
		return &service.ListDesiredArtifactsResult{DesiredRevision: req.DesiredRevision}, nil
	}
	return s.listResult, nil
}

func (s *driftAndDiffServiceStub) GetTextDiff(ctx context.Context, req service.GetTextDiffRequest) (*service.TextDiffResult, error) {
	s.textReq = req
	if s.textErr != nil {
		return nil, s.textErr
	}
	if s.textResult == nil {
		return &service.TextDiffResult{}, nil
	}
	return s.textResult, nil
}

func (s *driftAndDiffServiceStub) GetSemanticDiff(ctx context.Context, req service.GetSemanticDiffRequest) (*service.SemanticDiffResult, error) {
	s.semanticReq = req
	if s.semanticErr != nil {
		return nil, s.semanticErr
	}
	if s.semanticResult == nil {
		return &service.SemanticDiffResult{}, nil
	}
	return s.semanticResult, nil
}

func newAdminDiffRequest(method, target string, withAdmin bool) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	if withAdmin {
		req = req.WithContext(requestctx.WithAdminClaims(req.Context(), requestctx.AdminClaims{ID: "12"}))
	}
	return req
}

func TestAdminConfigCenterDiffHandlerListArtifactsSuccess(t *testing.T) {
	svc := &driftAndDiffServiceStub{
		listResult: &service.ListDesiredArtifactsResult{
			DesiredRevision: 9,
			Total:           1,
			Items: []*repository.DesiredArtifact{
				{
					ID:              7,
					AgentHostID:     11,
					CoreType:        "xray",
					DesiredRevision: 9,
					Filename:        "managed-a.json",
					SourceTag:       "in-a",
					Content:         []byte(`{"inbounds":[]}`),
					ContentHash:     "h1",
					GeneratedAt:     123,
				},
			},
		},
	}
	h := NewAdminConfigCenterDiffHandler(svc, nil)
	req := newAdminDiffRequest(http.MethodGet, "/api/v2/secure/config-center/artifacts?agent_host_id=11&core_type=xray&desired_revision=9&tag=in-a&filename=managed-a.json&limit=15&offset=4&include_content=true", true)
	resp := httptest.NewRecorder()

	h.ListArtifacts(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	if svc.listReq.AgentHostID != 11 || svc.listReq.CoreType != "xray" || svc.listReq.DesiredRevision != 9 {
		t.Fatalf("unexpected list base query: %+v", svc.listReq)
	}
	if svc.listReq.Tag != "in-a" || svc.listReq.Filename != "managed-a.json" {
		t.Fatalf("unexpected filters: %+v", svc.listReq)
	}
	if svc.listReq.Limit != 15 || svc.listReq.Offset != 4 {
		t.Fatalf("unexpected paging: %+v", svc.listReq)
	}
	if body := resp.Body.String(); !strings.Contains(body, `"content":"{\"inbounds\":[]}"`) {
		t.Fatalf("expected response include content, body=%s", body)
	}
}

func TestAdminConfigCenterDiffHandlerListArtifactsWithoutContent(t *testing.T) {
	svc := &driftAndDiffServiceStub{
		listResult: &service.ListDesiredArtifactsResult{
			DesiredRevision: 9,
			Total:           1,
			Items: []*repository.DesiredArtifact{{
				ID:              8,
				AgentHostID:     11,
				CoreType:        "xray",
				DesiredRevision: 9,
				Filename:        "managed-b.json",
				SourceTag:       "in-b",
				Content:         []byte(`{"inbounds":[1]}`),
				ContentHash:     "h2",
				GeneratedAt:     124,
			}},
		},
	}
	h := NewAdminConfigCenterDiffHandler(svc, nil)
	req := newAdminDiffRequest(http.MethodGet, "/api/v2/secure/config-center/artifacts?agent_host_id=11&core_type=xray&desired_revision=9", true)
	resp := httptest.NewRecorder()

	h.ListArtifacts(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	if body := resp.Body.String(); strings.Contains(body, `"content":`) {
		t.Fatalf("expected response hide content by default, body=%s", body)
	}
}

func TestAdminConfigCenterDiffHandlerGetTextDiffSuccess(t *testing.T) {
	svc := &driftAndDiffServiceStub{
		textResult: &service.TextDiffResult{
			DesiredRevision: 9,
			Filename:        "managed-a.json",
			Tag:             "in-a",
			DesiredText:     "desired\n",
			AppliedText:     "applied\n",
			UnifiedDiff:     "@@\n",
			Different:       true,
		},
	}
	h := NewAdminConfigCenterDiffHandler(svc, nil)
	req := newAdminDiffRequest(http.MethodGet, "/api/v2/secure/config-center/diff/text?agent_host_id=11&core_type=xray&desired_revision=9&tag=in-a", true)
	resp := httptest.NewRecorder()

	h.GetTextDiff(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	if svc.textReq.AgentHostID != 11 || svc.textReq.CoreType != "xray" || svc.textReq.DesiredRevision != 9 || svc.textReq.Tag != "in-a" {
		t.Fatalf("unexpected text req: %+v", svc.textReq)
	}
}

func TestAdminConfigCenterDiffHandlerGetSemanticDiffSuccess(t *testing.T) {
	svc := &driftAndDiffServiceStub{
		semanticResult: &service.SemanticDiffResult{
			DesiredRevision: 9,
			Items: []service.SemanticDiffItem{{
				Tag:             "in-a",
				DesiredFilename: "a.json",
				AppliedFilename: "a.json",
				DriftType:       "hash_mismatch",
			}},
		},
	}
	h := NewAdminConfigCenterDiffHandler(svc, nil)
	req := newAdminDiffRequest(http.MethodGet, "/api/v2/secure/config-center/diff/semantic?agent_host_id=11&core_type=xray&desired_revision=9&tag=in-a", true)
	resp := httptest.NewRecorder()

	h.GetSemanticDiff(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	if svc.semanticReq.AgentHostID != 11 || svc.semanticReq.CoreType != "xray" || svc.semanticReq.DesiredRevision != 9 || svc.semanticReq.Tag != "in-a" {
		t.Fatalf("unexpected semantic req: %+v", svc.semanticReq)
	}
}

func TestAdminConfigCenterDiffHandlerRequiresAuth(t *testing.T) {
	h := NewAdminConfigCenterDiffHandler(&driftAndDiffServiceStub{}, nil)
	req := newAdminDiffRequest(http.MethodGet, "/api/v2/secure/config-center/artifacts?agent_host_id=11&core_type=xray", false)
	resp := httptest.NewRecorder()

	h.ListArtifacts(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
}

func TestAdminConfigCenterDiffHandlerServiceUnavailable(t *testing.T) {
	h := NewAdminConfigCenterDiffHandler(nil, nil)
	req := newAdminDiffRequest(http.MethodGet, "/api/v2/secure/config-center/artifacts?agent_host_id=11&core_type=xray", true)
	resp := httptest.NewRecorder()

	h.ListArtifacts(resp, req)

	if resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp.Code)
	}
}

func TestAdminConfigCenterDiffHandlerBadRequest(t *testing.T) {
	h := NewAdminConfigCenterDiffHandler(&driftAndDiffServiceStub{}, nil)
	cases := []string{
		"/api/v2/secure/config-center/artifacts?agent_host_id=bad&core_type=xray",
		"/api/v2/secure/config-center/artifacts?agent_host_id=1",
		"/api/v2/secure/config-center/artifacts?agent_host_id=1&core_type=xray&desired_revision=0",
		"/api/v2/secure/config-center/artifacts?agent_host_id=1&core_type=xray&include_content=not-bool",
		"/api/v2/secure/config-center/diff/text?agent_host_id=1&core_type=xray&desired_revision=-1",
	}
	for _, target := range cases {
		req := newAdminDiffRequest(http.MethodGet, target, true)
		resp := httptest.NewRecorder()
		if strings.Contains(target, "/diff/text") {
			h.GetTextDiff(resp, req)
		} else {
			h.ListArtifacts(resp, req)
		}
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for %s, got %d", target, resp.Code)
		}
	}
}

func TestAdminConfigCenterDiffHandlerErrorMapping(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		expected int
		invoke   func(*AdminConfigCenterDiffHandler, http.ResponseWriter, *http.Request)
		target   string
	}{
		{
			name:     "invalid request",
			err:      service.ErrDriftAndDiffInvalidRequest,
			expected: http.StatusBadRequest,
			invoke:   (*AdminConfigCenterDiffHandler).GetTextDiff,
			target:   "/api/v2/secure/config-center/diff/text?agent_host_id=1&core_type=xray&tag=in-a",
		},
		{
			name:     "desired missing",
			err:      service.ErrDriftAndDiffDesiredMissing,
			expected: http.StatusOK,
			invoke:   (*AdminConfigCenterDiffHandler).GetSemanticDiff,
			target:   "/api/v2/secure/config-center/diff/semantic?agent_host_id=1&core_type=xray&tag=in-a",
		},
		{
			name:     "not configured",
			err:      service.ErrDriftAndDiffNotConfigured,
			expected: http.StatusServiceUnavailable,
			invoke:   (*AdminConfigCenterDiffHandler).ListArtifacts,
			target:   "/api/v2/secure/config-center/artifacts?agent_host_id=1&core_type=xray",
		},
		{
			name:     "internal",
			err:      errors.New("boom"),
			expected: http.StatusInternalServerError,
			invoke:   (*AdminConfigCenterDiffHandler).ListArtifacts,
			target:   "/api/v2/secure/config-center/artifacts?agent_host_id=1&core_type=xray",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &driftAndDiffServiceStub{
				listErr:     tc.err,
				textErr:     tc.err,
				semanticErr: tc.err,
			}
			h := NewAdminConfigCenterDiffHandler(svc, nil)
			req := newAdminDiffRequest(http.MethodGet, tc.target, true)
			resp := httptest.NewRecorder()

			tc.invoke(h, resp, req)

			if resp.Code != tc.expected {
				t.Fatalf("expected %d, got %d", tc.expected, resp.Code)
			}
		})
	}
}
