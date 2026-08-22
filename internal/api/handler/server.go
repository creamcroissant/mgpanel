// 文件路径: internal/api/handler/server.go
// 模块说明: 这是 internal 模块里的 server 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/creamcroissant/mgpanel/internal/api/requestctx"
	"github.com/creamcroissant/mgpanel/internal/async"
	"github.com/creamcroissant/mgpanel/internal/service"
	"github.com/creamcroissant/mgpanel/internal/support/i18n"
)

// ServerHandler handles node/provisioning callbacks.
type ServerHandler struct {
	Nodes     service.ServerNodeService
	Telemetry service.ServerTelemetryService
	Traffic   service.ServerTrafficService
	Queue     *async.TrafficQueue
	i18n      *i18n.Manager
}

func NewServerHandler(nodes service.ServerNodeService, telemetry service.ServerTelemetryService, traffic service.ServerTrafficService, queue *async.TrafficQueue, i18nMgr *i18n.Manager) *ServerHandler {
	return &ServerHandler{Nodes: nodes, Telemetry: telemetry, Traffic: traffic, Queue: queue, i18n: i18nMgr}
}

func (h *ServerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	action := serverActionPath(r.URL.Path)
	switch {
	case action == "/config" && r.Method == http.MethodGet:
		h.handleConfig(w, r)
	case action == "/user" && r.Method == http.MethodGet:
		h.handleUsers(w, r)
	case action == "/alive" && r.Method == http.MethodPost:
		h.handleAlive(w, r)
	case action == "/alivelist" && r.Method == http.MethodGet:
		h.handleAliveList(w, r)
	case action == "/status" && r.Method == http.MethodPost:
		h.handleStatus(w, r)
	default:
		respondNotImplemented(w, "server", r)
	}
}

func (h *ServerHandler) handleUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.Nodes == nil {
		RespondErrorI18nAction(ctx, w, http.StatusServiceUnavailable, "server.user", "error.service_unavailable", h.i18n)
		return
	}
	claims := requestctx.ServerFromContext(ctx)
	if claims.Server == nil {
		RespondErrorI18nAction(ctx, w, http.StatusUnauthorized, "server.user", "error.unauthorized", h.i18n)
		return
	}
	result, err := h.Nodes.Users(ctx, claims.Server)
	if err != nil {
		status := http.StatusInternalServerError
		key := "error.internal_server_error"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.not_found"
		}
		RespondErrorI18nAction(ctx, w, status, "server.user", key, h.i18n)
		return
	}
	if result == nil {
		RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "server.user", "error.internal_server_error", h.i18n)
		return
	}
	if h.Telemetry != nil {
		if err := h.Telemetry.TrackUserPull(ctx, claims.Server, len(result.Users)); err != nil {
			RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "server.user", "error.internal_server_error", h.i18n)
			return
		}
	}
	etag := formatETag(result.ETag)
	if etag != "" {
		w.Header().Set("ETag", etag)
		if requestTag := r.Header.Get("If-None-Match"); strings.Contains(requestTag, result.ETag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	respondJSON(w, http.StatusOK, map[string]any{"users": result.Users})
}

func (h *ServerHandler) handleConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.Nodes == nil {
		RespondErrorI18nAction(ctx, w, http.StatusServiceUnavailable, "server.config", "error.service_unavailable", h.i18n)
		return
	}
	claims := requestctx.ServerFromContext(ctx)
	if claims.Server == nil {
		RespondErrorI18nAction(ctx, w, http.StatusUnauthorized, "server.config", "error.unauthorized", h.i18n)
		return
	}
	result, err := h.Nodes.Config(ctx, claims.Server)
	if err != nil {
		status := http.StatusInternalServerError
		key := "error.internal_server_error"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.not_found"
		}
		RespondErrorI18nAction(ctx, w, status, "server.config", key, h.i18n)
		return
	}
	if result == nil {
		RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "server.config", "error.internal_server_error", h.i18n)
		return
	}
	etag := formatETag(result.ETag)
	if etag != "" {
		w.Header().Set("ETag", etag)
		if requestTag := r.Header.Get("If-None-Match"); strings.Contains(requestTag, result.ETag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	respondJSON(w, http.StatusOK, result.Payload)
}

func (h *ServerHandler) handleAlive(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.Telemetry == nil {
		RespondErrorI18nAction(ctx, w, http.StatusServiceUnavailable, "server.alive", "error.service_unavailable", h.i18n)
		return
	}
	claims := requestctx.ServerFromContext(ctx)
	if claims.Server == nil {
		RespondErrorI18nAction(ctx, w, http.StatusUnauthorized, "server.alive", "error.unauthorized", h.i18n)
		return
	}
	payload, err := decodeAlivePayload(r)
	if err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "server.alive", "error.bad_request", h.i18n)
		return
	}
	if err := h.Telemetry.RecordAlive(ctx, claims.Server, payload); err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "server.alive", "error.internal_server_error", h.i18n)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": true})
}

