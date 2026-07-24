package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/creamcroissant/xboard/internal/api/requestctx"
	"github.com/creamcroissant/xboard/internal/repository"
	"github.com/creamcroissant/xboard/internal/service"
	"github.com/creamcroissant/xboard/internal/support/i18n"
	"github.com/go-chi/chi/v5"
)

// AdminCDNHandler 处理 CDN 站点管理的 HTTP 请求
type AdminCDNHandler struct {
	cdn  service.CDNService
	i18n *i18n.Manager
}

// NewAdminCDNHandler 创建 CDN 站点处理器
func NewAdminCDNHandler(svc service.CDNService, i18nMgr *i18n.Manager) *AdminCDNHandler {
	return &AdminCDNHandler{cdn: svc, i18n: i18nMgr}
}

// requireAdmin 检查管理员权限，返回操作人 ID 指针
func (h *AdminCDNHandler) requireAdmin(w http.ResponseWriter, r *http.Request) (int64, bool) {
	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusUnauthorized, "admin.cdn.auth", "error.unauthorized", h.i18n)
		return 0, false
	}
	adminID, err := strconv.ParseInt(claims.ID, 10, 64)
	if err != nil {
		adminID = 0
	}
	return adminID, true
}

// --- 站点 CRUD ---

// CreateCDNSiteRequest 创建 CDN 站点的请求体
type CreateCDNSiteRequest struct {
	Name             string `json:"name"`
	Description      string `json:"description"`
	Domain           string `json:"domain"`
	OriginType       string `json:"origin_type"`
	OriginURL        string `json:"origin_url"`
	CacheTTL         int    `json:"cache_ttl"`
	SSLMode          string `json:"ssl_mode"`
	AccelerationMode string `json:"acceleration_mode"`
	Enabled          bool   `json:"enabled"`
}

// ListSites 处理 GET /api/v2/admin/cdn/sites
// 查询参数: keyword, status, enabled, limit, offset
func (h *AdminCDNHandler) ListSites(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}

	keyword := r.URL.Query().Get("keyword")
	limit := clampQueryInt(r.URL.Query().Get("limit"), 20)
	offset := clampNonNegativeQueryInt(r.URL.Query().Get("offset"), 0)

	var status *string
	if s := r.URL.Query().Get("status"); s != "" {
		status = &s
	}

	var enabled *bool
	if e := r.URL.Query().Get("enabled"); e != "" {
		v, err := strconv.ParseBool(e)
		if err != nil {
			RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.cdn.sites.list", "error.bad_request", h.i18n)
			return
		}
		enabled = &v
	}

	ctx := r.Context()
	filter := repository.CDNSiteFilter{
		Keyword: keyword,
		Status:  status,
		Enabled: enabled,
		Limit:   limit,
		Offset:  offset,
	}
	sites, total, err := h.cdn.ListSites(ctx, filter)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusInternalServerError, "admin.cdn.sites.list", "error.internal_server_error", h.i18n)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": map[string]any{
			"sites": sites,
			"total": total,
		},
	})
}

// CreateSite 处理 POST /api/v2/admin/cdn/sites
func (h *AdminCDNHandler) CreateSite(w http.ResponseWriter, r *http.Request) {
	_, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}

	var req CreateCDNSiteRequest
	if err := decodeJSON(r, &req); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.cdn.sites.create", "error.bad_request", h.i18n)
		return
	}

	ctx := r.Context()
	site, err := h.cdn.CreateSite(ctx, service.CreateCDNSiteRequest{
		Name:             req.Name,
		Description:      req.Description,
		Domain:           req.Domain,
		OriginType:       req.OriginType,
		OriginURL:        req.OriginURL,
		CacheTTL:         req.CacheTTL,
		SSLMode:          req.SSLMode,
		AccelerationMode: req.AccelerationMode,
		Enabled:          req.Enabled,
	})
	if err != nil {
		status := http.StatusInternalServerError
		key := "error.internal_server_error"
		if errors.Is(err, service.ErrCDNSiteInvalid) {
			status = http.StatusBadRequest
			key = "error.bad_request"
		}
		RespondErrorI18nAction(r.Context(), w, status, "admin.cdn.sites.create", key, h.i18n)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"data": site,
	})
}

