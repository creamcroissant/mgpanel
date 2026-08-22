// 文件路径: internal/api/handler/admin_stat.go
// 模块说明: 这是 internal 模块里的 admin_stat 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/api/requestctx"
	"github.com/creamcroissant/mgpanel/internal/service"
	"github.com/creamcroissant/mgpanel/internal/support/i18n"
)

// AdminStatHandler exposes analytics endpoints for the admin panel.
type AdminStatHandler struct {
	stats service.AdminStatService
	now   func() time.Time
	i18n  *i18n.Manager
}

// NewAdminStatHandler wires the admin stat service.
func NewAdminStatHandler(stats service.AdminStatService, i18nMgr *i18n.Manager) *AdminStatHandler {
	return &AdminStatHandler{stats: stats, now: time.Now, i18n: i18nMgr}
}

func (h *AdminStatHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	action := adminStatActionPath(r.URL.Path)
	switch {
	case (action == "/getStatUser" || action == "/fetch") && (r.Method == http.MethodGet || r.Method == http.MethodPost):
		h.handleGetStatUser(w, r)
	case action == "/getStats" && r.Method == http.MethodGet:
		h.handleGetStats(w, r)
	case action == "/getTrafficRank" && r.Method == http.MethodGet:
		h.handleGetTrafficRank(w, r)
	default:
		respondNotImplemented(w, "admin.stat", r)
	}
}

func (h *AdminStatHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if h.stats == nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, "admin.stat.dashboard", "error.service_unavailable", h.i18n)
		return
	}
	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusUnauthorized, "admin.stat.dashboard", "error.unauthorized", h.i18n)
		return
	}
	stats, err := h.stats.GetDashboardStats(r.Context())
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusInternalServerError, "admin.stat.dashboard", "error.internal_server_error", h.i18n)
		return
	}
	respondJSON(w, http.StatusOK, stats)
}

func (h *AdminStatHandler) handleGetTrafficRank(w http.ResponseWriter, r *http.Request) {
	if h.stats == nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, "admin.stat.traffic", "error.service_unavailable", h.i18n)
		return
	}
	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusUnauthorized, "admin.stat.traffic", "error.unauthorized", h.i18n)
		return
	}
	query := r.URL.Query()
	startUnix := pickFirstNonEmpty(query.Get("start_time"), query.Get("startTime"))
	startTime, err := parseDateOrUnix(startUnix, pickFirstNonEmpty(query.Get("start_date"), query.Get("startDate")), false)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.stat.traffic", "error.bad_request", h.i18n)
		return
	}
	endUnix := pickFirstNonEmpty(query.Get("end_time"), query.Get("endTime"))
	endTime, err := parseDateOrUnix(endUnix, pickFirstNonEmpty(query.Get("end_date"), query.Get("endDate")), strings.TrimSpace(endUnix) == "")
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.stat.traffic", "error.bad_request", h.i18n)
		return
	}
	limit := parsePositiveInt(query.Get("limit"), 10)
	if limit > 50 {
		limit = 50
	}
	result, err := h.stats.GetTrafficRank(r.Context(), service.AdminStatTrafficInput{
		Type:      strings.TrimSpace(query.Get("type")),
		StartTime: startTime,
		EndTime:   endTime,
		Limit:     limit,
	})
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusInternalServerError, "admin.stat.traffic", "error.internal_server_error", h.i18n)
		return
	}
	respondJSON(w, http.StatusOK, result)
}

func (h *AdminStatHandler) handleGetStatUser(w http.ResponseWriter, r *http.Request) {
	if h.stats == nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, "admin.stat.user", "error.service_unavailable", h.i18n)
		return
	}
	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusUnauthorized, "admin.stat.user", "error.unauthorized", h.i18n)
		return
	}
	query := r.URL.Query()
	limit := parsePositiveInt(query.Get("limit"), 20)
	//recordType := query.Get("type")
	recordTypeStr := query.Get("type")
	recordType := 1 // default to daily
	if recordTypeStr == "0" || strings.EqualFold(recordTypeStr, "h") {
		recordType = 0
	} else if recordTypeStr == "1" || strings.EqualFold(recordTypeStr, "d") {
		recordType = 1
	}

	date := strings.TrimSpace(query.Get("date"))
	days := parsePositiveInt(query.Get("days"), limit)
	userID := parsePositiveInt64(query.Get("user_id"), 0)
	var recordAt int64
	if date != "" {
		parsed, err := time.ParseInLocation("2006-01-02", date, time.UTC)
		if err != nil {
			RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.stat.user", "error.bad_request", h.i18n)
			return
		}
		recordAt = dayStart(parsed)
	}
	if days <= 0 {
		days = limit
	}
	if userID > 0 {
		historyLimit := limit
		if days > historyLimit {
			historyLimit = days
		}
		since := recordAt
		if since <= 0 {
			since = dayStart(h.now().UTC())
		}
		if days > 1 {
			since = since - int64(days-1)*24*60*60
			if since < 0 {
				since = 0
			}
		}
		stats, err := h.stats.GetUserStats(r.Context(), service.AdminStatUserInput{
			UserID:     userID,
			RecordAt:   since,
			RecordType: recordType,
			Limit:      historyLimit,
		})
		if err != nil {
			RespondErrorI18nAction(r.Context(), w, http.StatusInternalServerError, "admin.stat.user", "error.internal_server_error", h.i18n)
			return
		}
		respondJSON(w, http.StatusOK, stats)
		return
	}
	stats, err := h.stats.GetUserStats(r.Context(), service.AdminStatUserInput{
		RecordAt:   recordAt,
		RecordType: recordType,
		Limit:      limit,
	})
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusInternalServerError, "admin.stat.user", "error.internal_server_error", h.i18n)
		return
	}
	respondJSON(w, http.StatusOK, stats)
}

func parsePositiveInt(raw string, fallback int) int {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func parsePositiveInt64(raw string, fallback int64) int64 {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func parseDateOrUnix(unixRaw, dateRaw string, addDay bool) (int64, error) {
	if ts, err := parseUnixSeconds(unixRaw); err != nil {
		return 0, err
	} else if ts > 0 {
		return ts, nil
	}
	if strings.TrimSpace(dateRaw) == "" {
		return 0, nil
	}
	parsed, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(dateRaw), time.UTC)
	if err != nil {
		return 0, err
	}
	value := dayStart(parsed)
	if addDay {
		value += 24 * 60 * 60
	}
	return value, nil
}

func parseUnixSeconds(raw string) (int64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return 0, err
	}
	return value, nil
}

func pickFirstNonEmpty(values ...string) string {
	for _, val := range values {
		if strings.TrimSpace(val) != "" {
			return val
		}
	}
	return ""
}

func adminStatActionPath(fullPath string) string {
	idx := strings.Index(fullPath, "/stat")
	if idx == -1 {
		return "/"
	}
	tail := fullPath[idx+len("/stat"):]
	if tail == "" || tail == "/" {
		return "/"
	}
	if !strings.HasPrefix(tail, "/") {
		tail = "/" + tail
	}
	return tail
}

func dayStart(t time.Time) int64 {
	utc := t.UTC()
	y, m, d := utc.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix()
}
