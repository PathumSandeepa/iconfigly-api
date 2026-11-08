package organization

import (
	"context"

	"github.com/PathumSandeepa/iconfigly-api/internal/shared/valueobject"
	"github.com/google/uuid"
)

type OrganizationRepository interface {
	Count(ctx context.Context) (int64, error)
	Create(ctx context.Context, org *Organization) error
	GetByID(ctx context.Context, id uuid.UUID) (*Organization, error)
	GetBySlug(ctx context.Context, slug valueobject.Slug) (*Organization, error)
	Update(ctx context.Context, org *Organization) error
}
