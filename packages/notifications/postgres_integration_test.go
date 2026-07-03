package notifications_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/YASSERRMD/verdex/packages/caselifecycle"
	"github.com/YASSERRMD/verdex/packages/config"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/notifications"
	"github.com/YASSERRMD/verdex/packages/persistence"
	"github.com/YASSERRMD/verdex/packages/tenancy"
)

// containerStartTimeout mirrors packages/caseversioning/integration_test.go's
// constant of the same name.
const containerStartTimeout = 30 * time.Second

// requirePostgresContainer starts an ephemeral Postgres container for
// the duration of the test, following the exact skip/timeout pattern
// established in packages/caseversioning/integration_test.go. Run with
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
// mirroring packages/caseversioning/integration_test.go's helper of
// the same name exactly.
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
// composes with a real Postgres-backed TenantScopedRepository and
// TenantScopedPreferenceRepository (not just the in-memory fixtures
// used elsewhere in this package's test suite): Notify, List,
// UnreadCount, MarkRead, and preference suppression against real rows,
// with Row-Level Security enforcing the tenant boundary at the
// database layer — mirroring
// packages/caseversioning/integration_test.go's
// TestIntegration_Service_RealPostgresRepository exactly.
func TestIntegration_Service_RealPostgresRepository(t *testing.T) {
	pg := migratedAppPool(t)
	tenant := seedTenant(t, pg, "Tenant A", "tenant-a-notifications")

	caseRepo := caselifecycle.NewTenantScopedRepository(pg.Pool())
	actor := &identity.User{ID: uuid.New(), TenantID: tenant.ID, Roles: []identity.Role{identity.RoleClerk}, Status: identity.UserStatusActive}
	ctx := identity.WithUser(context.Background(), actor)

	c := seedIntegrationCase(t, caseRepo, tenant.ID, actor.ID, "Civil Matter")

	repo := notifications.NewTenantScopedRepository(pg.Pool())
	prefs := notifications.NewTenantScopedPreferenceRepository(pg.Pool())
	svc, err := notifications.NewService(repo, prefs)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	caseID := c.ID
	n, err := svc.Notify(ctx, notifications.NotifyInput{
		TenantID:    tenant.ID,
		RecipientID: actor.ID,
		Kind:        notifications.KindPendingSignoff,
		Title:       "Case awaiting your sign-off",
		CaseID:      &caseID,
	})
	if err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if n == nil {
		t.Fatal("Notify: expected a persisted Notification")
	}

	list, err := svc.List(ctx, tenant.ID, actor.ID, notifications.Filter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List: expected 1, got %d", len(list))
	}

	count, err := svc.UnreadCount(ctx, tenant.ID, actor.ID)
	if err != nil {
		t.Fatalf("UnreadCount: %v", err)
	}
	if count != 1 {
		t.Fatalf("UnreadCount: expected 1, got %d", count)
	}

	if err := svc.MarkRead(ctx, tenant.ID, actor.ID, n.ID); err != nil {
		t.Fatalf("MarkRead: %v", err)
	}
	count, err = svc.UnreadCount(ctx, tenant.ID, actor.ID)
	if err != nil {
		t.Fatalf("UnreadCount after MarkRead: %v", err)
	}
	if count != 0 {
		t.Fatalf("UnreadCount after MarkRead: expected 0, got %d", count)
	}

	// Preference suppression against real Postgres rows.
	if _, err := svc.SetPreference(ctx, tenant.ID, actor.ID, notifications.KindMention, false, nil); err != nil {
		t.Fatalf("SetPreference: %v", err)
	}
	suppressed, err := svc.Notify(ctx, notifications.NotifyInput{
		TenantID:    tenant.ID,
		RecipientID: actor.ID,
		Kind:        notifications.KindMention,
		Title:       "should be suppressed",
	})
	if err != nil {
		t.Fatalf("Notify (suppressed): %v", err)
	}
	if suppressed != nil {
		t.Fatalf("Notify: expected suppressed Kind to return nil, got %+v", suppressed)
	}

	// A second tenant must not see tenant A's notification, proving RLS
	// (not just the application-level guard) enforces the boundary.
	otherTenant := seedTenant(t, pg, "Tenant B", "tenant-b-notifications")
	otherActor := &identity.User{ID: uuid.New(), TenantID: otherTenant.ID, Roles: []identity.Role{identity.RoleClerk}, Status: identity.UserStatusActive}
	otherCtx := identity.WithUser(context.Background(), otherActor)
	if _, err := svc.List(otherCtx, otherTenant.ID, actor.ID, notifications.Filter{}); err != nil {
		t.Fatalf("List from tenant B's scope: %v", err)
	}
	otherList, err := svc.List(otherCtx, otherTenant.ID, otherActor.ID, notifications.Filter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(otherList) != 0 {
		t.Fatalf("expected tenant B to see 0 notifications, got %d", len(otherList))
	}
}
