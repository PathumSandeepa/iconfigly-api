package auth

import (
	"crypto/subtle"
	"net"
	"net/http"
	"strings"

	"github.com/PathumSandeepa/iconfigly-api/internal/access"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/httputil"
)

const SessionCookieName = "iconfigly_session"

func AuthenticateMiddleware(authSvc *Service) httputil.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := extractClientIP(r)

			cookie, err := r.Cookie(SessionCookieName)
			if err == nil && cookie.Value != "" {
				actor, err := authSvc.AuthenticateSession(r.Context(), cookie.Value, ip)
				if err != nil {
					httputil.WriteProblem(w, r, err)
					return
				}
				ctx := access.WithActor(r.Context(), *actor)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
					token := strings.TrimSpace(parts[1])
					actor, err := authSvc.AuthenticatePAT(r.Context(), token, ip)
					if err != nil {
						httputil.WriteProblem(w, r, err)
						return
					}
					ctx := access.WithActor(r.Context(), *actor)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
				httputil.WriteProblem(w, r, apperror.Unauthorized("invalid authorization header format"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func CSRFMiddleware() httputil.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			default:
				next.ServeHTTP(w, r)
				return
			}

			path := r.URL.Path
			if path == "/health" || path == "/ready" ||
				path == "/api/v1/setup/status" ||
				path == "/api/v1/setup/initialize" ||
				path == "/api/v1/auth/setup" ||
				path == "/api/v1/auth/login" {
				next.ServeHTTP(w, r)
				return
			}

			actor, ok := access.ActorFromContext(r.Context())
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			if actor.AuthMethod != "SESSION" {
				next.ServeHTTP(w, r)
				return
			}

			csrfHeader := r.Header.Get("X-CSRF-Token")
			if csrfHeader == "" || subtle.ConstantTimeCompare([]byte(csrfHeader), []byte(actor.CSRFToken)) != 1 {
				httputil.WriteProblem(w, r, apperror.Forbidden("invalid or missing CSRF token"))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func extractClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
