package service

import (
	"context"

	"github.com/PathumSandeepa/iconfigly-api/internal/access"
	"github.com/PathumSandeepa/iconfigly-api/internal/audit"
	"github.com/PathumSandeepa/iconfigly-api/internal/binding"
	"github.com/PathumSandeepa/iconfigly-api/internal/database"
	"github.com/PathumSandeepa/iconfigly-api/internal/environment"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/valueobject"
	"github.com/PathumSandeepa/iconfigly-api/internal/team"
	"github.com/google/uuid"
)

type TeamRepository interface {
	GetByID(ctx context.Context, orgID, teamID uuid.UUID) (*team.Team, error)
}

type EnvironmentRepository interface {
	GetByID(ctx context.Context, orgID, projectID, envID uuid.UUID) (*environment.Environment, error)
	GetBySlug(ctx context.Context, orgID, projectID uuid.UUID, slug valueobject.Slug) (*environment.Environment, error)
}

type BindingRepository interface {
	ListBoundResourcesByService(ctx context.Context, serviceID, envID uuid.UUID) ([]*binding.BoundResourceView, error)
	ListDependenciesForService(ctx context.Context, serviceID, envID uuid.UUID) (upstream []*binding.ServiceDependencyView, downstream []*binding.ServiceDependencyView, err error)
}

type AuditRepository interface {
	ListPaginated(ctx context.Context, filter audit.QueryFilter) ([]*audit.AuditEntry, error)
}

type ServiceService struct {
	repo        ServiceRepository
	teamRepo    TeamRepository
	envRepo     EnvironmentRepository
	bindingRepo BindingRepository
	auditRepo   AuditRepository
	authorizer  *access.Authorizer
	auditSvc    *audit.Service
	txManager   *database.TxManager
}

func NewService(
	repo ServiceRepository,
	teamRepo TeamRepository,
	envRepo EnvironmentRepository,
	bindingRepo BindingRepository,
	auditRepo AuditRepository,
	authorizer *access.Authorizer,
	auditSvc *audit.Service,
	txManager *database.TxManager,
) *ServiceService {
	return &ServiceService{
		repo:        repo,
		teamRepo:    teamRepo,
		envRepo:     envRepo,
		bindingRepo: bindingRepo,
		auditRepo:   auditRepo,
		authorizer:  authorizer,
		auditSvc:    auditSvc,
		txManager:   txManager,
	}
}

