package caseversioning_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/YASSERRMD/verdex/packages/caselifecycle"
	"github.com/YASSERRMD/verdex/packages/caseversioning"
	"github.com/YASSERRMD/verdex/packages/config"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/persistence"
	"github.com/YASSERRMD/verdex/packages/tenancy"
)

// containerStartTimeout mirrors packages/annotations/integration_test.go's
// constant of the same name.
const containerStartTimeout = 30 * time.Second

// requirePostgresContainer starts an ephemeral Postgres container for
// the duration of the test, following the exact skip/timeout pattern
// established in packages/annotations/integration_test.go. Run with
// -short to skip entirely, per this repo's convention that
// Docker-backed integration verification is a separate pass.
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

// migratedAppPool starts a Postgres container, applies every embedded
// packages/persistence migration as the bootstrap superuser, then
// returns a pool authenticated as the non-superuser verdex_app role,
// mirroring packages/annotations/integration_test.go's helper of the
// same name exactly.
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

	password, err := tenancy.GenerateAppRolePassword()
	if err != nil {
		t.Fatalf("GenerateAppRolePassword: %v", err)
	}
	if err := tenancy.BootstrapAppRolePassword(ctx, superuserPG.Pool(), password); err != nil {
		t.Fatalf("BootstrapAppRolePassword: %v", err)
	}
	appDSN, err := tenancy.BuildAppRoleDSN(dsn, password)
	if err != nil {
		t.Fatalf("BuildAppRoleDSN: %v", err)
	}

	appCfg := config.Default()
	appCfg.Database.DSN = appDSN
	appPG, err := persistence.Open(ctx, &appCfg)
	if err != nil {
		t.Fatalf("Open (app role): %v", err)
	}
	t.Cleanup(appPG.Close)

	if err := tenancy.VerifyRLSEnforceable(ctx, appPG.Pool()); err != nil {
		t.Fatalf("VerifyRLSEnforceable: %v", err)
	}
	return appPG
}

func seedTenant(t *testing.T, pg *persistence.Postgres, name, slug string) *persistence.Tenant {
	t.Helper()
	tenants := persistence.NewPostgresTenantRepository()
	tenant := &persistence.Tenant{Name: name, Slug: slug}
	if err := tenants.Create(context.Background(), pg.Pool(), tenant); err != nil {
		t.Fatalf("create tenant %s: %v", slug, err)
	}
	return tenant
}

func seedIntegrationCase(t *testing.T, caseRepo caselifecycle.Repository, tenantID, actorID uuid.UUID, title string) *caselifecycle.Case {
	t.Helper()
	c, err := caselifecycle.NewCase(caselifecycle.NewCaseInput{
		TenantID:       tenantID,
		JurisdictionID: uuid.New(),
		CategoryID:     "civil",
		Title:          title,
		CreatedBy:      actorID,
	})
	if err != nil {
		t.Fatalf("NewCase: %v", err)
	}
	if err := caseRepo.Create(context.Background(), tenantID, c); err != nil {
		t.Fatalf("caseRepo.Create: %v", err)
	}
	return c
}

// TestIntegration_Service_RealPostgresRepository proves Service
// composes with a real Postgres-backed caseversioning.
// TenantScopedRepository and caselifecycle.TenantScopedRepository (not
// just the in-memory fixtures used elsewhere in this package's test
// suite): snapshot, history, diff, and restore against real rows, with
// Row-Level Security enforcing the tenant boundary at the database
// layer.
func TestIntegration_Service_RealPostgresRepository(t *testing.T) {
	pg := migratedAppPool(t)
	tenant := seedTenant(t, pg, "Tenant A", "tenant-a-caseversioning")

	caseRepo := caselifecycle.NewTenantScopedRepository(pg.Pool())
	actor := &identity.User{ID: uuid.New(), TenantID: tenant.ID, Roles: []identity.Role{identity.RoleClerk}, Status: identity.UserStatusActive}
	ctx := identity.WithUser(context.Background(), actor)

	c := seedIntegrationCase(t, caseRepo, tenant.ID, actor.ID, "Civil Matter")

	snapRepo := caseversioning.NewTenantScopedRepository(pg.Pool())
	svc, err := caseversioning.NewService(snapRepo, caseRepo)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	original, err := svc.SnapshotCaseMetadata(ctx, tenant.ID, c.ID, "initial", "Original")
	if err != nil {
		t.Fatalf("SnapshotCaseMetadata: %v", err)
	}

	c.Title = "Civil Matter (Amended)"
	if err := caseRepo.Update(ctx, tenant.ID, c); err != nil {
		t.Fatalf("caseRepo.Update: %v", err)
	}
	amended, err := svc.SnapshotCaseMetadata(ctx, tenant.ID, c.ID, "manual edit", "Amended")
	if err != nil {
		t.Fatalf("SnapshotCaseMetadata (amended): %v", err)
	}

	history, err := svc.History(ctx, tenant.ID, c.ID, caseversioning.SnapshotFilter{})
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("len(history) = %d, want 2", len(history))
	}

	diff, err := svc.Diff(ctx, tenant.ID, original.ID, amended.ID)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if diff.Identical {
		t.Fatal("Identical = true, want false (title changed)")
	}

	restored, err := svc.Restore(ctx, tenant.ID, original.ID)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if restored.RestoredFromID == nil || *restored.RestoredFromID != original.ID {
		t.Fatalf("RestoredFromID = %v, want pointer to %v", restored.RestoredFromID, original.ID)
	}

	live, err := caseRepo.Get(ctx, tenant.ID, c.ID)
	if err != nil {
		t.Fatalf("caseRepo.Get: %v", err)
	}
	if live.Title != "Civil Matter" {
		t.Fatalf("live.Title = %q, want reverted to %q", live.Title, "Civil Matter")
	}

	// A second tenant must not see tenant A's snapshot, proving RLS (not
	// just the application-level guard) enforces the boundary.
	otherTenant := seedTenant(t, pg, "Tenant B", "tenant-b-caseversioning")
	otherActor := &identity.User{ID: uuid.New(), TenantID: otherTenant.ID, Roles: []identity.Role{identity.RoleClerk}, Status: identity.UserStatusActive}
	otherCtx := identity.WithUser(context.Background(), otherActor)
	if _, err := svc.Get(otherCtx, otherTenant.ID, original.ID); err == nil {
		t.Fatal("expected an error reading tenant A's snapshot from tenant B's scope")
	}
}
