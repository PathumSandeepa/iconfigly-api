package environment

import (
	"encoding/json"
	"net/http"

	"github.com/PathumSandeepa/iconfigly-api/internal/access"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/httputil"
	"github.com/google/uuid"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/projects/{projectID}/environments", h.List)
	mux.HandleFunc("PATCH /api/v1/projects/{projectID}/environments/{envID}", h.UpdatePolicy)
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

	envs, err := h.svc.ListEnvironments(r.Context(), actor, projectID)
	if err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, httputil.Envelope[[]*Environment]{Data: envs})
}

func (h *Handler) UpdatePolicy(w http.ResponseWriter, r *http.Request) {
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

	envID, err := uuid.Parse(r.PathValue("envID"))
	if err != nil {
		httputil.WriteProblem(w, r, apperror.Validation("invalid environment id", map[string]string{
			"envID": "must be a valid UUID",
		}))
		return
	}

	var cmd UpdatePolicyCmd
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		httputil.WriteProblem(w, r, apperror.Validation("malformed JSON body", map[string]string{
			"body": "could not decode request body",
		}))
		return
	}

	env, err := h.svc.UpdateEnvironmentPolicy(r.Context(), actor, projectID, envID, cmd)
	if err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, httputil.Envelope[*Environment]{Data: env})
}
