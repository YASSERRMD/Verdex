package compliance_test

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/YASSERRMD/verdex/packages/auditlog"
	"github.com/YASSERRMD/verdex/packages/compliance"
	"github.com/YASSERRMD/verdex/packages/config"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/persistence"
	"github.com/YASSERRMD/verdex/packages/tenancy"
)

// containerStartTimeout mirrors packages/privacy/postgres_integration_test.go's
// and packages/keymanagement/postgres_integration_test.go's constant of
// the same name.
const containerStartTimeout = 30 * time.Second

// requirePostgresContainer starts an ephemeral Postgres container for
// the duration of the test, following the exact skip/timeout pattern
// established in packages/privacy/postgres_integration_test.go. Run
// with -short to skip entirely, per this repo's convention that
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
// mirroring packages/privacy/postgres_integration_test.go's helper of
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

func seedIntegrationTenant(t *testing.T, pg *persistence.Postgres, name, slug string) *persistence.Tenant {
	t.Helper()
	tenants := persistence.NewPostgresTenantRepository()
	tenant := &persistence.Tenant{Name: name, Slug: slug}
	if err := tenants.Create(context.Background(), pg.Pool(), tenant); err != nil {
		t.Fatalf("create tenant %s: %v", slug, err)
	}
	return tenant
}

// TestIntegration_Engine_RealPostgresRepository proves Engine composes
// with real Postgres-backed repository implementations (not just the
// in-memory fixtures used elsewhere in this package's test suite):
// RegisterControl/ListControls against the unscoped
// PostgresControlRepository, and RecordEvidence/RunGapAnalysis/
// SetProfile against the RLS-backed TenantScopedX repositories, with
// Row-Level Security enforcing the tenant boundary at the database
// layer for the two tenant-scoped tables -- mirroring
// packages/privacy/postgres_integration_test.go's
// TestIntegration_Engine_RealPostgresRepository exactly.
func TestIntegration_Engine_RealPostgresRepository(t *testing.T) {
	pg := migratedAppPool(t)
	tenant := seedIntegrationTenant(t, pg, "Tenant A", "tenant-a-compliance")

	controlRepo := compliance.NewTenantScopedControlRepository(pg.Pool())
	evidenceRepo := compliance.NewTenantScopedEvidenceRepository(pg.Pool())
	profileRepo := compliance.NewTenantScopedProfileRepository(pg.Pool())

	auditStore, err := auditlog.NewStore(auditlog.NewTenantScopedRepository(pg.Pool()))
	if err != nil {
		t.Fatalf("auditlog.NewStore: %v", err)
	}
	sink, err := compliance.NewAuditSink(auditStore)
	if err != nil {
		t.Fatalf("compliance.NewAuditSink: %v", err)
	}

	engine, err := compliance.NewEngine(controlRepo, evidenceRepo, profileRepo, sink)
	if err != nil {
		t.Fatalf("compliance.NewEngine: %v", err)
	}

	admin := &identity.User{ID: tenant.ID, TenantID: tenant.ID, Roles: []identity.Role{identity.RoleAdmin}, Status: identity.UserStatusActive}
	ctx := identity.WithUser(context.Background(), admin)

	// Control catalogue (task 1): shared reference data, no tenant
	// scoping on the repository itself.
	control, err := engine.RegisterControl(ctx, compliance.Control{
		Code: "PG-INT-01", Title: "Postgres integration control",
		Framework: compliance.FrameworkUAEDataProtection, Category: compliance.CategoryLawfulBasis,
	})
	if err != nil {
		t.Fatalf("RegisterControl: %v", err)
	}
	list, err := engine.ListControls(ctx)
	if err != nil {
		t.Fatalf("ListControls: %v", err)
	}
	if len(list) != 1 || list[0].ID != control.ID {
		t.Fatalf("ListControls() = %v, want exactly the registered control", list)
	}

	// Control evidence (task 5), tenant-scoped.
	if _, err := engine.RecordEvidence(ctx, tenant.ID, compliance.ControlEvidence{
		ControlID: control.ID, Kind: compliance.EvidenceKindTestName, Reference: "TestPostgresIntegration",
	}); err != nil {
		t.Fatalf("RecordEvidence (test): %v", err)
	}
	if _, err := engine.RecordEvidence(ctx, tenant.ID, compliance.ControlEvidence{
		ControlID: control.ID, Kind: compliance.EvidenceKindDocument, Reference: "doc/compliance.md",
	}); err != nil {
		t.Fatalf("RecordEvidence (doc): %v", err)
	}

	// Compliance profile (task 7), tenant-scoped.
	if _, err := engine.SetProfile(ctx, tenant.ID, compliance.Profile{
		Frameworks: []compliance.Framework{compliance.FrameworkUAEDataProtection},
	}); err != nil {
		t.Fatalf("SetProfile: %v", err)
	}

	// Gap analysis (task 6) against real Postgres-backed evidence.
	report, err := engine.RunGapAnalysis(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("RunGapAnalysis: %v", err)
	}
	if len(report.Results) != 1 || report.Results[0].Status != compliance.StatusSatisfied {
		t.Fatalf("RunGapAnalysis() = %v, want exactly one StatusSatisfied result", report.Results)
	}

	// Dashboard (task 8).
	dashboard, err := engine.BuildDashboardReport(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("BuildDashboardReport: %v", err)
	}
	if dashboard.OverallStatusCounts[compliance.StatusSatisfied] != 1 {
		t.Fatalf("dashboard.OverallStatusCounts = %v, want satisfied=1", dashboard.OverallStatusCounts)
	}

	// A second tenant must not see tenant A's evidence or profile --
	// proving RLS (not just the application-level guard) enforces the
	// boundary, even though both tenants see the SAME shared
	// compliance_controls row (task 1's catalogue is intentionally
	// global, not per-tenant).
	otherTenant := seedIntegrationTenant(t, pg, "Tenant B", "tenant-b-compliance")
	otherAdmin := &identity.User{ID: otherTenant.ID, TenantID: otherTenant.ID, Roles: []identity.Role{identity.RoleAdmin}, Status: identity.UserStatusActive}
	otherCtx := identity.WithUser(context.Background(), otherAdmin)

	otherEvidence, err := engine.ListAllEvidence(otherCtx, otherTenant.ID)
	if err != nil {
		t.Fatalf("ListAllEvidence (tenant B): %v", err)
	}
	if len(otherEvidence) != 0 {
		t.Fatalf("tenant B's evidence list leaked %d entries from tenant A", len(otherEvidence))
	}

	if _, err := engine.GetProfile(otherCtx, otherTenant.ID); err == nil {
		t.Fatal("tenant B unexpectedly has a compliance profile set (should be ErrProfileNotFound)")
	}

	// Tenant B still sees the shared catalogue (by design), but its
	// own gap analysis reports a gap since it has recorded no evidence.
	otherReport, err := engine.RunGapAnalysis(otherCtx, otherTenant.ID)
	if err != nil {
		t.Fatalf("RunGapAnalysis (tenant B): %v", err)
	}
	if len(otherReport.Results) != 1 || otherReport.Results[0].Status != compliance.StatusGap {
		t.Fatalf("RunGapAnalysis (tenant B) = %v, want exactly one StatusGap result", otherReport.Results)
	}
}
