package project

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
	mux.HandleFunc("POST /api/v1/projects", h.Create)
	mux.HandleFunc("GET /api/v1/projects", h.List)
	mux.HandleFunc("GET /api/v1/projects/{projectID}", h.Get)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	actor, ok := access.ActorFromContext(r.Context())
	if !ok {
		httputil.WriteProblem(w, r, apperror.Unauthorized("authentication required"))
		return
	}

	var cmd CreateProjectCmd
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		httputil.WriteProblem(w, r, apperror.Validation("malformed JSON body", map[string]string{
			"body": "could not decode request body",
		}))
		return
	}

	p, err := h.svc.CreateProject(r.Context(), actor, cmd)
	if err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, httputil.Envelope[*Project]{Data: p})
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	actor, ok := access.ActorFromContext(r.Context())
	if !ok {
		httputil.WriteProblem(w, r, apperror.Unauthorized("authentication required"))
		return
	}

	projects, err := h.svc.ListProjects(r.Context(), actor)
	if err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, httputil.Envelope[[]*Project]{Data: projects})
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

	p, err := h.svc.GetProject(r.Context(), actor, projectID)
	if err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, httputil.Envelope[*Project]{Data: p})
}
