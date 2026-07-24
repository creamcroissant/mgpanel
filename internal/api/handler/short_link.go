package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/creamcroissant/xboard/internal/api/requestctx"
	"github.com/creamcroissant/xboard/internal/service"
	"github.com/creamcroissant/xboard/internal/support/i18n"
	"github.com/go-chi/chi/v5"
)

// ShortLinkHandler handles short link related HTTP requests.
type ShortLinkHandler struct {
	Service      service.ShortLinkService
	Subscription service.SubscriptionService
	i18n         *i18n.Manager
}

// NewShortLinkHandler creates a new short link handler.
func NewShortLinkHandler(shortLink service.ShortLinkService, subscription service.SubscriptionService, i18nMgr *i18n.Manager) *ShortLinkHandler {
	return &ShortLinkHandler{
		Service:      shortLink,
		Subscription: subscription,
		i18n:         i18nMgr,
	}
}

func (h *ShortLinkHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	action := shortLinkActionPath(r.URL.Path)
	switch {
	case action == "/create" && r.Method == http.MethodPost:
		h.handleCreate(w, r)
	case action == "/list" && r.Method == http.MethodGet:
		h.handleList(w, r)
	case strings.HasPrefix(action, "/delete/") && r.Method == http.MethodDelete:
		h.handleDelete(w, r)
	default:
		respondNotImplemented(w, "shortlink", r)
	}
}

// CreateShortLinkRequest represents a short link creation request.
type CreateShortLinkRequest struct {
	Code      string `json:"code,omitempty"`       // Optional custom code
	ExpiresAt int64  `json:"expires_at,omitempty"` // Optional expiration timestamp
}

// ShortLinkResponse represents a short link in API responses.
type ShortLinkResponse struct {
	ID             int64  `json:"id"`
	Code           string `json:"code"`
	ShortURL       string `json:"short_url"`
	AccessCount    int64  `json:"access_count"`
	LastAccessedAt int64  `json:"last_accessed_at,omitempty"`
	ExpiresAt      int64  `json:"expires_at,omitempty"`
	CreatedAt      int64  `json:"created_at"`
}

func (h *ShortLinkHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims := requestctx.UserFromContext(ctx)
	if claims.ID == "" {
		RespondErrorI18nAction(ctx, w, http.StatusUnauthorized, "shortlink.create", "error.unauthorized", h.i18n)
		return
	}

	userID, err := strconv.ParseInt(claims.ID, 10, 64)
	if err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "shortlink.create", "error.bad_request", h.i18n)
		return
	}

	var req CreateShortLinkRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "shortlink.create", "error.bad_request", h.i18n)
			return
		}
	}

	link, err := h.Service.Create(ctx, userID, req.Code, req.ExpiresAt)
	if err != nil {
		status := http.StatusInternalServerError
		key := "error.internal_server_error"
		if errors.Is(err, service.ErrNotFound) {
			status = http.StatusNotFound
			key = "error.not_found"
		}
		RespondErrorI18nAction(ctx, w, status, "shortlink.create", key, h.i18n)
		return
	}

	// Build short URL
	shortURL := "/s/" + link.Code
	if host := r.Host; host != "" {
		scheme := "https"
		if r.TLS == nil {
			if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
				scheme = proto
			} else {
				scheme = "http"
			}
		}
		shortURL = scheme + "://" + host + "/s/" + link.Code
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"data": ShortLinkResponse{
			ID:             link.ID,
			Code:           link.Code,
			ShortURL:       shortURL,
			AccessCount:    link.AccessCount,
			LastAccessedAt: link.LastAccessedAt,
			ExpiresAt:      link.ExpiresAt,
			CreatedAt:      link.CreatedAt,
		},
	})
}

