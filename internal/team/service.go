package team

import (
	"context"
	"time"

	"github.com/PathumSandeepa/iconfigly-api/internal/access"
	"github.com/PathumSandeepa/iconfigly-api/internal/audit"
	"github.com/PathumSandeepa/iconfigly-api/internal/database"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/valueobject"
	"github.com/PathumSandeepa/iconfigly-api/internal/user"
	"github.com/google/uuid"
)

type UserRepository interface {
	GetByID(ctx context.Context, orgID, userID uuid.UUID) (*user.User, error)
}

type Service struct {
	repo       TeamRepository
	userRepo   UserRepository
	authorizer *access.Authorizer
	auditSvc   *audit.Service
	txManager  *database.TxManager
}

func NewService(
	repo TeamRepository,
	userRepo UserRepository,
	authorizer *access.Authorizer,
	auditSvc *audit.Service,
	txManager *database.TxManager,
) *Service {
	return &Service{
		repo:       repo,
		userRepo:   userRepo,
		authorizer: authorizer,
		auditSvc:   auditSvc,
		txManager:  txManager,
	}
}

func (s *Service) CreateTeam(ctx context.Context, actor access.Actor, cmd CreateTeamCmd) (*Team, error) {
	orgScope := access.NewOrgScope(actor.OrganizationID)
	if err := s.authorizer.Require(ctx, actor, access.VerbCatalogWrite, orgScope); err != nil {
		return nil, err
	}

	slug, err := valueobject.NewSlug(cmd.Slug)
	if err != nil {
		return nil, err
	}

	t, err := NewTeam(actor.OrganizationID, slug, cmd.Name, cmd.Description)
	if err != nil {
		return nil, err
	}

	err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.Create(txCtx, t); err != nil {
			return err
		}

		if s.auditSvc != nil {
			_, err := s.auditSvc.Record(txCtx, audit.RecordCmd{
				OrganizationID: actor.OrganizationID,
				ActorUserID:    &actor.UserID,
				Action:         "team.created",
				TargetType:     audit.TargetTypeTeam,
				TargetID:       t.ID,
				IPAddress:      actor.ClientIP,
				Metadata: map[string]any{
					"slug":        t.Slug.String(),
					"name":        t.Name,
					"description": t.Description,
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

	return t, nil
}

func (s *Service) ListTeams(ctx context.Context, actor access.Actor) ([]*TeamWithMemberCount, error) {
	orgScope := access.NewOrgScope(actor.OrganizationID)
	if err := s.authorizer.Require(ctx, actor, access.VerbCatalogRead, orgScope); err != nil {
		return nil, err
	}

	teams, err := s.repo.ListByOrg(ctx, actor.OrganizationID)
	if err != nil {
		return nil, err
	}
	if teams == nil {
		teams = []*TeamWithMemberCount{}
	}

	return teams, nil
}

func (s *Service) GetTeamDetails(ctx context.Context, actor access.Actor, teamID uuid.UUID) (*TeamDetails, error) {
	orgScope := access.NewOrgScope(actor.OrganizationID)
	if err := s.authorizer.Require(ctx, actor, access.VerbCatalogRead, orgScope); err != nil {
		return nil, err
	}

	t, err := s.repo.GetByID(ctx, actor.OrganizationID, teamID)
	if err != nil {
		return nil, err
	}

	members, err := s.repo.ListMembers(ctx, teamID)
	if err != nil {
		return nil, err
	}
	if members == nil {
		members = []*TeamMember{}
	}

	return &TeamDetails{
		Team:    t,
		Members: members,
	}, nil
}

func (s *Service) AddTeamMember(ctx context.Context, actor access.Actor, teamID, userID uuid.UUID, role TeamRole) (*TeamMember, error) {
	if !s.isTeamLeadOrOrgWriter(ctx, actor, teamID) {
		return nil, apperror.Forbidden("requires catalog.write permission or team lead role")
	}

	if !IsValidTeamRole(role) {
		return nil, apperror.Validation("invalid team role", map[string]string{
			"role": "must be LEAD or MEMBER",
		})
	}

	t, err := s.repo.GetByID(ctx, actor.OrganizationID, teamID)
	if err != nil {
		return nil, err
	}

	targetUser, err := s.userRepo.GetByID(ctx, actor.OrganizationID, userID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	member := &TeamMember{
		ID:           uuid.Must(uuid.NewV7()),
		TeamID:       teamID,
		UserID:       userID,
		UserEmail:    targetUser.Email.String(),
		UserFullName: targetUser.FullName,
		Role:         role,
		CreatedAt:    now,
	}

	err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.AddMember(txCtx, member); err != nil {
			return err
		}

		if s.auditSvc != nil {
			_, err := s.auditSvc.Record(txCtx, audit.RecordCmd{
				OrganizationID: actor.OrganizationID,
				ActorUserID:    &actor.UserID,
				Action:         "team.member_added",
				TargetType:     audit.TargetTypeTeam,
				TargetID:       teamID,
				IPAddress:      actor.ClientIP,
				Metadata: map[string]any{
					"team_name":  t.Name,
					"user_id":    userID.String(),
					"user_email": targetUser.Email.String(),
					"role":       string(role),
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

	return member, nil
}

func (s *Service) RemoveTeamMember(ctx context.Context, actor access.Actor, teamID, userID uuid.UUID) error {
	if !s.isTeamLeadOrOrgWriter(ctx, actor, teamID) {
		return apperror.Forbidden("requires catalog.write permission or team lead role")
	}

	t, err := s.repo.GetByID(ctx, actor.OrganizationID, teamID)
	if err != nil {
		return err
	}

	return s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.RemoveMember(txCtx, teamID, userID); err != nil {
			return err
		}

		if s.auditSvc != nil {
			_, err := s.auditSvc.Record(txCtx, audit.RecordCmd{
				OrganizationID: actor.OrganizationID,
				ActorUserID:    &actor.UserID,
				Action:         "team.member_removed",
				TargetType:     audit.TargetTypeTeam,
				TargetID:       teamID,
				IPAddress:      actor.ClientIP,
				Metadata: map[string]any{
					"team_name": t.Name,
					"user_id":   userID.String(),
				},
			})
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (s *Service) isTeamLeadOrOrgWriter(ctx context.Context, actor access.Actor, teamID uuid.UUID) bool {
	if actor.IsSuperAdmin {
		return true
	}

	for _, leadID := range actor.LeadTeamIDs {
		if leadID == teamID {
			return true
		}
	}

	orgScope := access.NewOrgScope(actor.OrganizationID)
	return s.authorizer.Require(ctx, actor, access.VerbCatalogWrite, orgScope) == nil
}
