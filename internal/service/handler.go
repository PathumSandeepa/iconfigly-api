package service

import (
	"encoding/json"
	"net/http"

	"github.com/PathumSandeepa/iconfigly-api/internal/access"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/httputil"
	"github.com/google/uuid"
)

type Handler struct {
	svc *ServiceService
}

func NewHandler(svc *ServiceService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/projects/{projectID}/services", h.Create)
	mux.HandleFunc("GET /api/v1/projects/{projectID}/services", h.List)
	mux.HandleFunc("GET /api/v1/projects/{projectID}/services/{serviceSlug}/context", h.GetContext)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	actor, ok := access.ActorFromContext(r.Context())
	if !ok {
		httputil.WriteProblem(w, r, apperror.Unauthorized("authentication required"))
		return
	}

	projectID, err := uuid.Parse(r.PathValue("projectID"))
	if err != nil {
		httputil.WriteProblem(w, r, apperror.Validation("invalid project id", map[string]string{
			"projectID": "must be a valid UUID",
		}))
		return
	}

	var cmd CreateServiceCmd
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		httputil.WriteProblem(w, r, apperror.Validation("malformed JSON body", map[string]string{
			"body": "could not decode request body",
		}))
		return
	}

	s, err := h.svc.CreateService(r.Context(), actor, projectID, cmd)
	if err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, httputil.Envelope[*Service]{Data: s})
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	actor, ok := access.ActorFromContext(r.Context())
	if !ok {
		httputil.WriteProblem(w, r, apperror.Unauthorized("authentication required"))
		return
	}

	projectID, err := uuid.Parse(r.PathValue("projectID"))
	if err != nil {
		httputil.WriteProblem(w, r, apperror.Validation("invalid project id", map[string]string{
			"projectID": "must be a valid UUID",
		}))
		return
	}

	var ownerTeamID *uuid.UUID
	if rawTeam := r.URL.Query().Get("owner_team_id"); rawTeam != "" {
		parsed, err := uuid.Parse(rawTeam)
		if err != nil {
			httputil.WriteProblem(w, r, apperror.Validation("invalid owner_team_id query parameter", map[string]string{
				"owner_team_id": "must be a valid UUID",
			}))
			return
		}
		ownerTeamID = &parsed
	}

	search := r.URL.Query().Get("search")

	services, err := h.svc.ListServices(r.Context(), actor, projectID, ownerTeamID, search)
	if err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, httputil.Envelope[[]*Service]{Data: services})
}

func (h *Handler) GetContext(w http.ResponseWriter, r *http.Request) {
	actor, ok := access.ActorFromContext(r.Context())
	if !ok {
		httputil.WriteProblem(w, r, apperror.Unauthorized("authentication required"))
		return
	}

	projectID, err := uuid.Parse(r.PathValue("projectID"))
	if err != nil {
		httputil.WriteProblem(w, r, apperror.Validation("invalid project id", map[string]string{
			"projectID": "must be a valid UUID",
		}))
		return
	}

	serviceSlug := r.PathValue("serviceSlug")
	if serviceSlug == "" {
		httputil.WriteProblem(w, r, apperror.Validation("missing service slug", map[string]string{
			"serviceSlug": "must be provided in URL path",
		}))
		return
	}

	envSlug := r.URL.Query().Get("env")
	if envSlug == "" {
		httputil.WriteProblem(w, r, apperror.Validation("missing env query parameter", map[string]string{
			"env": "required query parameter (e.g. ?env=uat)",
		}))
		return
	}

	contextView, err := h.svc.GetServiceEnvironmentContext(r.Context(), actor, projectID, serviceSlug, envSlug)
	if err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, httputil.Envelope[*ServiceContextView]{Data: contextView})
}
