// 文件路径: internal/api/handler/client.go
// 模块说明: 这是 internal 模块里的 client 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
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

// ClientHandler covers client-specific endpoints such as subscription export.
type ClientHandler struct {
	Subscription service.SubscriptionService
	i18n         *i18n.Manager
}

func NewClientHandler(subscription service.SubscriptionService, i18nMgr *i18n.Manager) *ClientHandler {
	return &ClientHandler{Subscription: subscription, i18n: i18nMgr}
}

func (h *ClientHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	action := clientActionPath(r.URL.Path)
	switch {
	case action == "/subscribe" && r.Method == http.MethodGet:
		h.handleSubscribe(w, r)
	default:
		respondNotImplemented(w, "client", r)
	}
}

func (h *ClientHandler) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	if h.Subscription == nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusServiceUnavailable, "client.subscribe", "error.service_unavailable", h.i18n)
		return
	}
	claims := requestctx.UserFromContext(r.Context())
	tokenParam := strings.TrimSpace(r.URL.Query().Get("token"))
	templateID, _ := strconv.ParseInt(r.URL.Query().Get("template_id"), 10, 64)
	userRef := strings.TrimSpace(tokenParam)
	if userRef == "" {
		userRef = strings.TrimSpace(claims.ID)
	}
	if userRef == "" {
		RespondErrorI18nAction(r.Context(), w, http.StatusUnauthorized, "client.subscribe", "error.unauthorized", h.i18n)
		return
	}
	params := service.SubscriptionParams{
		Lang:         requestctx.GetLanguage(r.Context()),
		Types:        r.URL.Query().Get("types"),
		Filter:       r.URL.Query().Get("filter"),
		Flag:         r.URL.Query().Get("flag"),
		UserAgent:    r.UserAgent(),
		Host:         r.Host,
		Scheme:       requestScheme(r),
		URL:          absoluteURL(r),
		Tags:         r.URL.Query().Get("tags"),
		ShowUserInfo: r.URL.Query().Get("show_info") == "1" || r.URL.Query().Get("show_info") == "true",
		TemplateID:   templateID,
	}
	result, err := h.Subscription.Subscribe(r.Context(), userRef, params)
	if err != nil {
		status := http.StatusInternalServerError
		key := "error.internal_server_error"
		switch {
		case errors.Is(err, service.ErrNotFound):
			status = http.StatusNotFound
			key = "error.not_found"
		case errors.Is(err, service.ErrUserNotEligible):
			status = http.StatusForbidden
			key = "error.forbidden"
		}
		RespondErrorI18nAction(r.Context(), w, status, "client.subscribe", key, h.i18n)
		return
	}
	if result == nil {
		RespondErrorI18nAction(r.Context(), w, http.StatusInternalServerError, "client.subscribe", "error.internal_server_error", h.i18n)
		return
	}
	if result.ContentType != "" {
		w.Header().Set("Content-Type", result.ContentType)
	} else {
		w.Header().Set("Content-Type", "application/json")
	}
	for key, value := range result.Headers {
		if key == "" {
			continue
		}
		if strings.EqualFold(key, "content-type") {
			continue
		}
		w.Header().Set(key, value)
	}
	etag := formatETag(result.ETag)
	if etag != "" {
		w.Header().Set("ETag", etag)
	}
	if requestETag := r.Header.Get("If-None-Match"); etag != "" && strings.Contains(requestETag, result.ETag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(result.Payload)
}

func clientActionPath(fullPath string) string {
	idx := strings.Index(fullPath, "/client")
	if idx == -1 {
		return "/"
	}
	action := fullPath[idx+len("/client"):]
	if action == "" || action == "/" {
		return "/"
	}
	if !strings.HasPrefix(action, "/") {
		action = "/" + action
	}
	return action
}

func requestScheme(r *http.Request) string {
	if proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); proto != "" {
		return proto
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func absoluteURL(r *http.Request) string {
	host := strings.TrimSpace(r.Host)
	if host == "" {
		return ""
	}
	scheme := requestScheme(r)
	return scheme + "://" + host + r.URL.RequestURI()
}
