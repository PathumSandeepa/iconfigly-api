package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/PathumSandeepa/iconfigly-api/internal/access"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/httputil"
	"github.com/PathumSandeepa/iconfigly-api/internal/user"
	"github.com/google/uuid"
)

type Handler struct {
	svc      *Service
	userRepo UserRepository
}

func NewHandler(svc *Service, userRepo UserRepository) *Handler {
	return &Handler{
		svc:      svc,
		userRepo: userRepo,
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/setup/status", h.SetupStatus)
	mux.HandleFunc("GET /api/v1/auth/setup-status", h.SetupStatus)
	mux.HandleFunc("POST /api/v1/setup/initialize", h.InitializePlatform)
	mux.HandleFunc("POST /api/v1/auth/setup", h.InitializePlatform)
	mux.HandleFunc("POST /api/v1/auth/login", h.Login)
	mux.HandleFunc("POST /api/v1/auth/logout", h.Logout)
	mux.HandleFunc("GET /api/v1/auth/me", h.Me)
	mux.HandleFunc("POST /api/v1/auth/tokens", h.CreatePAT)
	mux.HandleFunc("GET /api/v1/auth/tokens", h.ListPATs)
	mux.HandleFunc("DELETE /api/v1/auth/tokens/{id}", h.RevokePAT)
}

func (h *Handler) SetupStatus(w http.ResponseWriter, r *http.Request) {
	initialized, err := h.svc.GetSetupStatus(r.Context())
	if err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, httputil.Envelope[SetupStatusResponse]{
		Data: SetupStatusResponse{Initialized: initialized},
	})
}

type AuthResponseData struct {
	User      *user.User `json:"user"`
	CSRFToken string     `json:"csrf_token"`
	ExpiresAt time.Time  `json:"expires_at"`
}

func (h *Handler) InitializePlatform(w http.ResponseWriter, r *http.Request) {
	var cmd SetupCmd
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		httputil.WriteProblem(w, r, apperror.Validation("malformed JSON body", map[string]string{
			"body": "could not decode request body",
		}))
		return
	}

	ip := extractClientIP(r)
	userAgent := r.UserAgent()

	result, err := h.svc.InitializePlatform(r.Context(), cmd, ip, userAgent)
	if err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	maxAge := int(time.Until(result.ExpiresAt).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    result.SessionToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  result.ExpiresAt,
		MaxAge:   maxAge,
	})

	httputil.WriteJSON(w, http.StatusCreated, httputil.Envelope[AuthResponseData]{
		Data: AuthResponseData{
			User:      result.User,
			CSRFToken: result.CSRFToken,
			ExpiresAt: result.ExpiresAt,
		},
	})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var cmd LoginCmd
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		httputil.WriteProblem(w, r, apperror.Validation("malformed JSON body", map[string]string{
			"body": "could not decode request body",
		}))
		return
	}

	ip := extractClientIP(r)
	userAgent := r.UserAgent()

	result, err := h.svc.Login(r.Context(), cmd, ip, userAgent)
	if err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	maxAge := int(time.Until(result.ExpiresAt).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    result.SessionToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  result.ExpiresAt,
		MaxAge:   maxAge,
	})

	httputil.WriteJSON(w, http.StatusOK, httputil.Envelope[AuthResponseData]{
		Data: AuthResponseData{
			User:      result.User,
			CSRFToken: result.CSRFToken,
			ExpiresAt: result.ExpiresAt,
		},
	})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(SessionCookieName)
	if err == nil && cookie.Value != "" {
		_ = h.svc.Logout(r.Context(), cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})

	w.WriteHeader(http.StatusNoContent)
}

type MeResponseData struct {
	User      *user.User   `json:"user"`
	Actor     access.Actor `json:"actor"`
	CSRFToken string       `json:"csrf_token"`
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	actor, ok := access.ActorFromContext(r.Context())
	if !ok {
		httputil.WriteProblem(w, r, apperror.Unauthorized("authentication required"))
		return
	}

	u, err := h.userRepo.GetByID(r.Context(), actor.OrganizationID, actor.UserID)
	if err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, httputil.Envelope[MeResponseData]{
		Data: MeResponseData{
			User:      u,
			Actor:     actor,
			CSRFToken: actor.CSRFToken,
		},
	})
}

func (h *Handler) CreatePAT(w http.ResponseWriter, r *http.Request) {
	actor, ok := access.ActorFromContext(r.Context())
	if !ok {
		httputil.WriteProblem(w, r, apperror.Unauthorized("authentication required"))
		return
	}

	var cmd CreatePATCmd
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		httputil.WriteProblem(w, r, apperror.Validation("malformed JSON body", map[string]string{
			"body": "could not decode request body",
		}))
		return
	}

	token, rawToken, err := h.svc.CreatePAT(r.Context(), actor, cmd)
	if err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, httputil.Envelope[CreatePATResult]{
		Data: CreatePATResult{
			Token:    token,
			RawToken: rawToken,
		},
	})
}

func (h *Handler) ListPATs(w http.ResponseWriter, r *http.Request) {
	actor, ok := access.ActorFromContext(r.Context())
	if !ok {
		httputil.WriteProblem(w, r, apperror.Unauthorized("authentication required"))
		return
	}

	tokens, err := h.svc.ListPATs(r.Context(), actor)
	if err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, httputil.Envelope[[]*PersonalAccessToken]{
		Data: tokens,
	})
}

func (h *Handler) RevokePAT(w http.ResponseWriter, r *http.Request) {
	actor, ok := access.ActorFromContext(r.Context())
	if !ok {
		httputil.WriteProblem(w, r, apperror.Unauthorized("authentication required"))
		return
	}

	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		httputil.WriteProblem(w, r, apperror.Validation("invalid token id", map[string]string{
			"id": "must be a valid UUID",
		}))
		return
	}

	if err := h.svc.RevokePAT(r.Context(), actor, id); err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
