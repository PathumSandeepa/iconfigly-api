package service

import (
	"context"

	"github.com/PathumSandeepa/iconfigly-api/internal/shared/valueobject"
	"github.com/google/uuid"
)

type ServiceRepository interface {
	Create(ctx context.Context, s *Service) error
	GetByID(ctx context.Context, orgID, projectID, serviceID uuid.UUID) (*Service, error)
	GetBySlug(ctx context.Context, orgID, projectID uuid.UUID, slug valueobject.Slug) (*Service, error)
	ListByProject(ctx context.Context, orgID, projectID uuid.UUID, ownerTeamID *uuid.UUID, search string) ([]*Service, error)
}
