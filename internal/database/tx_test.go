package database

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func newIntegrationTable(t *testing.T) (*pgxpool.Pool, *TxManager, string) {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set; skipping PostgreSQL integration test")
	}

	pool, err := NewPostgres(databaseURL)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	t.Cleanup(pool.Close)

	table := "tx_manager_" + strings.ToLower(strings.TrimPrefix(t.Name(), "Test"))
	ctx := context.Background()

	if _, err := pool.Exec(ctx, "CREATE TABLE "+table+" (id INT PRIMARY KEY)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, "DROP TABLE IF EXISTS "+table) })

	return pool, NewTxManager(pool), table
}

func countRows(t *testing.T, pool *pgxpool.Pool, table string) int {
	t.Helper()

	var count int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM "+table).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}

	return count
}

func insertRow(ctx context.Context, m *TxManager, table string, id int) error {
	_, err := m.Conn(ctx).Exec(ctx, "INSERT INTO "+table+" (id) VALUES ($1)", id)

	return err
}

func TestWithTxCommitsOnSuccess(t *testing.T) {
	pool, manager, table := newIntegrationTable(t)

	err := manager.WithTx(context.Background(), func(ctx context.Context) error {
		return insertRow(ctx, manager, table, 1)
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got := countRows(t, pool, table); got != 1 {
		t.Fatalf("expected 1 committed row, got %d", got)
	}
}

func TestWithTxRollsBackOnError(t *testing.T) {
	pool, manager, table := newIntegrationTable(t)
	failure := errors.New("business rule violated")

	err := manager.WithTx(context.Background(), func(ctx context.Context) error {
		if err := insertRow(ctx, manager, table, 1); err != nil {
			return err
		}
		return failure
	})

	if !errors.Is(err, failure) {
		t.Fatalf("expected the original error, got %v", err)
	}
	if got := countRows(t, pool, table); got != 0 {
		t.Fatalf("expected the insert to be rolled back, got %d rows", got)
	}
}

func TestWithTxRollsBackAndRepanicsOnPanic(t *testing.T) {
	pool, manager, table := newIntegrationTable(t)

	func() {
		defer func() {
			if recovered := recover(); recovered != "boom" {
				t.Fatalf("expected the panic to propagate, got %v", recovered)
			}
		}()

		_ = manager.WithTx(context.Background(), func(ctx context.Context) error {
			if err := insertRow(ctx, manager, table, 1); err != nil {
				return err
			}
			panic("boom")
		})
	}()

	if got := countRows(t, pool, table); got != 0 {
		t.Fatalf("expected the insert to be rolled back, got %d rows", got)
	}
}

func TestWithTxJoinsOuterTransaction(t *testing.T) {
	pool, manager, table := newIntegrationTable(t)
	failure := errors.New("outer failure")

	err := manager.WithTx(context.Background(), func(outer context.Context) error {
		inner := manager.WithTx(outer, func(innerCtx context.Context) error {
			return insertRow(innerCtx, manager, table, 1)
		})
		if inner != nil {
			return inner
		}
		return failure
	})

	if !errors.Is(err, failure) {
		t.Fatalf("expected the outer failure, got %v", err)
	}
	if got := countRows(t, pool, table); got != 0 {
		t.Fatalf("expected the nested insert to roll back with the outer transaction, got %d rows", got)
	}
}

func TestConnReturnsPoolOutsideTransactionAndTxInside(t *testing.T) {
	pool, manager, _ := newIntegrationTable(t)

	if manager.Conn(context.Background()) != DBTX(pool) {
		t.Fatal("expected the pool outside a transaction")
	}

	err := manager.WithTx(context.Background(), func(ctx context.Context) error {
		if manager.Conn(ctx) == DBTX(pool) {
			t.Fatal("expected the transaction inside WithTx")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
