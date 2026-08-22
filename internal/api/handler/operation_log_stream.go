package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/api/requestctx"
	"github.com/creamcroissant/mgpanel/internal/repository"
	"github.com/creamcroissant/mgpanel/internal/service"
	"github.com/creamcroissant/mgpanel/internal/support/i18n"
)

type OperationLogHandler struct {
	logs service.OperationLogService
	i18n *i18n.Manager
}

func NewOperationLogHandler(logs service.OperationLogService, i18nMgr *i18n.Manager) *OperationLogHandler {
	return &OperationLogHandler{logs: logs, i18n: i18nMgr}
}

func (h *OperationLogHandler) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusUnauthorized, "admin.operation_logs.auth", "error.unauthorized", h.i18n)
		return false
	}
	return true
}

func (h *OperationLogHandler) ensureService(w http.ResponseWriter, r *http.Request, action string) bool {
	if h.logs != nil {
		return true
	}
	RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, action, "error.service_unavailable", h.i18n)
	return false
}

func (h *OperationLogHandler) List(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	if !h.ensureService(w, r, "admin.operation_logs.list") {
		return
	}

	req, ok := h.parseListRequest(w, r, "admin.operation_logs.list")
	if !ok {
		return
	}
	result, err := h.logs.List(r.Context(), req)
	if err != nil {
		h.respondError(r.Context(), w, "admin.operation_logs.list", err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"data":  result.Items,
		"total": result.Total,
	})
}

func (h *OperationLogHandler) Stream(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	if !h.ensureService(w, r, "admin.operation_logs.stream") {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		RespondErrorI18nAction(r.Context(), w, http.StatusInternalServerError, "admin.operation_logs.stream", "error.internal_server_error", h.i18n)
		return
	}

	listReq, ok := h.parseListRequest(w, r, "admin.operation_logs.stream")
	if !ok {
		return
	}
	afterID := int64(0)
	if listReq.AfterID != nil {
		afterID = *listReq.AfterID
	}

	subscription, err := h.logs.Subscribe(r.Context(), service.SubscribeOperationLogsRequest{
		Scope:    listReq.Scope,
		TargetID: listReq.TargetID,
		AfterID:  afterID,
	})
	if err != nil {
		h.respondError(r.Context(), w, "admin.operation_logs.stream", err)
		return
	}
	defer subscription.Close()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	lastSentID := afterID
	replay, err := h.logs.List(r.Context(), listReq)
	if err != nil {
		writeOperationLogSSEError(w, flusher, "replay_failed")
		return
	}
	for _, entry := range replay.Items {
		select {
		case <-r.Context().Done():
			return
		default:
		}
		if entry.ID <= lastSentID {
			continue
		}
		if err := writeOperationLogSSE(w, entry); err != nil {
			return
		}
		lastSentID = entry.ID
	}
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case entry, ok := <-subscription.Events:
			if !ok {
				return
			}
			if entry == nil || entry.ID <= lastSentID {
				continue
			}
			if err := writeOperationLogSSE(w, entry); err != nil {
				return
			}
			lastSentID = entry.ID
			flusher.Flush()
		}
	}
}

func (h *OperationLogHandler) parseListRequest(w http.ResponseWriter, r *http.Request, action string) (service.ListOperationLogsRequest, bool) {
	query := r.URL.Query()
	req := service.ListOperationLogsRequest{
		Scope:    strings.TrimSpace(query.Get("scope")),
		TargetID: strings.TrimSpace(query.Get("target_id")),
		Level:    strings.TrimSpace(query.Get("level")),
		Limit:    clampQueryInt(query.Get("limit"), 100),
		Offset:   clampNonNegativeQueryInt(query.Get("offset"), 0),
	}
	if raw := strings.TrimSpace(query.Get("agent_host_id")); raw != "" {
		agentHostID, err := parseInt64(raw)
		if err != nil {
			RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, action, "error.bad_request", h.i18n)
			return service.ListOperationLogsRequest{}, false
		}
		req.AgentHostID = &agentHostID
	}
	if raw := strings.TrimSpace(query.Get("after_id")); raw != "" {
		afterID, err := parseInt64(raw)
		if err != nil || afterID < 0 {
			RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, action, "error.bad_request", h.i18n)
			return service.ListOperationLogsRequest{}, false
		}
		req.AfterID = &afterID
	}
	return req, true
}

func (h *OperationLogHandler) respondError(ctx context.Context, w http.ResponseWriter, action string, err error) {
	status := http.StatusInternalServerError
	key := "error.internal_server_error"
	if errors.Is(err, service.ErrOperationLogNotConfigured) {
		status = http.StatusServiceUnavailable
		key = "error.service_unavailable"
	} else if errors.Is(err, service.ErrOperationLogInvalidRequest) {
		status = http.StatusBadRequest
		key = "error.bad_request"
	}
	RespondErrorI18nAction(ctx, w, status, action, key, h.i18n)
}

func writeOperationLogSSE(w http.ResponseWriter, entry *repository.OperationLogEntry) error {
	payload, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "id: %d\nevent: operation_log\ndata: %s\n\n", entry.ID, payload)
	return err
}

func writeOperationLogSSEError(w http.ResponseWriter, flusher http.Flusher, code string) {
	_, _ = fmt.Fprintf(w, "event: error\ndata: {\"error\":%q}\n\n", code)
	flusher.Flush()
}
