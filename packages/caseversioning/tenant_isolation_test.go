package caseversioning_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/caselifecycle"
	"github.com/YASSERRMD/verdex/packages/caseversioning"
	"github.com/YASSERRMD/verdex/packages/identity"
)

// TestService_TenantIsolation_CrossTenantAccessBlocked proves that a
// user authenticated for tenant A cannot read, list, diff, or restore
// version-history snapshots belonging to a case in tenant B, even if
// they somehow learn the snapshot or case ID (mirroring
// packages/annotations's tenant_isolation_test.go exactly for this
// package's Service).
func TestService_TenantIsolation_CrossTenantAccessBlocked(t *testing.T) {
	caseRepo := caselifecycle.NewInMemoryRepository()
	repo := caseversioning.NewInMemoryRepository()
	svc, err := caseversioning.NewService(repo, caseRepo)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	tenantA := uuid.New()
	tenantB := uuid.New()
	caseA := seedCase(t, caseRepo, tenantA)
	caseB := seedCase(t, caseRepo, tenantB)

	userA := newTestUser(tenantA, identity.RoleClerk)
	userB := newTestUser(tenantB, identity.RoleClerk)

	snapA, err := svc.SnapshotCaseMetadata(ctxWithUser(userA), tenantA, caseA.ID, "initial", "")
	if err != nil {
		t.Fatalf("SnapshotCaseMetadata in tenant A: %v", err)
	}

	// Tenant B's user, scoped to tenant B, must not be able to read
	// tenant A's snapshot even by guessing its ID.
	if _, err := svc.Get(ctxWithUser(userB), tenantB, snapA.ID); !errors.Is(err, caseversioning.ErrNotFound) {
		t.Fatalf("Get across tenants error = %v, want ErrNotFound", err)
	}

	// Listing tenant B's case must not surface tenant A's snapshot.
	list, err := svc.History(ctxWithUser(userB), tenantB, caseB.ID, caseversioning.SnapshotFilter{})
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	for _, s := range list {
		if s.ID == snapA.ID {
			t.Fatalf("tenant A's snapshot leaked into tenant B's History result")
		}
	}

	// A tenant-B actor cannot snapshot tenant A's case by passing
	// tenantB as the scope: case-accessibility check fails since caseA
	// does not belong to tenantB.
	if _, err := svc.SnapshotCaseMetadata(ctxWithUser(userB), tenantB, caseA.ID, "attempted cross-tenant write", ""); !errors.Is(err, caseversioning.ErrForbidden) {
		t.Fatalf("cross-tenant SnapshotCaseMetadata error = %v, want ErrForbidden", err)
	}

	// Restore across tenants must also fail: the snapshot itself is not
	// visible to tenant B's scope.
	if _, err := svc.Restore(ctxWithUser(userB), tenantB, snapA.ID); !errors.Is(err, caseversioning.ErrNotFound) {
		t.Fatalf("cross-tenant Restore error = %v, want ErrNotFound", err)
	}

	// Directly at the repository layer (bypassing Service), an explicit
	// tenant mismatch on the Snapshot's own TenantID must also be
	// refused.
	mismatched := &caseversioning.Snapshot{
		TenantID:     tenantA,
		CaseID:       caseA.ID,
		ArtifactKind: caseversioning.ArtifactCaseMetadata,
		CreatedBy:    userA.ID,
	}
	if err := repo.Create(context.Background(), tenantB, mismatched); !errors.Is(err, caseversioning.ErrCrossTenantAccess) {
		t.Fatalf("repo.Create tenant mismatch error = %v, want ErrCrossTenantAccess", err)
	}
}

// TestService_Diff_ScopedToTenant proves Diff refuses to compare two
// snapshots when the caller's tenant scope cannot see them, even if
// both snapshot IDs are individually well-formed.
func TestService_Diff_ScopedToTenant(t *testing.T) {
	caseRepo := caselifecycle.NewInMemoryRepository()
	repo := caseversioning.NewInMemoryRepository()
	svc, err := caseversioning.NewService(repo, caseRepo)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	tenantA := uuid.New()
	tenantB := uuid.New()
	caseA := seedCase(t, caseRepo, tenantA)
	userA := newTestUser(tenantA, identity.RoleClerk)
	userB := newTestUser(tenantB, identity.RoleClerk)

	a, err := svc.SnapshotCaseMetadata(ctxWithUser(userA), tenantA, caseA.ID, "a", "")
	if err != nil {
		t.Fatalf("SnapshotCaseMetadata a: %v", err)
	}
	b, err := svc.SnapshotCaseMetadata(ctxWithUser(userA), tenantA, caseA.ID, "b", "")
	if err != nil {
		t.Fatalf("SnapshotCaseMetadata b: %v", err)
	}

	if _, err := svc.Diff(ctxWithUser(userB), tenantB, a.ID, b.ID); !errors.Is(err, caseversioning.ErrNotFound) {
		t.Fatalf("cross-tenant Diff error = %v, want ErrNotFound", err)
	}
}