func (h *ServerHandler) handleAliveList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.Telemetry == nil || h.Nodes == nil {
		RespondErrorI18nAction(ctx, w, http.StatusServiceUnavailable, "server.alivelist", "error.service_unavailable", h.i18n)
		return
	}
	claims := requestctx.ServerFromContext(ctx)
	if claims.Server == nil {
		RespondErrorI18nAction(ctx, w, http.StatusUnauthorized, "server.alivelist", "error.unauthorized", h.i18n)
		return
	}
	result, err := h.Nodes.Users(ctx, claims.Server)
	if err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "server.alivelist", "error.internal_server_error", h.i18n)
		return
	}
	if result == nil {
		RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "server.alivelist", "error.internal_server_error", h.i18n)
		return
	}
	ids := make([]int64, 0, len(result.Users))
	for _, user := range result.Users {
		if user.DeviceLimit == nil {
			continue
		}
		if limit := *user.DeviceLimit; limit <= 0 {
			continue
		}
		ids = append(ids, user.ID)
	}
	counts, err := h.Telemetry.AliveCounts(ctx, ids)
	if err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "server.alivelist", "error.internal_server_error", h.i18n)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"alive": counts})
}

func (h *ServerHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.Telemetry == nil {
		RespondErrorI18nAction(ctx, w, http.StatusServiceUnavailable, "server.status", "error.service_unavailable", h.i18n)
		return
	}
	claims := requestctx.ServerFromContext(ctx)
	if claims.Server == nil {
		RespondErrorI18nAction(ctx, w, http.StatusUnauthorized, "server.status", "error.unauthorized", h.i18n)
		return
	}
	report, err := decodeStatusPayload(r)
	if err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "server.status", "error.bad_request", h.i18n)
		return
	}
	report.Taken = time.Now()
	if err := h.Telemetry.RecordStatus(ctx, claims.Server, report); err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "server.status", "error.internal_server_error", h.i18n)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"data": true, "code": 0, "message": "success"})
}

func serverActionPath(fullPath string) string {
	idx := strings.Index(fullPath, "/server")
	if idx == -1 {
		return "/"
	}
	suffix := strings.Trim(fullPath[idx+len("/server"):], "/")
	if suffix == "" {
		return "/"
	}
	parts := strings.Split(suffix, "/")
	last := parts[len(parts)-1]
	if last == "" {
		return "/"
	}
	return "/" + last
}

func decodeStatusPayload(r *http.Request) (service.ServerStatusReport, error) {
	var payload statusRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return service.ServerStatusReport{}, err
	}
	if payload.CPU == nil {
		return service.ServerStatusReport{}, errors.New("cpu is required / cpu 不能为空")
	}
	if *payload.CPU < 0 || *payload.CPU > 100 {
		return service.ServerStatusReport{}, errors.New("cpu out of range / cpu 超出范围")
	}
	mem, err := payload.requireBlock("mem")
	if err != nil {
		return service.ServerStatusReport{}, err
	}
	swap, err := payload.requireBlock("swap")
	if err != nil {
		return service.ServerStatusReport{}, err
	}
	disk, err := payload.requireBlock("disk")
	if err != nil {
		return service.ServerStatusReport{}, err
	}

	// Parse optional traffic fields (node-level traffic delta)
	var trafficUpload, trafficDownload int64
	if payload.TrafficUpload != nil {
		trafficUpload = *payload.TrafficUpload
	}
	if payload.TrafficDownload != nil {
		trafficDownload = *payload.TrafficDownload
	}

	return service.ServerStatusReport{
		CPU:             *payload.CPU,
		Mem:             mem,
		Swap:            swap,
		Disk:            disk,
		TrafficUpload:   trafficUpload,
		TrafficDownload: trafficDownload,
	}, nil
}

type statusRequest struct {
	CPU             *float64       `json:"cpu"`
	Mem             *resourceBlock `json:"mem"`
	Swap            *resourceBlock `json:"swap"`
	Disk            *resourceBlock `json:"disk"`
	TrafficUpload   *int64         `json:"traffic_upload"`
	TrafficDownload *int64         `json:"traffic_download"`
}

type resourceBlock struct {
	Total *int64 `json:"total"`
	Used  *int64 `json:"used"`
}

func (r *statusRequest) requireBlock(name string) (service.StatusCapacity, error) {
	var block *resourceBlock
	switch name {
	case "mem":
		block = r.Mem
	case "swap":
		block = r.Swap
	case "disk":
		block = r.Disk
	default:
		return service.StatusCapacity{}, fmt.Errorf("unknown resource block / 未知的资源块 %s", name)
	}
	if block == nil || block.Total == nil || block.Used == nil {
		return service.StatusCapacity{}, fmt.Errorf("%s stats missing / %s 统计信息缺失", name, name)
	}
	if *block.Total < 0 || *block.Used < 0 {
		return service.StatusCapacity{}, fmt.Errorf("%s stats negative / %s 统计信息为负数", name, name)
	}
	return service.StatusCapacity{Total: *block.Total, Used: *block.Used}, nil
}

func decodeAlivePayload(r *http.Request) (map[int64][]string, error) {
	var raw map[string]any
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, errors.New("invalid alive payload / 无效的在线数据")
	}
	result := make(map[int64][]string, len(raw))
	for key, value := range raw {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		userID, err := strconv.ParseInt(key, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid user id / 无效的用户 id: %s", key)
		}
		ips, err := toStringSlice(value)
		if err != nil {
			return nil, err
		}
		result[userID] = ips
	}
	return result, nil
}

func toStringSlice(value any) ([]string, error) {
	items, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("invalid alive entry / 无效的在线列表条目")
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		switch v := item.(type) {
		case string:
			trimmed := strings.TrimSpace(v)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		default:
			return nil, fmt.Errorf("alive entry must be string / 在线列表条目必须是字符串")
		}
	}
	return result, nil
}

