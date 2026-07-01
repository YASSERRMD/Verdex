package tenancy_test

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/YASSERRMD/verdex/packages/config"
	"github.com/YASSERRMD/verdex/packages/persistence"
	"github.com/YASSERRMD/verdex/packages/tenancy"
)

// containerStartTimeout mirrors packages/persistence/integration_test.go's
// constant of the same name: it bounds how long we wait for Docker to
// pull and start the Postgres container, so an unreachable/unresponsive
// local Docker daemon causes tests to skip rather than hang.
const containerStartTimeout = 30 * time.Second

// requirePostgresContainer starts an ephemeral Postgres container for
// the duration of the test and returns its connection DSN, following
// the exact skip/timeout pattern established in
// packages/persistence/integration_test.go.
func requirePostgresContainer(t *testing.T) string {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping testcontainers-backed integration test in -short mode")
	}

	startCtx, cancel := context.WithTimeout(context.Background(), containerStartTimeout)
	defer cancel()

	ctr, err := tcpostgres.Run(startCtx, "postgres:16-alpine",
		tcpostgres.WithDatabase("verdex_test"),
		tcpostgres.WithUsername("verdex"),
		tcpostgres.WithPassword("verdex-test-password"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(containerStartTimeout),
		),
	)
	if err != nil {
		t.Skipf("skipping: could not start Postgres testcontainer (Docker unavailable?): %v", err)
	}

	t.Cleanup(func() {
		tearCtx, tearCancel := context.WithTimeout(context.Background(), containerStartTimeout)
		defer tearCancel()
		if err := ctr.Terminate(tearCtx); err != nil {
			t.Logf("warning: failed to terminate postgres container: %v", err)
		}
	})

	dsn, err := ctr.ConnectionString(startCtx, "sslmode=disable")
	if err != nil {
		t.Fatalf("get connection string: %v", err)
	}
	return dsn
}

// migratedPool starts a Postgres container, applies every embedded
// packages/persistence schema migration (including this phase's RLS
// and provisioning-record migrations), and returns an open pool
// against it.
func migratedPool(t *testing.T) *persistence.Postgres {
	t.Helper()

	dsn := requirePostgresContainer(t)

	ctx, cancel := context.WithTimeout(context.Background(), containerStartTimeout)
	defer cancel()

	migrator, err := persistence.NewEmbeddedMigrator(dsn)
	if err != nil {
		t.Fatalf("NewEmbeddedMigrator: %v", err)
	}
	t.Cleanup(func() { _ = migrator.Close() })

	if err := migrator.Up(ctx); err != nil {
		t.Fatalf("Up: %v", err)
	}

	cfg := config.Default()
	cfg.Database.DSN = dsn
	pg, err := persistence.Open(ctx, &cfg)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(pg.Close)

	return pg
}

func TestIntegration_SeedSandboxTenant_CreatesOnce(t *testing.T) {
	pg := migratedPool(t)
	ctx := context.Background()

	tenant, err := tenancy.SeedSandboxTenant(ctx, pg.Pool())
	if err != nil {
		t.Fatalf("SeedSandboxTenant: %v", err)
	}
	if tenant.Slug != tenancy.SandboxTenantSlug {
		t.Fatalf("expected slug %q, got %q", tenancy.SandboxTenantSlug, tenant.Slug)
	}

	tenants := persistence.NewPostgresTenantRepository()
	all, err := tenants.List(ctx, pg.Pool())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected exactly 1 tenant after seeding, got %d", len(all))
	}
}

func TestIntegration_SeedSandboxTenant_IsIdempotent(t *testing.T) {
	pg := migratedPool(t)
	ctx := context.Background()

	first, err := tenancy.SeedSandboxTenant(ctx, pg.Pool())
	if err != nil {
		t.Fatalf("first SeedSandboxTenant: %v", err)
	}

	second, err := tenancy.SeedSandboxTenant(ctx, pg.Pool())
	if err != nil {
		t.Fatalf("second SeedSandboxTenant: %v", err)
	}

	if first.ID != second.ID {
		t.Fatalf("expected the same tenant ID on re-seed, got %v then %v", first.ID, second.ID)
	}

	tenants := persistence.NewPostgresTenantRepository()
	all, err := tenants.List(ctx, pg.Pool())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("expected exactly 1 tenant after re-seeding, got %d", len(all))
	}
}
