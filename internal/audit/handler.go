package audit

import (
	"net/http"

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
	mux.HandleFunc("GET /api/v1/audit-logs", h.List)
	mux.HandleFunc("GET /api/v1/audit-logs/verify", h.Verify)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	orgIDStr := r.URL.Query().Get("organization_id")
	if orgIDStr == "" {
		httputil.WriteProblem(w, r, apperror.Validation("invalid audit query", map[string]string{
			"organization_id": "is required",
		}))
		return
	}

	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		httputil.WriteProblem(w, r, apperror.Validation("invalid audit query", map[string]string{
			"organization_id": "must be a valid UUID",
		}))
		return
	}

	pageReq, err := httputil.ParsePageRequest(r)
	if err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	filter := QueryFilter{
		OrganizationID: orgID,
		Action:         r.URL.Query().Get("action"),
		TargetType:     r.URL.Query().Get("target_type"),
		Limit:          pageReq.Limit,
		Cursor:         pageReq.Cursor,
	}

	if envIDStr := r.URL.Query().Get("environment_id"); envIDStr != "" {
		envID, err := uuid.Parse(envIDStr)
		if err != nil {
			httputil.WriteProblem(w, r, apperror.Validation("invalid audit query", map[string]string{
				"environment_id": "must be a valid UUID",
			}))
			return
		}
		filter.EnvironmentID = &envID
	}

	if actorIDStr := r.URL.Query().Get("actor_user_id"); actorIDStr != "" {
		actorID, err := uuid.Parse(actorIDStr)
		if err != nil {
			httputil.WriteProblem(w, r, apperror.Validation("invalid audit query", map[string]string{
				"actor_user_id": "must be a valid UUID",
			}))
			return
		}
		filter.ActorUserID = &actorID
	}

	result, err := h.svc.QueryAuditLogs(r.Context(), filter)
	if err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, result)
}

func (h *Handler) Verify(w http.ResponseWriter, r *http.Request) {
	orgIDStr := r.URL.Query().Get("organization_id")
	if orgIDStr == "" {
		httputil.WriteProblem(w, r, apperror.Validation("invalid verification query", map[string]string{
			"organization_id": "is required",
		}))
		return
	}

	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		httputil.WriteProblem(w, r, apperror.Validation("invalid verification query", map[string]string{
			"organization_id": "must be a valid UUID",
		}))
		return
	}

	result, err := h.svc.VerifyHashChain(r.Context(), orgID)
	if err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, httputil.Envelope[*VerificationResult]{Data: result})
}
