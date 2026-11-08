package team

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/PathumSandeepa/iconfigly-api/internal/database"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/valueobject"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type postgresRepository struct {
	txManager *database.TxManager
}

func NewPostgresRepository(txManager *database.TxManager) TeamRepository {
	return &postgresRepository{txManager: txManager}
}

func (r *postgresRepository) Create(ctx context.Context, t *Team) error {
	conn := r.txManager.Conn(ctx)

	if t.ID == uuid.Nil {
		t.ID = uuid.Must(uuid.NewV7())
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	if t.UpdatedAt.IsZero() {
		t.UpdatedAt = now
	}

	query := `
		INSERT INTO teams (
			id, organization_id, slug, name, description, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := conn.Exec(ctx, query,
		t.ID,
		t.OrganizationID,
		t.Slug.String(),
		t.Name,
		t.Description,
		t.CreatedAt,
		t.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperror.Conflict("team with this slug already exists in organization")
		}
		return fmt.Errorf("insert team: %w", err)
	}

	return nil
}

func (r *postgresRepository) GetByID(ctx context.Context, orgID, teamID uuid.UUID) (*Team, error) {
	conn := r.txManager.Conn(ctx)

	query := `
		SELECT id, organization_id, slug, name, description, created_at, updated_at
		FROM teams
		WHERE organization_id = $1 AND id = $2
	`
	return scanTeam(conn.QueryRow(ctx, query, orgID, teamID))
}

func (r *postgresRepository) GetBySlug(ctx context.Context, orgID uuid.UUID, slug valueobject.Slug) (*Team, error) {
	conn := r.txManager.Conn(ctx)

	query := `
		SELECT id, organization_id, slug, name, description, created_at, updated_at
		FROM teams
		WHERE organization_id = $1 AND slug = $2
	`
	return scanTeam(conn.QueryRow(ctx, query, orgID, slug.String()))
}

func (r *postgresRepository) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]*TeamWithMemberCount, error) {
	conn := r.txManager.Conn(ctx)

	query := `
		SELECT t.id, t.organization_id, t.slug, t.name, t.description,
		       COUNT(tm.id)::int AS member_count,
		       t.created_at, t.updated_at
		FROM teams t
		LEFT JOIN team_members tm ON t.id = tm.team_id
		WHERE t.organization_id = $1
		GROUP BY t.id, t.organization_id, t.slug, t.name, t.description, t.created_at, t.updated_at
		ORDER BY t.created_at ASC
	`
	rows, err := conn.Query(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("query teams by org: %w", err)
	}
	defer rows.Close()

	var teams []*TeamWithMemberCount
	for rows.Next() {
		var t TeamWithMemberCount
		var rawSlug string
		err := rows.Scan(
			&t.ID,
			&t.OrganizationID,
			&rawSlug,
			&t.Name,
			&t.Description,
			&t.MemberCount,
			&t.CreatedAt,
			&t.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan team with member count: %w", err)
		}

		slug, err := valueobject.NewSlug(rawSlug)
		if err != nil {
			return nil, err
		}
		t.Slug = slug

		teams = append(teams, &t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate team rows: %w", err)
	}

	return teams, nil
}

func (r *postgresRepository) AddMember(ctx context.Context, m *TeamMember) error {
	conn := r.txManager.Conn(ctx)

	if m.ID == uuid.Nil {
		m.ID = uuid.Must(uuid.NewV7())
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	if m.CreatedAt.IsZero() {
		m.CreatedAt = now
	}

	query := `
		INSERT INTO team_members (
			id, team_id, user_id, team_role, created_at
		) VALUES ($1, $2, $3, $4, $5)
	`
	_, err := conn.Exec(ctx, query,
		m.ID,
		m.TeamID,
		m.UserID,
		string(m.Role),
		m.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperror.Conflict("user is already a member of this team")
		}
		return fmt.Errorf("insert team member: %w", err)
	}

	return nil
}

func (r *postgresRepository) RemoveMember(ctx context.Context, teamID, userID uuid.UUID) error {
	conn := r.txManager.Conn(ctx)

	query := `DELETE FROM team_members WHERE team_id = $1 AND user_id = $2`
	tag, err := conn.Exec(ctx, query, teamID, userID)
	if err != nil {
		return fmt.Errorf("delete team member: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return apperror.NotFound("team member")
	}

	return nil
}

func (r *postgresRepository) ListMembers(ctx context.Context, teamID uuid.UUID) ([]*TeamMember, error) {
	conn := r.txManager.Conn(ctx)

	query := `
		SELECT tm.id, tm.team_id, tm.user_id, u.email, u.full_name, tm.team_role, tm.created_at
		FROM team_members tm
		JOIN users u ON tm.user_id = u.id
		WHERE tm.team_id = $1
		ORDER BY tm.created_at ASC
	`
	rows, err := conn.Query(ctx, query, teamID)
	if err != nil {
		return nil, fmt.Errorf("query team members: %w", err)
	}
	defer rows.Close()

	var members []*TeamMember
	for rows.Next() {
		var m TeamMember
		var rawRole string
		err := rows.Scan(
			&m.ID,
			&m.TeamID,
			&m.UserID,
			&m.UserEmail,
			&m.UserFullName,
			&rawRole,
			&m.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan team member: %w", err)
		}
		m.Role = TeamRole(rawRole)
		members = append(members, &m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate team member rows: %w", err)
	}

	return members, nil
}

func (r *postgresRepository) ListUserTeamMemberships(ctx context.Context, userID uuid.UUID) ([]UserTeamMembership, error) {
	conn := r.txManager.Conn(ctx)

	query := `SELECT team_id, team_role FROM team_members WHERE user_id = $1`
	rows, err := conn.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("query user team memberships: %w", err)
	}
	defer rows.Close()

	var list []UserTeamMembership
	for rows.Next() {
		var m UserTeamMembership
		var rawRole string
		if err := rows.Scan(&m.TeamID, &rawRole); err != nil {
			return nil, fmt.Errorf("scan user team membership: %w", err)
		}
		m.TeamRole = TeamRole(rawRole)
		list = append(list, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate user team memberships: %w", err)
	}

	return list, nil
}

func scanTeam(row pgx.Row) (*Team, error) {
	var t Team
	var rawSlug string

	err := row.Scan(
		&t.ID,
		&t.OrganizationID,
		&rawSlug,
		&t.Name,
		&t.Description,
		&t.CreatedAt,
		&t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NotFound("team")
		}
		return nil, fmt.Errorf("scan team: %w", err)
	}

	slug, err := valueobject.NewSlug(rawSlug)
	if err != nil {
		return nil, err
	}
	t.Slug = slug

	return &t, nil
}
