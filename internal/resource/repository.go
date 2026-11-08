package resource

import (
	"context"

	"github.com/PathumSandeepa/iconfigly-api/internal/shared/valueobject"
	"github.com/google/uuid"
)

type ResourceRepository interface {
	Create(ctx context.Context, r *Resource) error
	GetByID(ctx context.Context, orgID, projectID, resourceID uuid.UUID) (*Resource, error)
	GetBySlug(ctx context.Context, orgID, projectID, envID uuid.UUID, slug valueobject.Slug) (*Resource, error)
	List(ctx context.Context, orgID, projectID uuid.UUID, envID *uuid.UUID, resType *ResourceType) ([]*Resource, error)
}
