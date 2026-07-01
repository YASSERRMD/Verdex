package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// WithTx begins a transaction on pool, invokes fn with a tx-scoped
// Executor, and commits if fn returns nil or rolls back if fn returns
// an error. If fn panics, WithTx rolls back the transaction and then
// re-panics with the original value, so callers see the same panic
// they would have without WithTx in the call stack.
//
// Because repository methods accept an Executor rather than a
// concrete *pgxpool.Pool, the same repository calls used outside a
// transaction compose directly inside fn:
//
//	err := persistence.WithTx(ctx, pool, func(ctx context.Context, exec persistence.Executor) error {
//	    if err := tenants.Create(ctx, exec, tenant); err != nil {
//	        return err
//	    }
//	    return deployments.Create(ctx, exec, deployment)
//	})
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(ctx context.Context, exec Executor) error) (err error) {
	if pool == nil {
		return fmt.Errorf("persistence: WithTx: pool must not be nil")
	}
	if fn == nil {
		return fmt.Errorf("persistence: WithTx: fn must not be nil")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("persistence: WithTx: begin: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
				// Best-effort: surface the rollback failure by
				// wrapping it into the panic value's context via a
				// second panic is not possible without losing p, so
				// prefer re-panicking with the original value; the
				// rollback failure is still the more actionable
				// signal in most cases, but the panic is what the
				// caller's test/observability tooling expects to see.
				panic(fmt.Errorf("persistence: WithTx: panic recovered (%v), rollback also failed: %w", p, rbErr))
			}
			panic(p)
		}
	}()

	if err = fn(ctx, tx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			return fmt.Errorf("persistence: WithTx: fn failed: %w; rollback also failed: %v", err, rbErr)
		}
		return err
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("persistence: WithTx: commit: %w", err)
	}
	return nil
}
