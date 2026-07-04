package threatmodel_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/threatmodel"
)

// newTestUser builds an identity.User with the given permission-bearing
// role(s), scoped to tenantID, mirroring
// packages/compliance's helpers_test.go newTestUser convention.
func newTestUser(tenantID uuid.UUID, roles ...identity.Role) *identity.User {
	return &identity.User{
		ID:       uuid.New(),
		TenantID: tenantID,
		Email:    "threatmodel@example.test",
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
// (holds both PermManageThreatmodel and PermViewThreatmodel) scoped to
// tenantID.
func adminUser(tenantID uuid.UUID) *identity.User {
	return newTestUser(tenantID, identity.RoleAdmin)
}

// auditorUser is a small convenience wrapper building a RoleAuditor
// user (holds only PermViewThreatmodel) scoped to tenantID.
func auditorUser(tenantID uuid.UUID) *identity.User {
	return newTestUser(tenantID, identity.RoleAuditor)
}

// judgeUser is a small convenience wrapper building a RoleJudge user
// (holds neither threatmodel permission) scoped to tenantID, used to
// prove an unrelated role is correctly forbidden.
func judgeUser(tenantID uuid.UUID) *identity.User {
	return newTestUser(tenantID, identity.RoleJudge)
}

// newTestEngine builds a threatmodel.Engine backed by a fresh
// Catalogue seeded from SeedThreatModels and an in-memory-backed
// AuditSink, returning the Engine and a fresh tenant ID so tests can
// exercise a full round-trip without repeating this wiring.
func newTestEngine(t *testing.T) (*threatmodel.Engine, uuid.UUID) {
	t.Helper()
	engine, _, tenantID := newTestEngineWithAudit(t)
	return engine, tenantID
}

// newTestEngineWithAudit is newTestEngine's fuller form: it also
// returns the *auditlog.Store the Engine's AuditSink writes to, for
// tests that need to inspect the recorded events directly.
func newTestEngineWithAudit(t *testing.T) (*threatmodel.Engine, *auditlog.Store, uuid.UUID) {
	t.Helper()

	catalogue := threatmodel.NewCatalogue(threatmodel.SeedThreatModels())

	auditStore, err := auditlog.NewStore(auditlog.NewInMemoryRepository())
	if err != nil {
		t.Fatalf("auditlog.NewStore: %v", err)
	}
	sink, err := threatmodel.NewAuditSink(auditStore)
	if err != nil {
		t.Fatalf("threatmodel.NewAuditSink: %v", err)
	}

	engine, err := threatmodel.NewEngine(catalogue, sink)
	if err != nil {
		t.Fatalf("threatmodel.NewEngine: %v", err)
	}
	return engine, auditStore, uuid.New()
}

// findMitigationIDByStatus returns the ID of the first Mitigation with
// the given status found across the engine's catalogue, for tests
// that need a real, valid mitigation ID to transition without
// hardcoding one of SeedThreatModels' randomly-allocated UUIDs.
func findMitigationIDByStatus(t *testing.T, engine *threatmodel.Engine, tenantID uuid.UUID, status threatmodel.MitigationStatus) uuid.UUID {
	t.Helper()
	admin := adminUser(tenantID)
	models, err := engine.ListThreatModels(ctxWithUser(admin))
	if err != nil {
		t.Fatalf("ListThreatModels: %v", err)
	}
	for _, tm := range models {
		for _, th := range tm.Threats {
			for _, m := range th.Mitigations {
				if m.Status == status {
					return m.ID
				}
			}
		}
	}
	t.Fatalf("no mitigation with status %v found in seeded catalogue", status)
	return uuid.Nil
}
