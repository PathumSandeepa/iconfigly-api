package team

import (
	"context"

	"github.com/PathumSandeepa/iconfigly-api/internal/shared/valueobject"
	"github.com/google/uuid"
)

type TeamRepository interface {
	Create(ctx context.Context, team *Team) error
	GetByID(ctx context.Context, orgID, teamID uuid.UUID) (*Team, error)
	GetBySlug(ctx context.Context, orgID uuid.UUID, slug valueobject.Slug) (*Team, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]*TeamWithMemberCount, error)
	AddMember(ctx context.Context, member *TeamMember) error
	RemoveMember(ctx context.Context, teamID, userID uuid.UUID) error
	ListMembers(ctx context.Context, teamID uuid.UUID) ([]*TeamMember, error)
	ListUserTeamMemberships(ctx context.Context, userID uuid.UUID) ([]UserTeamMembership, error)
}
