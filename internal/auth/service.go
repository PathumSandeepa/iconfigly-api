package auth

import (
	"context"
	"strings"
	"time"

	"github.com/PathumSandeepa/iconfigly-api/internal/access"
	"github.com/PathumSandeepa/iconfigly-api/internal/audit"
	"github.com/PathumSandeepa/iconfigly-api/internal/database"
	"github.com/PathumSandeepa/iconfigly-api/internal/organization"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/crypto"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/valueobject"
	"github.com/PathumSandeepa/iconfigly-api/internal/user"
	"github.com/google/uuid"
)

var dummyPasswordHash, _ = crypto.HashPassword("iconfigly-timing-defense-sentinel")

type OrgRepository interface {
	Count(ctx context.Context) (int64, error)
	Create(ctx context.Context, org *organization.Organization) error
}

type UserRepository interface {
	Create(ctx context.Context, u *user.User) error
	GetByID(ctx context.Context, orgID, userID uuid.UUID) (*user.User, error)
	GetByEmail(ctx context.Context, email valueobject.Email) (*user.User, error)
}

type AccessRepository interface {
	Create(ctx context.Context, binding *access.RoleBinding) error
}

type Service struct {
	orgRepo            OrgRepository
	userRepo           UserRepository
	accessRepo         AccessRepository
	sessionRepo        SessionRepository
	tokenRepo          TokenRepository
	membershipResolver MembershipResolver
	auditSvc           *audit.Service
	txManager          *database.TxManager
}

func NewService(
	orgRepo OrgRepository,
	userRepo UserRepository,
	accessRepo AccessRepository,
	sessionRepo SessionRepository,
	tokenRepo TokenRepository,
	membershipResolver MembershipResolver,
	auditSvc *audit.Service,
	txManager *database.TxManager,
) *Service {
	return &Service{
		orgRepo:            orgRepo,
		userRepo:           userRepo,
		accessRepo:         accessRepo,
		sessionRepo:        sessionRepo,
		tokenRepo:          tokenRepo,
		membershipResolver: membershipResolver,
		auditSvc:           auditSvc,
		txManager:          txManager,
	}
}

