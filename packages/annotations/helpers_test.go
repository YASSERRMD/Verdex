package annotations_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/annotations"
	"github.com/YASSERRMD/verdex/packages/caselifecycle"
	"github.com/YASSERRMD/verdex/packages/identity"
)

// newTestUser builds an identity.User with the given permission-bearing
// role, scoped to tenantID, mirroring
// packages/casesearch's helpers_test.go newTestUser convention.
func newTestUser(tenantID uuid.UUID, roles ...identity.Role) *identity.User {
	return &identity.User{
		ID:       uuid.New(),
		TenantID: tenantID,
		Email:    "reviewer@example.test",
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

// seedCase creates and persists a new draft case for tenantID via
// repo, mirroring packages/casesearch's helpers_test.go seedCase
// convention.
func seedCase(t *testing.T, repo caselifecycle.Repository, tenantID uuid.UUID) *caselifecycle.Case {
	t.Helper()

	c, err := caselifecycle.NewCase(caselifecycle.NewCaseInput{
		TenantID:       tenantID,
		JurisdictionID: uuid.New(),
		CategoryID:     "civil",
		Title:          "Doe v. Acme Corp",
		CreatedBy:      uuid.New(),
	})
	if err != nil {
		t.Fatalf("NewCase: %v", err)
	}
	if err := repo.Create(context.Background(), tenantID, c); err != nil {
		t.Fatalf("repo.Create: %v", err)
	}
	return c
}

// newTestService builds an annotations.Service backed by fresh
// InMemory repositories for both annotations and cases, returning the
// Service along with the seeded case and tenant ID so tests can
// exercise a full Create/Get/... round-trip without repeating this
// wiring in every test.
func newTestService(t *testing.T) (*annotations.Service, *caselifecycle.Case, uuid.UUID) {
	t.Helper()

	tenantID := uuid.New()
	caseRepo := caselifecycle.NewInMemoryRepository()
	c := seedCase(t, caseRepo, tenantID)

	repo := annotations.NewInMemoryRepository()
	svc, err := annotations.NewService(repo, caseRepo, nil)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc, c, tenantID
}

// recordingSink is a annotations.MentionSink test double that records
// every Mention it receives.
type recordingSink struct {
	received []annotations.Mention
}

func (s *recordingSink) Notify(_ context.Context, m annotations.Mention) error {
	s.received = append(s.received, m)
	return nil
}

var _ annotations.MentionSink = (*recordingSink)(nil)
