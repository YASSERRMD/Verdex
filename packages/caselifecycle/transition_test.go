package caselifecycle_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/caselifecycle"
	"github.com/YASSERRMD/verdex/packages/identity"
)

func TestCanTransition_AllowsDocumentedLegalMoves(t *testing.T) {
	legal := []struct {
		from, to caselifecycle.State
	}{
		{caselifecycle.StateDraft, caselifecycle.StateActive},
		{caselifecycle.StateActive, caselifecycle.StateUnderReview},
		{caselifecycle.StateUnderReview, caselifecycle.StateClosed},
		{caselifecycle.StateUnderReview, caselifecycle.StateActive},
	}
	for _, tc := range legal {
		if !caselifecycle.CanTransition(tc.from, tc.to) {
			t.Errorf("CanTransition(%s, %s) = false, want true", tc.from, tc.to)
		}
	}
}

func TestCanTransition_RejectsIllegalMoves(t *testing.T) {
	illegal := []struct {
		from, to caselifecycle.State
	}{
		{caselifecycle.StateDraft, caselifecycle.StateUnderReview},
		{caselifecycle.StateDraft, caselifecycle.StateClosed},
		{caselifecycle.StateActive, caselifecycle.StateDraft},
		{caselifecycle.StateActive, caselifecycle.StateClosed},
		{caselifecycle.StateClosed, caselifecycle.StateActive}, // must go through Reopen, not plain Transition
		{caselifecycle.StateClosed, caselifecycle.StateDraft},
		{caselifecycle.StateArchived, caselifecycle.StateActive},
		{caselifecycle.StateArchived, caselifecycle.StateClosed},
	}
	for _, tc := range illegal {
		if caselifecycle.CanTransition(tc.from, tc.to) {
			t.Errorf("CanTransition(%s, %s) = true, want false", tc.from, tc.to)
		}
	}
}

func TestTransition_LegalMoveSucceedsAndRecordsAudit(t *testing.T) {
	tenantID := uuid.New()
	actor := newTestUser(tenantID, identity.RoleAdmin)
	ctx := ctxWithUser(actor)

	repo := caselifecycle.NewInMemoryRepository()
	c := seedCase(t, repo, tenantID, actor.ID)

	updated, err := caselifecycle.Transition(ctx, repo, caselifecycle.TransitionInput{
		TenantID: tenantID,
		CaseID:   c.ID,
		ToState:  caselifecycle.StateActive,
		Reason:   "intake complete",
	})
	if err != nil {
		t.Fatalf("Transition: %v", err)
	}
	if updated.State != caselifecycle.StateActive {
		t.Fatalf("State = %s, want %s", updated.State, caselifecycle.StateActive)
	}

	history, err := repo.ListTransitions(ctx, tenantID, c.ID)
	if err != nil {
		t.Fatalf("ListTransitions: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 transition record, got %d", len(history))
	}
	rec := history[0]
	if rec.FromState != caselifecycle.StateDraft || rec.ToState != caselifecycle.StateActive {
		t.Errorf("record from/to = %s/%s, want draft/active", rec.FromState, rec.ToState)
	}
	if rec.Actor != actor.ID {
		t.Errorf("record actor = %v, want %v", rec.Actor, actor.ID)
	}
	if rec.Reason != "intake complete" {
		t.Errorf("record reason = %q, want %q", rec.Reason, "intake complete")
	}
	if rec.CaseID != c.ID || rec.TenantID != tenantID {
		t.Errorf("record case/tenant = %v/%v, want %v/%v", rec.CaseID, rec.TenantID, c.ID, tenantID)
	}
}

