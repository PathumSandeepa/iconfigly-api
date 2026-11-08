package project

import (
	"context"

	"github.com/PathumSandeepa/iconfigly-api/internal/shared/valueobject"
	"github.com/google/uuid"
)

type ProjectRepository interface {
	Create(ctx context.Context, project *Project) error
	GetByID(ctx context.Context, orgID, projectID uuid.UUID) (*Project, error)
	GetBySlug(ctx context.Context, orgID uuid.UUID, slug valueobject.Slug) (*Project, error)
	ListByOrg(ctx context.Context, orgID uuid.UUID) ([]*Project, error)
}
