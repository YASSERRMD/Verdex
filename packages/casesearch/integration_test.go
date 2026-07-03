package casesearch_test

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
	"github.com/YASSERRMD/verdex/packages/casesearch"
	"github.com/YASSERRMD/verdex/packages/config"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/persistence"
	"github.com/YASSERRMD/verdex/packages/tenancy"
)

// containerStartTimeout mirrors packages/signoff/integration_test.go's
// constant of the same name.
const containerStartTimeout = 30 * time.Second

// requirePostgresContainer starts an ephemeral Postgres container for the
// duration of the test, following the exact skip/timeout pattern
// established in packages/caselifecycle/integration_test.go and
// packages/signoff/integration_test.go. Run with -short to skip
// entirely.
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
// mirroring packages/signoff/integration_test.go's helper of the same
// name exactly.
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

// TestIntegration_Engine_Search_RealCaselifecycleRepository proves
// Engine.Search composes with a real Postgres-backed
// caselifecycle.TenantScopedRepository (not just the in-memory fixture
// used elsewhere in this package's test suite), filtering by category
// against real persisted rows.
func TestIntegration_Engine_Search_RealCaselifecycleRepository(t *testing.T) {
	pg := migratedAppPool(t)
	tenant := seedTenant(t, pg, "Tenant A", "tenant-a-casesearch")

	caseRepo := caselifecycle.NewTenantScopedRepository(pg.Pool())
	actor := &identity.User{ID: uuid.New(), TenantID: tenant.ID, Roles: []identity.Role{identity.RoleJudge}, Status: identity.UserStatusActive}
	ctx := identity.WithUser(context.Background(), actor)

	civil := seedIntegrationCase(t, caseRepo, tenant.ID, actor.ID, "Civil Matter")
	seedIntegrationCase(t, caseRepo, tenant.ID, actor.ID, "Other Matter")

	engine, err := casesearch.NewEngine(caseRepo, func(context.Context, string) (casesearch.CaseSearcher, error) {
		return &fakeSearcher{keywordHits: []casesearch.Hit{{NodeID: "n1", Text: "matter", Score: 1}}}, nil
	})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	results, err := engine.Search(ctx, tenant.ID, casesearch.NewQuery("matter").
		WithMode(casesearch.ModeKeyword).
		WithFilter(casesearch.Filter{CategoryCode: "civil"}))
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results.Items) != 1 || results.Items[0].CaseID != civil.ID {
		t.Fatalf("expected exactly the civil case, got %+v", results.Items)
	}
}

// TestIntegration_SavedSearchRepository_TenantIsolation proves
// TenantScopedRepository's RLS-scoped transaction genuinely hides
// tenant A's saved search from tenant B's scope at the database layer,
// mirroring packages/signoff/integration_test.go's
// TestIntegration_TenantScopedRepository_CannotSeeOtherTenantsRecord.
func TestIntegration_SavedSearchRepository_TenantIsolation(t *testing.T) {
	pg := migratedAppPool(t)
	tenantA := seedTenant(t, pg, "Tenant A", "tenant-a-savedsearch")
	tenantB := seedTenant(t, pg, "Tenant B", "tenant-b-savedsearch")

	repo := casesearch.NewTenantScopedRepository(pg.Pool())
	owner := uuid.New()

	saved := &casesearch.SavedSearch{
		TenantID: tenantA.ID,
		OwnerID:  owner,
		Name:     "Tenant A's Search",
		Query:    casesearch.NewQuery("breach").WithMode(casesearch.ModeKeyword),
	}
	if err := repo.Create(context.Background(), tenantA.ID, saved); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Fetching under tenant A's own scope succeeds.
	got, err := repo.Get(context.Background(), tenantA.ID, saved.ID)
	if err != nil {
		t.Fatalf("Get (own tenant): %v", err)
	}
	if got.Query.Text != "breach" {
		t.Fatalf("expected round-tripped Query.Text = breach, got %q", got.Query.Text)
	}

	// Fetching under tenant B's scope must not find it — RLS enforces
	// this at the database layer even though the application code passed
	// the same saved.ID.
	if _, err := repo.Get(context.Background(), tenantB.ID, saved.ID); !errors.Is(err, casesearch.ErrNotFound) {
		t.Fatalf("expected ErrNotFound under a different tenant's RLS scope, got %v", err)
	}

	listB, err := repo.ListByOwner(context.Background(), tenantB.ID, owner)
	if err != nil {
		t.Fatalf("ListByOwner (tenant B): %v", err)
	}
	if len(listB) != 0 {
		t.Fatalf("expected tenant B to see no saved searches, got %+v", listB)
	}
}

// TestIntegration_SavedSearchRepository_CreateGetDeleteRoundTrip proves
// the full Postgres-backed lifecycle: create, get back with the Query
// preserved, delete, then confirm it is gone.
func TestIntegration_SavedSearchRepository_CreateGetDeleteRoundTrip(t *testing.T) {
	pg := migratedAppPool(t)
	tenant := seedTenant(t, pg, "Tenant A", "tenant-a-savedsearch-roundtrip")

	repo := casesearch.NewTenantScopedRepository(pg.Pool())
	owner := uuid.New()

	saved := &casesearch.SavedSearch{
		TenantID: tenant.ID,
		OwnerID:  owner,
		Name:     "Round Trip Search",
		Query: casesearch.NewQuery("landlord").
			WithMode(casesearch.ModeSemantic).
			WithFilter(casesearch.Filter{CategoryCode: "civil"}).
			WithPage(casesearch.Page{Number: 1, Size: 10}),
	}
	if err := repo.Create(context.Background(), tenant.ID, saved); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.Get(context.Background(), tenant.ID, saved.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Query.Mode != casesearch.ModeSemantic || got.Query.Filter.CategoryCode != "civil" {
		t.Fatalf("expected Query fields preserved through JSONB round-trip, got %+v", got.Query)
	}

	if err := repo.Delete(context.Background(), tenant.ID, saved.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.Get(context.Background(), tenant.ID, saved.ID); !errors.Is(err, casesearch.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}
