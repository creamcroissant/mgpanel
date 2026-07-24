package handler

import (
	"net/http"
	"strconv"

	"github.com/creamcroissant/xboard/internal/api/requestctx"
	"github.com/creamcroissant/xboard/internal/repository"
	"github.com/creamcroissant/xboard/internal/service"
)

type AdminAccessLogHandler struct {
	accessLogService service.AccessLogService
}

func NewAdminAccessLogHandler(accessLogService service.AccessLogService) *AdminAccessLogHandler {
	return &AdminAccessLogHandler{
		accessLogService: accessLogService,
	}
}

func (h *AdminAccessLogHandler) Fetch(w http.ResponseWriter, r *http.Request) {
	if claims := requestctx.AdminFromContext(r.Context()); claims.ID == "" {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	filter := repository.AccessLogFilter{
		Limit:  getIntQuery(r, "limit", 20),
		Offset: getIntQuery(r, "offset", 0),
	}

	if id := getInt64Query(r, "user_id"); id > 0 {
		filter.UserID = &id
	}
	if id := getInt64Query(r, "agent_host_id"); id > 0 {
		filter.AgentHostID = &id
	}
	if q := r.URL.Query().Get("target_domain"); q != "" {
		filter.TargetDomain = &q
	}
	if q := r.URL.Query().Get("source_ip"); q != "" {
		filter.SourceIP = &q
	}
	if q := r.URL.Query().Get("protocol"); q != "" {
		filter.Protocol = &q
	}
	if start := getInt64Query(r, "start_at"); start > 0 {
		filter.StartAt = &start
	}
	if end := getInt64Query(r, "end_at"); end > 0 {
		filter.EndAt = &end
	}

	logs, count, err := h.accessLogService.ListAccessLogs(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "fetch_logs", err)
		return
	}

	// Ensure logs is never nil to return [] instead of null in JSON
	if logs == nil {
		logs = []*repository.AccessLog{}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"total": count,
		"data":  logs,
	})
}

func (h *AdminAccessLogHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	if claims := requestctx.AdminFromContext(r.Context()); claims.ID == "" {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	filter := repository.AccessLogFilter{}
	if id := getInt64Query(r, "user_id"); id > 0 {
		filter.UserID = &id
	}
	if id := getInt64Query(r, "agent_host_id"); id > 0 {
		filter.AgentHostID = &id
	}
	if start := getInt64Query(r, "start_at"); start > 0 {
		filter.StartAt = &start
	}
	if end := getInt64Query(r, "end_at"); end > 0 {
		filter.EndAt = &end
	}

	stats, err := h.accessLogService.GetStats(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "get_stats", err)
		return
	}

	respondJSON(w, http.StatusOK, stats)
}

func (h *AdminAccessLogHandler) Cleanup(w http.ResponseWriter, r *http.Request) {
	if claims := requestctx.AdminFromContext(r.Context()); claims.ID == "" {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	count, err := h.accessLogService.CleanupOldLogs(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "cleanup_logs", err)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"count": count,
	})
}

func getIntQuery(r *http.Request, key string, def int) int {
	str := r.URL.Query().Get(key)
	if str == "" {
		return def
	}
	val, err := strconv.Atoi(str)
	if err != nil {
		return def
	}
	return val
}

func getInt64Query(r *http.Request, key string) int64 {
	str := r.URL.Query().Get(key)
	if str == "" {
		return 0
	}
	val, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		return 0
	}
	return val
}
