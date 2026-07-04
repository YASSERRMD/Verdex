package corpusupdater_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
	"github.com/YASSERRMD/verdex/packages/corpusupdater"
	"github.com/YASSERRMD/verdex/packages/identity"
)

// newTestUser builds an identity.User with the given permission-bearing
// role(s), scoped to tenantID, mirroring
// packages/privacy's helpers_test.go newTestUser convention.
func newTestUser(tenantID uuid.UUID, roles ...identity.Role) *identity.User {
	return &identity.User{
		ID:       uuid.New(),
		TenantID: tenantID,
		Email:    "corpusupdater@example.test",
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
// (holds both PermManageCorpusUpdater and PermViewCorpusUpdater) scoped
// to tenantID.
func adminUser(tenantID uuid.UUID) *identity.User {
	return newTestUser(tenantID, identity.RoleAdmin)
}

// auditorUser is a small convenience wrapper building a RoleAuditor
// user (holds only PermViewCorpusUpdater) scoped to tenantID.
func auditorUser(tenantID uuid.UUID) *identity.User {
	return newTestUser(tenantID, identity.RoleAuditor)
}

// advocateUser is a small convenience wrapper building a RoleAdvocate
// user (holds neither corpusupdater permission) scoped to tenantID.
func advocateUser(tenantID uuid.UUID) *identity.User {
	return newTestUser(tenantID, identity.RoleAdvocate)
}

// newTestEngine builds a corpusupdater.Engine backed by fresh
// in-memory repositories and an in-memory-backed AuditSink, returning
// the Engine and a fresh tenant ID so tests can exercise a full
// round-trip without repeating this wiring.
func newTestEngine(t *testing.T) (*corpusupdater.Engine, uuid.UUID) {
	t.Helper()
	engine, _, tenantID := newTestEngineWithAudit(t)
	return engine, tenantID
}

// newTestEngineWithAudit is newTestEngine's fuller form: it also
// returns the *auditlog.Store the Engine's AuditSink writes to, for
// tests that need to inspect the recorded events directly (task 8's
// "every job/amendment state change gets recorded there").
func newTestEngineWithAudit(t *testing.T) (*corpusupdater.Engine, *auditlog.Store, uuid.UUID) {
	t.Helper()

	jobs := corpusupdater.NewInMemoryJobRepository()
	amendments := corpusupdater.NewInMemoryAmendmentRepository()

	auditStore, err := auditlog.NewStore(auditlog.NewInMemoryRepository())
	if err != nil {
		t.Fatalf("auditlog.NewStore: %v", err)
	}
	sink, err := corpusupdater.NewAuditSink(auditStore)
	if err != nil {
		t.Fatalf("corpusupdater.NewAuditSink: %v", err)
	}

	engine, err := corpusupdater.NewEngine(jobs, amendments, sink)
	if err != nil {
		t.Fatalf("corpusupdater.NewEngine: %v", err)
	}
	return engine, auditStore, uuid.New()
}

// fakeTextStore is an in-memory CorpusTextStore fixture for tests,
// keyed by "<corpus>:<targetID>".
type fakeTextStore struct {
	texts map[string]corpusupdater.CorpusText
}

func newFakeTextStore() *fakeTextStore {
	return &fakeTextStore{texts: make(map[string]corpusupdater.CorpusText)}
}

func (s *fakeTextStore) key(corpus corpusupdater.CorpusTarget, targetID string) string {
	return string(corpus) + ":" + targetID
}

func (s *fakeTextStore) GetText(_ context.Context, corpus corpusupdater.CorpusTarget, targetID string) (corpusupdater.CorpusText, error) {
	t, ok := s.texts[s.key(corpus, targetID)]
	if !ok {
		return corpusupdater.CorpusText{}, nil
	}
	return t, nil
}

func (s *fakeTextStore) SetText(_ context.Context, corpus corpusupdater.CorpusTarget, targetID string, text corpusupdater.CorpusText) error {
	s.texts[s.key(corpus, targetID)] = text
	return nil
}

func (s *fakeTextStore) get(corpus corpusupdater.CorpusTarget, targetID string) corpusupdater.CorpusText {
	return s.texts[s.key(corpus, targetID)]
}

// fakeEmbedder is a mock Embedder counting how many times Embed was
// called and with what texts, proving Engine.ApplyAmendment invokes it
// exactly once per changed rule/precedent (task 4).
type fakeEmbedder struct {
	calls int
	texts [][]string
	err   error
}

func (f *fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float64, error) {
	f.calls++
	f.texts = append(f.texts, texts)
	if f.err != nil {
		return nil, f.err
	}
	out := make([][]float64, len(texts))
	for i := range texts {
		out[i] = []float64{1, 2, 3}
	}
	return out, nil
}

// fakeNotificationSink is a mock NotificationSink recording every
// ChangeNotification it receives.
type fakeNotificationSink struct {
	notifications []corpusupdater.ChangeNotification
	err           error
}

func (f *fakeNotificationSink) NotifyChange(_ context.Context, n corpusupdater.ChangeNotification) error {
	f.notifications = append(f.notifications, n)
	return f.err
}
