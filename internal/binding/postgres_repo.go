package binding

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/PathumSandeepa/iconfigly-api/internal/database"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type postgresRepository struct {
	txManager *database.TxManager
}

func NewPostgresRepository(txManager *database.TxManager) BindingRepository {
	return &postgresRepository{txManager: txManager}
}

func (r *postgresRepository) CreateBinding(ctx context.Context, b *ServiceResourceBinding) error {
	conn := r.txManager.Conn(ctx)

	if b.ID == uuid.Nil {
		b.ID = uuid.Must(uuid.NewV7())
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	if b.CreatedAt.IsZero() {
		b.CreatedAt = now
	}

	query := `
		INSERT INTO service_resource_bindings (
			id, organization_id, project_id, environment_id,
			service_id, resource_id, access_mode, notes, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := conn.Exec(ctx, query,
		b.ID,
		b.OrganizationID,
		b.ProjectID,
		b.EnvironmentID,
		b.ServiceID,
		b.ResourceID,
		string(b.AccessMode),
		b.Notes,
		b.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperror.Conflict("service is already bound to this resource in environment")
		}
		return fmt.Errorf("insert binding: %w", err)
	}

	return nil
}

func (r *postgresRepository) DeleteBinding(ctx context.Context, orgID, projectID, bindingID uuid.UUID) (*ServiceResourceBinding, error) {
	conn := r.txManager.Conn(ctx)

	query := `
		DELETE FROM service_resource_bindings
		WHERE organization_id = $1 AND project_id = $2 AND id = $3
		RETURNING id, organization_id, project_id, environment_id,
		          service_id, resource_id, access_mode, notes, created_at
	`
	var b ServiceResourceBinding
	var rawMode string
	err := conn.QueryRow(ctx, query, orgID, projectID, bindingID).Scan(
		&b.ID,
		&b.OrganizationID,
		&b.ProjectID,
		&b.EnvironmentID,
		&b.ServiceID,
		&b.ResourceID,
		&rawMode,
		&b.Notes,
		&b.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NotFound("service resource binding")
		}
		return nil, fmt.Errorf("delete binding: %w", err)
	}
	b.AccessMode = AccessMode(rawMode)

	return &b, nil
}

func (r *postgresRepository) ListBindingsByEnvironment(ctx context.Context, orgID, projectID, envID uuid.UUID) ([]*ServiceResourceBinding, error) {
	conn := r.txManager.Conn(ctx)

	query := `
		SELECT id, organization_id, project_id, environment_id,
		       service_id, resource_id, access_mode, notes, created_at
		FROM service_resource_bindings
		WHERE organization_id = $1 AND project_id = $2 AND environment_id = $3
	`
	rows, err := conn.Query(ctx, query, orgID, projectID, envID)
	if err != nil {
		return nil, fmt.Errorf("query bindings by env: %w", err)
	}
	defer rows.Close()

	var list []*ServiceResourceBinding
	for rows.Next() {
		var b ServiceResourceBinding
		var rawMode string
		err := rows.Scan(
			&b.ID,
			&b.OrganizationID,
			&b.ProjectID,
			&b.EnvironmentID,
			&b.ServiceID,
			&b.ResourceID,
			&rawMode,
			&b.Notes,
			&b.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan binding: %w", err)
		}
		b.AccessMode = AccessMode(rawMode)
		list = append(list, &b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate bindings: %w", err)
	}

	return list, nil
}

func (r *postgresRepository) ListBoundResourcesByService(ctx context.Context, serviceID, envID uuid.UUID) ([]*BoundResourceView, error) {
	conn := r.txManager.Conn(ctx)

	query := `
		SELECT srb.id, r.id, r.slug, r.name, r.resource_type,
		       srb.access_mode, r.connection_metadata, srb.notes
		FROM service_resource_bindings srb
		JOIN resources r ON srb.resource_id = r.id
		WHERE srb.service_id = $1 AND srb.environment_id = $2
		ORDER BY r.name ASC
	`
	rows, err := conn.Query(ctx, query, serviceID, envID)
	if err != nil {
		return nil, fmt.Errorf("query bound resources: %w", err)
	}
	defer rows.Close()

	var list []*BoundResourceView
	for rows.Next() {
		var item BoundResourceView
		var metaBytes []byte
		err := rows.Scan(
			&item.BindingID,
			&item.ResourceID,
			&item.ResourceSlug,
			&item.ResourceName,
			&item.ResourceType,
			&item.AccessMode,
			&metaBytes,
			&item.Notes,
		)
		if err != nil {
			return nil, fmt.Errorf("scan bound resource row: %w", err)
		}

		if len(metaBytes) > 0 {
			if err := json.Unmarshal(metaBytes, &item.ConnectionMetadata); err != nil {
				return nil, fmt.Errorf("unmarshal connection metadata: %w", err)
			}
		} else {
			item.ConnectionMetadata = make(map[string]any)
		}

		list = append(list, &item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate bound resources: %w", err)
	}

	return list, nil
}

func (r *postgresRepository) CreateDependency(ctx context.Context, d *ServiceDependency) error {
	conn := r.txManager.Conn(ctx)

	if d.ID == uuid.Nil {
		d.ID = uuid.Must(uuid.NewV7())
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	if d.CreatedAt.IsZero() {
		d.CreatedAt = now
	}

	query := `
		INSERT INTO service_dependencies (
			id, organization_id, project_id, environment_id,
			caller_service_id, callee_service_id, protocol, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := conn.Exec(ctx, query,
		d.ID,
		d.OrganizationID,
		d.ProjectID,
		d.EnvironmentID,
		d.CallerServiceID,
		d.CalleeServiceID,
		string(d.Protocol),
		d.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperror.Conflict("service dependency already exists in this environment")
		}
		return fmt.Errorf("insert dependency: %w", err)
	}

	return nil
}

func (r *postgresRepository) ListDependenciesByEnvironment(ctx context.Context, orgID, projectID, envID uuid.UUID) ([]*ServiceDependency, error) {
	conn := r.txManager.Conn(ctx)

	query := `
		SELECT id, organization_id, project_id, environment_id,
		       caller_service_id, callee_service_id, protocol, created_at
		FROM service_dependencies
		WHERE organization_id = $1 AND project_id = $2 AND environment_id = $3
	`
	rows, err := conn.Query(ctx, query, orgID, projectID, envID)
	if err != nil {
		return nil, fmt.Errorf("query dependencies by env: %w", err)
	}
	defer rows.Close()

	var list []*ServiceDependency
	for rows.Next() {
		var d ServiceDependency
		var rawProto string
		err := rows.Scan(
			&d.ID,
			&d.OrganizationID,
			&d.ProjectID,
			&d.EnvironmentID,
			&d.CallerServiceID,
			&d.CalleeServiceID,
			&rawProto,
			&d.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan dependency: %w", err)
		}
		d.Protocol = DependencyProtocol(rawProto)
		list = append(list, &d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate dependencies: %w", err)
	}

	return list, nil
}

func (r *postgresRepository) ListDependenciesForService(
	ctx context.Context,
	serviceID, envID uuid.UUID,
) ([]*ServiceDependencyView, []*ServiceDependencyView, error) {
	conn := r.txManager.Conn(ctx)

	upstreamQuery := `
		SELECT sd.id, s.id, s.slug, s.name, sd.protocol
		FROM service_dependencies sd
		JOIN services s ON sd.caller_service_id = s.id
		WHERE sd.callee_service_id = $1 AND sd.environment_id = $2
		ORDER BY s.name ASC
	`
	uRows, err := conn.Query(ctx, upstreamQuery, serviceID, envID)
	if err != nil {
		return nil, nil, fmt.Errorf("query upstream dependencies: %w", err)
	}
	defer uRows.Close()

	var upstream []*ServiceDependencyView
	for uRows.Next() {
		var v ServiceDependencyView
		if err := uRows.Scan(&v.DependencyID, &v.ServiceID, &v.ServiceSlug, &v.ServiceName, &v.Protocol); err != nil {
			return nil, nil, fmt.Errorf("scan upstream dep: %w", err)
		}
		upstream = append(upstream, &v)
	}
	if err := uRows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate upstream deps: %w", err)
	}

	downstreamQuery := `
		SELECT sd.id, s.id, s.slug, s.name, sd.protocol
		FROM service_dependencies sd
		JOIN services s ON sd.callee_service_id = s.id
		WHERE sd.caller_service_id = $1 AND sd.environment_id = $2
		ORDER BY s.name ASC
	`
	dRows, err := conn.Query(ctx, downstreamQuery, serviceID, envID)
	if err != nil {
		return nil, nil, fmt.Errorf("query downstream dependencies: %w", err)
	}
	defer dRows.Close()

	var downstream []*ServiceDependencyView
	for dRows.Next() {
		var v ServiceDependencyView
		if err := dRows.Scan(&v.DependencyID, &v.ServiceID, &v.ServiceSlug, &v.ServiceName, &v.Protocol); err != nil {
			return nil, nil, fmt.Errorf("scan downstream dep: %w", err)
		}
		downstream = append(downstream, &v)
	}
	if err := dRows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate downstream deps: %w", err)
	}

	return upstream, downstream, nil
}
