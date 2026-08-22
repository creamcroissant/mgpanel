// 文件路径: internal/api/middleware/auth.go
// 模块说明: 这是 internal 模块里的 auth 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package middleware

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/creamcroissant/mgpanel/internal/api/requestctx"
	"github.com/creamcroissant/mgpanel/internal/service"
	"github.com/go-chi/chi/v5"
)

// AdminGuard ensures requests originate from authenticated admins.
func AdminGuard(auth service.AuthService, paths service.AdminPathService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if paths != nil {
				expected, err := paths.SecurePath(r.Context())
				if err != nil {
					writeUnauthorized(w, "admin secure path unavailable")
					return
				}
				if expected != "" {
					param := strings.TrimSpace(chi.URLParam(r, "securePath"))
					if param == "" || param != expected {
						writeNotFound(w)
						return
					}
				}
			}
			if auth == nil {
				writeUnauthorized(w, "auth service unavailable")
				return
			}
			token := extractBearer(r.Header.Get("Authorization"))
			if token == "" {
				writeUnauthorized(w, "missing authorization header")
				return
			}
			claims, err := auth.Verify(r.Context(), token)
			if err != nil {
				slog.Warn("auth: admin guard rejected", "error", err)
				writeUnauthorized(w, "invalid or expired token")
				return
			}
			if !claims.IsAdmin {
				writeForbidden(w, "admin privileges required")
				return
			}
			ctx := requestctx.WithAdminClaims(r.Context(), requestctx.AdminClaims{ID: strconv.FormatInt(claims.UserID, 10), Email: claims.Email})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserGuard ensures requests are authenticated end users.
func UserGuard(auth service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if auth == nil {
				writeUnauthorized(w, "auth service unavailable")
				return
			}
			token := extractBearer(r.Header.Get("Authorization"))
			claims, err := auth.Verify(r.Context(), token)
			if err != nil {
				slog.Warn("auth: user guard rejected", "error", err)
				writeUnauthorized(w, "invalid or expired token")
				return
			}
			ctx := requestctx.WithUserClaims(r.Context(), requestctx.UserClaims{ID: strconv.FormatInt(claims.UserID, 10), Email: claims.Email})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ServerGuard ensures requests originate from trusted nodes.
func ServerGuard(auth service.ServerAuthService, defaultType string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if auth == nil {
				writeUnauthorized(w, "server auth unavailable")
				return
			}
			token, nodeID, nodeType, err := extractServerCredentials(r)
			if err != nil {
				slog.Warn("auth: server guard rejected", "error", err)
				writeBadRequest(w, "invalid server credentials")
				return
			}
			if nodeID == "" {
				writeUnprocessable(w, "node_id is required")
				return
			}
			if token == "" {
				writeUnprocessable(w, "token is required")
				return
			}
			if nodeType == "" {
				nodeType = defaultType
			}
			server, err := auth.Authenticate(r.Context(), token, nodeID, nodeType)
			if err != nil {
				switch {
				case errors.Is(err, service.ErrUnauthorized):
					writeUnauthorized(w, "invalid server credentials")
				case errors.Is(err, service.ErrInvalidServerType):
					writeUnprocessable(w, "invalid node_type")
				case errors.Is(err, service.ErrNotFound):
					writeNotFound(w)
				default:
					writeServerError(w, "server authentication failed")
				}
				return
			}
			claims := requestctx.ServerClaims{
				ID:     strconv.FormatInt(server.ID, 10),
				Type:   server.Type,
				Server: server,
			}
			ctx := requestctx.WithServerClaims(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractServerCredentials(r *http.Request) (token, nodeID, nodeType string, err error) {
	if r == nil {
		return "", "", "", errors.New("request unavailable / 请求不可用")
	}
	query := r.URL.Query()
	token = strings.TrimSpace(query.Get("token"))
	nodeID = strings.TrimSpace(query.Get("node_id"))
	nodeType = strings.TrimSpace(query.Get("node_type"))
	if shouldParseForm(r) {
		if err = r.ParseForm(); err != nil {
			return "", "", "", err
		}
		if token == "" && r.Form != nil {
			token = strings.TrimSpace(r.Form.Get("token"))
		}
		if nodeID == "" && r.Form != nil {
			nodeID = strings.TrimSpace(r.Form.Get("node_id"))
		}
		if nodeType == "" && r.Form != nil {
			nodeType = strings.TrimSpace(r.Form.Get("node_type"))
		}
	}
	return token, nodeID, nodeType, nil
}

func shouldParseForm(r *http.Request) bool {
	if r == nil {
		return false
	}
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	return strings.Contains(contentType, "application/x-www-form-urlencoded") || strings.Contains(contentType, "multipart/form-data")
}

func extractBearer(header string) string {
	trimmed := strings.TrimSpace(header)
	if trimmed == "" {
		return ""
	}
	parts := strings.SplitN(trimmed, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return strings.TrimSpace(parts[1])
	}
	return ""
}

func writeUnauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func writeNotImplemented(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func writeBadRequest(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func writeUnprocessable(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnprocessableEntity)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func writeServerError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func writeForbidden(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func writeNotFound(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": "not found",
	})
}
