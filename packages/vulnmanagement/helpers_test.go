package vulnmanagement_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/vulnmanagement"
)

// newTestUser builds an identity.User with the given permission-bearing
// role(s), scoped to tenantID, mirroring
// packages/compliance's helpers_test.go newTestUser convention.
func newTestUser(tenantID uuid.UUID, roles ...identity.Role) *identity.User {
	return &identity.User{
		ID:       uuid.New(),
		TenantID: tenantID,
		Email:    "vulnmanagement@example.test",
		Name:     "Test User",
		Roles:    roles,
		Status:   identity.UserStatusActive,
	}
}

// ctxWithUser returns a context carrying user, mirroring how an HTTP
// middleware layer would attach the authenticated actor.
func ctxWithUser(user *identity.User) context.Context {
	return identity.WithUser(context.Background(), user)
}

// adminUser is a small convenience wrapper building a RoleAdmin user
// (holds both PermManageVulnmanagement and PermViewVulnmanagement)
// scoped to tenantID.
func adminUser(tenantID uuid.UUID) *identity.User {
	return newTestUser(tenantID, identity.RoleAdmin)
}

// auditorUser is a small convenience wrapper building a RoleAuditor
// user (holds only PermViewVulnmanagement) scoped to tenantID.
func auditorUser(tenantID uuid.UUID) *identity.User {
	return newTestUser(tenantID, identity.RoleAuditor)
}

// newTestEngine builds a vulnmanagement.Engine backed by fresh
// in-memory repositories and an in-memory-backed AuditSink, returning
// the Engine and a fresh tenant ID so tests can exercise a full
// round-trip without repeating this wiring.
func newTestEngine(t *testing.T) (*vulnmanagement.Engine, uuid.UUID) {
	t.Helper()
	engine, _, tenantID := newTestEngineWithAudit(t)
	return engine, tenantID
}

// newTestEngineWithAudit is newTestEngine's fuller form: it also
// returns the *auditlog.Store the Engine's AuditSink writes to, for
// tests that need to inspect the recorded events directly.
func newTestEngineWithAudit(t *testing.T) (*vulnmanagement.Engine, *auditlog.Store, uuid.UUID) {
	t.Helper()

	findings := vulnmanagement.NewInMemoryFindingRepository()
	triage := vulnmanagement.NewInMemoryTriageRepository()

	auditStore, err := auditlog.NewStore(auditlog.NewInMemoryRepository())
	if err != nil {
		t.Fatalf("auditlog.NewStore: %v", err)
	}
	sink, err := vulnmanagement.NewAuditSink(auditStore)
	if err != nil {
		t.Fatalf("vulnmanagement.NewAuditSink: %v", err)
	}

	engine, err := vulnmanagement.NewEngine(findings, triage, sink)
	if err != nil {
		t.Fatalf("vulnmanagement.NewEngine: %v", err)
	}
	return engine, auditStore, uuid.New()
}

// newTestFinding builds a structurally valid Finding for tenantID,
// leaving ID/Status/timestamps for RecordFinding to fill in.
func newTestFinding(tenantID uuid.UUID) vulnmanagement.Finding {
	return vulnmanagement.Finding{
		TenantID:     tenantID,
		Source:       vulnmanagement.ScannerSourceSCA,
		Package:      "golang.org/x/example",
		Version:      "v1.2.3",
		Severity:     vulnmanagement.SeverityHigh,
		AdvisoryID:   "CVE-2024-00001",
		Title:        "Example vulnerability",
		Description:  "A test fixture vulnerability.",
		DiscoveredAt: time.Now().UTC(),
	}
}

// recordTestFinding records and returns a single Finding, using
// adminUser(tenantID) as the actor, for tests that need a real
// persisted Finding to triage or query without repeating the
// RecordFinding call inline.
func recordTestFinding(t *testing.T, engine *vulnmanagement.Engine, tenantID uuid.UUID) vulnmanagement.Finding {
	t.Helper()
	admin := adminUser(tenantID)
	f, err := engine.RecordFinding(ctxWithUser(admin), tenantID, newTestFinding(tenantID))
	if err != nil {
		t.Fatalf("RecordFinding: %v", err)
	}
	return f
}