// UpdateSiteRequest 更新 CDN 站点的请求体
type UpdateSiteRequest struct {
	Name             *string `json:"name,omitempty"`
	Description      *string `json:"description,omitempty"`
	Domain           *string `json:"domain,omitempty"`
	OriginType       *string `json:"origin_type,omitempty"`
	OriginURL        *string `json:"origin_url,omitempty"`
	CacheTTL         *int    `json:"cache_ttl,omitempty"`
	SSLMode          *string `json:"ssl_mode,omitempty"`
	AccelerationMode *string `json:"acceleration_mode,omitempty"`
	Enabled          *bool   `json:"enabled,omitempty"`
}

// UpdateSite 处理 PUT /api/v2/admin/cdn/sites/{id}
func (h *AdminCDNHandler) UpdateSite(w http.ResponseWriter, r *http.Request) {
	_, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.cdn.sites.update", "error.bad_request", h.i18n)
		return
	}

	var req UpdateSiteRequest
	if err := decodeJSON(r, &req); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.cdn.sites.update", "error.bad_request", h.i18n)
		return
	}

	ctx := r.Context()
	site, err := h.cdn.UpdateSite(ctx, id, service.UpdateCDNSiteRequest{
		Name:             req.Name,
		Description:      req.Description,
		Domain:           req.Domain,
		OriginType:       req.OriginType,
		OriginURL:        req.OriginURL,
		CacheTTL:         req.CacheTTL,
		SSLMode:          req.SSLMode,
		AccelerationMode: req.AccelerationMode,
		Enabled:          req.Enabled,
	})
	if err != nil {
		status := http.StatusInternalServerError
		key := "error.internal_server_error"
		if errors.Is(err, service.ErrCDNSiteInvalid) {
			status = http.StatusBadRequest
			key = "error.bad_request"
		} else if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.not_found"
		}
		RespondErrorI18nAction(r.Context(), w, status, "admin.cdn.sites.update", key, h.i18n)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": site,
	})
}

// DeleteSite 处理 DELETE /api/v2/admin/cdn/sites/{id}
func (h *AdminCDNHandler) DeleteSite(w http.ResponseWriter, r *http.Request) {
	_, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.cdn.sites.delete", "error.bad_request", h.i18n)
		return
	}

	ctx := r.Context()
	if err := h.cdn.DeleteSite(ctx, id); err != nil {
		status := http.StatusInternalServerError
		key := "error.internal_server_error"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.not_found"
		}
		RespondErrorI18nAction(r.Context(), w, status, "admin.cdn.sites.delete", key, h.i18n)
		return
	}

	RespondSuccessI18n(r.Context(), w, "success.deleted", h.i18n, nil)
}

// --- 边缘节点管理 ---

// AssignEdgeRequest 分配边缘节点的请求体
type AssignEdgeRequest struct {
	AgentHostID int64 `json:"agent_host_id"`
	Weight      int   `json:"weight"`
}

// ListEdges 处理 GET /api/v2/admin/cdn/sites/{id}/edges
func (h *AdminCDNHandler) ListEdges(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}

	idStr := chi.URLParam(r, "id")
	siteID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.cdn.edges.list", "error.bad_request", h.i18n)
		return
	}

	ctx := r.Context()
	edges, err := h.cdn.ListEdges(ctx, siteID)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusInternalServerError, "admin.cdn.edges.list", "error.internal_server_error", h.i18n)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": edges,
	})
}

// AssignEdge 处理 POST /api/v2/admin/cdn/sites/{id}/edges
func (h *AdminCDNHandler) AssignEdge(w http.ResponseWriter, r *http.Request) {
	_, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}

	idStr := chi.URLParam(r, "id")
	siteID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.cdn.edges.assign", "error.bad_request", h.i18n)
		return
	}

	var req AssignEdgeRequest
	if err := decodeJSON(r, &req); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.cdn.edges.assign", "error.bad_request", h.i18n)
		return
	}

	ctx := r.Context()
	edge, err := h.cdn.AssignEdge(ctx, siteID, req.AgentHostID, req.Weight)
	if err != nil {
		status := http.StatusInternalServerError
		key := "error.internal_server_error"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.not_found"
		}
		RespondErrorI18nAction(r.Context(), w, status, "admin.cdn.edges.assign", key, h.i18n)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"data": edge,
	})
}

