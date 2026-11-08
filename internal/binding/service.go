package binding

import (
	"context"

	"github.com/PathumSandeepa/iconfigly-api/internal/access"
	"github.com/PathumSandeepa/iconfigly-api/internal/audit"
	"github.com/PathumSandeepa/iconfigly-api/internal/database"
	"github.com/PathumSandeepa/iconfigly-api/internal/resource"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/google/uuid"
)

type ServiceSummary struct {
	ID   uuid.UUID `json:"id"`
	Slug string    `json:"slug"`
	Name string    `json:"name"`
}

type ServiceChecker interface {
	GetByID(ctx context.Context, orgID, projectID, serviceID uuid.UUID) (*ServiceSummary, error)
	ListByProject(ctx context.Context, orgID, projectID uuid.UUID) ([]*ServiceSummary, error)
}

type ResourceChecker interface {
	GetByID(ctx context.Context, orgID, projectID, resourceID uuid.UUID) (*resource.Resource, error)
	List(ctx context.Context, orgID, projectID uuid.UUID, envID *uuid.UUID, resType *resource.ResourceType) ([]*resource.Resource, error)
}

type Service struct {
	repo            BindingRepository
	serviceChecker  ServiceChecker
	resourceChecker ResourceChecker
	authorizer      *access.Authorizer
	auditSvc        *audit.Service
	txManager       *database.TxManager
}

func NewService(
	repo BindingRepository,
	serviceChecker ServiceChecker,
	resourceChecker ResourceChecker,
	authorizer *access.Authorizer,
	auditSvc *audit.Service,
	txManager *database.TxManager,
) *Service {
	return &Service{
		repo:            repo,
		serviceChecker:  serviceChecker,
		resourceChecker: resourceChecker,
		authorizer:      authorizer,
		auditSvc:        auditSvc,
		txManager:       txManager,
	}
}

