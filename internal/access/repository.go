package access

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type AccessRepository interface {
	Create(ctx context.Context, binding *RoleBinding) error
	GetByID(ctx context.Context, orgID uuid.UUID, id uuid.UUID) (*RoleBinding, error)
	Delete(ctx context.Context, orgID uuid.UUID, id uuid.UUID) error
	ListByProject(ctx context.Context, orgID uuid.UUID, projectID uuid.UUID) ([]*RoleBinding, error)
	ListByScope(ctx context.Context, orgID uuid.UUID, scopeType ScopeType, scopeID uuid.UUID) ([]*RoleBinding, error)
	FindActiveBindingsForActor(ctx context.Context, orgID uuid.UUID, userID uuid.UUID, teamIDs []uuid.UUID, scopes []ScopeRef, now time.Time) ([]*RoleBinding, error)
	FindActiveBindingsForScopes(ctx context.Context, orgID uuid.UUID, scopes []ScopeRef, now time.Time) ([]*RoleBinding, error)
}
