package resource

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
	mux.HandleFunc("POST /api/v1/projects/{projectID}/resources", h.Create)
	mux.HandleFunc("GET /api/v1/projects/{projectID}/resources", h.List)
	mux.HandleFunc("GET /api/v1/projects/{projectID}/resources/{resourceID}", h.Get)
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

	var cmd CreateResourceCmd
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		httputil.WriteProblem(w, r, apperror.Validation("malformed JSON body", map[string]string{
			"body": "could not decode request body",
		}))
		return
	}

	res, err := h.svc.CreateResource(r.Context(), actor, projectID, cmd)
	if err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, httputil.Envelope[*Resource]{Data: res})
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

	var envID *uuid.UUID
	if rawEnv := r.URL.Query().Get("env_id"); rawEnv != "" {
		parsed, err := uuid.Parse(rawEnv)
		if err != nil {
			httputil.WriteProblem(w, r, apperror.Validation("invalid env_id query parameter", map[string]string{
				"env_id": "must be a valid UUID",
			}))
			return
		}
		envID = &parsed
	}

	var resType *ResourceType
	if rawType := r.URL.Query().Get("type"); rawType != "" {
		rt := ResourceType(rawType)
		if !IsValidResourceType(rt) {
			httputil.WriteProblem(w, r, apperror.Validation("invalid type query parameter", map[string]string{
				"type": "unsupported resource type",
			}))
			return
		}
		resType = &rt
	}

	resources, err := h.svc.ListResources(r.Context(), actor, projectID, envID, resType)
	if err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, httputil.Envelope[[]*Resource]{Data: resources})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
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

	resourceID, err := uuid.Parse(r.PathValue("resourceID"))
	if err != nil {
		httputil.WriteProblem(w, r, apperror.Validation("invalid resource id", map[string]string{
			"resourceID": "must be a valid UUID",
		}))
		return
	}

	res, err := h.svc.GetResource(r.Context(), actor, projectID, resourceID)
	if err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, httputil.Envelope[*Resource]{Data: res})
}