// RemoveEdge 处理 DELETE /api/v2/admin/cdn/sites/{id}/edges/{edge_id}
func (h *AdminCDNHandler) RemoveEdge(w http.ResponseWriter, r *http.Request) {
	_, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}

	edgeIDStr := chi.URLParam(r, "edge_id")
	edgeID, err := strconv.ParseInt(edgeIDStr, 10, 64)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.cdn.edges.remove", "error.bad_request", h.i18n)
		return
	}

	ctx := r.Context()
	if err := h.cdn.RemoveEdge(ctx, edgeID); err != nil {
		status := http.StatusInternalServerError
		key := "error.internal_server_error"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.not_found"
		}
		RespondErrorI18nAction(r.Context(), w, status, "admin.cdn.edges.remove", key, h.i18n)
		return
	}

	RespondSuccessI18n(r.Context(), w, "success.deleted", h.i18n, nil)
}

// --- 缓存规则管理 ---

// CreateCacheRuleRequest 创建缓存规则的请求体
type CreateCacheRuleRequest struct {
	MatchType  string `json:"match_type"`
	MatchValue string `json:"match_value"`
	CacheTTL   int    `json:"cache_ttl"`
	Bypass     bool   `json:"bypass"`
	Priority   int    `json:"priority"`
}

// ListCacheRules 处理 GET /api/v2/admin/cdn/sites/{id}/rules
func (h *AdminCDNHandler) ListCacheRules(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}

	idStr := chi.URLParam(r, "id")
	siteID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.cdn.rules.list", "error.bad_request", h.i18n)
		return
	}

	ctx := r.Context()
	rules, err := h.cdn.ListCacheRules(ctx, siteID)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusInternalServerError, "admin.cdn.rules.list", "error.internal_server_error", h.i18n)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": rules,
	})
}

// CreateCacheRule 处理 POST /api/v2/admin/cdn/sites/{id}/rules
func (h *AdminCDNHandler) CreateCacheRule(w http.ResponseWriter, r *http.Request) {
	_, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}

	idStr := chi.URLParam(r, "id")
	siteID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.cdn.rules.create", "error.bad_request", h.i18n)
		return
	}

	var req CreateCacheRuleRequest
	if err := decodeJSON(r, &req); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.cdn.rules.create", "error.bad_request", h.i18n)
		return
	}

	ctx := r.Context()
	rule, err := h.cdn.CreateCacheRule(ctx, service.CreateCDNCacheRuleRequest{
		SiteID:     siteID,
		MatchType:  req.MatchType,
		MatchValue: req.MatchValue,
		CacheTTL:   req.CacheTTL,
		Bypass:     req.Bypass,
		Priority:   req.Priority,
	})
	if err != nil {
		status := http.StatusInternalServerError
		key := "error.internal_server_error"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.not_found"
		}
		RespondErrorI18nAction(r.Context(), w, status, "admin.cdn.rules.create", key, h.i18n)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"data": rule,
	})
}

// DeleteCacheRule 处理 DELETE /api/v2/admin/cdn/sites/{id}/rules/{rule_id}
func (h *AdminCDNHandler) DeleteCacheRule(w http.ResponseWriter, r *http.Request) {
	_, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}

	ruleIDStr := chi.URLParam(r, "rule_id")
	ruleID, err := strconv.ParseInt(ruleIDStr, 10, 64)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.cdn.rules.delete", "error.bad_request", h.i18n)
		return
	}

	ctx := r.Context()
	if err := h.cdn.DeleteCacheRule(ctx, ruleID); err != nil {
		status := http.StatusInternalServerError
		key := "error.internal_server_error"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.not_found"
		}
		RespondErrorI18nAction(r.Context(), w, status, "admin.cdn.rules.delete", key, h.i18n)
		return
	}

	RespondSuccessI18n(r.Context(), w, "success.deleted", h.i18n, nil)
}

// --- 部署操作 ---

// DeploySite 处理 POST /api/v2/admin/cdn/sites/{id}/deploy
func (h *AdminCDNHandler) DeploySite(w http.ResponseWriter, r *http.Request) {
	_, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}

	idStr := chi.URLParam(r, "id")
	siteID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.cdn.sites.deploy", "error.bad_request", h.i18n)
		return
	}

	ctx := r.Context()
	if err := h.cdn.DeploySite(ctx, siteID); err != nil {
		status := http.StatusInternalServerError
		key := "error.internal_server_error"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.not_found"
		}
		RespondErrorI18nAction(r.Context(), w, status, "admin.cdn.sites.deploy", key, h.i18n)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"message": "deploy initiated",
	})
}

