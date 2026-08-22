// 文件路径: internal/api/handler/admin_user.go
// 模块说明: 这是 internal 模块里的 admin_user 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/api/requestctx"
	"github.com/creamcroissant/mgpanel/internal/service"
	"github.com/go-chi/chi/v5"
)

// AdminUserHandler exposes minimal admin user endpoints.
type AdminUserHandler struct {
	users service.AdminUserService
}

// NewAdminUserHandler wires admin user service into HTTP surface.
func NewAdminUserHandler(users service.AdminUserService) *AdminUserHandler {
	return &AdminUserHandler{users: users}
}

func (h *AdminUserHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.users == nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, "admin.user", "error.service_unavailable", nil)
		return
	}
	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		RespondErrorI18n(r.Context(), w, http.StatusUnauthorized, "error.unauthorized", h.users.I18n())
		return
	}
	action := adminActionPath(r.URL.Path)
	switch {
	case strings.HasPrefix(action, "/user") && strings.HasSuffix(action, "/fetch") && (r.Method == http.MethodGet || r.Method == http.MethodPost):
		h.handleFetch(w, r)
	case strings.HasPrefix(action, "/user") && strings.HasSuffix(action, "/update") && r.Method == http.MethodPost:
		h.handleUpdate(w, r)
	case strings.HasPrefix(action, "/user") && strings.HasSuffix(action, "/generate") && r.Method == http.MethodPost:
		h.handleGenerate(w, r)
	case strings.HasPrefix(action, "/user") && strings.HasSuffix(action, "/export") && r.Method == http.MethodPost:
		h.handleExport(w, r)
	case strings.HasPrefix(action, "/user") && strings.HasSuffix(action, "/import") && r.Method == http.MethodPost:
		h.handleImport(w, r)
	default:
		respondNotImplemented(w, "admin.user", r)
	}
}

func (h *AdminUserHandler) handleFetch(w http.ResponseWriter, r *http.Request) {
	params, err := parseAdminUserFetchParams(r)
	if err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "admin.user.fetch", h.users.I18n())
		return
	}
	result, err := h.users.Fetch(r.Context(), params.input)
	if err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "admin.user.fetch", h.users.I18n())
		return
	}
	payload := map[string]any{
		"data":     result.Users,
		"count":    len(result.Users),
		"total":    result.Total,
		"page":     params.page,
		"pageSize": params.pageSize,
	}
	respondJSON(w, http.StatusOK, payload)
}

func (h *AdminUserHandler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	var payload service.AdminUserUpdateInput
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "admin.user.update", h.users.I18n())
		return
	}
	user, err := h.users.Update(r.Context(), payload)
	if err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "admin.user.update", h.users.I18n())
		return
	}
	RespondSuccessI18n(r.Context(), w, "success.updated", h.users.I18n(), user)
}

func (h *AdminUserHandler) handleGenerate(w http.ResponseWriter, r *http.Request) {
	var payload service.AdminUserGenerateInput
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "admin.user.generate", h.users.I18n())
		return
	}
	user, err := h.users.Generate(r.Context(), payload)
	if err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "admin.user.generate", h.users.I18n())
		return
	}
	RespondSuccessI18n(r.Context(), w, "success.created", h.users.I18n(), user)
}

func (h *AdminUserHandler) handleExport(w http.ResponseWriter, r *http.Request) {
	params, err := parseAdminUserFetchParams(r)
	if err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "admin.user.export", h.users.I18n())
		return
	}

	csvData, err := h.users.Export(r.Context(), params.input)
	if err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusInternalServerError, "admin.user.export", h.users.I18n())
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=users_export.csv")
	w.WriteHeader(http.StatusOK)
	w.Write(csvData)
}

func (h *AdminUserHandler) handleImport(w http.ResponseWriter, r *http.Request) {
	// Limit upload size to 10MB
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)

	file, _, err := r.FormFile("file")
	if err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "admin.user.import", h.users.I18n())
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusInternalServerError, "admin.user.import", h.users.I18n())
		return
	}

	result, err := h.users.Import(r.Context(), data)
	if err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "admin.user.import", h.users.I18n())
		return
	}

	RespondSuccessI18n(r.Context(), w, "success.updated", h.users.I18n(), result)
}

