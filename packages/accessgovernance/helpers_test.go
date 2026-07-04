package accessgovernance_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/accessgovernance"
	"github.com/YASSERRMD/verdex/packages/auditlog"
	"github.com/YASSERRMD/verdex/packages/identity"
)

// newTestUser builds an identity.User with the given permission-bearing
// role(s), scoped to tenantID, mirroring
// packages/keymanagement's helpers_test.go newTestUser convention.
func newTestUser(tenantID uuid.UUID, roles ...identity.Role) *identity.User {
	return &identity.User{
		ID:       uuid.New(),
		TenantID: tenantID,
		Email:    "accessgov@example.test",
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

// newTestEngine builds an accessgovernance.Engine backed by fresh
// in-memory repositories and an in-memory-backed AuditSink, returning
// the Engine and a fresh tenant ID so tests can exercise a full
// round-trip without repeating this wiring.
func newTestEngine(t *testing.T) (*accessgovernance.Engine, uuid.UUID) {
	t.Helper()

	policies := accessgovernance.NewInMemoryPolicyRepository()
	caseGrants := accessgovernance.NewInMemoryCaseGrantRepository()
	elevations := accessgovernance.NewInMemoryGrantRepository()
	reviews := accessgovernance.NewInMemoryReviewRepository()

	auditStore, err := auditlog.NewStore(auditlog.NewInMemoryRepository())
	if err != nil {
		t.Fatalf("auditlog.NewStore: %v", err)
	}
	sink, err := accessgovernance.NewAuditSink(auditStore)
	if err != nil {
		t.Fatalf("accessgovernance.NewAuditSink: %v", err)
	}

	engine, err := accessgovernance.NewEngine(policies, caseGrants, elevations, reviews, sink)
	if err != nil {
		t.Fatalf("accessgovernance.NewEngine: %v", err)
	}
	return engine, uuid.New()
}

// activePolicy builds a minimal, valid, Active Policy for tenantID
// with the given rules.
func activePolicy(tenantID uuid.UUID, rules ...accessgovernance.PolicyRule) accessgovernance.Policy {
	return accessgovernance.Policy{
		ID:       uuid.New(),
		TenantID: tenantID,
		Name:     "test-policy",
		Rules:    rules,
		Active:   true,
	}
}
