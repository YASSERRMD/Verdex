package localization_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/localization"
)

// newTestUser builds an identity.User with the given permission-bearing
// role(s), scoped to tenantID, mirroring
// packages/privacy's helpers_test.go newTestUser convention.
func newTestUser(tenantID uuid.UUID, roles ...identity.Role) *identity.User {
	return &identity.User{
		ID:       uuid.New(),
		TenantID: tenantID,
		Email:    "localization@example.test",
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

// newTestEngine builds a localization.Engine backed by a fresh seeded
// Catalog, an in-memory PreferenceRepository, and an in-memory-backed
// AuditSink, returning the Engine and a fresh tenant ID so tests can
// exercise a full round-trip without repeating this wiring.
func newTestEngine(t *testing.T) (*localization.Engine, uuid.UUID) {
	t.Helper()
	engine, _, tenantID := newTestEngineWithAudit(t)
	return engine, tenantID
}

// newTestEngineWithAudit is newTestEngine's fuller form: it also
// returns the *auditlog.Store the Engine's AuditSink writes to, for
// tests that need to inspect the recorded events directly.
func newTestEngineWithAudit(t *testing.T) (*localization.Engine, *auditlog.Store, uuid.UUID) {
	t.Helper()

	catalog := localization.NewSeededCatalog()
	prefs := localization.NewInMemoryPreferenceRepository()

	auditStore, err := auditlog.NewStore(auditlog.NewInMemoryRepository())
	if err != nil {
		t.Fatalf("auditlog.NewStore: %v", err)
	}
	sink, err := localization.NewAuditSink(auditStore)
	if err != nil {
		t.Fatalf("localization.NewAuditSink: %v", err)
	}

	engine, err := localization.NewEngine(catalog, prefs, sink)
	if err != nil {
		t.Fatalf("localization.NewEngine: %v", err)
	}
	return engine, auditStore, uuid.New()
}

// adminUser is a small convenience wrapper building a RoleAdmin user
// (holds PermManageUsers) scoped to tenantID.
func adminUser(tenantID uuid.UUID) *identity.User {
	return newTestUser(tenantID, identity.RoleAdmin)
}

// judgeUser is a small convenience wrapper building a RoleJudge user
// (does not hold PermManageUsers) scoped to tenantID.
func judgeUser(tenantID uuid.UUID) *identity.User {
	return newTestUser(tenantID, identity.RoleJudge)
}
