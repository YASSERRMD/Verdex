package persistence_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/YASSERRMD/verdex/packages/config"
	"github.com/YASSERRMD/verdex/packages/persistence"
)

// containerStartTimeout bounds how long we wait for Docker to pull
// and start the Postgres container. If Docker is unreachable (daemon
// down, socket unresponsive), container startup fails or times out
// well before this deadline, and the test skips instead of hanging.
const containerStartTimeout = 30 * time.Second

// requirePostgresContainer starts an ephemeral Postgres container for
// the duration of the test and returns its connection DSN. It skips
// the test (rather than failing) if Docker is not reachable, per this
// phase's environment constraints: local Docker Desktop may be
// unresponsive, while CI always has a working daemon.
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

// migratedPool starts a Postgres container, applies the embedded
// schema migrations, and returns an open pool against it.
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

func TestIntegration_PoolConnects(t *testing.T) {
	pg := migratedPool(t)

	if err := pg.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestIntegration_MigrationsApplyCleanly(t *testing.T) {
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

	version, dirty, err := migrator.Version()
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if dirty {
		t.Fatal("expected clean schema after Up, got dirty")
	}
	if version != 2 {
		t.Fatalf("expected schema version 2 after applying both migrations, got %d", version)
	}

	// Running Up again must be a no-op, not an error.
	if err := migrator.Up(ctx); err != nil {
		t.Fatalf("second Up call: %v", err)
	}
}

func TestIntegration_TenantCRUDRoundTrip(t *testing.T) {
	pg := migratedPool(t)
	ctx := context.Background()

	repo := persistence.NewPostgresTenantRepository()
	tenant := &persistence.Tenant{Name: "Acme Legal", Slug: "acme-legal"}

	if err := repo.Create(ctx, pg.Pool(), tenant); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tenant.ID == uuid.Nil {
		t.Fatal("expected Create to populate a non-nil ID")
	}

	got, err := repo.Get(ctx, pg.Pool(), tenant.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "Acme Legal" || got.Slug != "acme-legal" {
		t.Fatalf("Get returned unexpected tenant: %+v", got)
	}

	tenants, err := repo.List(ctx, pg.Pool())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(tenants) != 1 {
		t.Fatalf("expected 1 tenant, got %d", len(tenants))
	}

	got.Name = "Acme Legal Renamed"
	if err := repo.Update(ctx, pg.Pool(), got); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.Name != "Acme Legal Renamed" {
		t.Fatalf("expected Update to persist new name, got %q", got.Name)
	}

	if err := repo.Delete(ctx, pg.Pool(), got.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := repo.Get(ctx, pg.Pool(), got.ID); !errors.Is(err, persistence.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after Delete, got %v", err)
	}

	if err := repo.Delete(ctx, pg.Pool(), got.ID); !errors.Is(err, persistence.ErrNotFound) {
		t.Fatalf("expected ErrNotFound deleting an already-deleted tenant, got %v", err)
	}
}

func TestIntegration_DeploymentCRUDRoundTrip(t *testing.T) {
	pg := migratedPool(t)
	ctx := context.Background()

	tenantRepo := persistence.NewPostgresTenantRepository()
	tenant := &persistence.Tenant{Name: "Beta Chambers", Slug: "beta-chambers"}
	if err := tenantRepo.Create(ctx, pg.Pool(), tenant); err != nil {
		t.Fatalf("Create tenant: %v", err)
	}

	deploymentRepo := persistence.NewPostgresDeploymentRepository()
	deployment := &persistence.Deployment{TenantID: tenant.ID, Profile: "standard"}

	if err := deploymentRepo.Create(ctx, pg.Pool(), deployment); err != nil {
		t.Fatalf("Create deployment: %v", err)
	}
	if deployment.Status != persistence.DeploymentStatusProvisioning {
		t.Fatalf("expected default status %q, got %q", persistence.DeploymentStatusProvisioning, deployment.Status)
	}

	got, err := deploymentRepo.Get(ctx, pg.Pool(), deployment.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.TenantID != tenant.ID {
		t.Fatalf("expected TenantID %v, got %v", tenant.ID, got.TenantID)
	}

	got.Status = persistence.DeploymentStatusActive
	if err := deploymentRepo.Update(ctx, pg.Pool(), got); err != nil {
		t.Fatalf("Update: %v", err)
	}

	deployments, err := deploymentRepo.List(ctx, pg.Pool())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(deployments) != 1 {
		t.Fatalf("expected 1 deployment, got %d", len(deployments))
	}

	// Deleting the tenant must cascade-delete its deployments (FK ON
	// DELETE CASCADE from migrations/000002_create_deployments.up.sql).
	if err := tenantRepo.Delete(ctx, pg.Pool(), tenant.ID); err != nil {
		t.Fatalf("Delete tenant: %v", err)
	}
	if _, err := deploymentRepo.Get(ctx, pg.Pool(), deployment.ID); !errors.Is(err, persistence.ErrNotFound) {
		t.Fatalf("expected deployment to be cascade-deleted with its tenant, got %v", err)
	}
}

func TestIntegration_WithTx_RollsBackOnError(t *testing.T) {
	pg := migratedPool(t)
	ctx := context.Background()

	tenantRepo := persistence.NewPostgresTenantRepository()
	sentinelErr := errors.New("boom")

	tenant := &persistence.Tenant{Name: "Rollback Co", Slug: "rollback-co"}
	err := persistence.WithTx(ctx, pg.Pool(), func(ctx context.Context, exec persistence.Executor) error {
		if err := tenantRepo.Create(ctx, exec, tenant); err != nil {
			return err
		}
		return sentinelErr
	})
	if !errors.Is(err, sentinelErr) {
		t.Fatalf("expected WithTx to surface the sentinel error, got %v", err)
	}

	tenants, err := tenantRepo.List(ctx, pg.Pool())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, tn := range tenants {
		if tn.Slug == "rollback-co" {
			t.Fatal("expected rollback-co tenant to have been rolled back, but it was found")
		}
	}
}

func TestIntegration_WithTx_CommitsOnSuccess(t *testing.T) {
	pg := migratedPool(t)
	ctx := context.Background()

	tenantRepo := persistence.NewPostgresTenantRepository()
	deploymentRepo := persistence.NewPostgresDeploymentRepository()

	tenant := &persistence.Tenant{Name: "Commit Co", Slug: "commit-co"}
	deployment := &persistence.Deployment{Profile: "standard"}

	err := persistence.WithTx(ctx, pg.Pool(), func(ctx context.Context, exec persistence.Executor) error {
		if err := tenantRepo.Create(ctx, exec, tenant); err != nil {
			return err
		}
		deployment.TenantID = tenant.ID
		return deploymentRepo.Create(ctx, exec, deployment)
	})
	if err != nil {
		t.Fatalf("WithTx: %v", err)
	}

	if _, err := tenantRepo.Get(ctx, pg.Pool(), tenant.ID); err != nil {
		t.Fatalf("expected committed tenant to be visible, got %v", err)
	}
	if _, err := deploymentRepo.Get(ctx, pg.Pool(), deployment.ID); err != nil {
		t.Fatalf("expected committed deployment to be visible, got %v", err)
	}
}

func TestIntegration_HealthChecker(t *testing.T) {
	pg := migratedPool(t)

	healthy := persistence.PostgresChecker(pg)
	if err := healthy(context.Background()); err != nil {
		t.Fatalf("expected healthy checker against a live container, got %v", err)
	}

	pg.Close()

	unhealthy := persistence.PostgresChecker(pg)
	if err := unhealthy(context.Background()); err == nil {
		t.Fatal("expected unhealthy checker against a closed pool, got nil error")
	}
}

func TestIntegration_MigrationsRollback(t *testing.T) {
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
	defer pg.Close()

	tenantRepo := persistence.NewPostgresTenantRepository()
	if err := tenantRepo.Create(ctx, pg.Pool(), &persistence.Tenant{Name: "Rollback Target", Slug: "rollback-target"}); err != nil {
		t.Fatalf("seed Create: %v", err)
	}

	// Down must actually reverse the schema: after reverting both
	// migrations, the tenants table (and the row just inserted into
	// it) must no longer exist.
	if err := migrator.Down(ctx); err != nil {
		t.Fatalf("Down: %v", err)
	}

	_, dirty, err := migrator.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		t.Fatalf("Version after Down: %v", err)
	}
	if dirty {
		t.Fatal("expected clean schema after Down, got dirty")
	}

	if _, err := tenantRepo.List(ctx, pg.Pool()); err == nil {
		t.Fatal("expected List to fail against a reverted schema (tenants table dropped), got nil error")
	}

	// Re-applying Up after a full Down must cleanly recreate the
	// schema, proving Down actually reversed commit 4's migrations
	// rather than merely no-op'ing.
	if err := migrator.Up(ctx); err != nil {
		t.Fatalf("Up after Down: %v", err)
	}
	if _, err := tenantRepo.List(ctx, pg.Pool()); err != nil {
		t.Fatalf("expected List to succeed after re-applying Up, got %v", err)
	}
}

func TestIntegration_RecoverDirty(t *testing.T) {
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

	// RecoverDirty must refuse to act on a clean schema.
	if err := migrator.RecoverDirty(2); err == nil {
		t.Fatal("expected RecoverDirty to refuse a non-dirty schema, got nil error")
	}

	// Simulate the "failed migration left the schema dirty" scenario
	// golang-migrate documents: flip the dirty bit directly, the same
	// state golang-migrate itself would leave behind after a failed
	// migration step.
	cfg := config.Default()
	cfg.Database.DSN = dsn
	pg, err := persistence.Open(ctx, &cfg)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer pg.Close()

	if _, err := pg.Pool().Exec(ctx, "UPDATE schema_migrations SET dirty = true"); err != nil {
		t.Fatalf("mark dirty: %v", err)
	}

	if _, dirty, err := migrator.Version(); err != nil || !dirty {
		t.Fatalf("expected dirty=true after manual UPDATE, got dirty=%v err=%v", dirty, err)
	}

	if err := migrator.RecoverDirty(2); err != nil {
		t.Fatalf("RecoverDirty: %v", err)
	}

	if _, dirty, err := migrator.Version(); err != nil || dirty {
		t.Fatalf("expected dirty=false after RecoverDirty, got dirty=%v err=%v", dirty, err)
	}

	// Schema must be usable again after recovery.
	tenantRepo := persistence.NewPostgresTenantRepository()
	if _, err := tenantRepo.List(ctx, pg.Pool()); err != nil {
		t.Fatalf("expected List to succeed after RecoverDirty, got %v", err)
	}
}
