package environment

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

func NewPostgresRepository(txManager *database.TxManager) EnvironmentRepository {
	return &postgresRepository{txManager: txManager}
}

func (r *postgresRepository) Create(ctx context.Context, e *Environment) error {
	conn := r.txManager.Conn(ctx)

	if e.ID == uuid.Nil {
		e.ID = uuid.Must(uuid.NewV7())
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	if e.CreatedAt.IsZero() {
		e.CreatedAt = now
	}
	if e.UpdatedAt.IsZero() {
		e.UpdatedAt = now
	}

	query := `
		INSERT INTO environments (
			id, organization_id, project_id, slug, name,
			tier_order, is_production, required_approvals,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := conn.Exec(ctx, query,
		e.ID,
		e.OrganizationID,
		e.ProjectID,
		e.Slug.String(),
		e.Name,
		e.TierOrder,
		e.IsProduction,
		e.RequiredApprovals,
		e.CreatedAt,
		e.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperror.Conflict("environment with this slug already exists in project")
		}
		return fmt.Errorf("insert environment: %w", err)
	}

	return nil
}

func (r *postgresRepository) CreateBatch(ctx context.Context, envs []*Environment) error {
	for _, env := range envs {
		if err := r.Create(ctx, env); err != nil {
			return err
		}
	}
	return nil
}

func (r *postgresRepository) GetByID(ctx context.Context, orgID, projectID, envID uuid.UUID) (*Environment, error) {
	conn := r.txManager.Conn(ctx)

	query := `
		SELECT id, organization_id, project_id, slug, name,
		       tier_order, is_production, required_approvals,
		       created_at, updated_at
		FROM environments
		WHERE organization_id = $1 AND project_id = $2 AND id = $3
	`
	return scanEnvironment(conn.QueryRow(ctx, query, orgID, projectID, envID))
}

func (r *postgresRepository) GetBySlug(ctx context.Context, orgID, projectID uuid.UUID, slug valueobject.Slug) (*Environment, error) {
	conn := r.txManager.Conn(ctx)

	query := `
		SELECT id, organization_id, project_id, slug, name,
		       tier_order, is_production, required_approvals,
		       created_at, updated_at
		FROM environments
		WHERE organization_id = $1 AND project_id = $2 AND slug = $3
	`
	return scanEnvironment(conn.QueryRow(ctx, query, orgID, projectID, slug.String()))
}

func (r *postgresRepository) ListByProject(ctx context.Context, orgID, projectID uuid.UUID) ([]*Environment, error) {
	conn := r.txManager.Conn(ctx)

	query := `
		SELECT id, organization_id, project_id, slug, name,
		       tier_order, is_production, required_approvals,
		       created_at, updated_at
		FROM environments
		WHERE organization_id = $1 AND project_id = $2
		ORDER BY tier_order ASC
	`
	rows, err := conn.Query(ctx, query, orgID, projectID)
	if err != nil {
		return nil, fmt.Errorf("query environments by project: %w", err)
	}
	defer rows.Close()

	var envs []*Environment
	for rows.Next() {
		e, err := scanEnvironment(rows)
		if err != nil {
			return nil, err
		}
		envs = append(envs, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate environment rows: %w", err)
	}

	return envs, nil
}

func (r *postgresRepository) UpdatePolicy(ctx context.Context, orgID, projectID, envID uuid.UUID, isProduction bool, requiredApprovals int) error {
	conn := r.txManager.Conn(ctx)

	now := time.Now().UTC().Truncate(time.Microsecond)
	query := `
		UPDATE environments
		SET is_production = $1, required_approvals = $2, updated_at = $3
		WHERE organization_id = $4 AND project_id = $5 AND id = $6
	`
	tag, err := conn.Exec(ctx, query, isProduction, requiredApprovals, now, orgID, projectID, envID)
	if err != nil {
		return fmt.Errorf("update environment policy: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return apperror.NotFound("environment")
	}

	return nil
}

func scanEnvironment(row pgx.Row) (*Environment, error) {
	var e Environment
	var rawSlug string

	err := row.Scan(
		&e.ID,
		&e.OrganizationID,
		&e.ProjectID,
		&rawSlug,
		&e.Name,
		&e.TierOrder,
		&e.IsProduction,
		&e.RequiredApprovals,
		&e.CreatedAt,
		&e.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NotFound("environment")
		}
		return nil, fmt.Errorf("scan environment: %w", err)
	}

	slug, err := valueobject.NewSlug(rawSlug)
	if err != nil {
		return nil, err
	}
	e.Slug = slug

	return &e, nil
}