func (s *Service) GetSetupStatus(ctx context.Context) (bool, error) {
	count, err := s.orgRepo.Count(ctx)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Service) InitializePlatform(ctx context.Context, cmd SetupCmd, ip, userAgent string) (*LoginResult, error) {
	orgSlug, err := valueobject.NewSlug(cmd.OrgSlug)
	if err != nil {
		return nil, err
	}

	trimmedOrgName := strings.TrimSpace(cmd.OrgName)
	if len(trimmedOrgName) < 2 || len(trimmedOrgName) > 100 {
		return nil, apperror.Validation("invalid organization name", map[string]string{
			"org_name": "must be between 2 and 100 characters",
		})
	}

	adminEmail, err := valueobject.NewEmail(cmd.AdminEmail)
	if err != nil {
		return nil, err
	}

	trimmedAdminName := strings.TrimSpace(cmd.AdminFullName)
	if len(trimmedAdminName) < 2 || len(trimmedAdminName) > 100 {
		return nil, apperror.Validation("invalid admin full name", map[string]string{
			"admin_full_name": "must be between 2 and 100 characters",
		})
	}

	trimmedAdminPassword := strings.TrimSpace(cmd.AdminPassword)
	if len(trimmedAdminPassword) < 8 {
		return nil, apperror.Validation("invalid admin password", map[string]string{
			"admin_password": "must be at least 8 characters",
		})
	}

	hash, err := crypto.HashPassword(trimmedAdminPassword)
	if err != nil {
		return nil, err
	}

	var result *LoginResult

	err = s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		count, err := s.orgRepo.Count(txCtx)
		if err != nil {
			return err
		}
		if count > 0 {
			return apperror.Conflict("platform has already been initialized")
		}

		org, err := organization.NewOrganization(orgSlug, trimmedOrgName)
		if err != nil {
			return err
		}
		if err := s.orgRepo.Create(txCtx, org); err != nil {
			return err
		}

		adminUser, err := user.NewUser(org.ID, adminEmail, trimmedAdminName, hash, true)
		if err != nil {
			return err
		}
		if err := s.userRepo.Create(txCtx, adminUser); err != nil {
			return err
		}

		if s.accessRepo != nil {
			binding := &access.RoleBinding{
				ID:             uuid.Must(uuid.NewV7()),
				OrganizationID: org.ID,
				SubjectType:    access.SubjectUser,
				SubjectID:      adminUser.ID,
				RoleName:       access.RoleOrgAdmin,
				ScopeType:      access.ScopeOrg,
				ScopeID:        org.ID,
				CreatedAt:      time.Now().UTC().Truncate(time.Microsecond),
			}
			if err := s.accessRepo.Create(txCtx, binding); err != nil {
				return err
			}
		}

		if s.auditSvc != nil {
			_, err = s.auditSvc.Record(txCtx, audit.RecordCmd{
				OrganizationID: org.ID,
				ActorUserID:    &adminUser.ID,
				Action:         "org.initialized",
				TargetType:     audit.TargetTypeOrganization,
				TargetID:       org.ID,
				IPAddress:      ip,
				Metadata: map[string]any{
					"organization_name": org.Name,
					"organization_slug": org.Slug.String(),
					"admin_email":       adminUser.Email.String(),
					"admin_full_name":   adminUser.FullName,
				},
			})
			if err != nil {
				return err
			}
		}

		rawSessionToken, tokenHash, err := crypto.GenerateRandomToken("icfg_sess_", 32)
		if err != nil {
			return err
		}
		rawCSRFToken, _, err := crypto.GenerateRandomToken("icfg_csrf_", 32)
		if err != nil {
			return err
		}

		expiresAt := time.Now().UTC().Add(7 * 24 * time.Hour)
		sess := &Session{
			ID:             uuid.Must(uuid.NewV7()),
			UserID:         adminUser.ID,
			OrganizationID: org.ID,
			TokenHash:      tokenHash,
			CSRFToken:      rawCSRFToken,
			IPAddress:      ip,
			UserAgent:      userAgent,
			ExpiresAt:      expiresAt,
			CreatedAt:      time.Now().UTC().Truncate(time.Microsecond),
		}

		if err := s.sessionRepo.Create(txCtx, sess); err != nil {
			return err
		}

		result = &LoginResult{
			User:         adminUser,
			SessionToken: rawSessionToken,
			CSRFToken:    rawCSRFToken,
			ExpiresAt:    expiresAt,
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *Service) Login(ctx context.Context, cmd LoginCmd, ip, userAgent string) (*LoginResult, error) {
	emailVo, err := valueobject.NewEmail(cmd.Email)
	if err != nil {
		_, _ = crypto.VerifyPassword("dummy_pass", dummyPasswordHash)
		return nil, apperror.Unauthorized("invalid email or password")
	}

	u, err := s.userRepo.GetByEmail(ctx, emailVo)
	if err != nil {
		_, _ = crypto.VerifyPassword("dummy_pass", dummyPasswordHash)
		return nil, apperror.Unauthorized("invalid email or password")
	}

	matched, err := crypto.VerifyPassword(cmd.Password, u.PasswordHash)
	if err != nil || !matched {
		return nil, apperror.Unauthorized("invalid email or password")
	}

	if u.Status != user.StatusActive {
		return nil, apperror.Unauthorized("account is disabled")
	}

	rawSessionToken, tokenHash, err := crypto.GenerateRandomToken("icfg_sess_", 32)
	if err != nil {
		return nil, err
	}
	rawCSRFToken, _, err := crypto.GenerateRandomToken("icfg_csrf_", 32)
	if err != nil {
		return nil, err
	}

	expiresAt := time.Now().UTC().Add(7 * 24 * time.Hour)
	sess := &Session{
		ID:             uuid.Must(uuid.NewV7()),
		UserID:         u.ID,
		OrganizationID: u.OrganizationID,
		TokenHash:      tokenHash,
		CSRFToken:      rawCSRFToken,
		IPAddress:      ip,
		UserAgent:      userAgent,
		ExpiresAt:      expiresAt,
		CreatedAt:      time.Now().UTC().Truncate(time.Microsecond),
	}

	if err := s.sessionRepo.Create(ctx, sess); err != nil {
		return nil, err
	}

	if s.auditSvc != nil {
		_, _ = s.auditSvc.Record(ctx, audit.RecordCmd{
			OrganizationID: u.OrganizationID,
			ActorUserID:    &u.ID,
			Action:         "auth.login",
			TargetType:     audit.TargetTypeUser,
			TargetID:       u.ID,
			IPAddress:      ip,
			Metadata: map[string]any{
				"email":      u.Email.String(),
				"user_agent": userAgent,
			},
		})
	}

	return &LoginResult{
		User:         u,
		SessionToken: rawSessionToken,
		CSRFToken:    rawCSRFToken,
		ExpiresAt:    expiresAt,
	}, nil
}

func (s *Service) Logout(ctx context.Context, rawSessionToken string) error {
	tokenHash := crypto.HashToken(rawSessionToken)
	return s.sessionRepo.Revoke(ctx, tokenHash)
}

func (s *Service) AuthenticateSession(ctx context.Context, rawToken string, clientIP string) (*Actor, error) {
	tokenHash := crypto.HashToken(rawToken)
	sess, err := s.sessionRepo.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, apperror.Unauthorized("invalid or expired session")
	}

	if !sess.IsValid(time.Now().UTC()) {
		return nil, apperror.Unauthorized("invalid or expired session")
	}

	u, err := s.userRepo.GetByID(ctx, sess.OrganizationID, sess.UserID)
	if err != nil {
		return nil, apperror.Unauthorized("user not found")
	}

	if u.Status != user.StatusActive {
		return nil, apperror.Unauthorized("user is disabled")
	}

	var teamIDs []uuid.UUID
	var leadTeamIDs []uuid.UUID
	if s.membershipResolver != nil {
		resolvedTeams, resolvedLeads, err := s.membershipResolver.ResolveUserTeams(ctx, u.ID)
		if err == nil {
			teamIDs = resolvedTeams
			leadTeamIDs = resolvedLeads
		}
	}

	return &access.Actor{
		UserID:         u.ID,
		OrganizationID: u.OrganizationID,
		Email:          u.Email.String(),
		IsSuperAdmin:   u.IsSuperAdmin,
		TeamIDs:        teamIDs,
		LeadTeamIDs:    leadTeamIDs,
		AuthMethod:     "SESSION",
		CSRFToken:      sess.CSRFToken,
		ClientIP:       clientIP,
	}, nil
}

