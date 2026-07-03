package caselifecycle_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/caselifecycle"
	"github.com/YASSERRMD/verdex/packages/identity"
)

// newTestUser builds an identity.User with the given permission-bearing
// role, scoped to tenantID.
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

// ctxWithUser returns a context carrying user, mirroring how an HTTP
// middleware layer would attach the authenticated actor.
func ctxWithUser(user *identity.User) context.Context {
	return identity.WithUser(context.Background(), user)
}

// seedCase creates and persists a new draft case for tenantID via repo,
// created by actor, returning the persisted Case.
func seedCase(t *testing.T, repo caselifecycle.Repository, tenantID uuid.UUID, actor uuid.UUID) *caselifecycle.Case {
	t.Helper()

	c, err := caselifecycle.NewCase(caselifecycle.NewCaseInput{
		TenantID:       tenantID,
		JurisdictionID: uuid.New(),
		CategoryID:     "civil",
		Title:          "Doe v. Acme Corp",
		Reference:      "2026-CV-001",
		CreatedBy:      actor,
	})
	if err != nil {
		t.Fatalf("NewCase: %v", err)
	}
	if err := repo.Create(context.Background(), tenantID, c); err != nil {
		t.Fatalf("repo.Create: %v", err)
	}
	return c
}

// advanceTo drives c from StateDraft to target via legal Transition
// calls only (draft->active->under_review->closed), returning the
// final Case value. target must be reachable via that exact chain
// (StateActive, StateUnderReview, or StateClosed).
func advanceTo(t *testing.T, ctx context.Context, repo caselifecycle.Repository, tenantID uuid.UUID, caseID uuid.UUID, target caselifecycle.State) *caselifecycle.Case {
	t.Helper()

	chain := []caselifecycle.State{caselifecycle.StateActive, caselifecycle.StateUnderReview, caselifecycle.StateClosed}
	var last *caselifecycle.Case
	for _, s := range chain {
		c, err := caselifecycle.Transition(ctx, repo, caselifecycle.TransitionInput{
			TenantID: tenantID,
			CaseID:   caseID,
			ToState:  s,
		})
		if err != nil {
			t.Fatalf("advanceTo: Transition to %s: %v", s, err)
		}
		last = c
		if s == target {
			return last
		}
	}
	t.Fatalf("advanceTo: target state %s not reachable via chain", target)
	return nil
}
