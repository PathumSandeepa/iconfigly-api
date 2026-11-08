package resource

import (
	"context"
	"encoding/json"
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

func NewPostgresRepository(txManager *database.TxManager) ResourceRepository {
	return &postgresRepository{txManager: txManager}
}

func (r *postgresRepository) Create(ctx context.Context, res *Resource) error {
	conn := r.txManager.Conn(ctx)

	if res.ID == uuid.Nil {
		res.ID = uuid.Must(uuid.NewV7())
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	if res.CreatedAt.IsZero() {
		res.CreatedAt = now
	}
	if res.UpdatedAt.IsZero() {
		res.UpdatedAt = now
	}
	if res.Version <= 0 {
		res.Version = 1
	}

	metaBytes, err := json.Marshal(res.ConnectionMetadata)
	if err != nil {
		return fmt.Errorf("marshal connection metadata: %w", err)
	}

	query := `
		INSERT INTO resources (
			id, organization_id, project_id, environment_id, owner_team_id,
			slug, name, resource_type, connection_metadata, version,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	_, err = conn.Exec(ctx, query,
		res.ID,
		res.OrganizationID,
		res.ProjectID,
		res.EnvironmentID,
		res.OwnerTeamID,
		res.Slug.String(),
		res.Name,
		string(res.ResourceType),
		metaBytes,
		res.Version,
		res.CreatedAt,
		res.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperror.Conflict("resource with this slug already exists in environment")
		}
		return fmt.Errorf("insert resource: %w", err)
	}

	return nil
}

func (r *postgresRepository) GetByID(ctx context.Context, orgID, projectID, resourceID uuid.UUID) (*Resource, error) {
	conn := r.txManager.Conn(ctx)

	query := `
		SELECT id, organization_id, project_id, environment_id, owner_team_id,
		       slug, name, resource_type, connection_metadata, version,
		       created_at, updated_at
		FROM resources
		WHERE organization_id = $1 AND project_id = $2 AND id = $3
	`
	return scanResource(conn.QueryRow(ctx, query, orgID, projectID, resourceID))
}

func (r *postgresRepository) GetBySlug(ctx context.Context, orgID, projectID, envID uuid.UUID, slug valueobject.Slug) (*Resource, error) {
	conn := r.txManager.Conn(ctx)

	query := `
		SELECT id, organization_id, project_id, environment_id, owner_team_id,
		       slug, name, resource_type, connection_metadata, version,
		       created_at, updated_at
		FROM resources
		WHERE organization_id = $1 AND project_id = $2 AND environment_id = $3 AND slug = $4
	`
	return scanResource(conn.QueryRow(ctx, query, orgID, projectID, envID, slug.String()))
}

func (r *postgresRepository) List(
	ctx context.Context,
	orgID, projectID uuid.UUID,
	envID *uuid.UUID,
	resType *ResourceType,
) ([]*Resource, error) {
	conn := r.txManager.Conn(ctx)

	query := `
		SELECT id, organization_id, project_id, environment_id, owner_team_id,
		       slug, name, resource_type, connection_metadata, version,
		       created_at, updated_at
		FROM resources
		WHERE organization_id = $1 AND project_id = $2
	`
	args := []any{orgID, projectID}
	argIdx := 3

	if envID != nil {
		query += fmt.Sprintf(" AND environment_id = $%d", argIdx)
		args = append(args, *envID)
		argIdx++
	}

	if resType != nil {
		query += fmt.Sprintf(" AND resource_type = $%d", argIdx)
		args = append(args, string(*resType))
		argIdx++
	}

	query += " ORDER BY created_at ASC"

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query resources: %w", err)
	}
	defer rows.Close()

	var resources []*Resource
	for rows.Next() {
		res, err := scanResource(rows)
		if err != nil {
			return nil, err
		}
		resources = append(resources, res)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate resource rows: %w", err)
	}

	return resources, nil
}

func scanResource(row pgx.Row) (*Resource, error) {
	var r Resource
	var rawSlug string
	var rawType string
	var metaBytes []byte

	err := row.Scan(
		&r.ID,
		&r.OrganizationID,
		&r.ProjectID,
		&r.EnvironmentID,
		&r.OwnerTeamID,
		&rawSlug,
		&r.Name,
		&rawType,
		&metaBytes,
		&r.Version,
		&r.CreatedAt,
		&r.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NotFound("resource")
		}
		return nil, fmt.Errorf("scan resource: %w", err)
	}

	slug, err := valueobject.NewSlug(rawSlug)
	if err != nil {
		return nil, err
	}
	r.Slug = slug
	r.ResourceType = ResourceType(rawType)

	if len(metaBytes) > 0 {
		if err := json.Unmarshal(metaBytes, &r.ConnectionMetadata); err != nil {
			return nil, fmt.Errorf("unmarshal connection metadata: %w", err)
		}
	} else {
		r.ConnectionMetadata = make(map[string]any)
	}

	return &r, nil
}
