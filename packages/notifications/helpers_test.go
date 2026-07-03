package notifications_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/notifications"
)

// newTestUser builds an identity.User scoped to tenantID, mirroring
// packages/caseversioning's helpers_test.go newTestUser convention.
func newTestUser(tenantID uuid.UUID) *identity.User {
	return &identity.User{
		ID:       uuid.New(),
		TenantID: tenantID,
		Email:    "reviewer@example.test",
		Name:     "Test User",
		Roles:    []identity.Role{identity.RoleClerk},
		Status:   identity.UserStatusActive,
	}
}

// ctxWithUser returns a context carrying user, mirroring how an HTTP
// middleware layer would attach the authenticated actor.
func ctxWithUser(user *identity.User) context.Context {
	return identity.WithUser(context.Background(), user)
}

// newTestService builds a notifications.Service backed by fresh
// InMemory repositories for both notifications and preferences.
func newTestService(t *testing.T) *notifications.Service {
	t.Helper()

	svc, err := notifications.NewService(
		notifications.NewInMemoryRepository(),
		notifications.NewInMemoryPreferenceRepository(),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc
}
