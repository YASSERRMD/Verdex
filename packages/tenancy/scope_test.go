package tenancy_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/YASSERRMD/verdex/packages/persistence"
	"github.com/YASSERRMD/verdex/packages/tenancy"
)

// unconnectedPool builds a *pgxpool.Pool from valid config without
// dialing a real database (pgxpool connects lazily), so the
// input-validation paths of WithTenantScope can be unit tested without
// a live Postgres instance.
func unconnectedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	cfg, err := pgxpool.ParseConfig("postgres://user:pass@127.0.0.1:1/db")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewWithConfig: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestWithTenantScope_RejectsNilPool(t *testing.T) {
	err := tenancy.WithTenantScope(context.Background(), nil, uuid.New(),
		func(ctx context.Context, exec persistence.Executor) error { return nil })
	if err == nil {
		t.Fatal("expected an error for a nil pool")
	}
}

func TestWithTenantScope_RejectsNilTenantID(t *testing.T) {
	err := tenancy.WithTenantScope(context.Background(), unconnectedPool(t), uuid.Nil,
		func(ctx context.Context, exec persistence.Executor) error { return nil })
	if err == nil {
		t.Fatal("expected an error for a nil tenant ID")
	}
}

func TestWithTenantScope_RejectsNilFn(t *testing.T) {
	err := tenancy.WithTenantScope(context.Background(), unconnectedPool(t), uuid.New(), nil)
	if err == nil {
		t.Fatal("expected an error for a nil fn")
	}
}