func (s *Service) BindServiceToResource(
	ctx context.Context,
	actor access.Actor,
	projectID uuid.UUID,
	cmd BindServiceCmd,
) (*ServiceResourceBinding, error) {
	envScope := access.NewEnvironmentScope(actor.OrganizationID, projectID, cmd.EnvironmentID)
	if err := s.authorizer.Require(ctx, actor, access.VerbCatalogWrite, envScope); err != nil {
		return nil, err
	}

	if _, err := s.serviceChecker.GetByID(ctx, actor.OrganizationID, projectID, cmd.ServiceID); err != nil {
		return nil, err
	}

	res, err := s.resourceChecker.GetByID(ctx, actor.OrganizationID, projectID, cmd.ResourceID)
	if err != nil {
		return nil, err
	}

	if res.EnvironmentID != cmd.EnvironmentID {
		return nil, apperror.Validation("cross-environment binding forbidden", map[string]string{
			"environment_id": "service can only bind to a resource in the matching environment",
		})
	}

	binding, err := NewServiceResourceBinding(
		actor.OrganizationID,
		projectID,
		cmd.EnvironmentID,
		cmd.ServiceID,
		cmd.ResourceID,
		cmd.AccessMode,
		cmd.Notes,
	)
	if err != nil {
		return nil, err
	}

	err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.CreateBinding(txCtx, binding); err != nil {
			return err
		}

		if s.auditSvc != nil {
			_, err := s.auditSvc.Record(txCtx, audit.RecordCmd{
				OrganizationID: actor.OrganizationID,
				ActorUserID:    &actor.UserID,
				Action:         "binding.created",
				TargetType:     audit.TargetTypeBinding,
				TargetID:       binding.ID,
				EnvironmentID:  &cmd.EnvironmentID,
				IPAddress:      actor.ClientIP,
				Metadata: map[string]any{
					"service_id":  cmd.ServiceID.String(),
					"resource_id": cmd.ResourceID.String(),
					"access_mode": string(cmd.AccessMode),
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

	return binding, nil
}

func (s *Service) RemoveBinding(ctx context.Context, actor access.Actor, projectID, bindingID uuid.UUID) error {
	return s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		b, err := s.repo.DeleteBinding(txCtx, actor.OrganizationID, projectID, bindingID)
		if err != nil {
			return err
		}

		envScope := access.NewEnvironmentScope(actor.OrganizationID, projectID, b.EnvironmentID)
		if err := s.authorizer.Require(ctx, actor, access.VerbCatalogWrite, envScope); err != nil {
			return err
		}

		if s.auditSvc != nil {
			_, err := s.auditSvc.Record(txCtx, audit.RecordCmd{
				OrganizationID: actor.OrganizationID,
				ActorUserID:    &actor.UserID,
				Action:         "binding.removed",
				TargetType:     audit.TargetTypeBinding,
				TargetID:       bindingID,
				EnvironmentID:  &b.EnvironmentID,
				IPAddress:      actor.ClientIP,
				Metadata: map[string]any{
					"service_id":  b.ServiceID.String(),
					"resource_id": b.ResourceID.String(),
				},
			})
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (s *Service) AddServiceDependency(
	ctx context.Context,
	actor access.Actor,
	projectID uuid.UUID,
	cmd AddDependencyCmd,
) (*ServiceDependency, error) {
	envScope := access.NewEnvironmentScope(actor.OrganizationID, projectID, cmd.EnvironmentID)
	if err := s.authorizer.Require(ctx, actor, access.VerbCatalogWrite, envScope); err != nil {
		return nil, err
	}

	if _, err := s.serviceChecker.GetByID(ctx, actor.OrganizationID, projectID, cmd.CallerServiceID); err != nil {
		return nil, err
	}

	if _, err := s.serviceChecker.GetByID(ctx, actor.OrganizationID, projectID, cmd.CalleeServiceID); err != nil {
		return nil, err
	}

	dep, err := NewServiceDependency(
		actor.OrganizationID,
		projectID,
		cmd.EnvironmentID,
		cmd.CallerServiceID,
		cmd.CalleeServiceID,
		cmd.Protocol,
	)
	if err != nil {
		return nil, err
	}

	err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.CreateDependency(txCtx, dep); err != nil {
			return err
		}

		if s.auditSvc != nil {
			_, err := s.auditSvc.Record(txCtx, audit.RecordCmd{
				OrganizationID: actor.OrganizationID,
				ActorUserID:    &actor.UserID,
				Action:         "dependency.created",
				TargetType:     audit.TargetTypeService,
				TargetID:       cmd.CallerServiceID,
				EnvironmentID:  &cmd.EnvironmentID,
				IPAddress:      actor.ClientIP,
				Metadata: map[string]any{
					"caller_service_id": cmd.CallerServiceID.String(),
					"callee_service_id": cmd.CalleeServiceID.String(),
					"protocol":          string(cmd.Protocol),
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

	return dep, nil
}

func (s *Service) GetProjectTopologyGraph(
	ctx context.Context,
	actor access.Actor,
	projectID, envID uuid.UUID,
) (*TopologyGraph, error) {
	envScope := access.NewEnvironmentScope(actor.OrganizationID, projectID, envID)
	if err := s.authorizer.Require(ctx, actor, access.VerbCatalogRead, envScope); err != nil {
		return nil, err
	}

	services, err := s.serviceChecker.ListByProject(ctx, actor.OrganizationID, projectID)
	if err != nil {
		return nil, err
	}

	resources, err := s.resourceChecker.List(ctx, actor.OrganizationID, projectID, &envID, nil)
	if err != nil {
		return nil, err
	}

	bindings, err := s.repo.ListBindingsByEnvironment(ctx, actor.OrganizationID, projectID, envID)
	if err != nil {
		return nil, err
	}

	deps, err := s.repo.ListDependenciesByEnvironment(ctx, actor.OrganizationID, projectID, envID)
	if err != nil {
		return nil, err
	}

	var nodes []GraphNode
	for _, svc := range services {
		nodes = append(nodes, GraphNode{
			ID:   svc.ID,
			Type: NodeTypeService,
			Name: svc.Name,
			Slug: svc.Slug,
			Metadata: map[string]any{
				"kind": "service",
			},
		})
	}
	for _, res := range resources {
		nodes = append(nodes, GraphNode{
			ID:   res.ID,
			Type: NodeTypeResource,
			Name: res.Name,
			Slug: res.Slug.String(),
			Metadata: map[string]any{
				"kind":          "resource",
				"resource_type": string(res.ResourceType),
			},
		})
	}

	var edges []GraphEdge
	for _, b := range bindings {
		edges = append(edges, GraphEdge{
			ID:       b.ID,
			Type:     EdgeTypeBinding,
			SourceID: b.ServiceID,
			TargetID: b.ResourceID,
			Label:    string(b.AccessMode),
			Metadata: map[string]any{
				"access_mode": string(b.AccessMode),
				"notes":       b.Notes,
			},
		})
	}
	for _, d := range deps {
		edges = append(edges, GraphEdge{
			ID:       d.ID,
			Type:     EdgeTypeDependency,
			SourceID: d.CallerServiceID,
			TargetID: d.CalleeServiceID,
			Label:    string(d.Protocol),
			Metadata: map[string]any{
				"protocol": string(d.Protocol),
			},
		})
	}

	if nodes == nil {
		nodes = []GraphNode{}
	}
	if edges == nil {
		edges = []GraphEdge{}
	}

	return &TopologyGraph{
		Nodes: nodes,
		Edges: edges,
	}, nil
}
