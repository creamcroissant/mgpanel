package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/creamcroissant/mgpanel/internal/service"
	"github.com/creamcroissant/mgpanel/internal/support/i18n"
)

// AdminNodeStatHandler handles admin node statistics endpoints.
type AdminNodeStatHandler struct {
	svc  service.AdminNodeStatService
	i18n *i18n.Manager
}

// NewAdminNodeStatHandler creates a new admin node stat handler.
func NewAdminNodeStatHandler(svc service.AdminNodeStatService, i18nMgr *i18n.Manager) *AdminNodeStatHandler {
	return &AdminNodeStatHandler{svc: svc, i18n: i18nMgr}
}

// GetServerStats returns historical stats for a single server.
// GET /admin/nodes/stat/fetch?server_id=1&record_type=1&days=30
func (h *AdminNodeStatHandler) GetServerStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	serverID, err := strconv.ParseInt(r.URL.Query().Get("server_id"), 10, 64)
	if err != nil || serverID <= 0 {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "admin.node_stat.fetch", "error.bad_request", h.i18n)
		return
	}

	recordType := 1 // Default: daily
	if rt := r.URL.Query().Get("record_type"); rt != "" {
		if parsed, err := strconv.Atoi(rt); err == nil {
			recordType = parsed
		}
	}

	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 {
			days = parsed
		}
	}

	if h.svc == nil {
		RespondErrorI18nAction(ctx, w, http.StatusServiceUnavailable, "admin.node_stat.fetch", "error.service_unavailable", h.i18n)
		return
	}
	records, err := h.svc.GetServerStats(ctx, serverID, recordType, days)
	if err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "admin.node_stat.fetch", "error.internal_server_error", h.i18n)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"data": records,
	})
}

// GetTotalTraffic returns total traffic across all servers.
// GET /admin/nodes/stat/traffic?record_type=1&start_at=xxx&end_at=xxx
func (h *AdminNodeStatHandler) GetTotalTraffic(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	recordType := 1 // Default: daily
	if rt := r.URL.Query().Get("record_type"); rt != "" {
		if parsed, err := strconv.Atoi(rt); err == nil {
			recordType = parsed
		}
	}

	now := time.Now()
	startAt := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Unix() // First day of month
	endAt := now.Unix()

	if sa := r.URL.Query().Get("start_at"); sa != "" {
		if parsed, err := strconv.ParseInt(sa, 10, 64); err == nil {
			startAt = parsed
		}
	}
	if ea := r.URL.Query().Get("end_at"); ea != "" {
		if parsed, err := strconv.ParseInt(ea, 10, 64); err == nil {
			endAt = parsed
		}
	}

	if h.svc == nil {
		RespondErrorI18nAction(ctx, w, http.StatusServiceUnavailable, "admin.node_stat.traffic", "error.service_unavailable", h.i18n)
		return
	}
	result, err := h.svc.GetTotalTraffic(ctx, recordType, startAt, endAt)
	if err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "admin.node_stat.traffic", "error.internal_server_error", h.i18n)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"upload":   result.Upload,
			"download": result.Download,
			"total":    result.Upload + result.Download,
		},
	})
}

// GetTopServers returns servers ranked by traffic.
// GET /admin/nodes/stat/rank?record_type=1&start_at=xxx&end_at=xxx&limit=10
func (h *AdminNodeStatHandler) GetTopServers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	recordType := 1
	if rt := r.URL.Query().Get("record_type"); rt != "" {
		if parsed, err := strconv.Atoi(rt); err == nil {
			recordType = parsed
		}
	}

	now := time.Now()
	startAt := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Unix()
	endAt := now.Unix()

	if sa := r.URL.Query().Get("start_at"); sa != "" {
		if parsed, err := strconv.ParseInt(sa, 10, 64); err == nil {
			startAt = parsed
		}
	}
	if ea := r.URL.Query().Get("end_at"); ea != "" {
		if parsed, err := strconv.ParseInt(ea, 10, 64); err == nil {
			endAt = parsed
		}
	}

	limit := 10
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	if h.svc == nil {
		RespondErrorI18nAction(ctx, w, http.StatusServiceUnavailable, "admin.node_stat.rank", "error.service_unavailable", h.i18n)
		return
	}
	result, err := h.svc.GetTopServers(ctx, recordType, startAt, endAt, limit)
	if err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "admin.node_stat.rank", "error.internal_server_error", h.i18n)
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"data": result,
	})
}
