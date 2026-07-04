package securitytesting_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/securitytesting"
)

// newTestUser builds an identity.User with the given permission-bearing
// role(s), scoped to tenantID, mirroring
// packages/compliance's helpers_test.go newTestUser convention.
func newTestUser(tenantID uuid.UUID, roles ...identity.Role) *identity.User {
	return &identity.User{
		ID:       uuid.New(),
		TenantID: tenantID,
		Email:    "securitytesting@example.test",
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
// (holds both PermManageSecuritytesting and PermViewSecuritytesting)
// scoped to tenantID.
func adminUser(tenantID uuid.UUID) *identity.User {
	return newTestUser(tenantID, identity.RoleAdmin)
}

// auditorUser is a small convenience wrapper building a RoleAuditor
// user (holds only PermViewSecuritytesting) scoped to tenantID.
func auditorUser(tenantID uuid.UUID) *identity.User {
	return newTestUser(tenantID, identity.RoleAuditor)
}

// advocateUser holds neither PermViewSecuritytesting nor
// PermManageSecuritytesting -- an unprivileged actor for negative
// authorization tests.
func advocateUser(tenantID uuid.UUID) *identity.User {
	return newTestUser(tenantID, identity.RoleAdvocate)
}

// newTestEngine builds a securitytesting.Engine backed by a fresh
// Harness (via SeedHarness), fresh in-memory repositories, and an
// in-memory-backed AuditSink, returning the Engine and a fresh tenant
// ID so tests can exercise a full round-trip without repeating this
// wiring.
func newTestEngine(t *testing.T) (*securitytesting.Engine, uuid.UUID) {
	t.Helper()
	engine, _, tenantID := newTestEngineWithAudit(t)
	return engine, tenantID
}

// newTestEngineWithAudit is newTestEngine's fuller form: it also
// returns the *auditlog.Store the Engine's AuditSink writes to, for
// tests that need to inspect the recorded events directly.
func newTestEngineWithAudit(t *testing.T) (*securitytesting.Engine, *auditlog.Store, uuid.UUID) {
	t.Helper()
	return newTestEngineWithHarness(t, securitytesting.SeedHarness())
}

// newTestEngineWithHarness builds an Engine over a caller-supplied
// Harness, for tests that need tight control over exactly which
// Scenarios are registered (e.g. the self-test pair, or a single
// deliberately-scripted Scenario).
func newTestEngineWithHarness(t *testing.T, harness *securitytesting.Harness) (*securitytesting.Engine, *auditlog.Store, uuid.UUID) {
	t.Helper()

	runs := securitytesting.NewInMemoryRunRecordRepository()
	finds := securitytesting.NewInMemoryFindingRepository()

	auditStore, err := auditlog.NewStore(auditlog.NewInMemoryRepository())
	if err != nil {
		t.Fatalf("auditlog.NewStore: %v", err)
	}
	sink, err := securitytesting.NewAuditSink(auditStore)
	if err != nil {
		t.Fatalf("securitytesting.NewAuditSink: %v", err)
	}

	engine, err := securitytesting.NewEngine(harness, securitytesting.DefaultScopeDocument(), runs, finds, sink)
	if err != nil {
		t.Fatalf("securitytesting.NewEngine: %v", err)
	}
	return engine, auditStore, uuid.New()
}

// openTestFinding runs category's suite for tenantID as admin, opens a
// Finding from the first failed record it can find (falling back to
// the first record at all if none failed, so tests that need *some*
// Finding to exist can still proceed deterministically), and returns
// it.
func openTestFinding(t *testing.T, engine *securitytesting.Engine, tenantID uuid.UUID, category securitytesting.Category) securitytesting.Finding {
	t.Helper()
	admin := adminUser(tenantID)
	ctx := ctxWithUser(admin)

	records, err := engine.RunSuite(ctx, tenantID, category)
	if err != nil {
		t.Fatalf("RunSuite(%v): %v", category, err)
	}
	failed := securitytesting.FailedRecords(records)
	var target securitytesting.RunRecord
	if len(failed) > 0 {
		target = failed[0]
	} else if len(records) > 0 {
		target = records[0]
	} else {
		t.Fatalf("RunSuite(%v) produced zero records", category)
	}

	f, err := engine.OpenFinding(ctx, tenantID, target, securitytesting.SeverityMedium)
	if err != nil {
		t.Fatalf("OpenFinding: %v", err)
	}
	return f
}