func TestTransition_IllegalMoveRejectedWithClearError(t *testing.T) {
	tenantID := uuid.New()
	actor := newTestUser(tenantID, identity.RoleAdmin)
	ctx := ctxWithUser(actor)

	repo := caselifecycle.NewInMemoryRepository()
	c := seedCase(t, repo, tenantID, actor.ID)

	// draft -> closed is not a legal direct move.
	_, err := caselifecycle.Transition(ctx, repo, caselifecycle.TransitionInput{
		TenantID: tenantID,
		CaseID:   c.ID,
		ToState:  caselifecycle.StateClosed,
	})
	if !errors.Is(err, caselifecycle.ErrIllegalTransition) {
		t.Fatalf("expected ErrIllegalTransition, got %v", err)
	}

	// Case must remain unchanged, and no transition record should have
	// been written.
	unchanged, err := repo.Get(ctx, tenantID, c.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if unchanged.State != caselifecycle.StateDraft {
		t.Fatalf("State = %s, want unchanged %s", unchanged.State, caselifecycle.StateDraft)
	}
	history, err := repo.ListTransitions(ctx, tenantID, c.ID)
	if err != nil {
		t.Fatalf("ListTransitions: %v", err)
	}
	if len(history) != 0 {
		t.Fatalf("expected 0 transition records after a rejected transition, got %d", len(history))
	}
}

func TestTransition_UnknownStateRejected(t *testing.T) {
	tenantID := uuid.New()
	actor := newTestUser(tenantID, identity.RoleAdmin)
	ctx := ctxWithUser(actor)

	repo := caselifecycle.NewInMemoryRepository()
	c := seedCase(t, repo, tenantID, actor.ID)

	_, err := caselifecycle.Transition(ctx, repo, caselifecycle.TransitionInput{
		TenantID: tenantID,
		CaseID:   c.ID,
		ToState:  caselifecycle.State("not-a-real-state"),
	})
	if !errors.Is(err, caselifecycle.ErrIllegalTransition) {
		t.Fatalf("expected ErrIllegalTransition for unknown state, got %v", err)
	}
}

func TestTransition_UnauthenticatedContextRejected(t *testing.T) {
	tenantID := uuid.New()
	creator := uuid.New()

	repo := caselifecycle.NewInMemoryRepository()
	c := seedCase(t, repo, tenantID, creator)

	_, err := caselifecycle.Transition(context.Background(), repo, caselifecycle.TransitionInput{
		TenantID: tenantID,
		CaseID:   c.ID,
		ToState:  caselifecycle.StateActive,
	})
	if !errors.Is(err, caselifecycle.ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestTransition_NotFoundForMissingCase(t *testing.T) {
	tenantID := uuid.New()
	actor := newTestUser(tenantID, identity.RoleAdmin)
	ctx := ctxWithUser(actor)

	repo := caselifecycle.NewInMemoryRepository()

	_, err := caselifecycle.Transition(ctx, repo, caselifecycle.TransitionInput{
		TenantID: tenantID,
		CaseID:   uuid.New(),
		ToState:  caselifecycle.StateActive,
	})
	if !errors.Is(err, caselifecycle.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestTransition_NilRepositoryRejected(t *testing.T) {
	tenantID := uuid.New()
	actor := newTestUser(tenantID, identity.RoleAdmin)
	ctx := ctxWithUser(actor)

	_, err := caselifecycle.Transition(ctx, nil, caselifecycle.TransitionInput{
		TenantID: tenantID,
		CaseID:   uuid.New(),
		ToState:  caselifecycle.StateActive,
	})
	if !errors.Is(err, caselifecycle.ErrNilRepository) {
		t.Fatalf("expected ErrNilRepository, got %v", err)
	}
}

func TestState_IsTerminal(t *testing.T) {
	if caselifecycle.StateArchived.IsTerminal() != true {
		t.Errorf("StateArchived.IsTerminal() = false, want true")
	}
	nonTerminal := []caselifecycle.State{
		caselifecycle.StateDraft, caselifecycle.StateActive,
		caselifecycle.StateUnderReview, caselifecycle.StateClosed,
	}
	for _, s := range nonTerminal {
		if s.IsTerminal() {
			t.Errorf("%s.IsTerminal() = true, want false", s)
		}
	}
}

func TestState_Valid(t *testing.T) {
	valid := []caselifecycle.State{
		caselifecycle.StateDraft, caselifecycle.StateActive,
		caselifecycle.StateUnderReview, caselifecycle.StateClosed, caselifecycle.StateArchived,
	}
	for _, s := range valid {
		if !s.Valid() {
			t.Errorf("%s.Valid() = false, want true", s)
		}
	}
	if caselifecycle.State("bogus").Valid() {
		t.Error("State(\"bogus\").Valid() = true, want false")
	}
}
