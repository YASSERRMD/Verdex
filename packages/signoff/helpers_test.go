package signoff_test

import (
	"context"
	"sync"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/signoff"
)

// newTestUser builds an identity.User with the given permission-bearing
// role, scoped to tenantID, mirroring
// packages/caselifecycle/helpers_test.go's newTestUser.
func newTestUser(tenantID uuid.UUID, roles ...identity.Role) *identity.User {
	return &identity.User{
		ID:       uuid.New(),
		TenantID: tenantID,
		Email:    "judge@example.test",
		Name:     "Test User",
		Roles:    roles,
		Status:   identity.UserStatusActive,
	}
}

// ctxWithUser returns a context carrying user.
func ctxWithUser(user *identity.User) context.Context {
	return identity.WithUser(context.Background(), user)
}

// fakeCaseVersionReader is an in-memory CaseVersionReader stub for
// tests, letting callers set/bump a case's version without pulling in
// the full packages/caselifecycle.Repository.
type fakeCaseVersionReader struct {
	mu       sync.Mutex
	versions map[uuid.UUID]int
}

func newFakeCaseVersionReader() *fakeCaseVersionReader {
	return &fakeCaseVersionReader{versions: make(map[uuid.UUID]int)}
}

func (f *fakeCaseVersionReader) set(caseID uuid.UUID, version int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.versions[caseID] = version
}

func (f *fakeCaseVersionReader) CaseVersion(_ context.Context, _, caseID uuid.UUID) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.versions[caseID]
	if !ok {
		return 1, nil
	}
	return v, nil
}

var _ signoff.CaseVersionReader = (*fakeCaseVersionReader)(nil)

// recordingNotificationSink captures every PendingSignoffEvent it
// receives, for assertions in tests.
type recordingNotificationSink struct {
	mu     sync.Mutex
	events []signoff.PendingSignoffEvent
}

func (r *recordingNotificationSink) Notify(_ context.Context, event signoff.PendingSignoffEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
	return nil
}

func (r *recordingNotificationSink) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.events)
}

var _ signoff.NotificationSink = (*recordingNotificationSink)(nil)

// newTestService builds a Service wired to InMemoryRepository, a
// fakeCaseVersionReader (seeded with caseID at version 1), and a
// recordingNotificationSink, returning all three for assertions.
func newTestService(caseID uuid.UUID) (*signoff.Service, *fakeCaseVersionReader, *recordingNotificationSink) {
	repo := signoff.NewInMemoryRepository()
	reader := newFakeCaseVersionReader()
	reader.set(caseID, 1)
	notifier := &recordingNotificationSink{}

	svc, err := signoff.NewService(repo, reader, notifier)
	if err != nil {
		panic(err)
	}
	return svc, reader, notifier
}
