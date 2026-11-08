package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

var (
	_ DBTX = (*pgxpool.Pool)(nil)
	_ DBTX = (pgx.Tx)(nil)
)

type txContextKey struct{}

type TxManager struct {
	pool *pgxpool.Pool
}

func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{pool: pool}
}

func (m *TxManager) WithTx(ctx context.Context, fn func(txCtx context.Context) error) error {
	if _, active := txFromContext(ctx); active {
		return fn(ctx)
	}

	if m.pool == nil {
		return fn(ctx)
	}

	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			_ = tx.Rollback(context.WithoutCancel(ctx))
			panic(recovered)
		}
	}()

	if err := fn(context.WithValue(ctx, txContextKey{}, tx)); err != nil {
		return rollback(ctx, tx, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (m *TxManager) Conn(ctx context.Context) DBTX {
	if tx, active := txFromContext(ctx); active {
		return tx
	}

	return m.pool
}

func txFromContext(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txContextKey{}).(pgx.Tx)

	return tx, ok
}

func rollback(ctx context.Context, tx pgx.Tx, cause error) error {
	if err := tx.Rollback(context.WithoutCancel(ctx)); err != nil {
		return errors.Join(cause, fmt.Errorf("rollback transaction: %w", err))
	}

	return cause
}
