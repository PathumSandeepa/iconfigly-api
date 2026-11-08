package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type SessionRepository interface {
	Create(ctx context.Context, session *Session) error
	GetByTokenHash(ctx context.Context, tokenHash []byte) (*Session, error)
	Revoke(ctx context.Context, tokenHash []byte) error
	RevokeAllForUser(ctx context.Context, orgID, userID uuid.UUID) error
}

type TokenRepository interface {
	Create(ctx context.Context, token *PersonalAccessToken) error
	GetByTokenHash(ctx context.Context, tokenHash []byte) (*PersonalAccessToken, error)
	ListByUser(ctx context.Context, orgID, userID uuid.UUID) ([]*PersonalAccessToken, error)
	Revoke(ctx context.Context, orgID, userID, tokenID uuid.UUID) error
	UpdateLastUsed(ctx context.Context, tokenID uuid.UUID, lastUsed time.Time) error
}

type MembershipResolver interface {
	ResolveUserTeams(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, []uuid.UUID, error)
}
