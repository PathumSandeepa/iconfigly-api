package user

import (
	"context"
	"strings"

	"github.com/PathumSandeepa/iconfigly-api/internal/access"
	"github.com/PathumSandeepa/iconfigly-api/internal/audit"
	"github.com/PathumSandeepa/iconfigly-api/internal/database"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/crypto"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/valueobject"
	"github.com/google/uuid"
)

type SessionRevoker interface {
	RevokeAllForUser(ctx context.Context, orgID, userID uuid.UUID) error
}

type CreateUserCmd struct {
	Email        string `json:"email"`
	FullName     string `json:"full_name"`
	Password     string `json:"password"`
	IsSuperAdmin bool   `json:"is_super_admin"`
}

type Service struct {
	repo           UserRepository
	authorizer     *access.Authorizer
	auditSvc       *audit.Service
	accessSvc      *access.Service
	sessionRevoker SessionRevoker
	txManager      *database.TxManager
}

func NewService(
	repo UserRepository,
	authorizer *access.Authorizer,
	auditSvc *audit.Service,
	accessSvc *access.Service,
	sessionRevoker SessionRevoker,
	txManager *database.TxManager,
) *Service {
	return &Service{
		repo:           repo,
		authorizer:     authorizer,
		auditSvc:       auditSvc,
		accessSvc:      accessSvc,
		sessionRevoker: sessionRevoker,
		txManager:      txManager,
	}
}

func (s *Service) CreateUser(ctx context.Context, actor access.Actor, cmd CreateUserCmd) (*User, error) {
	orgScope := access.NewOrgScope(actor.OrganizationID)
	if err := s.authorizer.Require(ctx, actor, access.VerbOrgManage, orgScope); err != nil {
		return nil, err
	}

	email, err := valueobject.NewEmail(cmd.Email)
	if err != nil {
		return nil, err
	}

	trimmedPassword := strings.TrimSpace(cmd.Password)
	if len(trimmedPassword) < 8 {
		return nil, apperror.Validation("invalid password", map[string]string{
			"password": "must be at least 8 characters",
		})
	}

	hash, err := crypto.HashPassword(trimmedPassword)
	if err != nil {
		return nil, err
	}

	u, err := NewUser(actor.OrganizationID, email, cmd.FullName, hash, cmd.IsSuperAdmin)
	if err != nil {
		return nil, err
	}

	err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.Create(txCtx, u); err != nil {
			return err
		}

		if u.IsSuperAdmin && s.accessSvc != nil {
			_, err := s.accessSvc.GrantRole(txCtx, actor, orgScope, access.GrantRoleCmd{
				SubjectType: access.SubjectUser,
				SubjectID:   u.ID,
				RoleName:    access.RoleOrgAdmin,
				ScopeType:   access.ScopeOrg,
				ScopeID:     actor.OrganizationID,
			})
			if err != nil {
				return err
			}
		}

		if s.auditSvc != nil {
			_, err := s.auditSvc.Record(txCtx, audit.RecordCmd{
				OrganizationID: actor.OrganizationID,
				ActorUserID:    &actor.UserID,
				Action:         "user.created",
				TargetType:     audit.TargetTypeUser,
				TargetID:       u.ID,
				Metadata: map[string]any{
					"email":          u.Email.String(),
					"full_name":      u.FullName,
					"is_super_admin": u.IsSuperAdmin,
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

	return u, nil
}

func (s *Service) ListUsers(ctx context.Context, actor access.Actor) ([]*User, error) {
	orgScope := access.NewOrgScope(actor.OrganizationID)
	if err := s.authorizer.Require(ctx, actor, access.VerbCatalogRead, orgScope); err != nil {
		return nil, err
	}

	return s.repo.ListByOrg(ctx, actor.OrganizationID)
}

func (s *Service) ChangeStatus(ctx context.Context, actor access.Actor, userID uuid.UUID, status UserStatus) error {
	orgScope := access.NewOrgScope(actor.OrganizationID)
	if err := s.authorizer.Require(ctx, actor, access.VerbOrgManage, orgScope); err != nil {
		return err
	}

	if status != StatusActive && status != StatusDisabled {
		return apperror.Validation("invalid user status", map[string]string{
			"status": "must be ACTIVE or DISABLED",
		})
	}

	targetUser, err := s.repo.GetByID(ctx, actor.OrganizationID, userID)
	if err != nil {
		return err
	}

	if status == StatusDisabled && targetUser.IsSuperAdmin && targetUser.Status == StatusActive {
		count, err := s.repo.CountActiveSuperAdmins(ctx, actor.OrganizationID)
		if err != nil {
			return err
		}
		if count <= 1 {
			return apperror.Validation("cannot disable last super admin", map[string]string{
				"status": "cannot disable the last remaining active super admin",
			})
		}
	}

	return s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.UpdateStatus(txCtx, actor.OrganizationID, userID, status); err != nil {
			return err
		}

		if status == StatusDisabled && s.sessionRevoker != nil {
			if err := s.sessionRevoker.RevokeAllForUser(txCtx, actor.OrganizationID, userID); err != nil {
				return err
			}
		}

		if s.auditSvc != nil {
			_, err := s.auditSvc.Record(txCtx, audit.RecordCmd{
				OrganizationID: actor.OrganizationID,
				ActorUserID:    &actor.UserID,
				Action:         "user.status_changed",
				TargetType:     audit.TargetTypeUser,
				TargetID:       userID,
				Metadata: map[string]any{
					"old_status": string(targetUser.Status),
					"new_status": string(status),
				},
			})
			if err != nil {
				return err
			}
		}

		return nil
	})
}
