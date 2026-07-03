package annotations_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/annotations"
	"github.com/YASSERRMD/verdex/packages/caselifecycle"
	"github.com/YASSERRMD/verdex/packages/identity"
)

// TestService_TenantIsolation_CrossTenantAccessBlocked proves that a
// user authenticated for tenant A cannot read or write annotations
// belonging to a case in tenant B, even if they somehow learn the
// annotation or case ID (e.g. leaked in a log line) — the real
// cross-tenant-leakage test task 6 requires.
func TestService_TenantIsolation_CrossTenantAccessBlocked(t *testing.T) {
	caseRepo := caselifecycle.NewInMemoryRepository()
	repo := annotations.NewInMemoryRepository()
	svc, err := annotations.NewService(repo, caseRepo, nil)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	tenantA := uuid.New()
	tenantB := uuid.New()
	caseA := seedCase(t, caseRepo, tenantA)
	caseB := seedCase(t, caseRepo, tenantB)

	userA := newTestUser(tenantA, identity.RoleClerk)
	userB := newTestUser(tenantB, identity.RoleClerk)

	annA, err := svc.Create(ctxWithUser(userA), tenantA, &annotations.Annotation{
		CaseID:     caseA.ID,
		Body:       "tenant A note",
		AnchorType: annotations.AnchorCase,
	})
	if err != nil {
		t.Fatalf("Create in tenant A: %v", err)
	}

	// Tenant B's user, scoped to tenant B, must not be able to read
	// tenant A's annotation even by guessing its ID — the service call
	// itself carries tenantB as the scope, so this is the same shape a
	// compromised or misconfigured tenant-B API handler would produce.
	if _, err := svc.Get(ctxWithUser(userB), tenantB, annA.ID); !errors.Is(err, annotations.ErrNotFound) {
		t.Fatalf("Get across tenants error = %v, want ErrNotFound", err)
	}

	// Listing tenant B's case must not surface tenant A's annotation.
	list, err := svc.ListByCase(ctxWithUser(userB), tenantB, caseB.ID, annotations.AnchorFilter{})
	if err != nil {
		t.Fatalf("ListByCase: %v", err)
	}
	for _, a := range list {
		if a.ID == annA.ID {
			t.Fatalf("tenant A's annotation leaked into tenant B's ListByCase result")
		}
	}

	// A tenant-B actor cannot create an annotation against tenant A's
	// case by passing tenantB as the scope: case-accessibility check
	// fails since caseA does not belong to tenantB.
	if _, err := svc.Create(ctxWithUser(userB), tenantB, &annotations.Annotation{
		CaseID:     caseA.ID,
		Body:       "attempted cross-tenant write",
		AnchorType: annotations.AnchorCase,
	}); !errors.Is(err, annotations.ErrForbidden) {
		t.Fatalf("cross-tenant Create error = %v, want ErrForbidden", err)
	}

	// Directly at the repository layer (bypassing Service), an
	// explicit tenant mismatch on the Annotation's own TenantID must
	// also be refused.
	mismatched := &annotations.Annotation{
		TenantID:   tenantA,
		CaseID:     caseA.ID,
		AuthorID:   userA.ID,
		Body:       "explicit mismatch",
		AnchorType: annotations.AnchorCase,
	}
	if err := repo.Create(context.Background(), tenantB, mismatched); !errors.Is(err, annotations.ErrCrossTenantAccess) {
		t.Fatalf("repo.Create tenant mismatch error = %v, want ErrCrossTenantAccess", err)
	}
}

// TestService_MentionsFor_ScopedToTenant proves MentionsFor never
// returns a mention recorded under a different tenant, even for the
// same user ID value (in principle, user IDs are tenant-scoped UUIDs
// so this cannot happen in practice, but the repository must still
// enforce it defensively).
func TestService_MentionsFor_ScopedToTenant(t *testing.T) {
	caseRepo := caselifecycle.NewInMemoryRepository()
	repo := annotations.NewInMemoryRepository()
	svc, err := annotations.NewService(repo, caseRepo, nil)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	tenantA := uuid.New()
	tenantB := uuid.New()
	caseA := seedCase(t, caseRepo, tenantA)
	seedCase(t, caseRepo, tenantB) // exercises tenant B's own case space

	mentionedUser := uuid.New()
	userA := newTestUser(tenantA, identity.RoleClerk)
	userB := newTestUser(tenantB, identity.RoleClerk)

	if _, err := svc.Create(ctxWithUser(userA), tenantA, &annotations.Annotation{
		CaseID:     caseA.ID,
		Body:       "@" + mentionedUser.String(),
		AnchorType: annotations.AnchorCase,
	}); err != nil {
		t.Fatalf("Create in tenant A: %v", err)
	}

	// Same mentioned-user ID, but the lookup is scoped to tenant B,
	// where no such mention was recorded.
	mentions, err := svc.MentionsFor(ctxWithUser(userB), tenantB, mentionedUser)
	if err != nil {
		t.Fatalf("MentionsFor: %v", err)
	}
	if len(mentions) != 0 {
		t.Fatalf("len(mentions) in tenant B = %d, want 0", len(mentions))
	}

	mentionsA, err := svc.MentionsFor(ctxWithUser(userA), tenantA, mentionedUser)
	if err != nil {
		t.Fatalf("MentionsFor tenant A: %v", err)
	}
	if len(mentionsA) != 1 {
		t.Fatalf("len(mentions) in tenant A = %d, want 1", len(mentionsA))
	}
}
