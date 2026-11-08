package audit

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PathumSandeepa/iconfigly-api/internal/database"
	"github.com/PathumSandeepa/iconfigly-api/internal/shared/apperror"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type memoryRepository struct {
	mu      sync.Mutex
	entries []*AuditEntry
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{entries: make([]*AuditEntry, 0)}
}

func (m *memoryRepository) AppendWithLock(ctx context.Context, entry *AuditEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	latestHash := GenesisHash
	var latestCreatedAt time.Time
	for i := len(m.entries) - 1; i >= 0; i-- {
		if m.entries[i].OrganizationID == entry.OrganizationID {
			latestHash = m.entries[i].EntryHash
			latestCreatedAt = m.entries[i].CreatedAt
			break
		}
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
		return err
	}
	entry.EntryHash = hash

	m.entries = append(m.entries, entry)
	return nil
}

func (m *memoryRepository) GetLatestHash(ctx context.Context, orgID uuid.UUID) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := len(m.entries) - 1; i >= 0; i-- {
		if m.entries[i].OrganizationID == orgID {
			return m.entries[i].EntryHash, nil
		}
	}
	return GenesisHash, nil
}

func (m *memoryRepository) ListPaginated(ctx context.Context, filter QueryFilter) ([]*AuditEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []*AuditEntry
	for i := len(m.entries) - 1; i >= 0; i-- {
		e := m.entries[i]
		if e.OrganizationID != filter.OrganizationID {
			continue
		}
		if filter.Action != "" && e.Action != filter.Action {
			continue
		}
		if filter.TargetType != "" && e.TargetType != filter.TargetType {
			continue
		}
		result = append(result, e)
		if len(result) >= filter.Limit+1 {
			break
		}
	}
	return result, nil
}

func (m *memoryRepository) ListAllByOrg(ctx context.Context, orgID uuid.UUID) ([]*AuditEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []*AuditEntry
	for _, e := range m.entries {
		if e.OrganizationID == orgID {
			copied := *e
			result = append(result, &copied)
		}
	}
	return result, nil
}

func TestComputeHashDeterministic(t *testing.T) {
	orgID := uuid.Must(uuid.Parse("0192e226-9f44-7f1b-a951-8728d48bb789"))
	actorID := uuid.Must(uuid.Parse("0192e226-9f44-7f1b-a951-8728d48bb790"))
	targetID := uuid.Must(uuid.Parse("0192e226-9f44-7f1b-a951-8728d48bb791"))
	createdAt := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)

	entry1 := &AuditEntry{
		OrganizationID: orgID,
		ActorUserID:    &actorID,
		ActorType:      ActorTypeUser,
		Action:         "service.created",
		TargetType:     TargetTypeService,
		TargetID:       targetID,
		Metadata:       map[string]any{"slug": "payments", "tier": 1},
		PrevHash:       GenesisHash,
		CreatedAt:      createdAt,
	}

	entry2 := &AuditEntry{
		OrganizationID: orgID,
		ActorUserID:    &actorID,
		ActorType:      ActorTypeUser,
		Action:         "service.created",
		TargetType:     TargetTypeService,
		TargetID:       targetID,
		Metadata:       map[string]any{"tier": 1, "slug": "payments"},
		PrevHash:       GenesisHash,
		CreatedAt:      createdAt,
	}

	hash1, err := entry1.ComputeHash()
	if err != nil {
		t.Fatalf("compute hash 1: %v", err)
	}

	hash2, err := entry2.ComputeHash()
	if err != nil {
		t.Fatalf("compute hash 2: %v", err)
	}

	if hash1 != hash2 {
		t.Fatalf("expected identical hash regardless of key order, got %s != %s", hash1, hash2)
	}
}

