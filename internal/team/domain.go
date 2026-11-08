package team

import (
	"strings"
	"time"

	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/valueobject"
	"github.com/google/uuid"
)

type TeamRole string

const (
	RoleLead   TeamRole = "LEAD"
	RoleMember TeamRole = "MEMBER"
)

func IsValidTeamRole(role TeamRole) bool {
	return role == RoleLead || role == RoleMember
}

type Team struct {
	ID             uuid.UUID        `json:"id"`
	OrganizationID uuid.UUID        `json:"organization_id"`
	Slug           valueobject.Slug `json:"slug"`
	Name           string           `json:"name"`
	Description    string           `json:"description"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

func NewTeam(
	orgID uuid.UUID,
	slug valueobject.Slug,
	name string,
	description string,
) (*Team, error) {
	trimmedName := strings.TrimSpace(name)
	if len(trimmedName) < 2 || len(trimmedName) > 100 {
		return nil, apperror.Validation("invalid team name", map[string]string{
			"name": "must be between 2 and 100 characters",
		})
	}

	trimmedDesc := strings.TrimSpace(description)
	if len(trimmedDesc) > 500 {
		return nil, apperror.Validation("invalid team description", map[string]string{
			"description": "must not exceed 500 characters",
		})
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	return &Team{
		ID:             uuid.Must(uuid.NewV7()),
		OrganizationID: orgID,
		Slug:           slug,
		Name:           trimmedName,
		Description:    trimmedDesc,
		CreatedAt:      now,
		UpdatedAt:      now,
	}, nil
}

type TeamMember struct {
	ID           uuid.UUID `json:"id"`
	TeamID       uuid.UUID `json:"team_id"`
	UserID       uuid.UUID `json:"user_id"`
	UserEmail    string    `json:"user_email"`
	UserFullName string    `json:"user_full_name"`
	Role         TeamRole  `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

type TeamWithMemberCount struct {
	ID             uuid.UUID        `json:"id"`
	OrganizationID uuid.UUID        `json:"organization_id"`
	Slug           valueobject.Slug `json:"slug"`
	Name           string           `json:"name"`
	Description    string           `json:"description"`
	MemberCount    int              `json:"member_count"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

type TeamDetails struct {
	Team    *Team         `json:"team"`
	Members []*TeamMember `json:"members"`
}

type UserTeamMembership struct {
	TeamID   uuid.UUID `json:"team_id"`
	TeamRole TeamRole  `json:"team_role"`
}

type CreateTeamCmd struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type AddMemberCmd struct {
	UserID uuid.UUID `json:"user_id"`
	Role   TeamRole  `json:"role"`
}