type adminUserFetchParams struct {
	input    service.AdminUserFetchInput
	page     int
	pageSize int
}

type adminUserFetchPayload struct {
	PageSize int                    `json:"pageSize"`
	Current  int                    `json:"current"`
	Keyword  string                 `json:"keyword"`
	PlanID   *int64                 `json:"plan_id"`
	Status   *int                   `json:"status"`
	Filter   []adminUserTableFilter `json:"filter"`
	Sort     []adminUserTableSort   `json:"sort"`
}

type adminUserTableFilter struct {
	ID    string          `json:"id"`
	Value json.RawMessage `json:"value"`
}

type adminUserTableSort struct {
	ID   string `json:"id"`
	Desc bool   `json:"desc"`
}

func parseAdminUserFetchParams(r *http.Request) (adminUserFetchParams, error) {
	if r.Method == http.MethodPost {
		return parseAdminUserFetchBody(r)
	}
	return parseAdminUserFetchQuery(r)
}

func parseAdminUserFetchBody(r *http.Request) (adminUserFetchParams, error) {
	var payload adminUserFetchPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return adminUserFetchParams{}, err
	}
	pageSize := clampPageSize(payload.PageSize)
	page := payload.Current
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}
	keyword := strings.TrimSpace(payload.Keyword)
	planID := payload.PlanID
	status := payload.Status
	for _, filter := range payload.Filter {
		id := strings.ToLower(strings.TrimSpace(filter.ID))
		if id == "" {
			continue
		}
		switch id {
		case "keyword", "email", "account":
			if value := strings.TrimSpace(filter.firstString()); value != "" {
				keyword = value
			}
		case "remarks":
			// Assuming remarks search is handled by keyword for now as per repository search logic
			// If we want specific field search, we need to update repository search filter.
			// But since the current implementation merges email, username, remarks into keyword search,
			// setting keyword here works if user filters by remarks specifically in UI but maps to keyword in backend.
			if value := strings.TrimSpace(filter.firstString()); value != "" {
				keyword = value
			}
		case "plan_id", "planid", "plan":
			if parsed, ok := filter.firstInt64(); ok {
				planID = &parsed
			}
		case "status":
			if parsed, ok := filter.firstInt(); ok {
				status = &parsed
			}
		}
	}
	input := service.AdminUserFetchInput{
		Query:  keyword,
		Status: status,
		PlanID: planID,
		Limit:  pageSize,
		Offset: offset,
	}
	return adminUserFetchParams{input: input, page: page, pageSize: pageSize}, nil
}

func parseAdminUserFetchQuery(r *http.Request) (adminUserFetchParams, error) {
	query := r.URL.Query()
	var status *int
	if statusStr := strings.TrimSpace(query.Get("status")); statusStr != "" {
		if parsed, err := strconv.Atoi(statusStr); err == nil {
			status = &parsed
		}
	}
	var planID *int64
	if planStr := strings.TrimSpace(query.Get("plan_id")); planStr != "" {
		if parsed, err := strconv.ParseInt(planStr, 10, 64); err == nil {
			planID = &parsed
		}
	}
	limit := clampQueryInt(query.Get("limit"), 20)
	offset := clampQueryInt(query.Get("offset"), 0)
	page := 1
	if limit > 0 {
		page = (offset / limit) + 1
	}
	input := service.AdminUserFetchInput{
		Query:  query.Get("keyword"),
		Status: status,
		PlanID: planID,
		Limit:  limit,
		Offset: offset,
	}
	return adminUserFetchParams{input: input, page: page, pageSize: limit}, nil
}

func clampPageSize(value int) int {
	if value <= 0 {
		return 20
	}
	if value > 200 {
		return 200
	}
	return value
}

