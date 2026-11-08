package access

import (
	"context"
	"fmt"
	"time"

	"github.com/PathumSandeepa/iconfigly-api/internal/audit"
	"github.com/PathumSandeepa/iconfigly-api/internal/database"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/google/uuid"
)

type GrantRoleCmd struct {
	SubjectType SubjectType `json:"subject_type"`
	SubjectID   uuid.UUID   `json:"subject_id"`
	RoleName    RoleName    `json:"role_name"`
	ScopeType   ScopeType   `json:"scope_type"`
	ScopeID     uuid.UUID   `json:"scope_id"`
	ExpiresAt   *time.Time  `json:"expires_at,omitempty"`
}

type ListBindingsFilter struct {
	ScopeType ScopeType `json:"scope_type"`
	ScopeID   uuid.UUID `json:"scope_id"`
}

type Service struct {
	repo       AccessRepository
	authorizer *Authorizer
	auditSvc   *audit.Service
	txManager  *database.TxManager
}

func NewService(
	repo AccessRepository,
	authorizer *Authorizer,
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

func (s *Service) GrantRole(ctx context.Context, actor Actor, targetScope Scope, cmd GrantRoleCmd) (*RoleBinding, error) {
	fieldErrors := make(map[string]string)

	if !IsValidSubjectType(cmd.SubjectType) {
		fieldErrors["subject_type"] = "must be USER or TEAM"
	}
	if cmd.SubjectID == uuid.Nil {
		fieldErrors["subject_id"] = "is required"
	}
	if !IsValidRole(cmd.RoleName) {
		fieldErrors["role_name"] = "must be ORG_ADMIN, ENV_ADMIN, SERVICE_LEAD, DEVELOPER, or VIEWER"
	}
	if !IsValidScopeType(cmd.ScopeType) {
		fieldErrors["scope_type"] = "must be ORG, PROJECT, ENVIRONMENT, SERVICE, or RESOURCE"
	}
	if cmd.ScopeID == uuid.Nil {
		fieldErrors["scope_id"] = "is required"
	}
	if cmd.ExpiresAt != nil && !cmd.ExpiresAt.After(time.Now().UTC()) {
		fieldErrors["expires_at"] = "must be in the future"
	}

	if len(fieldErrors) > 0 {
		return nil, apperror.Validation("invalid grant command", fieldErrors)
	}

	if err := s.authorizer.Require(ctx, actor, VerbAccessManage, targetScope); err != nil {
		return nil, err
	}

	var grantedBy *uuid.UUID
	if actor.UserID != uuid.Nil {
		grantedBy = &actor.UserID
	}

	binding := &RoleBinding{
		ID:             uuid.Must(uuid.NewV7()),
		OrganizationID: actor.OrganizationID,
		SubjectType:    cmd.SubjectType,
		SubjectID:      cmd.SubjectID,
		RoleName:       cmd.RoleName,
		ScopeType:      cmd.ScopeType,
		ScopeID:        cmd.ScopeID,
		GrantedBy:      grantedBy,
		ExpiresAt:      cmd.ExpiresAt,
		CreatedAt:      time.Now().UTC().Truncate(time.Microsecond),
	}

	err := s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.Create(txCtx, binding); err != nil {
			return err
		}

		if s.auditSvc != nil {
			_, err := s.auditSvc.Record(txCtx, audit.RecordCmd{
				OrganizationID: actor.OrganizationID,
				ActorUserID:    &actor.UserID,
				Action:         "access.role_granted",
				TargetType:     audit.TargetTypeRoleBinding,
				TargetID:       binding.ID,
				Metadata: map[string]any{
					"subject_type": binding.SubjectType,
					"subject_id":   binding.SubjectID.String(),
					"role_name":    binding.RoleName,
					"scope_type":   binding.ScopeType,
					"scope_id":     binding.ScopeID.String(),
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

func (s *Service) RevokeRole(ctx context.Context, actor Actor, targetScope Scope, bindingID uuid.UUID) error {
	if bindingID == uuid.Nil {
		return apperror.Validation("invalid revoke command", map[string]string{
			"binding_id": "is required",
		})
	}

	if err := s.authorizer.Require(ctx, actor, VerbAccessManage, targetScope); err != nil {
		return err
	}

	return s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.Delete(txCtx, actor.OrganizationID, bindingID); err != nil {
			return err
		}

		if s.auditSvc != nil {
			_, err := s.auditSvc.Record(txCtx, audit.RecordCmd{
				OrganizationID: actor.OrganizationID,
				ActorUserID:    &actor.UserID,
				Action:         "access.role_revoked",
				TargetType:     audit.TargetTypeRoleBinding,
				TargetID:       bindingID,
			})
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (s *Service) ListBindings(
	ctx context.Context,
	actor Actor,
	targetScope Scope,
	projectID uuid.UUID,
	filter ListBindingsFilter,
) ([]*RoleBinding, error) {
	if err := s.authorizer.Require(ctx, actor, VerbAccessRead, targetScope); err != nil {
		return nil, err
	}

	if filter.ScopeType != "" && filter.ScopeID != uuid.Nil {
		return s.repo.ListByScope(ctx, actor.OrganizationID, filter.ScopeType, filter.ScopeID)
	}

	return s.repo.ListByProject(ctx, actor.OrganizationID, projectID)
}

func (s *Service) InspectTargetAccess(ctx context.Context, actor Actor, targetScope Scope) (*TargetAccessInspection, error) {
	if err := s.authorizer.Require(ctx, actor, VerbAccessRead, targetScope); err != nil {
		return nil, err
	}

	ancestors := targetScope.Ancestors()
	bindings, err := s.repo.FindActiveBindingsForScopes(ctx, actor.OrganizationID, ancestors, time.Now().UTC())
	if err != nil {
		return nil, err
	}

	inspection := &TargetAccessInspection{
		ScopeType:  targetScope.Type,
		ScopeID:    targetScope.ID,
		UserGrants: make([]EffectiveGrant, 0),
		TeamGrants: make([]EffectiveGrant, 0),
	}

	for _, b := range bindings {
		source := "Direct"
		if b.ScopeType != targetScope.Type || b.ScopeID != targetScope.ID {
			source = fmt.Sprintf("Inherited from %s", b.ScopeType)
		}

		grant := EffectiveGrant{
			BindingID:   b.ID,
			SubjectType: b.SubjectType,
			SubjectID:   b.SubjectID,
			RoleName:    b.RoleName,
			ScopeType:   b.ScopeType,
			ScopeID:     b.ScopeID,
			Source:      source,
			GrantedBy:   b.GrantedBy,
			ExpiresAt:   b.ExpiresAt,
			CreatedAt:   b.CreatedAt,
		}

		if b.SubjectType == SubjectUser {
			inspection.UserGrants = append(inspection.UserGrants, grant)
		} else if b.SubjectType == SubjectTeam {
			inspection.TeamGrants = append(inspection.TeamGrants, grant)
		}
	}

	return inspection, nil
}
