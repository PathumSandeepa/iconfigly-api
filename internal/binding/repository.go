package binding

import (
	"context"

	"github.com/google/uuid"
)

type BindingRepository interface {
	CreateBinding(ctx context.Context, b *ServiceResourceBinding) error
	DeleteBinding(ctx context.Context, orgID, projectID, bindingID uuid.UUID) (*ServiceResourceBinding, error)
	ListBindingsByEnvironment(ctx context.Context, orgID, projectID, envID uuid.UUID) ([]*ServiceResourceBinding, error)
	ListBoundResourcesByService(ctx context.Context, serviceID, envID uuid.UUID) ([]*BoundResourceView, error)
	CreateDependency(ctx context.Context, d *ServiceDependency) error
	ListDependenciesByEnvironment(ctx context.Context, orgID, projectID, envID uuid.UUID) ([]*ServiceDependency, error)
	ListDependenciesForService(ctx context.Context, serviceID, envID uuid.UUID) (upstream []*ServiceDependencyView, downstream []*ServiceDependencyView, err error)
}
