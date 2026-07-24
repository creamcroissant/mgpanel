package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/creamcroissant/xboard/internal/api/requestctx"
	"github.com/creamcroissant/xboard/internal/service"
	"github.com/creamcroissant/xboard/internal/support/i18n"
)

type AdminCDNReportHandler struct {
	cdn  service.CDNService
	i18n *i18n.Manager
}

func NewAdminCDNReportHandler(cdn service.CDNService, i18nMgr *i18n.Manager) *AdminCDNReportHandler {
	return &AdminCDNReportHandler{cdn: cdn, i18n: i18nMgr}
}

// POST /api/v2/admin/cdn/origin-latency

func (h *AdminCDNReportHandler) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return false
	}
	return true
}

func (h *AdminCDNReportHandler) ReportOriginLatency(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	var req struct {
		SiteID    int64  `json:"site_id"`
		Stack     string `json:"stack"`
		Domain    string `json:"domain"`
		LatencyMs int    `json:"latency_ms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, 400, map[string]string{"error": "invalid body"})
		return
	}
	if req.SiteID <= 0 || (req.Stack != "v4" && req.Stack != "v6") {
		respondJSON(w, 400, map[string]string{"error": "invalid params"})
		return
	}
	if err := h.cdn.ReportOriginLatency(r.Context(), req.SiteID, req.Stack, req.Domain, int64(req.LatencyMs)); err != nil {
		slog.Error("cdn report: report origin latency failed", "error", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	w.WriteHeader(200)
	if err := json.NewEncoder(w).Encode(map[string]any{"success": true}); err != nil {
		slog.Error("cdn report: encode response failed", "error", err)
	}
}

// GET /api/v2/admin/cdn/origin-latency
func (h *AdminCDNReportHandler) ListOriginLatencies(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	siteIDStr := r.URL.Query().Get("site_id")
	if siteIDStr != "" {
		h.listBySiteID(w, r, siteIDStr)
		return
	}

	latencies, err := h.cdn.GetOriginLatencies(r.Context())
	if err != nil {
		slog.Error("cdn report: get origin latencies failed", "error", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"latencies": latencies}); err != nil {
		slog.Error("cdn report: encode response failed", "error", err)
	}
}

func (h *AdminCDNReportHandler) listBySiteID(w http.ResponseWriter, r *http.Request, siteIDStr string) {
	siteID, err := strconv.ParseInt(siteIDStr, 10, 64)
	if err != nil || siteID <= 0 {
		respondJSON(w, 400, map[string]string{"error": "invalid site_id"})
		return
	}
	// 按站点查询需要调用 repository 直接获取，当前 service 层只支持全量查询
	// 暂不支持单站点筛选，此处 fallback 到全量后客户端自行过滤
	latencies, err := h.cdn.GetOriginLatencies(r.Context())
	if err != nil {
		slog.Error("cdn report: list by site id failed", "error", err)
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"latencies": latencies}); err != nil {
		slog.Error("cdn report: encode response failed", "error", err)
	}
}
