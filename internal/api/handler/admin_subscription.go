package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/api/requestctx"
	"github.com/creamcroissant/mgpanel/internal/service"
	"github.com/creamcroissant/mgpanel/internal/support/i18n"
	"github.com/go-chi/chi/v5"
)

type AdminSubscriptionHandler struct {
	filters service.SubscriptionFilterService
	sources service.SubscriptionSourceService
	i18n    *i18n.Manager
}

func NewAdminSubscriptionHandler(filters service.SubscriptionFilterService, sources service.SubscriptionSourceService, i18nMgr *i18n.Manager) *AdminSubscriptionHandler {
	return &AdminSubscriptionHandler{filters: filters, sources: sources, i18n: i18nMgr}
}

func (h *AdminSubscriptionHandler) ListSources(w http.ResponseWriter, r *http.Request) {
	action := "admin.subscription_source.list"
	if !h.requireAdmin(w, r, action) || !h.ensureSourceService(w, r, action) {
		return
	}
	req, ok := buildListSubscriptionSourcesRequest(w, r, action, h.i18n)
	if !ok {
		return
	}
	result, err := h.sources.List(r.Context(), req)
	if err != nil {
		h.respondServiceError(w, r, action, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": result})
}

func (h *AdminSubscriptionHandler) CreateSource(w http.ResponseWriter, r *http.Request) {
	action := "admin.subscription_source.create"
	if !h.requireAdmin(w, r, action) || !h.ensureSourceService(w, r, action) {
		return
	}
	var payload service.UpsertSubscriptionSourceRequest
	if err := decodeJSON(r, &payload); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, action, "error.bad_request", h.i18n)
		return
	}
	source, err := h.sources.Create(r.Context(), payload)
	if err != nil {
		h.respondServiceError(w, r, action, err)
		return
	}
	respondJSON(w, http.StatusCreated, map[string]any{"data": source})
}