// UndeploySite 处理 POST /api/v2/admin/cdn/sites/{id}/undeploy
func (h *AdminCDNHandler) UndeploySite(w http.ResponseWriter, r *http.Request) {
	_, ok := h.requireAdmin(w, r)
	if !ok {
		return
	}

	idStr := chi.URLParam(r, "id")
	siteID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.cdn.sites.undeploy", "error.bad_request", h.i18n)
		return
	}

	ctx := r.Context()
	if err := h.cdn.UndeploySite(ctx, siteID); err != nil {
		status := http.StatusInternalServerError
		key := "error.internal_server_error"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.not_found"
		}
		RespondErrorI18nAction(r.Context(), w, status, "admin.cdn.sites.undeploy", key, h.i18n)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"message": "undeploy initiated",
	})
}

// ---------------------------------------------------------------------------
// Cloudflare 方法
// ---------------------------------------------------------------------------

// SetCloudflareAPITokenRequest 设置 Cloudflare API Token 的请求体
type SetCloudflareAPITokenRequest struct {
	APIToken string `json:"api_token"`
}

// SetCloudflareAPIToken 处理 POST /api/v2/admin/cdn/cloudflare/token
func (h *AdminCDNHandler) SetCloudflareAPIToken(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	var req SetCloudflareAPITokenRequest
	if err := decodeJSON(r, &req); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.cdn.cloudflare.token", "error.bad_request", h.i18n)
		return
	}
	if req.APIToken == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.cdn.cloudflare.token", "error.bad_request", h.i18n)
		return
	}
	ctx := r.Context()
	if err := h.cdn.SaveCloudflareAPIToken(ctx, req.APIToken); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusInternalServerError, "admin.cdn.cloudflare.token", "error.internal_server_error", h.i18n)
		return
	}
	RespondSuccessI18n(r.Context(), w, "success.saved", h.i18n, nil)
}

// ListCloudflareZones 处理 GET /api/v2/admin/cdn/cloudflare/zones
func (h *AdminCDNHandler) ListCloudflareZones(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	ctx := r.Context()
	zones, err := h.cdn.ListCloudflareZones(ctx)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusInternalServerError, "admin.cdn.cloudflare.zones.list", "error.internal_server_error", h.i18n)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": zones,
	})
}

// AddCloudflareZoneRequest 添加 Cloudflare 区域的请求体
type AddCloudflareZoneRequest struct {
	Name   string `json:"name"`
	ZoneID string `json:"zone_id"`
}

// AddCloudflareZone 处理 POST /api/v2/admin/cdn/cloudflare/zones
func (h *AdminCDNHandler) AddCloudflareZone(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	var req AddCloudflareZoneRequest
	if err := decodeJSON(r, &req); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.cdn.cloudflare.zones.create", "error.bad_request", h.i18n)
		return
	}
	ctx := r.Context()
	zone, err := h.cdn.AddCloudflareZone(ctx, req.Name, req.ZoneID)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusInternalServerError, "admin.cdn.cloudflare.zones.create", "error.internal_server_error", h.i18n)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"data": zone,
	})
}

// RemoveCloudflareZone 处理 DELETE /api/v2/admin/cdn/cloudflare/zones/:zone_id
func (h *AdminCDNHandler) RemoveCloudflareZone(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	idStr := chi.URLParam(r, "zone_id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.cdn.cloudflare.zones.delete", "error.bad_request", h.i18n)
		return
	}
	ctx := r.Context()
	if err := h.cdn.RemoveCloudflareZone(ctx, id); err != nil {
		status := http.StatusInternalServerError
		key := "error.internal_server_error"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.not_found"
		}
		RespondErrorI18nAction(r.Context(), w, status, "admin.cdn.cloudflare.zones.delete", key, h.i18n)
		return
	}
	RespondSuccessI18n(r.Context(), w, "success.deleted", h.i18n, nil)
}

// ListDNSRecords 处理 GET /api/v2/admin/cdn/cloudflare/zones/:zone_id/dns
func (h *AdminCDNHandler) ListDNSRecords(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	idStr := chi.URLParam(r, "zone_id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.cdn.cloudflare.dns.list", "error.bad_request", h.i18n)
		return
	}
	ctx := r.Context()
	records, err := h.cdn.ListCloudflareDNSRecords(ctx, id)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusInternalServerError, "admin.cdn.cloudflare.dns.list", "error.internal_server_error", h.i18n)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": records,
	})
}