func (f adminUserTableFilter) firstString() string {
	if len(f.Value) == 0 {
		return ""
	}
	var str string
	if err := json.Unmarshal(f.Value, &str); err == nil {
		return str
	}
	var arr []string
	if err := json.Unmarshal(f.Value, &arr); err == nil {
		if len(arr) > 0 {
			return arr[0]
		}
		return ""
	}
	var singleFloat float64
	if err := json.Unmarshal(f.Value, &singleFloat); err == nil {
		return strconv.FormatInt(int64(singleFloat), 10)
	}
	var floatArr []float64
	if err := json.Unmarshal(f.Value, &floatArr); err == nil {
		if len(floatArr) > 0 {
			return strconv.FormatInt(int64(floatArr[0]), 10)
		}
		return ""
	}
	var boolean bool
	if err := json.Unmarshal(f.Value, &boolean); err == nil {
		if boolean {
			return "1"
		}
		return "0"
	}
	return ""
}

func (f adminUserTableFilter) firstInt64() (int64, bool) {
	value := strings.TrimSpace(f.firstString())
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func (f adminUserTableFilter) firstInt() (int, bool) {
	value := strings.TrimSpace(f.firstString())
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return parsed, true
}

func isSelfAdminTarget(claims requestctx.AdminClaims, id int64) bool {
	if id <= 0 {
		return false
	}
	return strings.TrimSpace(claims.ID) == strconv.FormatInt(id, 10)
}

func isSelfRestrictedAdminUpdate(claims requestctx.AdminClaims, payload service.AdminUserUpdateInput) bool {
	if !isSelfAdminTarget(claims, payload.ID) {
		return false
	}
	if payload.Banned != nil && *payload.Banned {
		return true
	}
	if payload.Status != nil && *payload.Status != 1 {
		return true
	}
	return false
}

// requireAdmin checks admin authentication
func (h *AdminUserHandler) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		RespondErrorI18n(r.Context(), w, http.StatusUnauthorized, "error.unauthorized", h.users.I18n())
		return false
	}
	return true
}

// List handles GET /user (RESTful alias for POST /user/fetch)
func (h *AdminUserHandler) List(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	h.handleFetch(w, r)
}

// Get handles GET /user/{id}
func (h *AdminUserHandler) Get(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "admin.user.get", h.users.I18n())
		return
	}

	user, err := h.users.GetByID(r.Context(), id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
		}
		RespondErrorI18n(r.Context(), w, status, "admin.user.get", h.users.I18n())
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{"data": user})
}

// Update handles PUT /user/{id}
func (h *AdminUserHandler) Update(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		RespondErrorI18n(r.Context(), w, http.StatusUnauthorized, "error.unauthorized", h.users.I18n())
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "admin.user.update", h.users.I18n())
		return
	}

	var payload service.AdminUserUpdateInput
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "admin.user.update", h.users.I18n())
		return
	}

	// Set ID from URL path
	payload.ID = id

	if isSelfRestrictedAdminUpdate(claims, payload) {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "admin.user.update", h.users.I18n())
		return
	}

	user, err := h.users.Update(r.Context(), payload)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
		}
		RespondErrorI18n(r.Context(), w, status, "admin.user.update", h.users.I18n())
		return
	}

	RespondSuccessI18n(r.Context(), w, "success.updated", h.users.I18n(), user)
}

// Delete handles DELETE /user/{id}
func (h *AdminUserHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}

	claims := requestctx.AdminFromContext(r.Context())
	if claims.ID == "" {
		RespondErrorI18n(r.Context(), w, http.StatusUnauthorized, "error.unauthorized", h.users.I18n())
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "admin.user.delete", h.users.I18n())
		return
	}

	if isSelfAdminTarget(claims, id) {
		RespondErrorI18n(r.Context(), w, http.StatusBadRequest, "admin.user.delete", h.users.I18n())
		return
	}

	if err := h.users.Delete(r.Context(), id); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
		}
		RespondErrorI18n(r.Context(), w, status, "admin.user.delete", h.users.I18n())
		return
	}

	RespondSuccessI18n(r.Context(), w, "success.deleted", h.users.I18n(), nil)
}

// Create handles POST /user (RESTful alias for POST /user/generate)
func (h *AdminUserHandler) Create(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	h.handleGenerate(w, r)
}
