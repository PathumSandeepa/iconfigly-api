package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/PathumSandeepa/iconfigly-api/internal/database"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/httputil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type postgresRepository struct {
	txManager *database.TxManager
}

func NewPostgresRepository(txManager *database.TxManager) AuditRepository {
	return &postgresRepository{txManager: txManager}
}

func (r *postgresRepository) AppendWithLock(ctx context.Context, entry *AuditEntry) error {
	conn := r.txManager.Conn(ctx)

	_, err := conn.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext($1::text))", entry.OrganizationID.String())
	if err != nil {
		return fmt.Errorf("acquire advisory lock: %w", err)
	}

	latestHash, latestCreatedAt, err := r.getLatestRowWithConn(ctx, conn, entry.OrganizationID)
	if err != nil {
		return err
	}
	entry.PrevHash = latestHash

	now := time.Now().UTC().Truncate(time.Microsecond)
	if !latestCreatedAt.IsZero() && !now.After(latestCreatedAt) {
		now = latestCreatedAt.Add(time.Microsecond)
	}
	entry.CreatedAt = now

	if entry.ID == uuid.Nil {
		entry.ID = uuid.Must(uuid.NewV7())
	}

	hash, err := entry.ComputeHash()
	if err != nil {
		return fmt.Errorf("compute entry hash: %w", err)
	}
	entry.EntryHash = hash

	meta := entry.Metadata
	if meta == nil {
		meta = map[string]any{}
	}
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	query := `
		INSERT INTO audit_logs (
			id, organization_id, actor_user_id, actor_type,
			action, target_type, target_id, environment_id,
			metadata, ip_address, prev_hash, entry_hash, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	_, err = conn.Exec(ctx, query,
		entry.ID,
		entry.OrganizationID,
		entry.ActorUserID,
		entry.ActorType,
		entry.Action,
		entry.TargetType,
		entry.TargetID,
		entry.EnvironmentID,
		metaBytes,
		entry.IPAddress,
		entry.PrevHash,
		entry.EntryHash,
		entry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}

	return nil
}

func (r *postgresRepository) GetLatestHash(ctx context.Context, orgID uuid.UUID) (string, error) {
	conn := r.txManager.Conn(ctx)
	hash, _, err := r.getLatestRowWithConn(ctx, conn, orgID)
	return hash, err
}

func (r *postgresRepository) getLatestRowWithConn(ctx context.Context, conn database.DBTX, orgID uuid.UUID) (string, time.Time, error) {
	query := `
		SELECT entry_hash, created_at
		FROM audit_logs
		WHERE organization_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`
	var latestHash string
	var latestCreatedAt time.Time
	err := conn.QueryRow(ctx, query, orgID).Scan(&latestHash, &latestCreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return GenesisHash, time.Time{}, nil
	}
	if err != nil {
		return "", time.Time{}, fmt.Errorf("query latest audit hash: %w", err)
	}
	return latestHash, latestCreatedAt, nil
}

func (r *postgresRepository) ListPaginated(ctx context.Context, filter QueryFilter) ([]*AuditEntry, error) {
	conn := r.txManager.Conn(ctx)

	limit := filter.Limit
	if limit <= 0 {
		limit = 25
	}
	fetchLimit := limit + 1

	query := `
		SELECT id, organization_id, actor_user_id, actor_type,
		       action, target_type, target_id, environment_id,
		       metadata, ip_address, prev_hash, entry_hash, created_at
		FROM audit_logs
		WHERE organization_id = $1
	`
	args := []any{filter.OrganizationID}
	argIdx := 2

	if filter.Action != "" {
		query += fmt.Sprintf(" AND action = $%d", argIdx)
		args = append(args, filter.Action)
		argIdx++
	}
	if filter.TargetType != "" {
		query += fmt.Sprintf(" AND target_type = $%d", argIdx)
		args = append(args, filter.TargetType)
		argIdx++
	}
	if filter.EnvironmentID != nil {
		query += fmt.Sprintf(" AND environment_id = $%d", argIdx)
		args = append(args, *filter.EnvironmentID)
		argIdx++
	}
	if filter.ActorUserID != nil {
		query += fmt.Sprintf(" AND actor_user_id = $%d", argIdx)
		args = append(args, *filter.ActorUserID)
		argIdx++
	}

	if filter.Cursor != "" {
		cursorID, cursorCreatedAt, err := httputil.DecodeCursor(filter.Cursor)
		if err == nil && cursorID != uuid.Nil {
			query += fmt.Sprintf(" AND (created_at < $%d OR (created_at = $%d AND id < $%d))", argIdx, argIdx, argIdx+1)
			args = append(args, cursorCreatedAt, cursorID)
			argIdx += 2
		}
	}

	query += fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d", argIdx)
	args = append(args, fetchLimit)

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query audit logs: %w", err)
	}
	defer rows.Close()

	var entries []*AuditEntry
	for rows.Next() {
		entry, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit logs: %w", err)
	}

	return entries, nil
}

func (r *postgresRepository) ListAllByOrg(ctx context.Context, orgID uuid.UUID) ([]*AuditEntry, error) {
	conn := r.txManager.Conn(ctx)

	query := `
		SELECT id, organization_id, actor_user_id, actor_type,
		       action, target_type, target_id, environment_id,
		       metadata, ip_address, prev_hash, entry_hash, created_at
		FROM audit_logs
		WHERE organization_id = $1
		ORDER BY created_at ASC, id ASC
	`
	rows, err := conn.Query(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("query all audit logs: %w", err)
	}
	defer rows.Close()

	var entries []*AuditEntry
	for rows.Next() {
		entry, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit logs: %w", err)
	}

	return entries, nil
}

func scanEntry(scanner pgx.Row) (*AuditEntry, error) {
	var entry AuditEntry
	var rawMeta []byte

	err := scanner.Scan(
		&entry.ID,
		&entry.OrganizationID,
		&entry.ActorUserID,
		&entry.ActorType,
		&entry.Action,
		&entry.TargetType,
		&entry.TargetID,
		&entry.EnvironmentID,
		&rawMeta,
		&entry.IPAddress,
		&entry.PrevHash,
		&entry.EntryHash,
		&entry.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan audit log entry: %w", err)
	}

	if len(rawMeta) > 0 {
		if err := json.Unmarshal(rawMeta, &entry.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal audit metadata: %w", err)
		}
	}
	if entry.Metadata == nil {
		entry.Metadata = map[string]any{}
	}

	return &entry, nil
}
