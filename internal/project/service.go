package project

import (
	"context"

	"github.com/PathumSandeepa/iconfigly-api/internal/access"
	"github.com/PathumSandeepa/iconfigly-api/internal/audit"
	"github.com/PathumSandeepa/iconfigly-api/internal/database"
	"github.com/PathumSandeepa/iconfigly-api/internal/environment"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/valueobject"
	"github.com/google/uuid"
)

type EnvironmentRepository interface {
	CreateBatch(ctx context.Context, envs []*environment.Environment) error
}

type Service struct {
	repo       ProjectRepository
	envRepo    EnvironmentRepository
	authorizer *access.Authorizer
	auditSvc   *audit.Service
	txManager  *database.TxManager
}

func NewService(
	repo ProjectRepository,
	envRepo EnvironmentRepository,
	authorizer *access.Authorizer,
	auditSvc *audit.Service,
	txManager *database.TxManager,
) *Service {
	return &Service{
		repo:       repo,
		envRepo:    envRepo,
		authorizer: authorizer,
		auditSvc:   auditSvc,
		txManager:  txManager,
	}
}

func (s *Service) CreateProject(ctx context.Context, actor access.Actor, cmd CreateProjectCmd) (*Project, error) {
	orgScope := access.NewOrgScope(actor.OrganizationID)
	if err := s.authorizer.Require(ctx, actor, access.VerbCatalogWrite, orgScope); err != nil {
		return nil, err
	}

	slug, err := valueobject.NewSlug(cmd.Slug)
	if err != nil {
		return nil, err
	}

	p, err := NewProject(actor.OrganizationID, slug, cmd.Name, cmd.Description)
	if err != nil {
		return nil, err
	}

	envs, err := environment.DefaultEnvironments(actor.OrganizationID, p.ID)
	if err != nil {
		return nil, err
	}

	err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.Create(txCtx, p); err != nil {
			return err
		}

		if err := s.envRepo.CreateBatch(txCtx, envs); err != nil {
			return err
		}

		if s.auditSvc != nil {
			_, err := s.auditSvc.Record(txCtx, audit.RecordCmd{
				OrganizationID: actor.OrganizationID,
				ActorUserID:    &actor.UserID,
				Action:         "project.created",
				TargetType:     audit.TargetTypeProject,
				TargetID:       p.ID,
				IPAddress:      actor.ClientIP,
				Metadata: map[string]any{
					"slug":                p.Slug.String(),
					"name":                p.Name,
					"seeded_environments": []string{"dev", "qa", "uat", "prod"},
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

	return p, nil
}

func (s *Service) ListProjects(ctx context.Context, actor access.Actor) ([]*Project, error) {
	orgScope := access.NewOrgScope(actor.OrganizationID)
	if err := s.authorizer.Require(ctx, actor, access.VerbCatalogRead, orgScope); err != nil {
		return nil, err
	}

	projects, err := s.repo.ListByOrg(ctx, actor.OrganizationID)
	if err != nil {
		return nil, err
	}
	if projects == nil {
		projects = []*Project{}
	}

	return projects, nil
}

func (s *Service) GetProject(ctx context.Context, actor access.Actor, projectID uuid.UUID) (*Project, error) {
	projScope := access.NewProjectScope(actor.OrganizationID, projectID)
	if err := s.authorizer.Require(ctx, actor, access.VerbCatalogRead, projScope); err != nil {
		return nil, err
	}

	return s.repo.GetByID(ctx, actor.OrganizationID, projectID)
}