func (h *AdminSubscriptionHandler) GetSource(w http.ResponseWriter, r *http.Request) {
	action := "admin.subscription_source.get"
	if !h.requireAdmin(w, r, action) || !h.ensureSourceService(w, r, action) {
		return
	}
	id, ok := h.parseSourceID(w, r, action)
	if !ok {
		return
	}
	source, err := h.sources.Get(r.Context(), id)
	if err != nil {
		h.respondServiceError(w, r, action, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": source})
}

func (h *AdminSubscriptionHandler) UpdateSource(w http.ResponseWriter, r *http.Request) {
	action := "admin.subscription_source.update"
	if !h.requireAdmin(w, r, action) || !h.ensureSourceService(w, r, action) {
		return
	}
	id, ok := h.parseSourceID(w, r, action)
	if !ok {
		return
	}
	var payload service.UpsertSubscriptionSourceRequest
	if err := decodeJSON(r, &payload); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, action, "error.bad_request", h.i18n)
		return
	}
	source, err := h.sources.Update(r.Context(), id, payload)
	if err != nil {
		h.respondServiceError(w, r, action, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": source})
}

func (h *AdminSubscriptionHandler) DeleteSource(w http.ResponseWriter, r *http.Request) {
	action := "admin.subscription_source.delete"
	if !h.requireAdmin(w, r, action) || !h.ensureSourceService(w, r, action) {
		return
	}
	id, ok := h.parseSourceID(w, r, action)
	if !ok {
		return
	}
	if err := h.sources.Delete(r.Context(), id); err != nil {
		h.respondServiceError(w, r, action, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": map[string]bool{"deleted": true}})
}

func (h *AdminSubscriptionHandler) SyncSource(w http.ResponseWriter, r *http.Request) {
	action := "admin.subscription_source.sync"
	if !h.requireAdmin(w, r, action) || !h.ensureSourceService(w, r, action) {
		return
	}
	id, ok := h.parseSourceID(w, r, action)
	if !ok {
		return
	}
	result, err := h.sources.SyncImported(r.Context(), id)
	if err != nil {
		h.respondServiceError(w, r, action, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": result})
}

func (h *AdminSubscriptionHandler) ListFilterReasons(w http.ResponseWriter, r *http.Request) {
	action := "admin.subscription_filter.list_reasons"
	if !h.requireAdmin(w, r, action) || !h.ensureFilterService(w, r, action) {
		return
	}
	req, ok := buildListSubscriptionFilterReasonsRequest(w, r, action, h.i18n)
	if !ok {
		return
	}
	result, err := h.filters.ListFilterReasons(r.Context(), req)
	if err != nil {
		h.respondServiceError(w, r, action, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": result})
}

func (h *AdminSubscriptionHandler) GetFilterSummary(w http.ResponseWriter, r *http.Request) {
	action := "admin.subscription_filter.summary"
	if !h.requireAdmin(w, r, action) || !h.ensureFilterService(w, r, action) {
		return
	}
	q := r.URL.Query()
	result, err := h.filters.GetFilterSummary(r.Context(), service.SubscriptionFilterSummaryRequest{
		Types:  q.Get("types"),
		Filter: q.Get("filter"),
		Tags:   q.Get("tags"),
	})
	if err != nil {
		h.respondServiceError(w, r, action, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": result})
}

func (h *AdminSubscriptionHandler) requireAdmin(w http.ResponseWriter, r *http.Request, action string) bool {
	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusUnauthorized, action, "error.unauthorized", h.i18n)
		return false
	}
	return true
}

func (h *AdminSubscriptionHandler) ensureFilterService(w http.ResponseWriter, r *http.Request, action string) bool {
	if h.filters != nil {
		return true
	}
	RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, action, "error.service_unavailable", h.i18n)
	return false
}

func (h *AdminSubscriptionHandler) ensureSourceService(w http.ResponseWriter, r *http.Request, action string) bool {
	if h.sources != nil {
		return true
	}
	RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, action, "error.service_unavailable", h.i18n)
	return false
}

func (h *AdminSubscriptionHandler) parseSourceID(w http.ResponseWriter, r *http.Request, action string) (int64, bool) {
	id, err := parseInt64(chi.URLParam(r, "id"))
	if err != nil || id <= 0 {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, action, "error.bad_request", h.i18n)
		return 0, false
	}
	return id, true
}

func (h *AdminSubscriptionHandler) respondServiceError(w http.ResponseWriter, r *http.Request, action string, err error) {
	switch {
	case errors.Is(err, service.ErrBadRequest):
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, action, "error.bad_request", h.i18n)
	case errors.Is(err, service.ErrNotImplemented):
		RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, action, "error.service_unavailable", h.i18n)
	case errors.Is(err, service.ErrNotFound):
		RespondErrorI18nAction(r.Context(), w, http.StatusNotFound, action, "error.not_found", h.i18n)
	default:
		RespondErrorI18nAction(r.Context(), w, http.StatusInternalServerError, action, "error.internal_server_error", h.i18n)
	}
}

func buildListSubscriptionSourcesRequest(w http.ResponseWriter, r *http.Request, action string, i18nMgr *i18n.Manager) (service.ListSubscriptionSourcesRequest, bool) {
	q := r.URL.Query()
	enabled, ok := optionalQueryBool(w, r, action, "enabled", i18nMgr)
	if !ok {
		return service.ListSubscriptionSourcesRequest{}, false
	}
	return service.ListSubscriptionSourcesRequest{
		Type:    q.Get("type"),
		Enabled: enabled,
		Keyword: q.Get("keyword"),
		Limit:   clampQueryInt(q.Get("limit"), 100),
		Offset:  clampNonNegativeQueryInt(q.Get("offset"), 0),
	}, true
}

func buildListSubscriptionFilterReasonsRequest(w http.ResponseWriter, r *http.Request, action string, i18nMgr *i18n.Manager) (service.ListSubscriptionFilterReasonsRequest, bool) {
	q := r.URL.Query()
	sourceID, ok := optionalQueryInt64(w, r, action, "source_id", i18nMgr)
	if !ok {
		return service.ListSubscriptionFilterReasonsRequest{}, false
	}
	serverID, ok := optionalQueryInt64(w, r, action, "server_id", i18nMgr)
	if !ok {
		return service.ListSubscriptionFilterReasonsRequest{}, false
	}
	createdAfter, ok := optionalQueryInt64(w, r, action, "created_after", i18nMgr)
	if !ok {
		return service.ListSubscriptionFilterReasonsRequest{}, false
	}
	createdBefore, ok := optionalQueryInt64(w, r, action, "created_before", i18nMgr)
	if !ok {
		return service.ListSubscriptionFilterReasonsRequest{}, false
	}
	return service.ListSubscriptionFilterReasonsRequest{
		SourceType:    q.Get("source_type"),
		SourceID:      sourceID,
		ServerID:      serverID,
		Reason:        q.Get("reason"),
		CreatedAfter:  createdAfter,
		CreatedBefore: createdBefore,
		Limit:         clampQueryInt(q.Get("limit"), 100),
		Offset:        clampNonNegativeQueryInt(q.Get("offset"), 0),
		Types:         q.Get("types"),
		Filter:        q.Get("filter"),
		Tags:          q.Get("tags"),
	}, true
}

func optionalQueryBool(w http.ResponseWriter, r *http.Request, action string, name string, i18nMgr *i18n.Manager) (*bool, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return nil, true
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, action, "error.bad_request", i18nMgr)
		return nil, false
	}
	return &value, true
}

func optionalQueryInt64(w http.ResponseWriter, r *http.Request, action string, name string, i18nMgr *i18n.Manager) (*int64, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return nil, true
	}
	value, err := parseInt64(raw)
	if err != nil || value < 0 {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, action, "error.bad_request", i18nMgr)
		return nil, false
	}
	return &value, true
}