func (s *Service) AuthenticatePAT(ctx context.Context, rawPAT string, clientIP string) (*Actor, error) {
	tokenHash := crypto.HashToken(rawPAT)
	pat, err := s.tokenRepo.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, apperror.Unauthorized("invalid or expired personal access token")
	}

	if !pat.IsValid(time.Now().UTC()) {
		return nil, apperror.Unauthorized("invalid or expired personal access token")
	}

	u, err := s.userRepo.GetByID(ctx, pat.OrganizationID, pat.UserID)
	if err != nil {
		return nil, apperror.Unauthorized("user not found")
	}

	if u.Status != user.StatusActive {
		return nil, apperror.Unauthorized("user is disabled")
	}

	_ = s.tokenRepo.UpdateLastUsed(ctx, pat.ID, time.Now().UTC())

	var teamIDs []uuid.UUID
	var leadTeamIDs []uuid.UUID
	if s.membershipResolver != nil {
		resolvedTeams, resolvedLeads, err := s.membershipResolver.ResolveUserTeams(ctx, u.ID)
		if err == nil {
			teamIDs = resolvedTeams
			leadTeamIDs = resolvedLeads
		}
	}

	return &access.Actor{
		UserID:         u.ID,
		OrganizationID: u.OrganizationID,
		Email:          u.Email.String(),
		IsSuperAdmin:   u.IsSuperAdmin,
		TeamIDs:        teamIDs,
		LeadTeamIDs:    leadTeamIDs,
		AuthMethod:     "TOKEN",
		CSRFToken:      "",
		ClientIP:       clientIP,
	}, nil
}

