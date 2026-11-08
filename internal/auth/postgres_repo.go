package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/PathumSandeepa/iconfigly-api/internal/database"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type postgresSessionRepository struct {
	txManager *database.TxManager
}

func NewPostgresSessionRepository(txManager *database.TxManager) SessionRepository {
	return &postgresSessionRepository{txManager: txManager}
}

func (r *postgresSessionRepository) Create(ctx context.Context, s *Session) error {
	conn := r.txManager.Conn(ctx)

	if s.ID == uuid.Nil {
		s.ID = uuid.Must(uuid.NewV7())
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	if s.CreatedAt.IsZero() {
		s.CreatedAt = now
	}

	query := `
		INSERT INTO sessions (
			id, user_id, organization_id, token_hash, csrf_token,
			ip_address, user_agent, expires_at, revoked_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := conn.Exec(ctx, query,
		s.ID,
		s.UserID,
		s.OrganizationID,
		s.TokenHash,
		s.CSRFToken,
		s.IPAddress,
		s.UserAgent,
		s.ExpiresAt,
		s.RevokedAt,
		s.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}

	return nil
}

func (r *postgresSessionRepository) GetByTokenHash(ctx context.Context, tokenHash []byte) (*Session, error) {
	conn := r.txManager.Conn(ctx)

	query := `
		SELECT id, user_id, organization_id, token_hash, csrf_token,
		       ip_address, user_agent, expires_at, revoked_at, created_at
		FROM sessions
		WHERE token_hash = $1
	`
	var s Session
	err := conn.QueryRow(ctx, query, tokenHash).Scan(
		&s.ID,
		&s.UserID,
		&s.OrganizationID,
		&s.TokenHash,
		&s.CSRFToken,
		&s.IPAddress,
		&s.UserAgent,
		&s.ExpiresAt,
		&s.RevokedAt,
		&s.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NotFound("session")
		}
		return nil, fmt.Errorf("scan session: %w", err)
	}

	return &s, nil
}

func (r *postgresSessionRepository) Revoke(ctx context.Context, tokenHash []byte) error {
	conn := r.txManager.Conn(ctx)

	now := time.Now().UTC().Truncate(time.Microsecond)
	query := `UPDATE sessions SET revoked_at = $1 WHERE token_hash = $2 AND revoked_at IS NULL`
	_, err := conn.Exec(ctx, query, now, tokenHash)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}

	return nil
}

func (r *postgresSessionRepository) RevokeAllForUser(ctx context.Context, orgID, userID uuid.UUID) error {
	conn := r.txManager.Conn(ctx)

	now := time.Now().UTC().Truncate(time.Microsecond)
	query := `UPDATE sessions SET revoked_at = $1 WHERE organization_id = $2 AND user_id = $3 AND revoked_at IS NULL`
	_, err := conn.Exec(ctx, query, now, orgID, userID)
	if err != nil {
		return fmt.Errorf("revoke all sessions for user: %w", err)
	}

	return nil
}

type postgresTokenRepository struct {
	txManager *database.TxManager
}

func NewPostgresTokenRepository(txManager *database.TxManager) TokenRepository {
	return &postgresTokenRepository{txManager: txManager}
}

func (r *postgresTokenRepository) Create(ctx context.Context, t *PersonalAccessToken) error {
	conn := r.txManager.Conn(ctx)

	if t.ID == uuid.Nil {
		t.ID = uuid.Must(uuid.NewV7())
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	if t.Scopes == nil {
		t.Scopes = []string{}
	}

	query := `
		INSERT INTO personal_access_tokens (
			id, user_id, organization_id, name, token_prefix,
			token_hash, scopes, expires_at, last_used_at, revoked_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := conn.Exec(ctx, query,
		t.ID,
		t.UserID,
		t.OrganizationID,
		t.Name,
		t.TokenPrefix,
		t.TokenHash,
		t.Scopes,
		t.ExpiresAt,
		t.LastUsedAt,
		t.RevokedAt,
		t.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert personal access token: %w", err)
	}

	return nil
}

func (r *postgresTokenRepository) GetByTokenHash(ctx context.Context, tokenHash []byte) (*PersonalAccessToken, error) {
	conn := r.txManager.Conn(ctx)

	query := `
		SELECT id, user_id, organization_id, name, token_prefix,
		       token_hash, scopes, expires_at, last_used_at, revoked_at, created_at
		FROM personal_access_tokens
		WHERE token_hash = $1
	`
	var t PersonalAccessToken
	err := conn.QueryRow(ctx, query, tokenHash).Scan(
		&t.ID,
		&t.UserID,
		&t.OrganizationID,
		&t.Name,
		&t.TokenPrefix,
		&t.TokenHash,
		&t.Scopes,
		&t.ExpiresAt,
		&t.LastUsedAt,
		&t.RevokedAt,
		&t.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NotFound("personal access token")
		}
		return nil, fmt.Errorf("scan personal access token: %w", err)
	}

	return &t, nil
}

func (r *postgresTokenRepository) ListByUser(ctx context.Context, orgID, userID uuid.UUID) ([]*PersonalAccessToken, error) {
	conn := r.txManager.Conn(ctx)

	query := `
		SELECT id, user_id, organization_id, name, token_prefix,
		       token_hash, scopes, expires_at, last_used_at, revoked_at, created_at
		FROM personal_access_tokens
		WHERE organization_id = $1 AND user_id = $2
		ORDER BY created_at DESC
	`
	rows, err := conn.Query(ctx, query, orgID, userID)
	if err != nil {
		return nil, fmt.Errorf("query personal access tokens: %w", err)
	}
	defer rows.Close()

	var tokens []*PersonalAccessToken
	for rows.Next() {
		var t PersonalAccessToken
		err := rows.Scan(
			&t.ID,
			&t.UserID,
			&t.OrganizationID,
			&t.Name,
			&t.TokenPrefix,
			&t.TokenHash,
			&t.Scopes,
			&t.ExpiresAt,
			&t.LastUsedAt,
			&t.RevokedAt,
			&t.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan token row: %w", err)
		}
		tokens = append(tokens, &t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate token rows: %w", err)
	}

	return tokens, nil
}

func (r *postgresTokenRepository) Revoke(ctx context.Context, orgID, userID, tokenID uuid.UUID) error {
	conn := r.txManager.Conn(ctx)

	now := time.Now().UTC().Truncate(time.Microsecond)
	query := `
		UPDATE personal_access_tokens
		SET revoked_at = $1
		WHERE organization_id = $2 AND user_id = $3 AND id = $4 AND revoked_at IS NULL
	`
	tag, err := conn.Exec(ctx, query, now, orgID, userID, tokenID)
	if err != nil {
		return fmt.Errorf("revoke personal access token: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return apperror.NotFound("personal access token")
	}

	return nil
}

func (r *postgresTokenRepository) UpdateLastUsed(ctx context.Context, tokenID uuid.UUID, lastUsed time.Time) error {
	conn := r.txManager.Conn(ctx)

	query := `UPDATE personal_access_tokens SET last_used_at = $1 WHERE id = $2`
	_, err := conn.Exec(ctx, query, lastUsed, tokenID)
	if err != nil {
		return fmt.Errorf("update token last used: %w", err)
	}

	return nil
}

type postgresMembershipResolver struct {
	txManager *database.TxManager
}

func NewPostgresMembershipResolver(txManager *database.TxManager) MembershipResolver {
	return &postgresMembershipResolver{txManager: txManager}
}

func (r *postgresMembershipResolver) ResolveUserTeams(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, []uuid.UUID, error) {
	conn := r.txManager.Conn(ctx)

	query := `SELECT team_id, team_role FROM team_members WHERE user_id = $1`
	rows, err := conn.Query(ctx, query, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve user teams: %w", err)
	}
	defer rows.Close()

	var teamIDs []uuid.UUID
	var leadTeamIDs []uuid.UUID

	for rows.Next() {
		var tID uuid.UUID
		var role string
		if err := rows.Scan(&tID, &role); err != nil {
			return nil, nil, fmt.Errorf("scan team membership: %w", err)
		}
		teamIDs = append(teamIDs, tID)
		if role == "LEAD" {
			leadTeamIDs = append(leadTeamIDs, tID)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate team memberships: %w", err)
	}

	return teamIDs, leadTeamIDs, nil
}
