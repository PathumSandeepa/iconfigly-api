package access

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/httputil"
	"github.com/google/uuid"
)

type actorContextKey struct{}

func WithActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, actorContextKey{}, actor)
}

func ActorFromContext(ctx context.Context) (Actor, bool) {
	actor, ok := ctx.Value(actorContextKey{}).(Actor)
	return actor, ok
}

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/projects/{projectID}/access/bindings", h.ListBindings)
	mux.HandleFunc("POST /api/v1/projects/{projectID}/access/bindings", h.GrantBinding)
	mux.HandleFunc("DELETE /api/v1/projects/{projectID}/access/bindings/{bindingID}", h.RevokeBinding)
	mux.HandleFunc("GET /api/v1/projects/{projectID}/access/inspect", h.InspectAccess)
}

func (h *Handler) ListBindings(w http.ResponseWriter, r *http.Request) {
	actor, err := h.resolveActor(r)
	if err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	projectID, err := uuid.Parse(r.PathValue("projectID"))
	if err != nil {
		httputil.WriteProblem(w, r, apperror.Validation("invalid project id", map[string]string{
			"projectID": "must be a valid UUID",
		}))
		return
	}

	filter := ListBindingsFilter{
		ScopeType: ScopeType(r.URL.Query().Get("scope_type")),
	}
	if scopeIDStr := r.URL.Query().Get("scope_id"); scopeIDStr != "" {
		scopeID, err := uuid.Parse(scopeIDStr)
		if err != nil {
			httputil.WriteProblem(w, r, apperror.Validation("invalid scope id", map[string]string{
				"scope_id": "must be a valid UUID",
			}))
			return
		}
		filter.ScopeID = scopeID
	}

	targetScope := NewProjectScope(actor.OrganizationID, projectID)
	bindings, err := h.svc.ListBindings(r.Context(), actor, targetScope, projectID, filter)
	if err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, httputil.Envelope[[]*RoleBinding]{Data: bindings})
}

func (h *Handler) GrantBinding(w http.ResponseWriter, r *http.Request) {
	actor, err := h.resolveActor(r)
	if err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	projectID, err := uuid.Parse(r.PathValue("projectID"))
	if err != nil {
		httputil.WriteProblem(w, r, apperror.Validation("invalid project id", map[string]string{
			"projectID": "must be a valid UUID",
		}))
		return
	}

	var cmd GrantRoleCmd
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		httputil.WriteProblem(w, r, apperror.Validation("malformed JSON body", map[string]string{
			"body": "could not decode request body",
		}))
		return
	}

	targetScope := Scope{
		Type:           cmd.ScopeType,
		ID:             cmd.ScopeID,
		OrganizationID: actor.OrganizationID,
		ProjectID:      &projectID,
	}

	binding, err := h.svc.GrantRole(r.Context(), actor, targetScope, cmd)
	if err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, httputil.Envelope[*RoleBinding]{Data: binding})
}

func (h *Handler) RevokeBinding(w http.ResponseWriter, r *http.Request) {
	actor, err := h.resolveActor(r)
	if err != nil {
		httputil.WriteProblem(w, r, err)
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

	targetScope := NewProjectScope(actor.OrganizationID, projectID)
	if err := h.svc.RevokeRole(r.Context(), actor, targetScope, bindingID); err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

func (h *Handler) InspectAccess(w http.ResponseWriter, r *http.Request) {
	actor, err := h.resolveActor(r)
	if err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	projectID, err := uuid.Parse(r.PathValue("projectID"))
	if err != nil {
		httputil.WriteProblem(w, r, apperror.Validation("invalid project id", map[string]string{
			"projectID": "must be a valid UUID",
		}))
		return
	}

	scopeTypeStr := r.URL.Query().Get("scope_type")
	scopeIDStr := r.URL.Query().Get("scope_id")

	if scopeTypeStr == "" || scopeIDStr == "" {
		httputil.WriteProblem(w, r, apperror.Validation("missing parameters", map[string]string{
			"scope_type": "is required",
			"scope_id":   "is required",
		}))
		return
	}

	scopeID, err := uuid.Parse(scopeIDStr)
	if err != nil {
		httputil.WriteProblem(w, r, apperror.Validation("invalid scope id", map[string]string{
			"scope_id": "must be a valid UUID",
		}))
		return
	}

	var envIDPtr *uuid.UUID
	if envIDStr := r.URL.Query().Get("environment_id"); envIDStr != "" {
		if envID, err := uuid.Parse(envIDStr); err == nil {
			envIDPtr = &envID
		}
	}

	targetScope := Scope{
		Type:           ScopeType(scopeTypeStr),
		ID:             scopeID,
		OrganizationID: actor.OrganizationID,
		ProjectID:      &projectID,
		EnvironmentID:  envIDPtr,
	}

	inspection, err := h.svc.InspectTargetAccess(r.Context(), actor, targetScope)
	if err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, httputil.Envelope[*TargetAccessInspection]{Data: inspection})
}

func (h *Handler) resolveActor(r *http.Request) (Actor, error) {
	if actor, ok := ActorFromContext(r.Context()); ok {
		return actor, nil
	}

	orgIDStr := r.Header.Get("X-Organization-ID")
	if orgIDStr == "" {
		orgIDStr = r.URL.Query().Get("organization_id")
	}

	if orgIDStr == "" {
		return Actor{}, apperror.Unauthorized("authentication required")
	}

	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		return Actor{}, apperror.Validation("invalid organization id", map[string]string{
			"organization_id": "must be a valid UUID",
		})
	}

	userID := uuid.Nil
	if userIDStr := r.Header.Get("X-User-ID"); userIDStr != "" {
		if parsed, err := uuid.Parse(userIDStr); err == nil {
			userID = parsed
		}
	}
	if userID == uuid.Nil {
		userID = uuid.Must(uuid.NewV7())
	}

	isSuperAdmin := r.Header.Get("X-Is-Super-Admin") == "true"

	return Actor{
		UserID:         userID,
		OrganizationID: orgID,
		IsSuperAdmin:   isSuperAdmin,
		TeamIDs:        []uuid.UUID{},
		LeadTeamIDs:    []uuid.UUID{},
	}, nil
}