func (s *Service) CreatePAT(ctx context.Context, actor access.Actor, cmd CreatePATCmd) (*PersonalAccessToken, string, error) {
	trimmedName := strings.TrimSpace(cmd.Name)
	if len(trimmedName) < 1 || len(trimmedName) > 100 {
		return nil, "", apperror.Validation("invalid token name", map[string]string{
			"name": "must be between 1 and 100 characters",
		})
	}

	days := cmd.ExpiresInDays
	if days <= 0 {
		days = 30
	}
	if days > 365 {
		days = 365
	}

	scopes := cmd.Scopes
	if len(scopes) == 0 {
		scopes = []string{"*"}
	}

	rawToken, tokenHash, err := crypto.GenerateRandomToken("icfg_pat_", 32)
	if err != nil {
		return nil, "", err
	}

	prefix := rawToken
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	expiresAt := now.Add(time.Duration(days) * 24 * time.Hour)

	pat := &PersonalAccessToken{
		ID:             uuid.Must(uuid.NewV7()),
		UserID:         actor.UserID,
		OrganizationID: actor.OrganizationID,
		Name:           trimmedName,
		TokenPrefix:    prefix,
		TokenHash:      tokenHash,
		Scopes:         scopes,
		ExpiresAt:      expiresAt,
		CreatedAt:      now,
	}

	if err := s.tokenRepo.Create(ctx, pat); err != nil {
		return nil, "", err
	}

	if s.auditSvc != nil {
		_, _ = s.auditSvc.Record(ctx, audit.RecordCmd{
			OrganizationID: actor.OrganizationID,
			ActorUserID:    &actor.UserID,
			Action:         "pat.created",
			TargetType:     audit.TargetTypeUser,
			TargetID:       actor.UserID,
			IPAddress:      actor.ClientIP,
			Metadata: map[string]any{
				"token_name":   pat.Name,
				"token_prefix": pat.TokenPrefix,
				"expires_at":   pat.ExpiresAt.Format(time.RFC3339),
			},
		})
	}

	return pat, rawToken, nil
}

func (s *Service) ListPATs(ctx context.Context, actor access.Actor) ([]*PersonalAccessToken, error) {
	return s.tokenRepo.ListByUser(ctx, actor.OrganizationID, actor.UserID)
}

func (s *Service) RevokePAT(ctx context.Context, actor access.Actor, tokenID uuid.UUID) error {
	if err := s.tokenRepo.Revoke(ctx, actor.OrganizationID, actor.UserID, tokenID); err != nil {
		return err
	}

	if s.auditSvc != nil {
		_, _ = s.auditSvc.Record(ctx, audit.RecordCmd{
			OrganizationID: actor.OrganizationID,
			ActorUserID:    &actor.UserID,
			Action:         "pat.revoked",
			TargetType:     audit.TargetTypeUser,
			TargetID:       actor.UserID,
			IPAddress:      actor.ClientIP,
			Metadata: map[string]any{
				"token_id": tokenID.String(),
			},
		})
	}

	return nil
}

func (s *Service) RevokeAllForUser(ctx context.Context, orgID, userID uuid.UUID) error {
	return s.sessionRepo.RevokeAllForUser(ctx, orgID, userID)
}
