package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/creamcroissant/xboard/internal/api/requestctx"
	"github.com/creamcroissant/xboard/internal/service"
	"github.com/creamcroissant/xboard/internal/support/i18n"
)

// AdminConfigCenterDiffHandler exposes admin endpoints for desired artifact preview and diff retrieval.
type AdminConfigCenterDiffHandler struct {
	diff service.DriftAndDiffService
	i18n *i18n.Manager
}

// NewAdminConfigCenterDiffHandler creates a config-center diff handler.
func NewAdminConfigCenterDiffHandler(diff service.DriftAndDiffService, i18nMgr *i18n.Manager) *AdminConfigCenterDiffHandler {
	return &AdminConfigCenterDiffHandler{diff: diff, i18n: i18nMgr}
}

type configCenterDiffBaseQuery struct {
	agentHostID     int64
	coreType        string
	desiredRevision int64
}

func (h *AdminConfigCenterDiffHandler) requireAdmin(w http.ResponseWriter, r *http.Request) (int64, bool) {
	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusUnauthorized, "admin.config_center.diff.auth", "error.unauthorized", h.i18n)
		return 0, false
	}
	adminID, err := parseInt64(claims.ID)
	if err != nil {
		adminID = 0
	}
	return adminID, true
}

func (h *AdminConfigCenterDiffHandler) ensureService(w http.ResponseWriter, r *http.Request, action string) bool {
	if h.diff != nil {
		return true
	}
	RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, action, "error.service_unavailable", h.i18n)
	return false
}

func (h *AdminConfigCenterDiffHandler) parseBaseQuery(w http.ResponseWriter, r *http.Request, action string) (configCenterDiffBaseQuery, bool) {
	query := r.URL.Query()
	base := configCenterDiffBaseQuery{}

	agentHostIDRaw := strings.TrimSpace(query.Get("agent_host_id"))
	agentHostID, err := parseInt64(agentHostIDRaw)
	if err != nil || agentHostID <= 0 {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, action, "error.bad_request", h.i18n)
		return base, false
	}
	coreType := strings.TrimSpace(query.Get("core_type"))
	if coreType == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, action, "error.bad_request", h.i18n)
		return base, false
	}

	base.agentHostID = agentHostID
	base.coreType = coreType

	if desiredRevisionRaw := strings.TrimSpace(query.Get("desired_revision")); desiredRevisionRaw != "" {
		desiredRevision, parseErr := parseInt64(desiredRevisionRaw)
		if parseErr != nil || desiredRevision <= 0 {
			RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, action, "error.bad_request", h.i18n)
			return base, false
		}
		base.desiredRevision = desiredRevision
	}

	return base, true
}

// ListArtifacts handles GET /api/v2/{securePath}/config-center/artifacts.
func (h *AdminConfigCenterDiffHandler) ListArtifacts(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	if !h.ensureService(w, r, "admin.config_center.diff.artifacts") {
		return
	}

	base, ok := h.parseBaseQuery(w, r, "admin.config_center.diff.artifacts")
	if !ok {
		return
	}
	query := r.URL.Query()
	includeContent := false
	if includeContentRaw := strings.TrimSpace(query.Get("include_content")); includeContentRaw != "" {
		parsedIncludeContent, err := strconv.ParseBool(includeContentRaw)
		if err != nil {
			RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.config_center.diff.artifacts", "error.bad_request", h.i18n)
			return
		}
		includeContent = parsedIncludeContent
	}

	result, err := h.diff.ListArtifacts(r.Context(), service.ListDesiredArtifactsRequest{
		AgentHostID:     base.agentHostID,
		CoreType:        base.coreType,
		DesiredRevision: base.desiredRevision,
		Tag:             strings.TrimSpace(query.Get("tag")),
		Filename:        strings.TrimSpace(query.Get("filename")),
		IncludeContent:  includeContent,
		Limit:           clampQueryInt(query.Get("limit"), 20),
		Offset:          clampNonNegativeQueryInt(query.Get("offset"), 0),
	})
	if err != nil {
		h.respondDiffError(r.Context(), w, "admin.config_center.diff.artifacts", err)
		return
	}

	items := make([]map[string]any, 0, len(result.Items))
	for _, item := range result.Items {
		if item == nil {
			continue
		}
		row := map[string]any{
			"id":               item.ID,
			"agent_host_id":    item.AgentHostID,
			"core_type":        item.CoreType,
			"desired_revision": item.DesiredRevision,
			"filename":         item.Filename,
			"source_tag":       item.SourceTag,
			"content_hash":     item.ContentHash,
			"generated_at":     item.GeneratedAt,
		}
		if includeContent {
			row["content"] = string(item.Content)
		}
		items = append(items, row)
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"desired_revision": result.DesiredRevision,
		"total":            result.Total,
		"data":             items,
	})
}

