package signoff_test

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
	"github.com/YASSERRMD/verdex/packages/guardrail"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/persistence"
	"github.com/YASSERRMD/verdex/packages/signoff"
	"github.com/YASSERRMD/verdex/packages/tenancy"
)

// containerStartTimeout mirrors
// packages/caselifecycle/integration_test.go's constant of the same
// name.
const containerStartTimeout = 30 * time.Second

// requirePostgresContainer starts an ephemeral Postgres container for
// the duration of the test, following the exact skip/timeout pattern
// established in packages/caselifecycle/integration_test.go.
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
// mirroring packages/caselifecycle/integration_test.go's helper of the
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

func seedCase(t *testing.T, caseRepo caselifecycle.Repository, tenantID, actorID uuid.UUID) *caselifecycle.Case {
	t.Helper()
	c, err := caselifecycle.NewCase(caselifecycle.NewCaseInput{
		TenantID:       tenantID,
		JurisdictionID: uuid.New(),
		Title:          "Doe v. Acme Corp",
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

// TestIntegration_TenantScopedRepository_ApproveThenCanFinalize proves
// the full persisted stack: a real Postgres-backed caselifecycle.Case,
// a real Postgres-backed signoff.SignoffRecord, Approve through
// signoff.Service, and guardrail.CanFinalize genuinely unblocking
// only after that Approve call.
func TestIntegration_TenantScopedRepository_ApproveThenCanFinalize(t *testing.T) {
	pg := migratedAppPool(t)
	tenant := seedTenant(t, pg, "Tenant A", "tenant-a-signoff")

	caseRepo := caselifecycle.NewTenantScopedRepository(pg.Pool())
	signoffRepo := signoff.NewTenantScopedRepository(pg.Pool())
	reader := signoff.NewCaselifecycleVersionReader(caseRepo)
	svc, err := signoff.NewService(signoffRepo, reader, nil)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	actor := &identity.User{ID: uuid.New(), TenantID: tenant.ID, Roles: []identity.Role{identity.RoleJudge}, Status: identity.UserStatusActive}
	ctx := identity.WithUser(context.Background(), actor)

	c := seedCase(t, caseRepo, tenant.ID, actor.ID)

	gate, err := signoff.NewGate(signoffRepo, tenant.ID)
	if err != nil {
		t.Fatalf("NewGate: %v", err)
	}

	ok, err := guardrail.CanFinalize(ctx, c.ID.String(), gate)
	if ok {
		t.Fatal("expected CanFinalize to block before any sign-off")
	}
	if !errors.Is(err, guardrail.ErrSignoffNotApproved) {
		t.Fatalf("expected ErrSignoffNotApproved, got %v", err)
	}

	if _, err := svc.Approve(ctx, signoff.DecisionInput{
		TenantID:        tenant.ID,
		CaseID:          c.ID,
		CaseVersion:     c.MetadataVersion,
		Acknowledgement: signoff.AcknowledgementConfirmation,
	}); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	ok, err = guardrail.CanFinalize(ctx, c.ID.String(), gate)
	if err != nil {
		t.Fatalf("CanFinalize after Approve: %v", err)
	}
	if !ok {
		t.Fatal("expected CanFinalize to allow after Approve")
	}
}

// TestIntegration_TenantScopedRepository_CannotSeeOtherTenantsRecord
// proves Row-Level Security hides tenant A's sign-off record from
// tenant B's scope, exactly as
// packages/caselifecycle/integration_test.go proves for cases.
func TestIntegration_TenantScopedRepository_CannotSeeOtherTenantsRecord(t *testing.T) {
	pg := migratedAppPool(t)
	tenantA := seedTenant(t, pg, "Tenant A", "tenant-a-signoff-rls")
	tenantB := seedTenant(t, pg, "Tenant B", "tenant-b-signoff-rls")

	caseRepo := caselifecycle.NewTenantScopedRepository(pg.Pool())
	signoffRepo := signoff.NewTenantScopedRepository(pg.Pool())

	actorA := &identity.User{ID: uuid.New(), TenantID: tenantA.ID, Roles: []identity.Role{identity.RoleJudge}, Status: identity.UserStatusActive}
	ctxA := identity.WithUser(context.Background(), actorA)

	c := seedCase(t, caseRepo, tenantA.ID, actorA.ID)

	reader := signoff.NewCaselifecycleVersionReader(caseRepo)
	svc, err := signoff.NewService(signoffRepo, reader, nil)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if _, err := svc.Approve(ctxA, signoff.DecisionInput{
		TenantID:        tenantA.ID,
		CaseID:          c.ID,
		CaseVersion:     c.MetadataVersion,
		Acknowledgement: signoff.AcknowledgementConfirmation,
	}); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	// Row-Level Security must hide tenant A's record from tenant B's
	// scope, even though the application-level guard also
	// independently enforces this.
	if _, err := signoffRepo.Get(context.Background(), tenantB.ID, c.ID); !errors.Is(err, signoff.ErrNotFound) {
		t.Fatalf("expected ErrNotFound fetching tenant A's record under tenant B's scope, got %v", err)
	}
}

// TestIntegration_TenantScopedRepository_ReReviewPersistsAcrossReads
// proves that a re-review reversion, once persisted through
// TenantScopedRepository, is durably visible on a fresh read.
func TestIntegration_TenantScopedRepository_ReReviewPersistsAcrossReads(t *testing.T) {
	pg := migratedAppPool(t)
	tenant := seedTenant(t, pg, "Tenant A", "tenant-a-signoff-rereview")

	caseRepo := caselifecycle.NewTenantScopedRepository(pg.Pool())
	signoffRepo := signoff.NewTenantScopedRepository(pg.Pool())
	reader := signoff.NewCaselifecycleVersionReader(caseRepo)
	svc, err := signoff.NewService(signoffRepo, reader, nil)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	actor := &identity.User{ID: uuid.New(), TenantID: tenant.ID, Roles: []identity.Role{identity.RoleJudge}, Status: identity.UserStatusActive}
	ctx := identity.WithUser(context.Background(), actor)

	c := seedCase(t, caseRepo, tenant.ID, actor.ID)

	if _, err := svc.Approve(ctx, signoff.DecisionInput{
		TenantID:        tenant.ID,
		CaseID:          c.ID,
		CaseVersion:     c.MetadataVersion,
		Acknowledgement: signoff.AcknowledgementConfirmation,
	}); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	// Bump the case's metadata version, simulating a content change.
	updated, err := caselifecycle.SetMetadata(ctx, caseRepo, caselifecycle.MetadataUpdateInput{
		TenantID:        tenant.ID,
		CaseID:          c.ID,
		ExpectedVersion: c.MetadataVersion,
		Values:          map[string]string{"docket_note": "amended filing received"},
	})
	if err != nil {
		t.Fatalf("SetMetadata: %v", err)
	}
	if updated.MetadataVersion == c.MetadataVersion {
		t.Fatal("expected MetadataVersion to advance after SetMetadata")
	}

	rec, reverted, err := svc.ReReviewOnCaseUpdate(ctx, tenant.ID, c.ID)
	if err != nil {
		t.Fatalf("ReReviewOnCaseUpdate: %v", err)
	}
	if !reverted {
		t.Fatal("expected reverted = true after metadata version bump")
	}
	if rec.Status != guardrail.SignoffPending {
		t.Fatalf("Status = %v, want Pending", rec.Status)
	}

	// A fresh read must show the reverted state, not a stale cached
	// value.
	fresh, err := signoffRepo.Get(context.Background(), tenant.ID, c.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if fresh.Status != guardrail.SignoffPending {
		t.Fatalf("fresh Status = %v, want Pending", fresh.Status)
	}

	history, err := signoffRepo.ListAudit(context.Background(), tenant.ID, c.ID)
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}
	if len(history) < 2 {
		t.Fatalf("expected at least 2 audit entries (approve + re-review), got %d", len(history))
	}
}
