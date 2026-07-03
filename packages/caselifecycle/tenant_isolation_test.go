package caselifecycle_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/caselifecycle"
	"github.com/YASSERRMD/verdex/packages/identity"
)

func TestInMemoryRepository_TenantAIsolatedFromTenantB(t *testing.T) {
	tenantA := uuid.New()
	tenantB := uuid.New()
	actorA := newTestUser(tenantA, identity.RoleAdmin)
	ctx := context.Background()

	repo := caselifecycle.NewInMemoryRepository()
	caseA := seedCase(t, repo, tenantA, actorA.ID)

	// Tenant B must not be able to see tenant A's case at all.
	if _, err := repo.Get(ctx, tenantB, caseA.ID); !errors.Is(err, caselifecycle.ErrNotFound) {
		t.Fatalf("expected ErrNotFound fetching tenant A's case under tenant B's scope, got %v", err)
	}

	// List under tenant B must never include tenant A's case.
	listB, err := repo.List(ctx, tenantB, caselifecycle.CaseFilter{})
	if err != nil {
		t.Fatalf("List under tenant B: %v", err)
	}
	if len(listB) != 0 {
		t.Fatalf("expected 0 cases visible to tenant B, got %d", len(listB))
	}

	// List under tenant A must return exactly caseA.
	listA, err := repo.List(ctx, tenantA, caselifecycle.CaseFilter{})
	if err != nil {
		t.Fatalf("List under tenant A: %v", err)
	}
	if len(listA) != 1 || listA[0].ID != caseA.ID {
		t.Fatalf("expected tenant A's list to contain exactly caseA, got %+v", listA)
	}
}

func TestInMemoryRepository_CrossTenantUpdateRejected(t *testing.T) {
	tenantA := uuid.New()
	tenantB := uuid.New()
	actorA := newTestUser(tenantA, identity.RoleAdmin)
	ctx := context.Background()

	repo := caselifecycle.NewInMemoryRepository()
	caseA := seedCase(t, repo, tenantA, actorA.ID)

	// Attempting to update tenant A's case while scoped to tenant B
	// must fail before any mutation happens.
	tampered := *caseA
	tampered.Title = "tampered"
	err := repo.Update(ctx, tenantB, &tampered)
	if err == nil {
		t.Fatal("expected an error updating tenant A's case under tenant B's scope, got nil")
	}
	if !errors.Is(err, caselifecycle.ErrCrossTenantAccess) && !errors.Is(err, caselifecycle.ErrNotFound) {
		t.Fatalf("expected ErrCrossTenantAccess or ErrNotFound, got %v", err)
	}

	unchanged, err := repo.Get(ctx, tenantA, caseA.ID)
	if err != nil {
		t.Fatalf("Get caseA under tenant A's own scope: %v", err)
	}
	if unchanged.Title != caseA.Title {
		t.Fatalf("expected caseA.Title to remain %q, got %q", caseA.Title, unchanged.Title)
	}
}

func TestInMemoryRepository_CrossTenantTransitionRejected(t *testing.T) {
	tenantA := uuid.New()
	tenantB := uuid.New()
	actorB := newTestUser(tenantB, identity.RoleAdmin)
	ctx := ctxWithUser(actorB)

	repo := caselifecycle.NewInMemoryRepository()
	actorA := newTestUser(tenantA, identity.RoleAdmin)
	caseA := seedCase(t, repo, tenantA, actorA.ID)

	// Tenant B's actor tries to transition tenant A's case by scoping
	// the call to tenantB: Get itself must fail (ErrNotFound), so the
	// transition never even reaches the guard.
	_, err := caselifecycle.Transition(ctx, repo, caselifecycle.TransitionInput{
		TenantID: tenantB,
		CaseID:   caseA.ID,
		ToState:  caselifecycle.StateActive,
	})
	if !errors.Is(err, caselifecycle.ErrNotFound) {
		t.Fatalf("expected ErrNotFound transitioning tenant A's case under tenant B's scope, got %v", err)
	}

	// caseA must remain untouched.
	unchanged, err := repo.Get(context.Background(), tenantA, caseA.ID)
	if err != nil {
		t.Fatalf("Get caseA under tenant A's own scope: %v", err)
	}
	if unchanged.State != caselifecycle.StateDraft {
		t.Fatalf("expected caseA.State to remain %s, got %s", caselifecycle.StateDraft, unchanged.State)
	}
}

func TestInMemoryRepository_ListTransitionsScopedToTenant(t *testing.T) {
	tenantA := uuid.New()
	tenantB := uuid.New()
	actorA := newTestUser(tenantA, identity.RoleAdmin)
	ctx := ctxWithUser(actorA)

	repo := caselifecycle.NewInMemoryRepository()
	caseA := seedCase(t, repo, tenantA, actorA.ID)
	advanceTo(t, ctx, repo, tenantA, caseA.ID, caselifecycle.StateActive)

	if _, err := repo.ListTransitions(context.Background(), tenantB, caseA.ID); !errors.Is(err, caselifecycle.ErrNotFound) {
		t.Fatalf("expected ErrNotFound listing tenant A's transitions under tenant B's scope, got %v", err)
	}

	history, err := repo.ListTransitions(context.Background(), tenantA, caseA.ID)
	if err != nil {
		t.Fatalf("ListTransitions under tenant A: %v", err)
	}
	if len(history) == 0 {
		t.Fatal("expected at least one transition record visible to tenant A")
	}
}

func TestInMemoryRepository_CreateStampsTenantIDWhenUnset(t *testing.T) {
	tenantID := uuid.New()
	actor := uuid.New()
	ctx := context.Background()

	repo := caselifecycle.NewInMemoryRepository()
	// Built directly (bypassing NewCase, which itself requires a
	// non-nil TenantID) to exercise Repository.Create's own
	// unset-TenantID-stamping behavior in isolation, mirroring
	// packages/tenancy.TestTenantScopedDeploymentRepository_Create_StampsUnsetTenantID.
	c := &caselifecycle.Case{
		JurisdictionID: uuid.New(),
		Title:          "Unset tenant case",
		State:          caselifecycle.StateDraft,
		CreatedBy:      actor,
	}
	if err := repo.Create(ctx, tenantID, c); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.TenantID != tenantID {
		t.Fatalf("TenantID = %v, want stamped %v", c.TenantID, tenantID)
	}
}

func TestInMemoryRepository_CreateRejectsMismatchedTenant(t *testing.T) {
	scopeTenant := uuid.New()
	otherTenant := uuid.New()
	ctx := context.Background()

	repo := caselifecycle.NewInMemoryRepository()
	c, err := caselifecycle.NewCase(caselifecycle.NewCaseInput{
		TenantID:       otherTenant,
		JurisdictionID: uuid.New(),
		Title:          "Mismatched tenant case",
		CreatedBy:      uuid.New(),
	})
	if err != nil {
		t.Fatalf("NewCase: %v", err)
	}

	err = repo.Create(ctx, scopeTenant, c)
	if !errors.Is(err, caselifecycle.ErrCrossTenantAccess) {
		t.Fatalf("expected ErrCrossTenantAccess, got %v", err)
	}
}
