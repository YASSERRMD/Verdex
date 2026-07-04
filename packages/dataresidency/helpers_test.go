package dataresidency_test

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
	"github.com/YASSERRMD/verdex/packages/dataresidency"
	"github.com/YASSERRMD/verdex/packages/identity"
)

// newTestStore builds an auditlog.Store backed by a fresh
// InMemoryRepository, mirroring packages/auditlog's own
// helpers_test.go convention.
func newTestStore(t *testing.T) *auditlog.Store {
	t.Helper()
	store, err := auditlog.NewStore(auditlog.NewInMemoryRepository())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return store
}

// newTestAuditSink builds a dataresidency.AuditSink over a fresh
// in-memory store.
func newTestAuditSink(t *testing.T) (*dataresidency.AuditSink, *auditlog.Store) {
	t.Helper()
	store := newTestStore(t)
	sink, err := dataresidency.NewAuditSink(store)
	if err != nil {
		t.Fatalf("NewAuditSink: %v", err)
	}
	return sink, store
}

// newTestUser builds an identity.User with the given permission-bearing
// role, scoped to tenantID.
func newTestUser(tenantID uuid.UUID, roles ...identity.Role) *identity.User {
	return &identity.User{
		ID:       uuid.New(),
		TenantID: tenantID,
		Email:    "operator@example.test",
		Name:     "Test Operator",
		Roles:    roles,
		Status:   identity.UserStatusActive,
	}
}

// ctxWithUser returns a context carrying user.
func ctxWithUser(user *identity.User) context.Context {
	return identity.WithUser(context.Background(), user)
}

// fakePolicySource is an in-memory dataresidency.PolicySource fake for
// tests, avoiding any database dependency.
type fakePolicySource struct {
	mu       sync.Mutex
	policies map[uuid.UUID]*dataresidency.ResidencyPolicy
	pins     map[uuid.UUID]*dataresidency.RegionPin
}

func newFakePolicySource() *fakePolicySource {
	return &fakePolicySource{
		policies: make(map[uuid.UUID]*dataresidency.ResidencyPolicy),
		pins:     make(map[uuid.UUID]*dataresidency.RegionPin),
	}
}

func (f *fakePolicySource) setPolicy(p dataresidency.ResidencyPolicy) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.policies[p.DeploymentID] = &p
}

func (f *fakePolicySource) setPin(p dataresidency.RegionPin) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pins[p.DeploymentID] = &p
}

func (f *fakePolicySource) Policy(_ context.Context, deploymentID uuid.UUID) (*dataresidency.ResidencyPolicy, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.policies[deploymentID]
	if !ok {
		return nil, errNotConfigured
	}
	return p, nil
}

func (f *fakePolicySource) RegionPin(_ context.Context, deploymentID uuid.UUID) (*dataresidency.RegionPin, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.pins[deploymentID]
	if !ok {
		return nil, errNotConfigured
	}
	return p, nil
}

var errNotConfigured = &notConfiguredError{}

type notConfiguredError struct{}

func (*notConfiguredError) Error() string { return "dataresidency_test: not configured" }
