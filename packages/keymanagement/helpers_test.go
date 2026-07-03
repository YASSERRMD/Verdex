package keymanagement_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/keymanagement"
	"github.com/YASSERRMD/verdex/packages/observability"
)

// newTestUser builds an identity.User with the given permission-bearing
// role, scoped to tenantID, mirroring
// packages/caseversioning's helpers_test.go newTestUser convention.
func newTestUser(tenantID uuid.UUID, roles ...identity.Role) *identity.User {
	return &identity.User{
		ID:       uuid.New(),
		TenantID: tenantID,
		Email:    "keyops@example.test",
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

// newTestService builds a keymanagement.Service backed by a fresh
// InMemoryProvider (see inmemory_provider_test.go), InMemoryRepository,
// AuditRecorder, and InMemoryBreakGlassStore, returning the Service
// along with its Repository and a fresh tenant ID so tests can
// exercise a full round-trip without repeating this wiring.
func newTestService(t *testing.T) (*keymanagement.Service, keymanagement.Repository, uuid.UUID) {
	t.Helper()

	repo := keymanagement.NewInMemoryRepository()
	provider := newInMemoryProvider(repo)
	auditRepo := keymanagement.NewInMemoryAuditRepository()
	auditLogger := observability.NewAuditLogger(discardWriter{})
	recorder, err := keymanagement.NewAuditRecorder(auditLogger, auditRepo)
	if err != nil {
		t.Fatalf("NewAuditRecorder: %v", err)
	}
	breakGlass := keymanagement.NewInMemoryBreakGlassStore()

	svc, err := keymanagement.NewService(provider, repo, recorder, breakGlass)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc, repo, uuid.New()
}

// discardWriter is an io.Writer that discards everything written to
// it, used so tests do not spam stdout with audit log JSON lines.
type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }
