package auth

import (
	"time"

	"github.com/PathumSandeepa/iconfigly-api/internal/access"
	"github.com/PathumSandeepa/iconfigly-api/internal/user"
	"github.com/google/uuid"
)

type Actor = access.Actor

type Session struct {
	ID             uuid.UUID  `json:"id"`
	UserID         uuid.UUID  `json:"user_id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	TokenHash      []byte     `json:"-"`
	CSRFToken      string     `json:"csrf_token"`
	IPAddress      string     `json:"ip_address"`
	UserAgent      string     `json:"user_agent"`
	ExpiresAt      time.Time  `json:"expires_at"`
	RevokedAt      *time.Time `json:"revoked_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

func (s *Session) IsValid(now time.Time) bool {
	if s.RevokedAt != nil {
		return false
	}
	return s.ExpiresAt.After(now)
}

type PersonalAccessToken struct {
	ID             uuid.UUID  `json:"id"`
	UserID         uuid.UUID  `json:"user_id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	Name           string     `json:"name"`
	TokenPrefix    string     `json:"token_prefix"`
	TokenHash      []byte     `json:"-"`
	Scopes         []string   `json:"scopes"`
	ExpiresAt      time.Time  `json:"expires_at"`
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
	RevokedAt      *time.Time `json:"revoked_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

func (p *PersonalAccessToken) IsValid(now time.Time) bool {
	if p.RevokedAt != nil {
		return false
	}
	return p.ExpiresAt.After(now)
}

type SetupCmd struct {
	OrgSlug       string `json:"org_slug"`
	OrgName       string `json:"org_name"`
	AdminEmail    string `json:"admin_email"`
	AdminFullName string `json:"admin_full_name"`
	AdminPassword string `json:"admin_password"`
}

type LoginCmd struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type CreatePATCmd struct {
	Name          string   `json:"name"`
	Scopes        []string `json:"scopes"`
	ExpiresInDays int      `json:"expires_in_days"`
}

type LoginResult struct {
	User         *user.User `json:"user"`
	SessionToken string     `json:"session_token"`
	CSRFToken    string     `json:"csrf_token"`
	ExpiresAt    time.Time  `json:"expires_at"`
}

type CreatePATResult struct {
	Token    *PersonalAccessToken `json:"token"`
	RawToken string               `json:"raw_token"`
}

type SetupStatusResponse struct {
	Initialized bool `json:"initialized"`
}