func (s *ServiceService) CreateService(
	ctx context.Context,
	actor access.Actor,
	projectID uuid.UUID,
	cmd CreateServiceCmd,
) (*Service, error) {
	projScope := access.NewProjectScope(actor.OrganizationID, projectID)
	if err := s.authorizer.Require(ctx, actor, access.VerbCatalogWrite, projScope); err != nil {
		return nil, err
	}

	if _, err := s.teamRepo.GetByID(ctx, actor.OrganizationID, cmd.OwnerTeamID); err != nil {
		return nil, err
	}

	slug, err := valueobject.NewSlug(cmd.Slug)
	if err != nil {
		return nil, err
	}

	svc, err := NewServiceEntity(
		actor.OrganizationID,
		projectID,
		cmd.OwnerTeamID,
		slug,
		cmd.Name,
		cmd.Description,
		cmd.RepoURL,
		cmd.RunbookURL,
	)
	if err != nil {
		return nil, err
	}

	err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.Create(txCtx, svc); err != nil {
			return err
		}

		if s.auditSvc != nil {
			_, err := s.auditSvc.Record(txCtx, audit.RecordCmd{
				OrganizationID: actor.OrganizationID,
				ActorUserID:    &actor.UserID,
				Action:         "service.created",
				TargetType:     audit.TargetTypeService,
				TargetID:       svc.ID,
				IPAddress:      actor.ClientIP,
				Metadata: map[string]any{
					"name":          svc.Name,
					"slug":          svc.Slug.String(),
					"owner_team_id": svc.OwnerTeamID.String(),
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

	return svc, nil
}

func (s *ServiceService) ListServices(
	ctx context.Context,
	actor access.Actor,
	projectID uuid.UUID,
	ownerTeamID *uuid.UUID,
	search string,
) ([]*Service, error) {
	projScope := access.NewProjectScope(actor.OrganizationID, projectID)
	if err := s.authorizer.Require(ctx, actor, access.VerbCatalogRead, projScope); err != nil {
		return nil, err
	}

	services, err := s.repo.ListByProject(ctx, actor.OrganizationID, projectID, ownerTeamID, search)
	if err != nil {
		return nil, err
	}
	if services == nil {
		services = []*Service{}
	}

	return services, nil
}

func (s *ServiceService) GetService(
	ctx context.Context,
	actor access.Actor,
	projectID, serviceID uuid.UUID,
) (*Service, error) {
	projScope := access.NewProjectScope(actor.OrganizationID, projectID)
	if err := s.authorizer.Require(ctx, actor, access.VerbCatalogRead, projScope); err != nil {
		return nil, err
	}

	return s.repo.GetByID(ctx, actor.OrganizationID, projectID, serviceID)
}

func (s *ServiceService) GetServiceEnvironmentContext(
	ctx context.Context,
	actor access.Actor,
	projectID uuid.UUID,
	serviceSlug string,
	envSlug string,
) (*ServiceContextView, error) {
	sSlug, err := valueobject.NewSlug(serviceSlug)
	if err != nil {
		return nil, err
	}

	eSlug, err := valueobject.NewSlug(envSlug)
	if err != nil {
		return nil, err
	}

	svc, err := s.repo.GetBySlug(ctx, actor.OrganizationID, projectID, sSlug)
	if err != nil {
		return nil, err
	}

	env, err := s.envRepo.GetBySlug(ctx, actor.OrganizationID, projectID, eSlug)
	if err != nil {
		return nil, err
	}

	svcScope := access.NewServiceScope(actor.OrganizationID, projectID, env.ID, svc.ID)
	if err := s.authorizer.Require(ctx, actor, access.VerbCatalogRead, svcScope); err != nil {
		return nil, err
	}

	role, _ := s.authorizer.ResolveEffectiveRole(ctx, actor, svcScope)

	ownerTeam, _ := s.teamRepo.GetByID(ctx, actor.OrganizationID, svc.OwnerTeamID)
	teamSummary := OwnerTeamSummary{}
	if ownerTeam != nil {
		teamSummary = OwnerTeamSummary{
			ID:   ownerTeam.ID,
			Slug: ownerTeam.Slug.String(),
			Name: ownerTeam.Name,
		}
	}

	envSummary := EnvironmentSummary{
		ID:           env.ID,
		Slug:         env.Slug.String(),
		Name:         env.Name,
		TierOrder:    env.TierOrder,
		IsProduction: env.IsProduction,
	}

	boundViews, _ := s.bindingRepo.ListBoundResourcesByService(ctx, svc.ID, env.ID)
	boundItems := make([]BoundResourceItem, 0, len(boundViews))
	for _, bv := range boundViews {
		boundItems = append(boundItems, BoundResourceItem{
			ID:                 bv.ResourceID,
			BindingID:          bv.BindingID,
			Slug:               bv.ResourceSlug,
			Name:               bv.ResourceName,
			ResourceType:       bv.ResourceType,
			AccessMode:         bv.AccessMode,
			ConnectionMetadata: bv.ConnectionMetadata,
			Notes:              bv.Notes,
		})
	}

	upstreamViews, downstreamViews, _ := s.bindingRepo.ListDependenciesForService(ctx, svc.ID, env.ID)
	upstreamItems := make([]DependencyItem, 0, len(upstreamViews))
	for _, uv := range upstreamViews {
		upstreamItems = append(upstreamItems, DependencyItem{
			ServiceID:   uv.ServiceID,
			ServiceSlug: uv.ServiceSlug,
			ServiceName: uv.ServiceName,
			Protocol:    uv.Protocol,
		})
	}
	downstreamItems := make([]DependencyItem, 0, len(downstreamViews))
	for _, dv := range downstreamViews {
		downstreamItems = append(downstreamItems, DependencyItem{
			ServiceID:   dv.ServiceID,
			ServiceSlug: dv.ServiceSlug,
			ServiceName: dv.ServiceName,
			Protocol:    dv.Protocol,
		})
	}

	var latestAudit *LatestAuditSummary
	if s.auditRepo != nil {
		entries, err := s.auditRepo.ListPaginated(ctx, audit.QueryFilter{
			OrganizationID: actor.OrganizationID,
			EnvironmentID:  &env.ID,
			Limit:          1,
		})
		if err == nil && len(entries) > 0 {
			latestAudit = &LatestAuditSummary{
				Action:      entries[0].Action,
				ActorUserID: entries[0].ActorUserID,
				Timestamp:   entries[0].CreatedAt,
			}
		}
	}

	return &ServiceContextView{
		Service:                svc,
		OwnerTeam:              teamSummary,
		Environment:            envSummary,
		BoundResources:         boundItems,
		UpstreamDependencies:   upstreamItems,
		DownstreamDependencies: downstreamItems,
		CallerEffectiveRole:    string(role),
		LatestAuditEntry:       latestAudit,
	}, nil
}
