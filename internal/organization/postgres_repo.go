package organization

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

func NewPostgresRepository(txManager *database.TxManager) OrganizationRepository {
	return &postgresRepository{txManager: txManager}
}

func (r *postgresRepository) Count(ctx context.Context) (int64, error) {
	conn := r.txManager.Conn(ctx)

	var count int64
	err := conn.QueryRow(ctx, "SELECT COUNT(*) FROM organizations").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count organizations: %w", err)
	}

	return count, nil
}

func (r *postgresRepository) Create(ctx context.Context, org *Organization) error {
	conn := r.txManager.Conn(ctx)

	if org.ID == uuid.Nil {
		org.ID = uuid.Must(uuid.NewV7())
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	if org.CreatedAt.IsZero() {
		org.CreatedAt = now
	}
	if org.UpdatedAt.IsZero() {
		org.UpdatedAt = now
	}

	query := `
		INSERT INTO organizations (id, slug, name, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := conn.Exec(ctx, query, org.ID, org.Slug.String(), org.Name, org.CreatedAt, org.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperror.Conflict("organization with this slug already exists")
		}
		return fmt.Errorf("insert organization: %w", err)
	}

	return nil
}

func (r *postgresRepository) GetByID(ctx context.Context, id uuid.UUID) (*Organization, error) {
	conn := r.txManager.Conn(ctx)

	query := `SELECT id, slug, name, created_at, updated_at FROM organizations WHERE id = $1`
	return scanOrg(conn.QueryRow(ctx, query, id))
}

func (r *postgresRepository) GetBySlug(ctx context.Context, slug valueobject.Slug) (*Organization, error) {
	conn := r.txManager.Conn(ctx)

	query := `SELECT id, slug, name, created_at, updated_at FROM organizations WHERE slug = $1`
	return scanOrg(conn.QueryRow(ctx, query, slug.String()))
}

func (r *postgresRepository) Update(ctx context.Context, org *Organization) error {
	conn := r.txManager.Conn(ctx)

	org.UpdatedAt = time.Now().UTC().Truncate(time.Microsecond)
	query := `UPDATE organizations SET name = $1, updated_at = $2 WHERE id = $3`
	tag, err := conn.Exec(ctx, query, org.Name, org.UpdatedAt, org.ID)
	if err != nil {
		return fmt.Errorf("update organization: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return apperror.NotFound("organization")
	}

	return nil
}

func scanOrg(row pgx.Row) (*Organization, error) {
	var org Organization
	var rawSlug string

	err := row.Scan(&org.ID, &rawSlug, &org.Name, &org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NotFound("organization")
		}
		return nil, fmt.Errorf("scan organization: %w", err)
	}

	slug, err := valueobject.NewSlug(rawSlug)
	if err != nil {
		return nil, err
	}
	org.Slug = slug

	return &org, nil
}
