package binding

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
	mux.HandleFunc("POST /api/v1/projects/{projectID}/bindings", h.Bind)
	mux.HandleFunc("DELETE /api/v1/projects/{projectID}/bindings/{bindingID}", h.RemoveBinding)
	mux.HandleFunc("POST /api/v1/projects/{projectID}/dependencies", h.AddDependency)
	mux.HandleFunc("GET /api/v1/projects/{projectID}/graph", h.GetGraph)
}

func (h *Handler) Bind(w http.ResponseWriter, r *http.Request) {
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

	var cmd BindServiceCmd
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		httputil.WriteProblem(w, r, apperror.Validation("malformed JSON body", map[string]string{
			"body": "could not decode request body",
		}))
		return
	}

	b, err := h.svc.BindServiceToResource(r.Context(), actor, projectID, cmd)
	if err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, httputil.Envelope[*ServiceResourceBinding]{Data: b})
}

func (h *Handler) RemoveBinding(w http.ResponseWriter, r *http.Request) {
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

	bindingID, err := uuid.Parse(r.PathValue("bindingID"))
	if err != nil {
		httputil.WriteProblem(w, r, apperror.Validation("invalid binding id", map[string]string{
			"bindingID": "must be a valid UUID",
		}))
		return
	}

	if err := h.svc.RemoveBinding(r.Context(), actor, projectID, bindingID); err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) AddDependency(w http.ResponseWriter, r *http.Request) {
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

	var cmd AddDependencyCmd
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		httputil.WriteProblem(w, r, apperror.Validation("malformed JSON body", map[string]string{
			"body": "could not decode request body",
		}))
		return
	}

	dep, err := h.svc.AddServiceDependency(r.Context(), actor, projectID, cmd)
	if err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, httputil.Envelope[*ServiceDependency]{Data: dep})
}

func (h *Handler) GetGraph(w http.ResponseWriter, r *http.Request) {
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

	rawEnv := r.URL.Query().Get("env_id")
	if rawEnv == "" {
		httputil.WriteProblem(w, r, apperror.Validation("missing env_id query parameter", map[string]string{
			"env_id": "required query parameter to filter graph by environment",
		}))
		return
	}

	envID, err := uuid.Parse(rawEnv)
	if err != nil {
		httputil.WriteProblem(w, r, apperror.Validation("invalid env_id query parameter", map[string]string{
			"env_id": "must be a valid UUID",
		}))
		return
	}

	graph, err := h.svc.GetProjectTopologyGraph(r.Context(), actor, projectID, envID)
	if err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, httputil.Envelope[*TopologyGraph]{Data: graph})
}
