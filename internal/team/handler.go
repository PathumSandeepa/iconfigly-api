package team

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
	mux.HandleFunc("POST /api/v1/teams", h.HandleCreateTeam)
	mux.HandleFunc("GET /api/v1/teams", h.HandleListTeams)
	mux.HandleFunc("GET /api/v1/teams/{teamID}", h.HandleGetTeam)
	mux.HandleFunc("POST /api/v1/teams/{teamID}/members", h.HandleAddMember)
	mux.HandleFunc("DELETE /api/v1/teams/{teamID}/members/{userID}", h.HandleRemoveMember)
}

func (h *Handler) HandleCreateTeam(w http.ResponseWriter, r *http.Request) {
	actor, ok := access.ActorFromContext(r.Context())
	if !ok {
		httputil.WriteProblem(w, r, apperror.Unauthorized("authentication required"))
		return
	}

	var cmd CreateTeamCmd
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		httputil.WriteProblem(w, r, apperror.Validation("malformed JSON body", map[string]string{
			"body": "could not decode request body",
		}))
		return
	}

	t, err := h.svc.CreateTeam(r.Context(), actor, cmd)
	if err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, httputil.Envelope[*Team]{Data: t})
}

func (h *Handler) HandleListTeams(w http.ResponseWriter, r *http.Request) {
	actor, ok := access.ActorFromContext(r.Context())
	if !ok {
		httputil.WriteProblem(w, r, apperror.Unauthorized("authentication required"))
		return
	}

	teams, err := h.svc.ListTeams(r.Context(), actor)
	if err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, httputil.Envelope[[]*TeamWithMemberCount]{Data: teams})
}

func (h *Handler) HandleGetTeam(w http.ResponseWriter, r *http.Request) {
	actor, ok := access.ActorFromContext(r.Context())
	if !ok {
		httputil.WriteProblem(w, r, apperror.Unauthorized("authentication required"))
		return
	}

	teamID, err := uuid.Parse(r.PathValue("teamID"))
	if err != nil {
		httputil.WriteProblem(w, r, apperror.Validation("invalid team id", map[string]string{
			"teamID": "must be a valid UUID",
		}))
		return
	}

	details, err := h.svc.GetTeamDetails(r.Context(), actor, teamID)
	if err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusOK, httputil.Envelope[*TeamDetails]{Data: details})
}

func (h *Handler) HandleAddMember(w http.ResponseWriter, r *http.Request) {
	actor, ok := access.ActorFromContext(r.Context())
	if !ok {
		httputil.WriteProblem(w, r, apperror.Unauthorized("authentication required"))
		return
	}

	teamID, err := uuid.Parse(r.PathValue("teamID"))
	if err != nil {
		httputil.WriteProblem(w, r, apperror.Validation("invalid team id", map[string]string{
			"teamID": "must be a valid UUID",
		}))
		return
	}

	var cmd AddMemberCmd
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		httputil.WriteProblem(w, r, apperror.Validation("malformed JSON body", map[string]string{
			"body": "could not decode request body",
		}))
		return
	}

	member, err := h.svc.AddTeamMember(r.Context(), actor, teamID, cmd.UserID, cmd.Role)
	if err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	httputil.WriteJSON(w, http.StatusCreated, httputil.Envelope[*TeamMember]{Data: member})
}

func (h *Handler) HandleRemoveMember(w http.ResponseWriter, r *http.Request) {
	actor, ok := access.ActorFromContext(r.Context())
	if !ok {
		httputil.WriteProblem(w, r, apperror.Unauthorized("authentication required"))
		return
	}

	teamID, err := uuid.Parse(r.PathValue("teamID"))
	if err != nil {
		httputil.WriteProblem(w, r, apperror.Validation("invalid team id", map[string]string{
			"teamID": "must be a valid UUID",
		}))
		return
	}

	userID, err := uuid.Parse(r.PathValue("userID"))
	if err != nil {
		httputil.WriteProblem(w, r, apperror.Validation("invalid user id", map[string]string{
			"userID": "must be a valid UUID",
		}))
		return
	}

	if err := h.svc.RemoveTeamMember(r.Context(), actor, teamID, userID); err != nil {
		httputil.WriteProblem(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
