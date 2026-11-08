package project

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

func NewPostgresRepository(txManager *database.TxManager) ProjectRepository {
	return &postgresRepository{txManager: txManager}
}

func (r *postgresRepository) Create(ctx context.Context, p *Project) error {
	conn := r.txManager.Conn(ctx)

	if p.ID == uuid.Nil {
		p.ID = uuid.Must(uuid.NewV7())
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt = now
	}

	query := `
		INSERT INTO projects (
			id, organization_id, slug, name, description, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := conn.Exec(ctx, query,
		p.ID,
		p.OrganizationID,
		p.Slug.String(),
		p.Name,
		p.Description,
		p.CreatedAt,
		p.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperror.Conflict("project with this slug already exists in organization")
		}
		return fmt.Errorf("insert project: %w", err)
	}

	return nil
}

func (r *postgresRepository) GetByID(ctx context.Context, orgID, projectID uuid.UUID) (*Project, error) {
	conn := r.txManager.Conn(ctx)

	query := `
		SELECT id, organization_id, slug, name, description, created_at, updated_at
		FROM projects
		WHERE organization_id = $1 AND id = $2
	`
	return scanProject(conn.QueryRow(ctx, query, orgID, projectID))
}

func (r *postgresRepository) GetBySlug(ctx context.Context, orgID uuid.UUID, slug valueobject.Slug) (*Project, error) {
	conn := r.txManager.Conn(ctx)

	query := `
		SELECT id, organization_id, slug, name, description, created_at, updated_at
		FROM projects
		WHERE organization_id = $1 AND slug = $2
	`
	return scanProject(conn.QueryRow(ctx, query, orgID, slug.String()))
}

func (r *postgresRepository) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]*Project, error) {
	conn := r.txManager.Conn(ctx)

	query := `
		SELECT id, organization_id, slug, name, description, created_at, updated_at
		FROM projects
		WHERE organization_id = $1
		ORDER BY created_at ASC
	`
	rows, err := conn.Query(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("query projects by org: %w", err)
	}
	defer rows.Close()

	var projects []*Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate project rows: %w", err)
	}

	return projects, nil
}

func scanProject(row pgx.Row) (*Project, error) {
	var p Project
	var rawSlug string

	err := row.Scan(
		&p.ID,
		&p.OrganizationID,
		&rawSlug,
		&p.Name,
		&p.Description,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NotFound("project")
		}
		return nil, fmt.Errorf("scan project: %w", err)
	}

	slug, err := valueobject.NewSlug(rawSlug)
	if err != nil {
		return nil, err
	}
	p.Slug = slug

	return &p, nil
}
