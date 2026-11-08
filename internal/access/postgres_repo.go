package access

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

func NewPostgresRepository(txManager *database.TxManager) AccessRepository {
	return &postgresRepository{txManager: txManager}
}

func (r *postgresRepository) Create(ctx context.Context, binding *RoleBinding) error {
	conn := r.txManager.Conn(ctx)

	if binding.ID == uuid.Nil {
		binding.ID = uuid.Must(uuid.NewV7())
	}
	if binding.CreatedAt.IsZero() {
		binding.CreatedAt = time.Now().UTC().Truncate(time.Microsecond)
	}

	query := `
		INSERT INTO role_bindings (
			id, organization_id, subject_type, subject_id,
			role_name, scope_type, scope_id, granted_by,
			expires_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := conn.Exec(ctx, query,
		binding.ID,
		binding.OrganizationID,
		binding.SubjectType,
		binding.SubjectID,
		binding.RoleName,
		binding.ScopeType,
		binding.ScopeID,
		binding.GrantedBy,
		binding.ExpiresAt,
		binding.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperror.Conflict("role binding already exists for this subject, role, and scope")
		}
		return fmt.Errorf("insert role binding: %w", err)
	}

	return nil
}

func (r *postgresRepository) GetByID(ctx context.Context, orgID uuid.UUID, id uuid.UUID) (*RoleBinding, error) {
	conn := r.txManager.Conn(ctx)

	query := `
		SELECT id, organization_id, subject_type, subject_id,
		       role_name, scope_type, scope_id, granted_by,
		       expires_at, created_at
		FROM role_bindings
		WHERE organization_id = $1 AND id = $2
	`
	binding, err := scanRoleBinding(conn.QueryRow(ctx, query, orgID, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NotFound("role binding")
		}
		return nil, fmt.Errorf("get role binding by id: %w", err)
	}

	return binding, nil
}

func (r *postgresRepository) Delete(ctx context.Context, orgID uuid.UUID, id uuid.UUID) error {
	conn := r.txManager.Conn(ctx)

	query := `DELETE FROM role_bindings WHERE organization_id = $1 AND id = $2`
	tag, err := conn.Exec(ctx, query, orgID, id)
	if err != nil {
		return fmt.Errorf("delete role binding: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return apperror.NotFound("role binding")
	}

	return nil
}

func (r *postgresRepository) ListByProject(ctx context.Context, orgID uuid.UUID, projectID uuid.UUID) ([]*RoleBinding, error) {
	conn := r.txManager.Conn(ctx)

	query := `
		SELECT id, organization_id, subject_type, subject_id,
		       role_name, scope_type, scope_id, granted_by,
		       expires_at, created_at
		FROM role_bindings
		WHERE organization_id = $1
		  AND (
		    (scope_type = 'PROJECT' AND scope_id = $2)
		    OR scope_id IN (SELECT id FROM environments WHERE project_id = $2)
		    OR scope_id IN (SELECT id FROM services WHERE project_id = $2)
		    OR scope_id IN (SELECT id FROM resources WHERE project_id = $2)
		  )
		ORDER BY created_at DESC
	`
	rows, err := conn.Query(ctx, query, orgID, projectID)
	if err != nil {
		return nil, fmt.Errorf("list role bindings by project: %w", err)
	}
	defer rows.Close()

	var result []*RoleBinding
	for rows.Next() {
		b, err := scanRoleBinding(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate role bindings: %w", err)
	}

	return result, nil
}

func (r *postgresRepository) ListByScope(ctx context.Context, orgID uuid.UUID, scopeType ScopeType, scopeID uuid.UUID) ([]*RoleBinding, error) {
	conn := r.txManager.Conn(ctx)

	query := `
		SELECT id, organization_id, subject_type, subject_id,
		       role_name, scope_type, scope_id, granted_by,
		       expires_at, created_at
		FROM role_bindings
		WHERE organization_id = $1 AND scope_type = $2 AND scope_id = $3
		ORDER BY created_at DESC
	`
	rows, err := conn.Query(ctx, query, orgID, scopeType, scopeID)
	if err != nil {
		return nil, fmt.Errorf("list role bindings by scope: %w", err)
	}
	defer rows.Close()

	var result []*RoleBinding
	for rows.Next() {
		b, err := scanRoleBinding(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate role bindings: %w", err)
	}

	return result, nil
}

func (r *postgresRepository) FindActiveBindingsForActor(
	ctx context.Context,
	orgID uuid.UUID,
	userID uuid.UUID,
	teamIDs []uuid.UUID,
	scopes []ScopeRef,
	now time.Time,
) ([]*RoleBinding, error) {
	if len(scopes) == 0 {
		return nil, nil
	}

	conn := r.txManager.Conn(ctx)

	query := `
		SELECT id, organization_id, subject_type, subject_id,
		       role_name, scope_type, scope_id, granted_by,
		       expires_at, created_at
		FROM role_bindings
		WHERE organization_id = $1
		  AND (expires_at IS NULL OR expires_at > $2)
	`
	args := []any{orgID, now}
	argIdx := 3

	if len(teamIDs) > 0 {
		query += fmt.Sprintf(" AND ((subject_type = 'USER' AND subject_id = $%d) OR (subject_type = 'TEAM' AND subject_id = ANY($%d)))", argIdx, argIdx+1)
		args = append(args, userID, teamIDs)
		argIdx += 2
	} else {
		query += fmt.Sprintf(" AND (subject_type = 'USER' AND subject_id = $%d)", argIdx)
		args = append(args, userID)
		argIdx++
	}

	scopeClauses := make([]string, 0, len(scopes))
	for _, sc := range scopes {
		scopeClauses = append(scopeClauses, fmt.Sprintf("(scope_type = $%d AND scope_id = $%d)", argIdx, argIdx+1))
		args = append(args, sc.Type, sc.ID)
		argIdx += 2
	}
	query += " AND (" + strings.Join(scopeClauses, " OR ") + ") ORDER BY created_at DESC"

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("find active bindings for actor: %w", err)
	}
	defer rows.Close()

	var result []*RoleBinding
	for rows.Next() {
		b, err := scanRoleBinding(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active role bindings: %w", err)
	}

	return result, nil
}

func (r *postgresRepository) FindActiveBindingsForScopes(
	ctx context.Context,
	orgID uuid.UUID,
	scopes []ScopeRef,
	now time.Time,
) ([]*RoleBinding, error) {
	if len(scopes) == 0 {
		return nil, nil
	}

	conn := r.txManager.Conn(ctx)

	query := `
		SELECT id, organization_id, subject_type, subject_id,
		       role_name, scope_type, scope_id, granted_by,
		       expires_at, created_at
		FROM role_bindings
		WHERE organization_id = $1
		  AND (expires_at IS NULL OR expires_at > $2)
	`
	args := []any{orgID, now}
	argIdx := 3

	scopeClauses := make([]string, 0, len(scopes))
	for _, sc := range scopes {
		scopeClauses = append(scopeClauses, fmt.Sprintf("(scope_type = $%d AND scope_id = $%d)", argIdx, argIdx+1))
		args = append(args, sc.Type, sc.ID)
		argIdx += 2
	}
	query += " AND (" + strings.Join(scopeClauses, " OR ") + ") ORDER BY created_at DESC"

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("find active bindings for scopes: %w", err)
	}
	defer rows.Close()

	var result []*RoleBinding
	for rows.Next() {
		b, err := scanRoleBinding(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active role bindings: %w", err)
	}

	return result, nil
}

func scanRoleBinding(scanner pgx.Row) (*RoleBinding, error) {
	var b RoleBinding
	err := scanner.Scan(
		&b.ID,
		&b.OrganizationID,
		&b.SubjectType,
		&b.SubjectID,
		&b.RoleName,
		&b.ScopeType,
		&b.ScopeID,
		&b.GrantedBy,
		&b.ExpiresAt,
		&b.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &b, nil
}
