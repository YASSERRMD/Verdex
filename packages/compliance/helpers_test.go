package compliance_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
	"github.com/YASSERRMD/verdex/packages/compliance"
	"github.com/YASSERRMD/verdex/packages/identity"
)

// newTestUser builds an identity.User with the given permission-bearing
// role(s), scoped to tenantID, mirroring
// packages/privacy's helpers_test.go newTestUser convention.
func newTestUser(tenantID uuid.UUID, roles ...identity.Role) *identity.User {
	return &identity.User{
		ID:       uuid.New(),
		TenantID: tenantID,
		Email:    "compliance@example.test",
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

// newTestEngine builds a compliance.Engine backed by fresh in-memory
// repositories and an in-memory-backed AuditSink, returning the Engine
// and a fresh tenant ID so tests can exercise a full round-trip
// without repeating this wiring.
func newTestEngine(t *testing.T) (*compliance.Engine, uuid.UUID) {
	t.Helper()
	engine, _, tenantID := newTestEngineWithAudit(t)
	return engine, tenantID
}

// newTestEngineWithAudit is newTestEngine's fuller form: it also
// returns the *auditlog.Store the Engine's AuditSink writes to, for
// tests that need to inspect the recorded events directly.
func newTestEngineWithAudit(t *testing.T) (*compliance.Engine, *auditlog.Store, uuid.UUID) {
	t.Helper()

	controls := compliance.NewInMemoryControlRepository()
	evidence := compliance.NewInMemoryEvidenceRepository()
	profiles := compliance.NewInMemoryProfileRepository()

	auditStore, err := auditlog.NewStore(auditlog.NewInMemoryRepository())
	if err != nil {
		t.Fatalf("auditlog.NewStore: %v", err)
	}
	sink, err := compliance.NewAuditSink(auditStore)
	if err != nil {
		t.Fatalf("compliance.NewAuditSink: %v", err)
	}

	engine, err := compliance.NewEngine(controls, evidence, profiles, sink)
	if err != nil {
		t.Fatalf("compliance.NewEngine: %v", err)
	}
	return engine, auditStore, uuid.New()
}

// adminUser is a small convenience wrapper building a RoleAdmin user
// (holds both PermManageCompliance and PermViewCompliance) scoped to
// tenantID.
func adminUser(tenantID uuid.UUID) *identity.User {
	return newTestUser(tenantID, identity.RoleAdmin)
}

// auditorUser is a small convenience wrapper building a RoleAuditor
// user (holds only PermViewCompliance) scoped to tenantID.
func auditorUser(tenantID uuid.UUID) *identity.User {
	return newTestUser(tenantID, identity.RoleAuditor)
}

// registerTestControl registers and returns a single Control, using
// adminUser(tenantID) as the actor, for tests that need a real
// catalogued Control to attach evidence to without seeding the whole
// SeedControls set.
func registerTestControl(t *testing.T, engine *compliance.Engine, tenantID uuid.UUID) compliance.Control {
	t.Helper()
	admin := adminUser(tenantID)
	c, err := engine.RegisterControl(ctxWithUser(admin), compliance.Control{
		Code:        "TEST-01",
		Title:       "Test control",
		Description: "A control registered for test fixtures.",
		Framework:   compliance.FrameworkUAEDataProtection,
		Category:    compliance.CategoryLawfulBasis,
	})
	if err != nil {
		t.Fatalf("RegisterControl: %v", err)
	}
	return c
}
