package persistence

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestWithTx_NilPool(t *testing.T) {
	t.Parallel()

	called := false
	err := WithTx(context.Background(), nil, func(_ context.Context, _ Executor) error {
		called = true
		return nil
	})
	if err == nil {
		t.Fatal("expected error for nil pool, got nil")
	}
	if called {
		t.Fatal("fn must not be called when pool is nil")
	}
}

func TestWithTx_NilFn(t *testing.T) {
	t.Parallel()

	// A zero-value *pgxpool.Pool is non-nil and sufficient here: the
	// nil-fn validation in WithTx runs before any method is invoked
	// on pool, so this never dials a database.
	pool := &pgxpool.Pool{}
	err := WithTx(context.Background(), pool, nil)
	if err == nil {
		t.Fatal("expected error for nil fn, got nil")
	}
}
