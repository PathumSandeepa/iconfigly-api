package environment

import (
	"context"

	"github.com/PathumSandeepa/iconfigly-api/internal/shared/valueobject"
	"github.com/google/uuid"
)

type EnvironmentRepository interface {
	Create(ctx context.Context, env *Environment) error
	CreateBatch(ctx context.Context, envs []*Environment) error
	GetByID(ctx context.Context, orgID, projectID, envID uuid.UUID) (*Environment, error)
	GetBySlug(ctx context.Context, orgID, projectID uuid.UUID, slug valueobject.Slug) (*Environment, error)
	ListByProject(ctx context.Context, orgID, projectID uuid.UUID) ([]*Environment, error)
	UpdatePolicy(ctx context.Context, orgID, projectID, envID uuid.UUID, isProduction bool, requiredApprovals int) error
}