// GetTextDiff handles GET /api/v2/{securePath}/config-center/diff/text.
func (h *AdminConfigCenterDiffHandler) GetTextDiff(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	if !h.ensureService(w, r, "admin.config_center.diff.text") {
		return
	}

	base, ok := h.parseBaseQuery(w, r, "admin.config_center.diff.text")
	if !ok {
		return
	}
	query := r.URL.Query()
	result, err := h.diff.GetTextDiff(r.Context(), service.GetTextDiffRequest{
		AgentHostID:     base.agentHostID,
		CoreType:        base.coreType,
		DesiredRevision: base.desiredRevision,
		Filename:        strings.TrimSpace(query.Get("filename")),
		Tag:             strings.TrimSpace(query.Get("tag")),
	})
	if err != nil {
		h.respondDiffError(r.Context(), w, "admin.config_center.diff.text", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"data": result,
	})
}

// GetSemanticDiff handles GET /api/v2/{securePath}/config-center/diff/semantic.
func (h *AdminConfigCenterDiffHandler) GetSemanticDiff(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	if !h.ensureService(w, r, "admin.config_center.diff.semantic") {
		return
	}

	base, ok := h.parseBaseQuery(w, r, "admin.config_center.diff.semantic")
	if !ok {
		return
	}
	query := r.URL.Query()
	result, err := h.diff.GetSemanticDiff(r.Context(), service.GetSemanticDiffRequest{
		AgentHostID:     base.agentHostID,
		CoreType:        base.coreType,
		DesiredRevision: base.desiredRevision,
		Tag:             strings.TrimSpace(query.Get("tag")),
	})
	if err != nil {
		if errors.Is(err, service.ErrDriftAndDiffDesiredMissing) && base.desiredRevision == 0 {
			result = &service.SemanticDiffResult{Items: []service.SemanticDiffItem{}}
		} else {
			h.respondDiffError(r.Context(), w, "admin.config_center.diff.semantic", err)
			return
		}
	}
	if result == nil {
		result = &service.SemanticDiffResult{}
	}
	if result.Items == nil {
		result.Items = []service.SemanticDiffItem{}
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"data": result,
	})
}

func (h *AdminConfigCenterDiffHandler) respondDiffError(ctx context.Context, w http.ResponseWriter, action string, err error) {
	status := http.StatusInternalServerError
	key := "error.internal_server_error"

	if errors.Is(err, service.ErrDriftAndDiffNotConfigured) {
		status = http.StatusServiceUnavailable
		key = "error.service_unavailable"
	} else if errors.Is(err, service.ErrDriftAndDiffInvalidRequest) {
		status = http.StatusBadRequest
		key = "error.bad_request"
	} else if errors.Is(err, service.ErrDriftAndDiffDesiredMissing) || errors.Is(err, service.ErrDriftAndDiffTagMissing) {
		status = http.StatusNotFound
		key = "error.not_found"
	}

	RespondErrorI18nAction(ctx, w, status, action, key, h.i18n)
}
