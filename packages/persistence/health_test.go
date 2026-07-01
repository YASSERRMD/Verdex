package persistence

import (
	"context"
	"testing"

	"github.com/YASSERRMD/verdex/packages/observability"
)

func TestPostgresChecker_NilPostgres(t *testing.T) {
	t.Parallel()

	checker := PostgresChecker(nil)
	if err := checker(context.Background()); err == nil {
		t.Fatal("expected error for nil *Postgres, got nil")
	}
}

func TestPostgresChecker_UninitializedPool(t *testing.T) {
	t.Parallel()

	checker := PostgresChecker(&Postgres{})
	if err := checker(context.Background()); err == nil {
		t.Fatal("expected error for uninitialized pool, got nil")
	}
}

// TestChecker_CompatibleWithObservabilityNamedChecker locks in that
// persistence.Checker's signature matches observability.Checker's
// (func(ctx context.Context) error), the contract commit 7 of this
// phase is required to honor without importing packages/observability
// as a build dependency of this package.
func TestChecker_CompatibleWithObservabilityNamedChecker(t *testing.T) {
	t.Parallel()

	checker := PostgresChecker(nil)
	named := observability.NamedChecker{
		Name:    "postgres",
		Checker: observability.Checker(checker),
	}
	if err := named.Checker(context.Background()); err == nil {
		t.Fatal("expected error from wrapped nil-postgres checker, got nil")
	}
}
