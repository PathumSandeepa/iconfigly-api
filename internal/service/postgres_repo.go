package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

func NewPostgresRepository(txManager *database.TxManager) ServiceRepository {
	return &postgresRepository{txManager: txManager}
}

func (r *postgresRepository) Create(ctx context.Context, s *Service) error {
	conn := r.txManager.Conn(ctx)

	if s.ID == uuid.Nil {
		s.ID = uuid.Must(uuid.NewV7())
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	if s.CreatedAt.IsZero() {
		s.CreatedAt = now
	}
	if s.UpdatedAt.IsZero() {
		s.UpdatedAt = now
	}

	query := `
		INSERT INTO services (
			id, organization_id, project_id, owner_team_id,
			slug, name, description, repo_url, runbook_url,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := conn.Exec(ctx, query,
		s.ID,
		s.OrganizationID,
		s.ProjectID,
		s.OwnerTeamID,
		s.Slug.String(),
		s.Name,
		s.Description,
		s.RepoURL,
		s.RunbookURL,
		s.CreatedAt,
		s.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperror.Conflict("service with this slug already exists in project")
		}
		return fmt.Errorf("insert service: %w", err)
	}

	return nil
}

func (r *postgresRepository) GetByID(ctx context.Context, orgID, projectID, serviceID uuid.UUID) (*Service, error) {
	conn := r.txManager.Conn(ctx)

	query := `
		SELECT id, organization_id, project_id, owner_team_id,
		       slug, name, description, repo_url, runbook_url,
		       created_at, updated_at
		FROM services
		WHERE organization_id = $1 AND project_id = $2 AND id = $3
	`
	return scanService(conn.QueryRow(ctx, query, orgID, projectID, serviceID))
}

func (r *postgresRepository) GetBySlug(ctx context.Context, orgID, projectID uuid.UUID, slug valueobject.Slug) (*Service, error) {
	conn := r.txManager.Conn(ctx)

	query := `
		SELECT id, organization_id, project_id, owner_team_id,
		       slug, name, description, repo_url, runbook_url,
		       created_at, updated_at
		FROM services
		WHERE organization_id = $1 AND project_id = $2 AND slug = $3
	`
	return scanService(conn.QueryRow(ctx, query, orgID, projectID, slug.String()))
}

func (r *postgresRepository) ListByProject(
	ctx context.Context,
	orgID, projectID uuid.UUID,
	ownerTeamID *uuid.UUID,
	search string,
) ([]*Service, error) {
	conn := r.txManager.Conn(ctx)

	query := `
		SELECT id, organization_id, project_id, owner_team_id,
		       slug, name, description, repo_url, runbook_url,
		       created_at, updated_at
		FROM services
		WHERE organization_id = $1 AND project_id = $2
	`
	args := []any{orgID, projectID}
	argIdx := 3

	if ownerTeamID != nil {
		query += fmt.Sprintf(" AND owner_team_id = $%d", argIdx)
		args = append(args, *ownerTeamID)
		argIdx++
	}

	if trimmedSearch := strings.TrimSpace(search); trimmedSearch != "" {
		query += fmt.Sprintf(" AND (name ILIKE $%d OR slug ILIKE $%d)", argIdx, argIdx)
		args = append(args, "%"+trimmedSearch+"%")
		argIdx++
	}

	query += " ORDER BY name ASC"

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query services: %w", err)
	}
	defer rows.Close()

	var services []*Service
	for rows.Next() {
		s, err := scanService(rows)
		if err != nil {
			return nil, err
		}
		services = append(services, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate services: %w", err)
	}

	return services, nil
}

func scanService(row pgx.Row) (*Service, error) {
	var s Service
	var rawSlug string

	err := row.Scan(
		&s.ID,
		&s.OrganizationID,
		&s.ProjectID,
		&s.OwnerTeamID,
		&rawSlug,
		&s.Name,
		&s.Description,
		&s.RepoURL,
		&s.RunbookURL,
		&s.CreatedAt,
		&s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NotFound("service")
		}
		return nil, fmt.Errorf("scan service: %w", err)
	}

	slug, err := valueobject.NewSlug(rawSlug)
	if err != nil {
		return nil, err
	}
	s.Slug = slug

	return &s, nil
}
