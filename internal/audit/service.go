package audit

import (
	"context"
	"fmt"

	"github.com/PathumSandeepa/iconfigly-api/internal/database"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/httputil"
	"github.com/google/uuid"
)

type RecordCmd struct {
	OrganizationID uuid.UUID
	ActorUserID    *uuid.UUID
	ActorType      string
	Action         string
	TargetType     string
	TargetID       uuid.UUID
	EnvironmentID  *uuid.UUID
	Metadata       map[string]any
	IPAddress      string
}

type VerificationResult struct {
	Valid      bool   `json:"valid"`
	TotalCount int    `json:"total_count"`
	BrokenAtID string `json:"broken_at_id,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

type Service struct {
	repo      AuditRepository
	txManager *database.TxManager
}

func NewService(repo AuditRepository, txManager *database.TxManager) *Service {
	return &Service{
		repo:      repo,
		txManager: txManager,
	}
}

func (s *Service) Record(ctx context.Context, cmd RecordCmd) (*AuditEntry, error) {
	fieldErrors := make(map[string]string)

	if cmd.OrganizationID == uuid.Nil {
		fieldErrors["organization_id"] = "is required"
	}
	if cmd.Action == "" {
		fieldErrors["action"] = "is required"
	}
	if cmd.TargetType == "" {
		fieldErrors["target_type"] = "is required"
	}
	if cmd.TargetID == uuid.Nil {
		fieldErrors["target_id"] = "is required"
	}

	if len(fieldErrors) > 0 {
		return nil, apperror.Validation("invalid audit record command", fieldErrors)
	}

	actorType := cmd.ActorType
	if actorType == "" {
		if cmd.ActorUserID != nil {
			actorType = ActorTypeUser
		} else {
			actorType = ActorTypeSystem
		}
	}

	meta := cmd.Metadata
	if meta == nil {
		meta = map[string]any{}
	}

	entry := &AuditEntry{
		OrganizationID: cmd.OrganizationID,
		ActorUserID:    cmd.ActorUserID,
		ActorType:      actorType,
		Action:         cmd.Action,
		TargetType:     cmd.TargetType,
		TargetID:       cmd.TargetID,
		EnvironmentID:  cmd.EnvironmentID,
		Metadata:       meta,
		IPAddress:      cmd.IPAddress,
	}

	err := s.txManager.WithTx(ctx, func(txCtx context.Context) error {
		return s.repo.AppendWithLock(txCtx, entry)
	})
	if err != nil {
		return nil, err
	}

	return entry, nil
}

func (s *Service) QueryAuditLogs(ctx context.Context, filter QueryFilter) (httputil.PageResponse[*AuditEntry], error) {
	if filter.OrganizationID == uuid.Nil {
		return httputil.PageResponse[*AuditEntry]{}, apperror.Validation("invalid audit query", map[string]string{
			"organization_id": "is required",
		})
	}

	if filter.Limit <= 0 {
		filter.Limit = 25
	}
	if filter.Limit > 100 {
		return httputil.PageResponse[*AuditEntry]{}, apperror.Validation("invalid audit query", map[string]string{
			"limit": "must be an integer between 1 and 100",
		})
	}

	entries, err := s.repo.ListPaginated(ctx, filter)
	if err != nil {
		return httputil.PageResponse[*AuditEntry]{}, err
	}

	return httputil.NewPageResponse(entries, filter.Limit, func(e *AuditEntry) string {
		return httputil.EncodeCursor(e.ID, e.CreatedAt)
	}), nil
}

func (s *Service) VerifyHashChain(ctx context.Context, orgID uuid.UUID) (*VerificationResult, error) {
	if orgID == uuid.Nil {
		return nil, apperror.Validation("invalid verification query", map[string]string{
			"organization_id": "is required",
		})
	}

	entries, err := s.repo.ListAllByOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}

	if len(entries) == 0 {
		return &VerificationResult{
			Valid:      true,
			TotalCount: 0,
		}, nil
	}

	expectedPrevHash := GenesisHash
	for _, entry := range entries {
		if entry.PrevHash != expectedPrevHash {
			return &VerificationResult{
				Valid:      false,
				TotalCount: len(entries),
				BrokenAtID: entry.ID.String(),
				Reason:     fmt.Sprintf("prev_hash mismatch at entry %s: expected %s, got %s", entry.ID, expectedPrevHash, entry.PrevHash),
			}, nil
		}

		expectedEntryHash, err := entry.ComputeHash()
		if err != nil {
			return nil, fmt.Errorf("recompute hash for entry %s: %w", entry.ID, err)
		}

		if entry.EntryHash != expectedEntryHash {
			return &VerificationResult{
				Valid:      false,
				TotalCount: len(entries),
				BrokenAtID: entry.ID.String(),
				Reason:     fmt.Sprintf("entry_hash mismatch at entry %s: recorded %s, recomputed %s", entry.ID, entry.EntryHash, expectedEntryHash),
			}, nil
		}

		expectedPrevHash = entry.EntryHash
	}

	return &VerificationResult{
		Valid:      true,
		TotalCount: len(entries),
	}, nil
}