func TestRecordValidatesRequiredFields(t *testing.T) {
	repo := newMemoryRepository()
	manager := database.NewTxManager(nil)
	svc := NewService(repo, manager)
	ctx := context.Background()

	validOrg := uuid.Must(uuid.NewV7())
	validTarget := uuid.Must(uuid.NewV7())

	tests := []struct {
		name      string
		cmd       RecordCmd
		wantField string
	}{
		{
			name: "missing organization_id",
			cmd: RecordCmd{
				Action:     "service.created",
				TargetType: TargetTypeService,
				TargetID:   validTarget,
			},
			wantField: "organization_id",
		},
		{
			name: "missing action",
			cmd: RecordCmd{
				OrganizationID: validOrg,
				TargetType:     TargetTypeService,
				TargetID:       validTarget,
			},
			wantField: "action",
		},
		{
			name: "missing target_type",
			cmd: RecordCmd{
				OrganizationID: validOrg,
				Action:         "service.created",
				TargetID:       validTarget,
			},
			wantField: "target_type",
		},
		{
			name: "missing target_id",
			cmd: RecordCmd{
				OrganizationID: validOrg,
				Action:         "service.created",
				TargetType:     TargetTypeService,
			},
			wantField: "target_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Record(ctx, tt.cmd)
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
			appErr, ok := apperror.From(err)
			if !ok {
				t.Fatalf("expected apperror, got %v", err)
			}
			if appErr.Code != apperror.CodeValidationFailed {
				t.Fatalf("expected code %s, got %s", apperror.CodeValidationFailed, appErr.Code)
			}
			if _, exists := appErr.Fields[tt.wantField]; !exists {
				t.Fatalf("expected field error for %s, got %v", tt.wantField, appErr.Fields)
			}
		})
	}
}

func TestVerifyHashChain(t *testing.T) {
	repo := newMemoryRepository()
	manager := database.NewTxManager(nil)
	svc := NewService(repo, manager)
	ctx := context.Background()
	orgID := uuid.Must(uuid.NewV7())

	resEmpty, err := svc.VerifyHashChain(ctx, orgID)
	if err != nil {
		t.Fatalf("verify empty: %v", err)
	}
	if !resEmpty.Valid || resEmpty.TotalCount != 0 {
		t.Fatalf("expected valid empty chain, got %+v", resEmpty)
	}

	for i := 0; i < 3; i++ {
		targetID := uuid.Must(uuid.NewV7())
		_, err := svc.Record(ctx, RecordCmd{
			OrganizationID: orgID,
			Action:         "resource.created",
			TargetType:     TargetTypeResource,
			TargetID:       targetID,
			Metadata:       map[string]any{"index": i},
		})
		if err != nil {
			t.Fatalf("record %d: %v", i, err)
		}
	}

	resValid, err := svc.VerifyHashChain(ctx, orgID)
	if err != nil {
		t.Fatalf("verify valid chain: %v", err)
	}
	if !resValid.Valid || resValid.TotalCount != 3 {
		t.Fatalf("expected valid chain with 3 entries, got %+v", resValid)
	}

	repo.entries[1].Metadata = map[string]any{"tampered": true}
	resTampered, err := svc.VerifyHashChain(ctx, orgID)
	if err != nil {
		t.Fatalf("verify tampered: %v", err)
	}
	if resTampered.Valid {
		t.Fatal("expected tampered chain to fail verification")
	}
	if resTampered.BrokenAtID != repo.entries[1].ID.String() {
		t.Fatalf("expected broken at id %s, got %s", repo.entries[1].ID, resTampered.BrokenAtID)
	}
}

func setupIntegrationDB(t *testing.T) (*pgxpool.Pool, *database.TxManager, uuid.UUID) {
	t.Helper()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping integration test")
	}

	pool, err := database.NewPostgres(dbURL)
	if err != nil {
		t.Fatalf("connect to database: %v", err)
	}
	t.Cleanup(pool.Close)

	orgID := uuid.Must(uuid.NewV7())
	testName := strings.ToLower(strings.ReplaceAll(t.Name(), "_", "-"))
	if len(testName) > 30 {
		testName = testName[:30]
	}
	slug := "audit-" + testName + "-" + orgID.String()[24:]
	_, err = pool.Exec(context.Background(),
		"INSERT INTO organizations (id, slug, name) VALUES ($1, $2, $3)",
		orgID, slug, "Audit Test Org",
	)
	if err != nil {
		t.Fatalf("insert test org: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM organizations WHERE id = $1", orgID)
	})

	txManager := database.NewTxManager(pool)
	return pool, txManager, orgID
}