// ---------------------------------------------------------------------------
// CloudFront 方法
// ---------------------------------------------------------------------------

// SetCloudFrontCredentialsRequest 设置 CloudFront 凭证的请求体
type SetCloudFrontCredentialsRequest struct {
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
}

// SetCloudFrontCredentials 处理 POST /api/v2/admin/cdn/cloudfront/credentials
func (h *AdminCDNHandler) SetCloudFrontCredentials(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	var req SetCloudFrontCredentialsRequest
	if err := decodeJSON(r, &req); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.cdn.cloudfront.credentials", "error.bad_request", h.i18n)
		return
	}
	if req.AccessKeyID == "" || req.SecretAccessKey == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.cdn.cloudfront.credentials", "error.bad_request", h.i18n)
		return
	}
	ctx := r.Context()
	if err := h.cdn.SetCloudFrontCredentials(ctx, req.AccessKeyID, req.SecretAccessKey); err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusInternalServerError, "admin.cdn.cloudfront.credentials", "error.internal_server_error", h.i18n)
		return
	}
	RespondSuccessI18n(r.Context(), w, "success.saved", h.i18n, nil)
}

// GetCloudFrontCredentials 处理 GET /api/v2/admin/cdn/cloudfront/credentials
func (h *AdminCDNHandler) GetCloudFrontCredentials(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	ctx := r.Context()
	cred, err := h.cdn.GetCloudFrontCredentials(ctx)
	if err != nil {
		// 未配置时返回空
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": nil,
		})
		return
	}
	// 返回时 mask secret key
	masked := cred.SecretAccessKey
	if len(masked) > 8 {
		masked = masked[:4] + "****" + masked[len(masked)-4:]
	} else if len(masked) > 0 {
		masked = "****"
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": map[string]any{
			"access_key_id":     cred.AccessKeyID,
			"secret_access_key": masked,
		},
	})
}

// ListCloudFrontDistributions 处理 GET /api/v2/admin/cdn/cloudfront/distributions
func (h *AdminCDNHandler) ListCloudFrontDistributions(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	ctx := r.Context()
	dists, err := h.cdn.ListCloudFrontDistributions(ctx)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusInternalServerError, "admin.cdn.cloudfront.distributions.list", "error.internal_server_error", h.i18n)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": dists,
	})
}

// ---------------------------------------------------------------------------
// Provider 通用方法
// ---------------------------------------------------------------------------

// SyncToProvider 处理 POST /api/v2/admin/cdn/sites/:id/sync
func (h *AdminCDNHandler) SyncToProvider(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.cdn.sites.sync", "error.bad_request", h.i18n)
		return
	}
	ctx := r.Context()
	if err := h.cdn.SyncToProvider(ctx, id); err != nil {
		status := http.StatusInternalServerError
		key := "error.internal_server_error"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.not_found"
		}
		RespondErrorI18nAction(r.Context(), w, status, "admin.cdn.sites.sync", key, h.i18n)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"message": "sync initiated",
	})
}

// InvalidateCache 处理 POST /api/v2/admin/cdn/sites/:id/invalidate
func (h *AdminCDNHandler) InvalidateCache(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.cdn.sites.invalidate", "error.bad_request", h.i18n)
		return
	}
	ctx := r.Context()
	if err := h.cdn.InvalidateCache(ctx, id); err != nil {
		status := http.StatusInternalServerError
		key := "error.internal_server_error"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.not_found"
		}
		RespondErrorI18nAction(r.Context(), w, status, "admin.cdn.sites.invalidate", key, h.i18n)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"message": "invalidation initiated",
	})
}

// GetProviderStatus 处理 GET /api/v2/admin/cdn/sites/:id/provider-status
func (h *AdminCDNHandler) GetProviderStatus(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdmin(w, r); !ok {
		return
	}
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusBadRequest, "admin.cdn.sites.provider-status", "error.bad_request", h.i18n)
		return
	}
	ctx := r.Context()
	status, err := h.cdn.GetProviderStatus(ctx, id)
	if err != nil {
		statusCode := http.StatusInternalServerError
		key := "error.internal_server_error"
		if errors.Is(err, service.ErrNotFound) {
			statusCode = http.StatusNotFound
			key = "error.not_found"
		}
		RespondErrorI18nAction(r.Context(), w, statusCode, "admin.cdn.sites.provider-status", key, h.i18n)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": status,
	})
}
