package caselifecycle_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/YASSERRMD/verdex/packages/caselifecycle"
	"github.com/YASSERRMD/verdex/packages/config"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/persistence"
	"github.com/YASSERRMD/verdex/packages/tenancy"
)

// containerStartTimeout mirrors packages/tenancy/integration_test.go's
// constant of the same name.
const containerStartTimeout = 30 * time.Second

// requirePostgresContainer starts an ephemeral Postgres container for
// the duration of the test and returns its connection DSN, following
// the exact skip/timeout pattern established in
// packages/persistence/integration_test.go and
// packages/tenancy/integration_test.go.
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
// returns a pool authenticated as the non-superuser verdex_app role
// (the role RLS policies actually apply to), mirroring
// packages/tenancy/integration_test.go's migratedAppPool helper
// exactly.
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

// seedTenant inserts a tenant directly (via the superuser-equivalent
// app pool, which can still write tenants since the tenants table
// carries no RLS policy) for use as a case's TenantID.
func seedTenant(t *testing.T, pg *persistence.Postgres, name, slug string) *persistence.Tenant {
	t.Helper()
	tenants := persistence.NewPostgresTenantRepository()
	tenant := &persistence.Tenant{Name: name, Slug: slug}
	if err := tenants.Create(context.Background(), pg.Pool(), tenant); err != nil {
		t.Fatalf("create tenant %s: %v", slug, err)
	}
	return tenant
}

func TestIntegration_TenantScopedRepository_CreateAndGetRoundTrip(t *testing.T) {
	pg := migratedAppPool(t)
	tenant := seedTenant(t, pg, "Tenant A", "tenant-a-caselifecycle")

	repo := caselifecycle.NewTenantScopedRepository(pg.Pool())
	ctx := context.Background()

	c, err := caselifecycle.NewCase(caselifecycle.NewCaseInput{
		TenantID:       tenant.ID,
		JurisdictionID: uuid.New(),
		CategoryID:     "civil",
		Title:          "Doe v. Acme Corp",
		CreatedBy:      uuid.New(),
	})
	if err != nil {
		t.Fatalf("NewCase: %v", err)
	}

	if err := repo.Create(ctx, tenant.ID, c); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.Get(ctx, tenant.ID, c.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != c.Title || got.State != caselifecycle.StateDraft {
		t.Fatalf("got = %+v, want Title=%q State=%s", got, c.Title, caselifecycle.StateDraft)
	}
}

func TestIntegration_TenantScopedRepository_CannotSeeOtherTenantsCase(t *testing.T) {
	pg := migratedAppPool(t)
	tenantA := seedTenant(t, pg, "Tenant A", "tenant-a-case-rls")
	tenantB := seedTenant(t, pg, "Tenant B", "tenant-b-case-rls")

	repo := caselifecycle.NewTenantScopedRepository(pg.Pool())
	ctx := context.Background()

	c, err := caselifecycle.NewCase(caselifecycle.NewCaseInput{
		TenantID:       tenantA.ID,
		JurisdictionID: uuid.New(),
		Title:          "Tenant A's Case",
		CreatedBy:      uuid.New(),
	})
	if err != nil {
		t.Fatalf("NewCase: %v", err)
	}
	if err := repo.Create(ctx, tenantA.ID, c); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Row-Level Security must hide tenant A's case from tenant B's
	// scope, even though the application-level guard also independently
	// enforces this.
	if _, err := repo.Get(ctx, tenantB.ID, c.ID); !errors.Is(err, caselifecycle.ErrNotFound) {
		t.Fatalf("expected ErrNotFound fetching tenant A's case under tenant B's scope, got %v", err)
	}

	listB, err := repo.List(ctx, tenantB.ID, caselifecycle.CaseFilter{})
	if err != nil {
		t.Fatalf("List under tenant B: %v", err)
	}
	if len(listB) != 0 {
		t.Fatalf("expected 0 cases visible to tenant B, got %d", len(listB))
	}
}

func TestIntegration_TenantScopedRepository_TransitionAndAuditPersist(t *testing.T) {
	pg := migratedAppPool(t)
	tenant := seedTenant(t, pg, "Tenant A", "tenant-a-transition")

	repo := caselifecycle.NewTenantScopedRepository(pg.Pool())
	actor := &identity.User{ID: uuid.New(), TenantID: tenant.ID, Roles: []identity.Role{identity.RoleAdmin}, Status: identity.UserStatusActive}
	ctx := identity.WithUser(context.Background(), actor)

	c, err := caselifecycle.NewCase(caselifecycle.NewCaseInput{
		TenantID:       tenant.ID,
		JurisdictionID: uuid.New(),
		Title:          "Doe v. Acme Corp",
		CreatedBy:      actor.ID,
	})
	if err != nil {
		t.Fatalf("NewCase: %v", err)
	}
	if err := repo.Create(ctx, tenant.ID, c); err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := caselifecycle.Transition(ctx, repo, caselifecycle.TransitionInput{
		TenantID: tenant.ID,
		CaseID:   c.ID,
		ToState:  caselifecycle.StateActive,
		Reason:   "intake complete",
	})
	if err != nil {
		t.Fatalf("Transition: %v", err)
	}
	if updated.State != caselifecycle.StateActive {
		t.Fatalf("State = %s, want %s", updated.State, caselifecycle.StateActive)
	}

	history, err := repo.ListTransitions(ctx, tenant.ID, c.ID)
	if err != nil {
		t.Fatalf("ListTransitions: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 transition record, got %d", len(history))
	}
	if history[0].FromState != caselifecycle.StateDraft || history[0].ToState != caselifecycle.StateActive {
		t.Fatalf("record from/to = %s/%s, want draft/active", history[0].FromState, history[0].ToState)
	}
	if history[0].Actor != actor.ID {
		t.Fatalf("record actor = %v, want %v", history[0].Actor, actor.ID)
	}
}
