// 文件路径: internal/api/handler/user_knowledge.go
// 模块说明: 这是 internal 模块里的 user_knowledge 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/api/requestctx"
	"github.com/creamcroissant/mgpanel/internal/service"
	"github.com/creamcroissant/mgpanel/internal/support/i18n"
)

// UserKnowledgeHandler exposes read-only endpoints for the knowledge base.
type UserKnowledgeHandler struct {
	knowledge service.UserKnowledgeService
	i18n      *i18n.Manager
}

// NewUserKnowledgeHandler constructs a knowledge handler for user routes.
func NewUserKnowledgeHandler(knowledge service.UserKnowledgeService, i18nMgr *i18n.Manager) *UserKnowledgeHandler {
	return &UserKnowledgeHandler{knowledge: knowledge, i18n: i18nMgr}
}

func (h *UserKnowledgeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	action := userKnowledgeActionPath(r.URL.Path)
	switch {
	case action == "/fetch" && r.Method == http.MethodGet:
		h.handleFetch(w, r)
	case action == "/getCategory" && r.Method == http.MethodGet:
		h.handleCategories(w, r)
	default:
		respondNotImplemented(w, "user.knowledge", r)
	}
}

func (h *UserKnowledgeHandler) handleFetch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.knowledge == nil {
		RespondErrorI18nAction(ctx, w, http.StatusServiceUnavailable, "user.knowledge.fetch", "error.service_unavailable", h.i18n)
		return
	}
	claims := requestctx.UserFromContext(ctx)
	if claims.ID == "" {
		RespondErrorI18nAction(ctx, w, http.StatusUnauthorized, "user.knowledge.fetch", "error.unauthorized", h.i18n)
		return
	}
	query := r.URL.Query()
	if idStr := strings.TrimSpace(query.Get("id")); idStr != "" {
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "user.knowledge.fetch", "error.bad_request", h.i18n)
			return
		}
		article, err := h.knowledge.Detail(ctx, claims.ID, id)
		if err != nil {
			status := http.StatusInternalServerError
			key := "error.internal_server_error"
			if errors.Is(err, service.ErrNotFound) {
				status = http.StatusNotFound
				key = "error.not_found"
			}
			RespondErrorI18nAction(ctx, w, status, "user.knowledge.fetch", key, h.i18n)
			return
		}
		respondJSON(w, http.StatusOK, article)
		return
	}
	language := strings.TrimSpace(query.Get("language"))
	keyword := strings.TrimSpace(query.Get("keyword"))
	payload, err := h.knowledge.List(ctx, claims.ID, service.UserKnowledgeListInput{
		Language: language,
		Keyword:  keyword,
	})
	if err != nil {
		status := http.StatusInternalServerError
		key := "error.internal_server_error"
		if errors.Is(err, service.ErrNotFound) || errors.Is(err, service.ErrLanguageRequired) {
			status = http.StatusBadRequest
			key = "error.bad_request"
		}
		RespondErrorI18nAction(ctx, w, status, "user.knowledge.fetch", key, h.i18n)
		return
	}
	respondJSON(w, http.StatusOK, payload)
}

func (h *UserKnowledgeHandler) handleCategories(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if h.knowledge == nil {
		RespondErrorI18nAction(ctx, w, http.StatusServiceUnavailable, "user.knowledge.category", "error.service_unavailable", h.i18n)
		return
	}
	language := strings.TrimSpace(r.URL.Query().Get("language"))
	categories, err := h.knowledge.Categories(ctx, language)
	if err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "user.knowledge.category", "error.internal_server_error", h.i18n)
		return
	}
	respondJSON(w, http.StatusOK, categories)
}

func userKnowledgeActionPath(fullPath string) string {
	idx := strings.Index(fullPath, "/knowledge")
	if idx == -1 {
		return "/"
	}
	action := fullPath[idx+len("/knowledge"):]
	if action == "" || action == "/" {
		return "/"
	}
	if !strings.HasPrefix(action, "/") {
		action = "/" + action
	}
	return action
}