func TestAuditLogIntegrationSequential(t *testing.T) {
	_, txManager, orgID := setupIntegrationDB(t)
	repo := NewPostgresRepository(txManager)
	svc := NewService(repo, txManager)
	ctx := context.Background()

	target1 := uuid.Must(uuid.NewV7())
	entry1, err := svc.Record(ctx, RecordCmd{
		OrganizationID: orgID,
		Action:         "org.initialized",
		TargetType:     TargetTypeOrganization,
		TargetID:       target1,
		Metadata:       map[string]any{"name": "Audit Test Org"},
	})
	if err != nil {
		t.Fatalf("record entry 1: %v", err)
	}
	if entry1.PrevHash != GenesisHash {
		t.Fatalf("expected first entry prev_hash to be GENESIS, got %s", entry1.PrevHash)
	}

	target2 := uuid.Must(uuid.NewV7())
	entry2, err := svc.Record(ctx, RecordCmd{
		OrganizationID: orgID,
		Action:         "project.created",
		TargetType:     TargetTypeProject,
		TargetID:       target2,
		Metadata:       map[string]any{"name": "Core"},
	})
	if err != nil {
		t.Fatalf("record entry 2: %v", err)
	}
	if entry2.PrevHash != entry1.EntryHash {
		t.Fatalf("expected entry 2 prev_hash to equal entry 1 entry_hash, got %s != %s", entry2.PrevHash, entry1.EntryHash)
	}

	res, err := svc.VerifyHashChain(ctx, orgID)
	if err != nil {
		t.Fatalf("verify hash chain: %v", err)
	}
	if !res.Valid || res.TotalCount != 2 {
		t.Fatalf("expected valid chain with 2 entries, got %+v", res)
	}
}

func TestAuditLogIntegrationConcurrentUnbrokenChain(t *testing.T) {
	_, txManager, orgID := setupIntegrationDB(t)
	repo := NewPostgresRepository(txManager)
	svc := NewService(repo, txManager)
	ctx := context.Background()

	concurrentWriters := 8
	var wg sync.WaitGroup
	errCh := make(chan error, concurrentWriters)

	for i := 0; i < concurrentWriters; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			targetID := uuid.Must(uuid.NewV7())
			_, err := svc.Record(ctx, RecordCmd{
				OrganizationID: orgID,
				Action:         "concurrent.mutation",
				TargetType:     TargetTypeService,
				TargetID:       targetID,
				Metadata:       map[string]any{"writer_index": idx},
			})
			if err != nil {
				errCh <- err
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatalf("concurrent record failed: %v", err)
	}

	res, err := svc.VerifyHashChain(ctx, orgID)
	if err != nil {
		t.Fatalf("verify concurrent hash chain: %v", err)
	}
	if !res.Valid {
		t.Fatalf("concurrent insertions broke the hash chain: %s", res.Reason)
	}
	if res.TotalCount != concurrentWriters {
		t.Fatalf("expected %d entries, got %d", concurrentWriters, res.TotalCount)
	}
}

func TestAuditLogIntegrationImmutabilityTrigger(t *testing.T) {
	pool, txManager, orgID := setupIntegrationDB(t)
	repo := NewPostgresRepository(txManager)
	svc := NewService(repo, txManager)
	ctx := context.Background()

	targetID := uuid.Must(uuid.NewV7())
	entry, err := svc.Record(ctx, RecordCmd{
		OrganizationID: orgID,
		Action:         "test.mutation",
		TargetType:     TargetTypeResource,
		TargetID:       targetID,
	})
	if err != nil {
		t.Fatalf("record entry: %v", err)
	}

	_, err = pool.Exec(ctx, "UPDATE audit_logs SET action = 'tampered' WHERE id = $1", entry.ID)
	if err == nil {
		t.Fatal("expected UPDATE on audit_logs to be rejected by trigger")
	}

	_, err = pool.Exec(ctx, "DELETE FROM audit_logs WHERE id = $1", entry.ID)
	if err == nil {
		t.Fatal("expected DELETE on audit_logs to be rejected by trigger")
	}
}

func TestAuditLogIntegrationRollback(t *testing.T) {
	_, txManager, orgID := setupIntegrationDB(t)
	repo := NewPostgresRepository(txManager)
	svc := NewService(repo, txManager)
	ctx := context.Background()

	failureErr := errors.New("business rule failed")

	err := txManager.WithTx(ctx, func(txCtx context.Context) error {
		targetID := uuid.Must(uuid.NewV7())
		_, err := svc.Record(txCtx, RecordCmd{
			OrganizationID: orgID,
			Action:         "should.rollback",
			TargetType:     TargetTypeTeam,
			TargetID:       targetID,
		})
		if err != nil {
			return err
		}
		return failureErr
	})

	if !errors.Is(err, failureErr) {
		t.Fatalf("expected failure error, got %v", err)
	}

	res, err := svc.VerifyHashChain(ctx, orgID)
	if err != nil {
		t.Fatalf("verify chain: %v", err)
	}
	if res.TotalCount != 0 {
		t.Fatalf("expected 0 entries after rollback, got %d", res.TotalCount)
	}
}
