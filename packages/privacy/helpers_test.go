package privacy_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/privacy"
)

// newTestUser builds an identity.User with the given permission-bearing
// role(s), scoped to tenantID, mirroring
// packages/accessgovernance's helpers_test.go newTestUser convention.
func newTestUser(tenantID uuid.UUID, roles ...identity.Role) *identity.User {
	return &identity.User{
		ID:       uuid.New(),
		TenantID: tenantID,
		Email:    "privacy@example.test",
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

// newTestEngine builds a privacy.Engine backed by fresh in-memory
// repositories and an in-memory-backed AuditSink, returning the
// Engine and a fresh tenant ID so tests can exercise a full
// round-trip without repeating this wiring.
func newTestEngine(t *testing.T) (*privacy.Engine, uuid.UUID) {
	t.Helper()
	engine, _, tenantID := newTestEngineWithAudit(t)
	return engine, tenantID
}

// newTestEngineWithAudit is newTestEngine's fuller form: it also
// returns the *auditlog.Store the Engine's AuditSink writes to, for
// tests that need to inspect the recorded events directly (task 7's
// "every SAR/erasure/consent state change gets recorded there").
func newTestEngineWithAudit(t *testing.T) (*privacy.Engine, *auditlog.Store, uuid.UUID) {
	t.Helper()

	inventory := privacy.NewInMemoryInventoryRepository()
	consent := privacy.NewInMemoryConsentRepository()
	sars := privacy.NewInMemorySARRepository()
	erasures := privacy.NewInMemoryErasureRepository()

	auditStore, err := auditlog.NewStore(auditlog.NewInMemoryRepository())
	if err != nil {
		t.Fatalf("auditlog.NewStore: %v", err)
	}
	sink, err := privacy.NewAuditSink(auditStore)
	if err != nil {
		t.Fatalf("privacy.NewAuditSink: %v", err)
	}

	engine, err := privacy.NewEngine(inventory, consent, sars, erasures, sink)
	if err != nil {
		t.Fatalf("privacy.NewEngine: %v", err)
	}
	return engine, auditStore, uuid.New()
}

// adminUser is a small convenience wrapper building a RoleAdmin user
// (holds both PermManagePrivacy and PermViewPrivacy) scoped to
// tenantID.
func adminUser(tenantID uuid.UUID) *identity.User {
	return newTestUser(tenantID, identity.RoleAdmin)
}

// auditorUser is a small convenience wrapper building a RoleAuditor
// user (holds only PermViewPrivacy) scoped to tenantID.
func auditorUser(tenantID uuid.UUID) *identity.User {
	return newTestUser(tenantID, identity.RoleAuditor)
}
