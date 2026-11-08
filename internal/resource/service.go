package resource

import (
	"context"

	"github.com/PathumSandeepa/iconfigly-api/internal/access"
	"github.com/PathumSandeepa/iconfigly-api/internal/audit"
	"github.com/PathumSandeepa/iconfigly-api/internal/database"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/valueobject"
	"github.com/PathumSandeepa/iconfigly-api/internal/team"
	"github.com/google/uuid"
)

type TeamRepository interface {
	GetByID(ctx context.Context, orgID, teamID uuid.UUID) (*team.Team, error)
}

type Service struct {
	repo       ResourceRepository
	teamRepo   TeamRepository
	authorizer *access.Authorizer
	auditSvc   *audit.Service
	txManager  *database.TxManager
}

func NewService(
	repo ResourceRepository,
	teamRepo TeamRepository,
	authorizer *access.Authorizer,
	auditSvc *audit.Service,
	txManager *database.TxManager,
) *Service {
	return &Service{
		repo:       repo,
		teamRepo:   teamRepo,
		authorizer: authorizer,
		auditSvc:   auditSvc,
		txManager:  txManager,
	}
}

func (s *Service) CreateResource(ctx context.Context, actor access.Actor, projectID uuid.UUID, cmd CreateResourceCmd) (*Resource, error) {
	envScope := access.NewEnvironmentScope(actor.OrganizationID, projectID, cmd.EnvironmentID)
	if err := s.authorizer.Require(ctx, actor, access.VerbCatalogWrite, envScope); err != nil {
		return nil, err
	}

	if _, err := s.teamRepo.GetByID(ctx, actor.OrganizationID, cmd.OwnerTeamID); err != nil {
		return nil, err
	}

	slug, err := valueobject.NewSlug(cmd.Slug)
	if err != nil {
		return nil, err
	}

	res, err := NewResource(
		actor.OrganizationID,
		projectID,
		cmd.EnvironmentID,
		cmd.OwnerTeamID,
		slug,
		cmd.Name,
		cmd.ResourceType,
		cmd.ConnectionMetadata,
	)
	if err != nil {
		return nil, err
	}

	err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.Create(txCtx, res); err != nil {
			return err
		}

		if s.auditSvc != nil {
			_, err := s.auditSvc.Record(txCtx, audit.RecordCmd{
				OrganizationID: actor.OrganizationID,
				ActorUserID:    &actor.UserID,
				Action:         "resource.created",
				TargetType:     audit.TargetTypeResource,
				TargetID:       res.ID,
				EnvironmentID:  &res.EnvironmentID,
				IPAddress:      actor.ClientIP,
				Metadata: map[string]any{
					"name":          res.Name,
					"slug":          res.Slug.String(),
					"resource_type": string(res.ResourceType),
					"owner_team_id": res.OwnerTeamID.String(),
				},
			})
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s *Service) ListResources(
	ctx context.Context,
	actor access.Actor,
	projectID uuid.UUID,
	envID *uuid.UUID,
	resType *ResourceType,
) ([]*Resource, error) {
	projScope := access.NewProjectScope(actor.OrganizationID, projectID)
	if err := s.authorizer.Require(ctx, actor, access.VerbCatalogRead, projScope); err != nil {
		return nil, err
	}

	resources, err := s.repo.List(ctx, actor.OrganizationID, projectID, envID, resType)
	if err != nil {
		return nil, err
	}
	if resources == nil {
		resources = []*Resource{}
	}

	return resources, nil
}

func (s *Service) GetResource(ctx context.Context, actor access.Actor, projectID, resourceID uuid.UUID) (*Resource, error) {
	projScope := access.NewProjectScope(actor.OrganizationID, projectID)
	if err := s.authorizer.Require(ctx, actor, access.VerbCatalogRead, projScope); err != nil {
		return nil, err
	}

	return s.repo.GetByID(ctx, actor.OrganizationID, projectID, resourceID)
}
