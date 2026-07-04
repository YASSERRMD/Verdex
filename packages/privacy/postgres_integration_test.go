package privacy_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/YASSERRMD/verdex/packages/auditlog"
	"github.com/YASSERRMD/verdex/packages/config"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/persistence"
	"github.com/YASSERRMD/verdex/packages/privacy"
	"github.com/YASSERRMD/verdex/packages/tenancy"
)

// containerStartTimeout mirrors packages/keymanagement/postgres_integration_test.go's
// constant of the same name.
const containerStartTimeout = 30 * time.Second

// requirePostgresContainer starts an ephemeral Postgres container for
// the duration of the test, following the exact skip/timeout pattern
// established in packages/keymanagement/postgres_integration_test.go
// and packages/notifications/postgres_integration_test.go. Run with
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
// mirroring packages/keymanagement/postgres_integration_test.go's
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

// TestIntegration_Engine_RealPostgresRepository proves Engine composes
// with real Postgres-backed TenantScopedXRepository implementations
// (not just the in-memory fixtures used elsewhere in this package's
// test suite): RegisterInventoryEntry/ListInventory, SubmitSAR/
// AdvanceSAR, RecordConsent/CheckConsent, and SubmitErasureRequest/
// ExecuteErasure (with its provenance-hash-survives guarantee) against
// real rows, with Row-Level Security enforcing the tenant boundary at
// the database layer -- mirroring
// packages/keymanagement/postgres_integration_test.go's
// TestIntegration_Service_RealPostgresRepository exactly.
func TestIntegration_Engine_RealPostgresRepository(t *testing.T) {
	pg := migratedAppPool(t)
	tenant := seedIntegrationTenant(t, pg, "Tenant A", "tenant-a-privacy")

	inventoryRepo := privacy.NewTenantScopedInventoryRepository(pg.Pool())
	consentRepo := privacy.NewTenantScopedConsentRepository(pg.Pool())
	sarRepo := privacy.NewTenantScopedSARRepository(pg.Pool())
	erasureRepo := privacy.NewTenantScopedErasureRepository(pg.Pool())

	auditStore, err := auditlog.NewStore(auditlog.NewTenantScopedRepository(pg.Pool()))
	if err != nil {
		t.Fatalf("auditlog.NewStore: %v", err)
	}
	sink, err := privacy.NewAuditSink(auditStore)
	if err != nil {
		t.Fatalf("privacy.NewAuditSink: %v", err)
	}

	engine, err := privacy.NewEngine(inventoryRepo, consentRepo, sarRepo, erasureRepo, sink)
	if err != nil {
		t.Fatalf("privacy.NewEngine: %v", err)
	}

	admin := &identity.User{ID: tenant.ID, TenantID: tenant.ID, Roles: []identity.Role{identity.RoleAdmin}, Status: identity.UserStatusActive}
	ctx := identity.WithUser(context.Background(), admin)

	// Data inventory (task 1).
	entry, err := engine.RegisterInventoryEntry(ctx, tenant.ID, privacy.DataInventoryEntry{
		Category: privacy.CategoryCaseParty, SourceTag: "case.parties",
		Sensitivity: privacy.SensitivityHigh, LegalBasis: privacy.BasisPublicTask,
		RetentionPeriod: 24 * time.Hour,
	})
	if err != nil {
		t.Fatalf("RegisterInventoryEntry: %v", err)
	}
	inventoryList, err := engine.ListInventory(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("ListInventory: %v", err)
	}
	if len(inventoryList) != 1 || inventoryList[0].ID != entry.ID {
		t.Fatalf("ListInventory = %v, want exactly the registered entry", inventoryList)
	}

	// Consent (task 6).
	if _, err := engine.RecordConsent(ctx, tenant.ID, privacy.ConsentRecord{
		SubjectID: "subject-1", Purpose: "case_analytics", LegalBasis: privacy.BasisConsent,
	}); err != nil {
		t.Fatalf("RecordConsent: %v", err)
	}
	valid, err := engine.CheckConsent(ctx, tenant.ID, "subject-1", "case_analytics")
	if err != nil {
		t.Fatalf("CheckConsent: %v", err)
	}
	if !valid {
		t.Fatal("CheckConsent() = false, want true after RecordConsent")
	}

	// Subject access request (task 4).
	sar, err := engine.SubmitSAR(ctx, tenant.ID, privacy.SubjectAccessRequest{SubjectID: "subject-1"})
	if err != nil {
		t.Fatalf("SubmitSAR: %v", err)
	}
	if _, err := engine.AdvanceSAR(ctx, tenant.ID, sar.ID, privacy.SARStatusInProgress, ""); err != nil {
		t.Fatalf("AdvanceSAR: %v", err)
	}
	fulfilled, err := engine.AdvanceSAR(ctx, tenant.ID, sar.ID, privacy.SARStatusFulfilled, "delivered")
	if err != nil {
		t.Fatalf("AdvanceSAR to fulfilled: %v", err)
	}
	if fulfilled.Status != privacy.SARStatusFulfilled {
		t.Fatalf("fulfilled.Status = %q, want %q", fulfilled.Status, privacy.SARStatusFulfilled)
	}

	// Erasure with provenance preservation (task 5's centerpiece).
	provenanceRecordID := uuid.New()
	const provenanceHash = "integration-test-provenance-hash"
	erasureReq, err := engine.SubmitErasureRequest(ctx, tenant.ID, privacy.ErasureRequest{
		SubjectID: "subject-1", Category: privacy.CategoryCaseParty, SourceTag: "case.parties",
		ProvenanceRecordID: provenanceRecordID, ProvenanceHash: provenanceHash,
	})
	if err != nil {
		t.Fatalf("SubmitErasureRequest: %v", err)
	}
	scrub := func(context.Context, privacy.ErasureRequest) error { return nil }
	result, err := engine.ExecuteErasure(ctx, tenant.ID, erasureReq.ID, scrub)
	if err != nil {
		t.Fatalf("ExecuteErasure: %v", err)
	}
	if result.ProvenanceHash != provenanceHash || result.ProvenanceRecordID != provenanceRecordID {
		t.Fatalf("ExecuteErasure result provenance = (%s, %s), want (%s, %s)",
			result.ProvenanceRecordID, result.ProvenanceHash, provenanceRecordID, provenanceHash)
	}
	if !result.ProvenancePreserved {
		t.Fatal("result.ProvenancePreserved = false, want true")
	}

	// A second tenant must not see tenant A's inventory, consent,
	// SAR, or erasure rows -- proving RLS (not just the
	// application-level guard) enforces the boundary.
	otherTenant := seedIntegrationTenant(t, pg, "Tenant B", "tenant-b-privacy")
	otherAdmin := &identity.User{ID: otherTenant.ID, TenantID: otherTenant.ID, Roles: []identity.Role{identity.RoleAdmin}, Status: identity.UserStatusActive}
	otherCtx := identity.WithUser(context.Background(), otherAdmin)

	otherInventory, err := engine.ListInventory(otherCtx, otherTenant.ID)
	if err != nil {
		t.Fatalf("ListInventory (tenant B): %v", err)
	}
	if len(otherInventory) != 0 {
		t.Fatalf("tenant B's inventory leaked %d entries from tenant A", len(otherInventory))
	}

	otherConsentValid, err := engine.CheckConsent(otherCtx, otherTenant.ID, "subject-1", "case_analytics")
	if err != nil {
		t.Fatalf("CheckConsent (tenant B): %v", err)
	}
	if otherConsentValid {
		t.Fatal("tenant B saw tenant A's consent record")
	}

	otherSARs, err := engine.ListSARsForSubject(otherCtx, otherTenant.ID, "subject-1")
	if err != nil {
		t.Fatalf("ListSARsForSubject (tenant B): %v", err)
	}
	if len(otherSARs) != 0 {
		t.Fatalf("tenant B's SAR list leaked %d entries from tenant A", len(otherSARs))
	}

	otherErasures, err := engine.ListErasuresForSubject(otherCtx, otherTenant.ID, "subject-1")
	if err != nil {
		t.Fatalf("ListErasuresForSubject (tenant B): %v", err)
	}
	for _, e := range otherErasures {
		if e.ProvenanceHash == provenanceHash {
			t.Fatal("tenant B's erasure list leaked tenant A's provenance hash")
		}
	}
	if len(otherErasures) != 0 {
		t.Fatalf("tenant B's erasure list leaked %d entries from tenant A", len(otherErasures))
	}
}
