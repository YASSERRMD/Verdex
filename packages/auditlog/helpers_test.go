package auditlog_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
	"github.com/YASSERRMD/verdex/packages/identity"
)

// newTestUser builds an identity.User with the given permission-bearing
// role, scoped to tenantID, mirroring
// packages/keymanagement's helpers_test.go newTestUser convention.
func newTestUser(tenantID uuid.UUID, roles ...identity.Role) *identity.User {
	return &identity.User{
		ID:       uuid.New(),
		TenantID: tenantID,
		Email:    "auditor@example.test",
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

// newTestStore builds an auditlog.Store backed by a fresh
// InMemoryRepository.
func newTestStore(t *testing.T) *auditlog.Store {
	t.Helper()
	store, err := auditlog.NewStore(auditlog.NewInMemoryRepository())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return store
}

// mustAppend appends event via a system-permission-free path (Append
// carries no gate) and fails the test on error.
func mustAppend(t *testing.T, store *auditlog.Store, event auditlog.Event) auditlog.Event {
	t.Helper()
	got, err := store.Append(context.Background(), event)
	if err != nil {
		t.Fatalf("Append: %v", err)
	}
	return got
}

// newEvent builds a minimal valid Event for tenantID.
func newEvent(tenantID uuid.UUID, actor, action string, kind auditlog.Kind) auditlog.Event {
	ev := auditlog.Event{
		TenantID: tenantID,
		Kind:     kind,
	}
	ev.Actor = actor
	ev.Action = action
	ev.Outcome = "success"
	return ev
}
