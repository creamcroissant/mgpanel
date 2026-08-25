package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/creamcroissant/mgpanel/internal/service"
)

// TopologyHandler 拓扑画布后端（契约：docs/plans/20260824-topology-editor.md）。
type TopologyHandler struct {
	topology service.TopologyService
	logger   *slog.Logger
}

// NewTopologyHandler 构造拓扑处理器。
func NewTopologyHandler(topology service.TopologyService, logger *slog.Logger) *TopologyHandler {
	return &TopologyHandler{topology: topology, logger: logger}
}

// GetTopology GET /admin/topology?core_type=xxx — 画布原子快照。
func (h *TopologyHandler) GetTopology(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	start := time.Now()
	coreType := r.URL.Query().Get("core_type")

	snap, err := h.topology.Snapshot(ctx, coreType)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "get_topology", err)
		return
	}
	slog.Info("topology snapshot served",
		"core_type", coreType, "elapsed_ms", time.Since(start).Milliseconds())
	writeJSON(w, http.StatusOK, map[string]any{"data": snap})
}

// ValidateTopology POST /admin/topology/validate — 持久化状态一致性校验。
func (h *TopologyHandler) ValidateTopology(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	report, err := h.topology.Validate(ctx)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "validate_topology", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": report})
}

// topologyReorderRequest 重排序请求体。
type topologyReorderRequest struct {
	OrderedIDs []int64 `json:"ordered_ids"`
}

// ReorderPolicies PUT /admin/routing-policies/reorder — 批量重写优先级。
func (h *TopologyHandler) ReorderPolicies(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req topologyReorderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "reorder_policies", err)
		return
	}
	if len(req.OrderedIDs) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "ordered_ids 不能为空"})
		return
	}
	updated, err := h.topology.ReorderPolicies(ctx, req.OrderedIDs)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "reorder_policies", err)
		return
	}
	slog.Info("routing policies reorder served", "requested", len(req.OrderedIDs), "updated", updated)
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]int64{"updated": updated}})
}
