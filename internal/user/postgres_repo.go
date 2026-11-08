package user

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

func NewPostgresRepository(txManager *database.TxManager) UserRepository {
	return &postgresRepository{txManager: txManager}
}

func (r *postgresRepository) Create(ctx context.Context, u *User) error {
	conn := r.txManager.Conn(ctx)

	if u.ID == uuid.Nil {
		u.ID = uuid.Must(uuid.NewV7())
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	if u.CreatedAt.IsZero() {
		u.CreatedAt = now
	}
	if u.UpdatedAt.IsZero() {
		u.UpdatedAt = now
	}
	if u.Status == "" {
		u.Status = StatusActive
	}

	query := `
		INSERT INTO users (
			id, organization_id, email, full_name,
			password_hash, is_super_admin, status,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := conn.Exec(ctx, query,
		u.ID,
		u.OrganizationID,
		u.Email.String(),
		u.FullName,
		u.PasswordHash,
		u.IsSuperAdmin,
		string(u.Status),
		u.CreatedAt,
		u.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return apperror.Conflict("email already registered")
		}
		return fmt.Errorf("insert user: %w", err)
	}

	return nil
}

func (r *postgresRepository) GetByID(ctx context.Context, orgID, userID uuid.UUID) (*User, error) {
	conn := r.txManager.Conn(ctx)

	query := `
		SELECT id, organization_id, email, full_name,
		       password_hash, is_super_admin, status,
		       created_at, updated_at
		FROM users
		WHERE organization_id = $1 AND id = $2
	`
	return scanUser(conn.QueryRow(ctx, query, orgID, userID))
}

func (r *postgresRepository) GetByEmail(ctx context.Context, email valueobject.Email) (*User, error) {
	conn := r.txManager.Conn(ctx)

	query := `
		SELECT id, organization_id, email, full_name,
		       password_hash, is_super_admin, status,
		       created_at, updated_at
		FROM users
		WHERE email = $1
	`
	return scanUser(conn.QueryRow(ctx, query, email.String()))
}

func (r *postgresRepository) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]*User, error) {
	conn := r.txManager.Conn(ctx)

	query := `
		SELECT id, organization_id, email, full_name,
		       password_hash, is_super_admin, status,
		       created_at, updated_at
		FROM users
		WHERE organization_id = $1
		ORDER BY created_at ASC
	`
	rows, err := conn.Query(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("query users by org: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}

	return users, nil
}

func (r *postgresRepository) UpdateStatus(ctx context.Context, orgID, userID uuid.UUID, status UserStatus) error {
	conn := r.txManager.Conn(ctx)

	now := time.Now().UTC().Truncate(time.Microsecond)
	query := `UPDATE users SET status = $1, updated_at = $2 WHERE organization_id = $3 AND id = $4`
	tag, err := conn.Exec(ctx, query, string(status), now, orgID, userID)
	if err != nil {
		return fmt.Errorf("update user status: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return apperror.NotFound("user")
	}

	return nil
}

func (r *postgresRepository) CountActiveSuperAdmins(ctx context.Context, orgID uuid.UUID) (int64, error) {
	conn := r.txManager.Conn(ctx)

	query := `SELECT COUNT(*) FROM users WHERE organization_id = $1 AND is_super_admin = TRUE AND status = 'ACTIVE'`
	var count int64
	err := conn.QueryRow(ctx, query, orgID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count active super admins: %w", err)
	}

	return count, nil
}

func scanUser(row pgx.Row) (*User, error) {
	var u User
	var rawEmail string
	var rawStatus string

	err := row.Scan(
		&u.ID,
		&u.OrganizationID,
		&rawEmail,
		&u.FullName,
		&u.PasswordHash,
		&u.IsSuperAdmin,
		&rawStatus,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperror.NotFound("user")
		}
		return nil, fmt.Errorf("scan user: %w", err)
	}

	email, err := valueobject.NewEmail(rawEmail)
	if err != nil {
		return nil, err
	}
	u.Email = email
	u.Status = UserStatus(rawStatus)

	return &u, nil
}
