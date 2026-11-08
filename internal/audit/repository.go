package audit

import (
	"context"

	"github.com/google/uuid"
)

type QueryFilter struct {
	OrganizationID uuid.UUID
	Action         string
	TargetType     string
	EnvironmentID  *uuid.UUID
	ActorUserID    *uuid.UUID
	Limit          int
	Cursor         string
}

type AuditRepository interface {
	AppendWithLock(ctx context.Context, entry *AuditEntry) error
	GetLatestHash(ctx context.Context, orgID uuid.UUID) (string, error)
	ListPaginated(ctx context.Context, filter QueryFilter) ([]*AuditEntry, error)
	ListAllByOrg(ctx context.Context, orgID uuid.UUID) ([]*AuditEntry, error)
}
