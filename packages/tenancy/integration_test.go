package tenancy_test

import (
	"context"
	"errors"
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

// migratedPool starts a Postgres container as the bootstrap
// superuser, applies every embedded packages/persistence schema
// migration (including this phase's RLS, provisioning-record, and
// app-role migrations), and returns an open pool against it.
//
// This pool is authenticated as the container's bootstrap user, which
// testcontainers' postgres module (like most managed Postgres
// providers' default admin user, and Postgres's own initdb bootstrap
// role) creates as a superuser. PostgreSQL never applies Row-Level
// Security to a superuser connection, FORCE ROW LEVEL SECURITY
// notwithstanding — so this pool must be used only for schema
// migration and cross-tenant setup/verification in tests, never for
// the tenant-scoped operations actually under test. Use appScopedPool
// for those.
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

// appScopedPool takes a superuser pool already returned by
// migratedPool (over the same dsn used to build it), bootstraps
// tenancy.AppRoleName's login password via that superuser connection,
// and returns a second pool authenticated as tenancy.AppRoleName —
// the non-superuser, non-BYPASSRLS role RLS policies actually apply
// to. All tenant-scoped repository operations under test must run
// against this pool, not the superuser one, or the isolation
// assertions would pass for the wrong reason (or not at all, as
// discovered when this phase's integration tests first ran against
// live Postgres in CI: every isolation test failed because the
// superuser pool silently bypassed RLS).
func appScopedPool(t *testing.T, superuserPG *persistence.Postgres, superuserDSN string) *persistence.Postgres {
	t.Helper()
	ctx := context.Background()

	password, err := tenancy.GenerateAppRolePassword()
	if err != nil {
		t.Fatalf("GenerateAppRolePassword: %v", err)
	}
	if err := tenancy.BootstrapAppRolePassword(ctx, superuserPG.Pool(), password); err != nil {
		t.Fatalf("BootstrapAppRolePassword: %v", err)
	}

	appDSN, err := tenancy.BuildAppRoleDSN(superuserDSN, password)
	if err != nil {
		t.Fatalf("BuildAppRoleDSN: %v", err)
	}

	cfg := config.Default()
	cfg.Database.DSN = appDSN
	appPG, err := persistence.Open(ctx, &cfg)
	if err != nil {
		t.Fatalf("Open (app role): %v", err)
	}
	t.Cleanup(appPG.Close)

	if err := tenancy.VerifyRLSEnforceable(ctx, appPG.Pool()); err != nil {
		t.Fatalf("VerifyRLSEnforceable: expected the app role pool to enforce RLS, got %v", err)
	}

	return appPG
}

// migratedAppPool is the common case: migrate as superuser, then
// return only the app-role-scoped pool tenant-scoped operations
// should use.
func migratedAppPool(t *testing.T) *persistence.Postgres {
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
	superuserPG, err := persistence.Open(ctx, &cfg)
	if err != nil {
		t.Fatalf("Open (superuser): %v", err)
	}
	defer superuserPG.Close()

	return appScopedPool(t, superuserPG, dsn)
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

// seedTwoTenantsWithDeployments creates two tenants, each with one
// deployment, and returns both tenants and both deployments for the
// isolation tests below. pg must be an app-role pool (from
// migratedAppPool), since deployment creation runs through
// WithTenantScope and therefore through the RLS policy's WITH CHECK
// clause on INSERT.
func seedTwoTenantsWithDeployments(t *testing.T, pg *persistence.Postgres) (tenantA, tenantB *persistence.Tenant, deploymentA, deploymentB *persistence.Deployment) {
	t.Helper()
	ctx := context.Background()

	tenants := persistence.NewPostgresTenantRepository()
	tenantA = &persistence.Tenant{Name: "Tenant A", Slug: "tenant-a"}
	if err := tenants.Create(ctx, pg.Pool(), tenantA); err != nil {
		t.Fatalf("create tenant A: %v", err)
	}
	tenantB = &persistence.Tenant{Name: "Tenant B", Slug: "tenant-b"}
	if err := tenants.Create(ctx, pg.Pool(), tenantB); err != nil {
		t.Fatalf("create tenant B: %v", err)
	}

	scopedRepo := tenancy.NewTenantScopedDeploymentRepository(pg.Pool(), persistence.NewPostgresDeploymentRepository())

	deploymentA = &persistence.Deployment{TenantID: tenantA.ID, Profile: "standard"}
	if err := scopedRepo.Create(ctx, tenantA.ID, deploymentA); err != nil {
		t.Fatalf("create deployment A: %v", err)
	}
	deploymentB = &persistence.Deployment{TenantID: tenantB.ID, Profile: "standard"}
	if err := scopedRepo.Create(ctx, tenantB.ID, deploymentB); err != nil {
		t.Fatalf("create deployment B: %v", err)
	}

	return tenantA, tenantB, deploymentA, deploymentB
}

func TestIntegration_TenantScopedDeploymentRepository_CannotSeeOtherTenantsDeployment(t *testing.T) {
	pg := migratedAppPool(t)
	ctx := context.Background()
	tenantA, tenantB, deploymentA, deploymentB := seedTwoTenantsWithDeployments(t, pg)

	scopedRepo := tenancy.NewTenantScopedDeploymentRepository(pg.Pool(), persistence.NewPostgresDeploymentRepository())

	// Tenant A's scope must not see tenant B's deployment.
	if _, err := scopedRepo.Get(ctx, tenantA.ID, deploymentB.ID); !errors.Is(err, persistence.ErrNotFound) {
		t.Fatalf("expected ErrNotFound fetching tenant B's deployment under tenant A's scope, got %v", err)
	}
	// And vice versa.
	if _, err := scopedRepo.Get(ctx, tenantB.ID, deploymentA.ID); !errors.Is(err, persistence.ErrNotFound) {
		t.Fatalf("expected ErrNotFound fetching tenant A's deployment under tenant B's scope, got %v", err)
	}

	// List under tenant A's scope must only ever return tenant A's own
	// deployment.
	listA, err := scopedRepo.List(ctx, tenantA.ID)
	if err != nil {
		t.Fatalf("List under tenant A: %v", err)
	}
	if len(listA) != 1 || listA[0].ID != deploymentA.ID {
		t.Fatalf("expected List under tenant A's scope to return only deployment A, got %+v", listA)
	}
}

func TestIntegration_TenantScopedDeploymentRepository_CannotUpdateOtherTenantsDeployment(t *testing.T) {
	pg := migratedAppPool(t)
	ctx := context.Background()
	tenantA, tenantB, _, deploymentB := seedTwoTenantsWithDeployments(t, pg)

	scopedRepo := tenancy.NewTenantScopedDeploymentRepository(pg.Pool(), persistence.NewPostgresDeploymentRepository())

	// Attempting to update tenant B's deployment while scoped to
	// tenant A must fail: the RLS policy hides the row from the
	// UPDATE's WHERE clause, so the underlying repository reports
	// ErrNotFound (0 rows affected) exactly as it would for a
	// genuinely nonexistent id.
	mutated := &persistence.Deployment{ID: deploymentB.ID, TenantID: tenantA.ID, Profile: "tampered"}
	if err := scopedRepo.Update(ctx, tenantA.ID, mutated); err != nil && !errors.Is(err, persistence.ErrNotFound) && !errors.Is(err, tenancy.ErrCrossTenantAccess) {
		t.Fatalf("expected ErrNotFound or ErrCrossTenantAccess, got %v", err)
	}

	// Whichever way it failed, tenant B's deployment must be
	// completely unchanged.
	unchanged, err := scopedRepo.Get(ctx, tenantB.ID, deploymentB.ID)
	if err != nil {
		t.Fatalf("Get deployment B under tenant B's own scope: %v", err)
	}
	if unchanged.Profile != "standard" {
		t.Fatalf("expected deployment B's profile to remain unchanged, got %q", unchanged.Profile)
	}
}

func TestIntegration_TenantScopedDeploymentRepository_CannotDeleteOtherTenantsDeployment(t *testing.T) {
	pg := migratedAppPool(t)
	ctx := context.Background()
	tenantA, tenantB, _, deploymentB := seedTwoTenantsWithDeployments(t, pg)

	scopedRepo := tenancy.NewTenantScopedDeploymentRepository(pg.Pool(), persistence.NewPostgresDeploymentRepository())

	if err := scopedRepo.Delete(ctx, tenantA.ID, deploymentB.ID); !errors.Is(err, persistence.ErrNotFound) {
		t.Fatalf("expected ErrNotFound deleting tenant B's deployment under tenant A's scope, got %v", err)
	}

	// Tenant B's deployment must still exist.
	if _, err := scopedRepo.Get(ctx, tenantB.ID, deploymentB.ID); err != nil {
		t.Fatalf("expected deployment B to still exist after the denied cross-tenant delete, got %v", err)
	}
}

func TestIntegration_UnscopedQuery_SeesZeroRowsNotError(t *testing.T) {
	pg := migratedAppPool(t)
	ctx := context.Background()
	_, _, deploymentA, _ := seedTwoTenantsWithDeployments(t, pg)

	// Bypass WithTenantScope entirely: query the (still non-superuser,
	// RLS-subject) app-role pool directly, with no
	// app.current_tenant_id ever set on this connection/session. This
	// is the load-bearing behavior documented in
	// migrations/000003_enable_rls_deployments.up.sql: RLS must yield
	// zero rows, not raise an error, when the setting is unset.
	deploymentRepo := persistence.NewPostgresDeploymentRepository()

	all, err := deploymentRepo.List(ctx, pg.Pool())
	if err != nil {
		t.Fatalf("expected List with no tenant scope to succeed (returning zero rows), got error: %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("expected zero rows visible with no tenant scope set, got %d", len(all))
	}

	if _, err := deploymentRepo.Get(ctx, pg.Pool(), deploymentA.ID); !errors.Is(err, persistence.ErrNotFound) {
		t.Fatalf("expected ErrNotFound (not a raised error) fetching a real deployment with no tenant scope set, got %v", err)
	}
}

func TestIntegration_TenantScopedDeploymentRepository_CrossTenantGuardRejectsBeforeDB(t *testing.T) {
	pg := migratedAppPool(t)
	ctx := context.Background()
	tenantA, tenantB, _, deploymentB := seedTwoTenantsWithDeployments(t, pg)

	scopedRepo := tenancy.NewTenantScopedDeploymentRepository(pg.Pool(), persistence.NewPostgresDeploymentRepository())

	// deploymentB.TenantID is tenantB's id; scoping the call to
	// tenantA with that mismatched TenantID must be rejected by
	// requireMatchingTenant before any statement reaches the database
	// - i.e. this must fail with ErrCrossTenantAccess specifically,
	// not merely ErrNotFound from RLS.
	mutated := &persistence.Deployment{ID: deploymentB.ID, TenantID: tenantB.ID, Profile: "tampered"}
	err := scopedRepo.Update(ctx, tenantA.ID, mutated)
	if !errors.Is(err, tenancy.ErrCrossTenantAccess) {
		t.Fatalf("expected ErrCrossTenantAccess, got %v", err)
	}

	// Confirm no partial write happened.
	unchanged, err := scopedRepo.Get(ctx, tenantB.ID, deploymentB.ID)
	if err != nil {
		t.Fatalf("Get deployment B under tenant B's own scope: %v", err)
	}
	if unchanged.Profile != "standard" {
		t.Fatalf("expected deployment B's profile to remain unchanged, got %q", unchanged.Profile)
	}
}

// TestIntegration_VerifyRLSEnforceable_RejectsSuperuserPool documents,
// with a live database, exactly the failure mode this phase's RLS
// design first hit in CI: a pool authenticated as a superuser (as
// testcontainers' bootstrap user, and many managed Postgres
// providers' default admin user, both are) silently bypasses every
// RLS policy, so VerifyRLSEnforceable must reject it.
func TestIntegration_VerifyRLSEnforceable_RejectsSuperuserPool(t *testing.T) {
	superuserPG := migratedPool(t)

	if err := tenancy.VerifyRLSEnforceable(context.Background(), superuserPG.Pool()); err == nil {
		t.Fatal("expected VerifyRLSEnforceable to reject a superuser pool, got nil error")
	}
}

// TestIntegration_VerifyRLSEnforceable_AcceptsAppRolePool is the
// positive counterpart: the app-role pool migratedAppPool hands back
// must pass VerifyRLSEnforceable (this is also asserted inline inside
// appScopedPool itself, but an explicit test makes the guarantee
// discoverable on its own).
func TestIntegration_VerifyRLSEnforceable_AcceptsAppRolePool(t *testing.T) {
	appPG := migratedAppPool(t)

	if err := tenancy.VerifyRLSEnforceable(context.Background(), appPG.Pool()); err != nil {
		t.Fatalf("expected VerifyRLSEnforceable to accept the app-role pool, got %v", err)
	}
}