func (h *ShortLinkHandler) handleList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims := requestctx.UserFromContext(ctx)
	if claims.ID == "" {
		RespondErrorI18nAction(ctx, w, http.StatusUnauthorized, "shortlink.list", "error.unauthorized", h.i18n)
		return
	}

	userID, err := strconv.ParseInt(claims.ID, 10, 64)
	if err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "shortlink.list", "error.bad_request", h.i18n)
		return
	}

	links, err := h.Service.List(ctx, userID)
	if err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "shortlink.list", "error.internal_server_error", h.i18n)
		return
	}

	// Build response with full URLs
	host := r.Host
	scheme := "https"
	if r.TLS == nil {
		if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
			scheme = proto
		} else {
			scheme = "http"
		}
	}

	var responses []ShortLinkResponse
	for _, link := range links {
		shortURL := "/s/" + link.Code
		if host != "" {
			shortURL = scheme + "://" + host + "/s/" + link.Code
		}
		responses = append(responses, ShortLinkResponse{
			ID:             link.ID,
			Code:           link.Code,
			ShortURL:       shortURL,
			AccessCount:    link.AccessCount,
			LastAccessedAt: link.LastAccessedAt,
			ExpiresAt:      link.ExpiresAt,
			CreatedAt:      link.CreatedAt,
		})
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"data": responses,
	})
}

func (h *ShortLinkHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims := requestctx.UserFromContext(ctx)
	if claims.ID == "" {
		RespondErrorI18nAction(ctx, w, http.StatusUnauthorized, "shortlink.delete", "error.unauthorized", h.i18n)
		return
	}

	userID, err := strconv.ParseInt(claims.ID, 10, 64)
	if err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "shortlink.delete", "error.bad_request", h.i18n)
		return
	}

	// Extract link ID from path
	action := shortLinkActionPath(r.URL.Path)
	idStr := strings.TrimPrefix(action, "/delete/")
	linkID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondErrorI18nAction(ctx, w, http.StatusBadRequest, "shortlink.delete", "error.bad_request", h.i18n)
		return
	}

	if err := h.Service.Delete(ctx, userID, linkID); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			RespondErrorI18nAction(ctx, w, http.StatusNotFound, "shortlink.delete", "error.not_found", h.i18n)
			return
		}
		RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "shortlink.delete", "error.internal_server_error", h.i18n)
		return
	}

	RespondSuccessI18n(ctx, w, "success.deleted", h.i18n, nil)
}

// HandleRedirect handles short link redirection.
// This is called directly from the router for GET /s/{code}.
func (h *ShortLinkHandler) HandleRedirect(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	if code == "" {
		http.NotFound(w, r)
		return
	}

	ctx := r.Context()
	result, err := h.Service.Resolve(ctx, code)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		RespondErrorI18nAction(ctx, w, http.StatusInternalServerError, "shortlink.redirect", "error.internal_server_error", h.i18n)
		return
	}

	if result.Expired {
		RespondErrorI18nAction(ctx, w, http.StatusGone, "shortlink.redirect", "error.expired", h.i18n)
		return
	}

	// Instead of redirecting, we can serve the subscription directly
	// This provides a better UX for proxy clients
	if h.Subscription != nil && result.UserToken != "" {
		params := service.SubscriptionParams{
			Lang:      requestctx.GetLanguage(ctx),
			UserAgent: r.UserAgent(),
			Host:      r.Host,
			Scheme:    requestScheme(r),
			URL:       absoluteURL(r),
		}

		subResult, err := h.Subscription.Subscribe(ctx, result.UserToken, params)
		if err == nil && subResult != nil {
			if subResult.ContentType != "" {
				w.Header().Set("Content-Type", subResult.ContentType)
			}
			for key, value := range subResult.Headers {
				if key != "" && !strings.EqualFold(key, "content-type") {
					if strings.ContainsAny(value, "\n\r") {
						slog.Warn("subscription: skipping header with CR/LF", "key", key)
						continue
					}
					w.Header().Set(key, value)
				}
			}
			if subResult.ETag != "" {
				w.Header().Set("ETag", formatETag(subResult.ETag))
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(subResult.Payload)
			return
		}
	}

	// Fallback to redirect
	http.Redirect(w, r, result.RedirectTo, http.StatusTemporaryRedirect)
}

func shortLinkActionPath(fullPath string) string {
	idx := strings.Index(fullPath, "/shortlink")
	if idx == -1 {
		return "/"
	}
	action := fullPath[idx+len("/shortlink"):]
	if action == "" || action == "/" {
		return "/"
	}
	if !strings.HasPrefix(action, "/") {
		action = "/" + action
	}
	return action
}
