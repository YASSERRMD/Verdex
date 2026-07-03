package caselifecycle_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/caselifecycle"
	"github.com/YASSERRMD/verdex/packages/identity"
)

func TestReopen_ClosedCaseReturnsToActiveWithJustification(t *testing.T) {
	tenantID := uuid.New()
	actor := newTestUser(tenantID, identity.RoleAdmin)
	ctx := ctxWithUser(actor)

	repo := caselifecycle.NewInMemoryRepository()
	c := seedCase(t, repo, tenantID, actor.ID)
	advanceTo(t, ctx, repo, tenantID, c.ID, caselifecycle.StateClosed)

	reopened, err := caselifecycle.Reopen(ctx, repo, tenantID, c.ID, "new evidence submitted on appeal")
	if err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	if reopened.State != caselifecycle.StateActive {
		t.Fatalf("State = %s, want %s", reopened.State, caselifecycle.StateActive)
	}

	history, err := repo.ListTransitions(ctx, tenantID, c.ID)
	if err != nil {
		t.Fatalf("ListTransitions: %v", err)
	}
	last := history[len(history)-1]
	if last.FromState != caselifecycle.StateClosed || last.ToState != caselifecycle.StateActive {
		t.Errorf("last record from/to = %s/%s, want closed/active", last.FromState, last.ToState)
	}
	if last.Reason != "new evidence submitted on appeal" {
		t.Errorf("last record reason = %q, want the supplied justification", last.Reason)
	}
}

func TestReopen_BlankJustificationRejected(t *testing.T) {
	tenantID := uuid.New()
	actor := newTestUser(tenantID, identity.RoleAdmin)
	ctx := ctxWithUser(actor)

	repo := caselifecycle.NewInMemoryRepository()
	c := seedCase(t, repo, tenantID, actor.ID)
	advanceTo(t, ctx, repo, tenantID, c.ID, caselifecycle.StateClosed)

	_, err := caselifecycle.Reopen(ctx, repo, tenantID, c.ID, "   ")
	if !errors.Is(err, caselifecycle.ErrReasonRequired) {
		t.Fatalf("expected ErrReasonRequired, got %v", err)
	}
}

func TestReopen_NonClosedCaseRejected(t *testing.T) {
	tenantID := uuid.New()
	actor := newTestUser(tenantID, identity.RoleAdmin)
	ctx := ctxWithUser(actor)

	repo := caselifecycle.NewInMemoryRepository()
	c := seedCase(t, repo, tenantID, actor.ID) // still StateDraft

	_, err := caselifecycle.Reopen(ctx, repo, tenantID, c.ID, "some reason")
	if !errors.Is(err, caselifecycle.ErrIllegalTransition) {
		t.Fatalf("expected ErrIllegalTransition reopening a non-closed case, got %v", err)
	}
}

func TestArchive_ClosedCaseBecomesTerminal(t *testing.T) {
	tenantID := uuid.New()
	actor := newTestUser(tenantID, identity.RoleAdmin)
	ctx := ctxWithUser(actor)

	repo := caselifecycle.NewInMemoryRepository()
	c := seedCase(t, repo, tenantID, actor.ID)
	advanceTo(t, ctx, repo, tenantID, c.ID, caselifecycle.StateClosed)

	archived, err := caselifecycle.Archive(ctx, repo, tenantID, c.ID, "retention policy")
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if archived.State != caselifecycle.StateArchived {
		t.Fatalf("State = %s, want %s", archived.State, caselifecycle.StateArchived)
	}
	if archived.ArchivedAt == nil {
		t.Fatal("ArchivedAt = nil, want non-nil after Archive")
	}
	if !archived.State.IsTerminal() {
		t.Fatal("expected StateArchived to be terminal")
	}
}

func TestArchive_CannotBeReopened(t *testing.T) {
	tenantID := uuid.New()
	actor := newTestUser(tenantID, identity.RoleAdmin)
	ctx := ctxWithUser(actor)

	repo := caselifecycle.NewInMemoryRepository()
	c := seedCase(t, repo, tenantID, actor.ID)
	advanceTo(t, ctx, repo, tenantID, c.ID, caselifecycle.StateClosed)
	if _, err := caselifecycle.Archive(ctx, repo, tenantID, c.ID, "done"); err != nil {
		t.Fatalf("Archive: %v", err)
	}

	_, err := caselifecycle.Reopen(ctx, repo, tenantID, c.ID, "trying to reopen an archived case")
	if !errors.Is(err, caselifecycle.ErrIllegalTransition) {
		t.Fatalf("expected ErrIllegalTransition reopening an archived case, got %v", err)
	}

	// Also confirm plain Transition cannot move an archived case
	// anywhere.
	for _, target := range []caselifecycle.State{
		caselifecycle.StateDraft, caselifecycle.StateActive,
		caselifecycle.StateUnderReview, caselifecycle.StateClosed,
	} {
		_, err := caselifecycle.Transition(ctx, repo, caselifecycle.TransitionInput{
			TenantID: tenantID,
			CaseID:   c.ID,
			ToState:  target,
		})
		if !errors.Is(err, caselifecycle.ErrIllegalTransition) {
			t.Errorf("Transition(archived -> %s) error = %v, want ErrIllegalTransition", target, err)
		}
	}
}

func TestArchive_NonClosedCaseRejected(t *testing.T) {
	tenantID := uuid.New()
	actor := newTestUser(tenantID, identity.RoleAdmin)
	ctx := ctxWithUser(actor)

	repo := caselifecycle.NewInMemoryRepository()
	c := seedCase(t, repo, tenantID, actor.ID)
	advanceTo(t, ctx, repo, tenantID, c.ID, caselifecycle.StateActive)

	_, err := caselifecycle.Archive(ctx, repo, tenantID, c.ID, "premature")
	if !errors.Is(err, caselifecycle.ErrIllegalTransition) {
		t.Fatalf("expected ErrIllegalTransition archiving a non-closed case, got %v", err)
	}
}
