package keymanagement_test

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/YASSERRMD/verdex/packages/config"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/keymanagement"
	"github.com/YASSERRMD/verdex/packages/observability"
	"github.com/YASSERRMD/verdex/packages/persistence"
	"github.com/YASSERRMD/verdex/packages/tenancy"
)

// containerStartTimeout mirrors packages/notifications/postgres_integration_test.go's
// constant of the same name.
const containerStartTimeout = 30 * time.Second

// requirePostgresContainer starts an ephemeral Postgres container for
// the duration of the test, following the exact skip/timeout pattern
// established in packages/caseversioning/integration_test.go and
// packages/notifications/postgres_integration_test.go. Run with
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
// mirroring packages/notifications/postgres_integration_test.go's
// helper of the same name exactly.
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

func seedIntegrationTenant(t *testing.T, pg *persistence.Postgres, name, slug string) *persistence.Tenant {
	t.Helper()
	tenants := persistence.NewPostgresTenantRepository()
	tenant := &persistence.Tenant{Name: name, Slug: slug}
	if err := tenants.Create(context.Background(), pg.Pool(), tenant); err != nil {
		t.Fatalf("create tenant %s: %v", slug, err)
	}
	return tenant
}

// TestIntegration_Service_RealPostgresRepository proves Service
// composes with a real Postgres-backed TenantScopedRepository and
// TenantScopedAuditRepository (not just the in-memory fixtures used
// elsewhere in this package's test suite): Rotate, CurrentKey,
// ListKeys, Revoke, and the audit trail against real rows, with Row-
// Level Security enforcing the tenant boundary at the database layer
// -- mirroring
// packages/notifications/postgres_integration_test.go's
// TestIntegration_Service_RealPostgresRepository exactly.
func TestIntegration_Service_RealPostgresRepository(t *testing.T) {
	pg := migratedAppPool(t)
	tenant := seedIntegrationTenant(t, pg, "Tenant A", "tenant-a-keymanagement")

	repo := keymanagement.NewTenantScopedRepository(pg.Pool())
	auditRepo := keymanagement.NewTenantScopedAuditRepository(pg.Pool())
	auditLogger := observability.NewAuditLogger(discardWriter{})
	recorder, err := keymanagement.NewAuditRecorder(auditLogger, auditRepo)
	if err != nil {
		t.Fatalf("NewAuditRecorder: %v", err)
	}

	fp, err := keymanagement.NewFileProvider(t.TempDir(), keymanagement.DeriveMasterKey("integration-test-passphrase"), repo)
	if err != nil {
		t.Fatalf("NewFileProvider: %v", err)
	}

	svc, err := keymanagement.NewService(fp, repo, recorder, keymanagement.NewInMemoryBreakGlassStore())
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	admin := &identity.User{TenantID: tenant.ID, Roles: []identity.Role{identity.RoleAdmin}, Status: identity.UserStatusActive}
	admin.ID = tenant.ID // deterministic, unused beyond identity
	ctx := identity.WithUser(context.Background(), admin)

	firstID, err := svc.Rotate(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}

	current, err := svc.CurrentKey(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("CurrentKey: %v", err)
	}
	if current.Metadata.ID != firstID {
		t.Fatalf("CurrentKey().Metadata.ID = %q, want %q", current.Metadata.ID, firstID)
	}

	secondID, err := svc.Rotate(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("second Rotate: %v", err)
	}
	if secondID == firstID {
		t.Fatal("second Rotate returned the same key ID")
	}

	list, err := svc.ListKeys(ctx, tenant.ID, keymanagement.Filter{})
	if err != nil {
		t.Fatalf("ListKeys: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("ListKeys: expected 2 versions, got %d", len(list))
	}

	if err := svc.Revoke(ctx, tenant.ID, firstID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	revoked, err := repo.Get(ctx, tenant.ID, firstID)
	if err != nil {
		t.Fatalf("repo.Get: %v", err)
	}
	if revoked.State != keymanagement.KeyStateRevoked {
		t.Fatalf("revoked.State = %q, want %q", revoked.State, keymanagement.KeyStateRevoked)
	}

	history, err := svc.AuditHistory(ctx, tenant.ID, 0)
	if err != nil {
		t.Fatalf("AuditHistory: %v", err)
	}
	if len(history) == 0 {
		t.Fatal("expected at least one audit entry")
	}

	// A second tenant must not see tenant A's key metadata or audit
	// trail, proving RLS (not just the application-level guard)
	// enforces the boundary.
	otherTenant := seedIntegrationTenant(t, pg, "Tenant B", "tenant-b-keymanagement")
	otherAdmin := &identity.User{ID: otherTenant.ID, TenantID: otherTenant.ID, Roles: []identity.Role{identity.RoleAdmin}, Status: identity.UserStatusActive}
	otherCtx := identity.WithUser(context.Background(), otherAdmin)

	if _, err := svc.CurrentKey(otherCtx, otherTenant.ID); err == nil {
		t.Fatal("expected tenant B to have no active key of its own")
	}
	otherList, err := svc.ListKeys(otherCtx, otherTenant.ID, keymanagement.Filter{})
	if err != nil {
		t.Fatalf("ListKeys tenant B: %v", err)
	}
	if len(otherList) != 0 {
		t.Fatalf("expected tenant B to see 0 keys, got %d", len(otherList))
	}
	// Tenant B's own CurrentKey/ListKeys calls above are themselves
	// auditable actions (Service.recordAudit runs even for a denied or
	// empty-result read), so tenant B's history is legitimately
	// non-empty here -- asserting "0 entries" would be wrong regardless
	// of tenant isolation. The property that actually proves RLS (not
	// just the application-level guard) enforces the boundary is that
	// every entry tenant B can see belongs to tenant B, and none of
	// tenant A's key IDs or audit entry IDs (from firstID/secondID and
	// history above) ever appear in tenant B's view.
	otherHistory, err := svc.AuditHistory(otherCtx, otherTenant.ID, 0)
	if err != nil {
		t.Fatalf("AuditHistory tenant B: %v", err)
	}
	tenantAEntryIDs := make(map[string]bool, len(history))
	for _, e := range history {
		tenantAEntryIDs[e.ID.String()] = true
	}
	for _, e := range otherHistory {
		if e.TenantID != otherTenant.ID {
			t.Fatalf("tenant B's audit history leaked a row for tenant %s (entry %s)", e.TenantID, e.ID)
		}
		if e.KeyID == firstID || e.KeyID == secondID {
			t.Fatalf("tenant B's audit history leaked an entry referencing tenant A's key %q", e.KeyID)
		}
		if tenantAEntryIDs[e.ID.String()] {
			t.Fatalf("tenant B's audit history leaked tenant A's audit entry %s", e.ID)
		}
	}
}
