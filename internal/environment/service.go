package environment

import (
	"context"

	"github.com/PathumSandeepa/iconfigly-api/internal/access"
	"github.com/PathumSandeepa/iconfigly-api/internal/audit"
	"github.com/PathumSandeepa/iconfigly-api/internal/database"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/google/uuid"
)

type Service struct {
	repo       EnvironmentRepository
	authorizer *access.Authorizer
	auditSvc   *audit.Service
	txManager  *database.TxManager
}

func NewService(
	repo EnvironmentRepository,
	authorizer *access.Authorizer,
	auditSvc *audit.Service,
	txManager *database.TxManager,
) *Service {
	return &Service{
		repo:       repo,
		authorizer: authorizer,
		auditSvc:   auditSvc,
		txManager:  txManager,
	}
}

func (s *Service) ListEnvironments(ctx context.Context, actor access.Actor, projectID uuid.UUID) ([]*Environment, error) {
	projScope := access.NewProjectScope(actor.OrganizationID, projectID)
	if err := s.authorizer.Require(ctx, actor, access.VerbCatalogRead, projScope); err != nil {
		return nil, err
	}

	envs, err := s.repo.ListByProject(ctx, actor.OrganizationID, projectID)
	if err != nil {
		return nil, err
	}
	if envs == nil {
		envs = []*Environment{}
	}

	return envs, nil
}

func (s *Service) UpdateEnvironmentPolicy(
	ctx context.Context,
	actor access.Actor,
	projectID, envID uuid.UUID,
	cmd UpdatePolicyCmd,
) (*Environment, error) {
	envScope := access.NewEnvironmentScope(actor.OrganizationID, projectID, envID)
	if err := s.authorizer.Require(ctx, actor, access.VerbAccessManage, envScope); err != nil {
		return nil, err
	}

	curr, err := s.repo.GetByID(ctx, actor.OrganizationID, projectID, envID)
	if err != nil {
		return nil, err
	}

	isProd := curr.IsProduction
	if cmd.IsProduction != nil {
		isProd = *cmd.IsProduction
	}

	approvals := curr.RequiredApprovals
	if cmd.RequiredApprovals != nil {
		approvals = *cmd.RequiredApprovals
		if approvals < 0 {
			return nil, apperror.Validation("invalid required approvals", map[string]string{
				"required_approvals": "must be greater than or equal to 0",
			})
		}
	}

	err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.UpdatePolicy(txCtx, actor.OrganizationID, projectID, envID, isProd, approvals); err != nil {
			return err
		}

		if s.auditSvc != nil {
			_, err := s.auditSvc.Record(txCtx, audit.RecordCmd{
				OrganizationID: actor.OrganizationID,
				ActorUserID:    &actor.UserID,
				Action:         "environment.updated",
				TargetType:     audit.TargetTypeEnvironment,
				TargetID:       envID,
				EnvironmentID:  &envID,
				IPAddress:      actor.ClientIP,
				Metadata: map[string]any{
					"project_id":             projectID.String(),
					"old_is_production":      curr.IsProduction,
					"new_is_production":      isProd,
					"old_required_approvals": curr.RequiredApprovals,
					"new_required_approvals": approvals,
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

	return s.repo.GetByID(ctx, actor.OrganizationID, projectID, envID)
}
