package organization

import (
	"context"
	"strings"

	"github.com/PathumSandeepa/iconfigly-api/internal/access"
	"github.com/PathumSandeepa/iconfigly-api/internal/audit"
	"github.com/PathumSandeepa/iconfigly-api/internal/database"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/google/uuid"
)

type Service struct {
	repo       OrganizationRepository
	authorizer *access.Authorizer
	auditSvc   *audit.Service
	txManager  *database.TxManager
}

func NewService(
	repo OrganizationRepository,
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

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Organization, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) Update(ctx context.Context, actor access.Actor, id uuid.UUID, name string) (*Organization, error) {
	scope := access.NewOrgScope(id)
	if err := s.authorizer.Require(ctx, actor, access.VerbOrgManage, scope); err != nil {
		return nil, err
	}

	trimmedName := strings.TrimSpace(name)
	if len(trimmedName) < 2 || len(trimmedName) > 100 {
		return nil, apperror.Validation("invalid organization name", map[string]string{
			"name": "must be between 2 and 100 characters",
		})
	}

	org, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	org.Name = trimmedName

	err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.Update(txCtx, org); err != nil {
			return err
		}

		if s.auditSvc != nil {
			_, err := s.auditSvc.Record(txCtx, audit.RecordCmd{
				OrganizationID: org.ID,
				ActorUserID:    &actor.UserID,
				Action:         "org.updated",
				TargetType:     audit.TargetTypeOrganization,
				TargetID:       org.ID,
				Metadata:       map[string]any{"name": org.Name},
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

	return org, nil
}
