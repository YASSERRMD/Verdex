package caselifecycle

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/identity"
)

// allowedTransitions is the single authoritative source of truth for
// which State-to-State moves Transition permits. Reopen (reopen.go)
// and Archive (archive.go) are deliberately not represented here even
// though they also change State: they are distinct, audited
// operations with their own preconditions (a required justification
// for Reopen; closed-only origin for Archive), not ordinary
// Transition calls, so a caller cannot accidentally reopen or archive
// a case by calling plain Transition.
var allowedTransitions = map[State][]State{
	StateDraft:       {StateActive},
	StateActive:      {StateUnderReview},
	StateUnderReview: {StateClosed, StateActive},
	StateClosed:      {},
	StateArchived:    {},
}

// CanTransition reports whether moving directly from `from` to `to`
// via Transition is permitted by allowedTransitions.
func CanTransition(from, to State) bool {
	for _, s := range allowedTransitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

// TransitionInput bundles the arguments for Transition beyond the
// target state itself.
type TransitionInput struct {
	// TenantID scopes the operation.
	TenantID uuid.UUID

	// CaseID identifies the case to transition.
	CaseID uuid.UUID

	// ToState is the requested destination state.
	ToState State

	// Reason optionally explains the transition; recorded on the
	// resulting TransitionRecord verbatim (may be blank for ordinary
	// transitions).
	Reason string
}

// Transition moves the case identified by input.CaseID from its
// current State to input.ToState, provided that move is present in
// allowedTransitions, and appends a TransitionRecord to the audit
// log. The actor is read from ctx via identity.UserFromContext.
//
// Returns ErrUnauthenticated if ctx carries no authenticated user,
// ErrNotFound if the case does not exist or is not visible to
// input.TenantID, and ErrIllegalTransition if the requested move is
// not permitted from the case's current state. On success it returns
// the updated Case.
func Transition(ctx context.Context, repo Repository, input TransitionInput) (*Case, error) {
	if repo == nil {
		return nil, ErrNilRepository
	}
	user, ok := identity.UserFromContext(ctx)
	if !ok {
		return nil, ErrUnauthenticated
	}
	if !input.ToState.Valid() {
		return nil, wrapf("Transition", ErrIllegalTransition)
	}

	c, err := repo.Get(ctx, input.TenantID, input.CaseID)
	if err != nil {
		return nil, err
	}

	if !CanTransition(c.State, input.ToState) {
		return nil, wrapf("Transition", ErrIllegalTransition)
	}

	fromState := c.State
	now := time.Now().UTC()

	c.State = input.ToState
	c.UpdatedAt = now

	if err := repo.Update(ctx, input.TenantID, c); err != nil {
		return nil, err
	}

	record := &TransitionRecord{
		ID:         uuid.New(),
		CaseID:     c.ID,
		TenantID:   c.TenantID,
		FromState:  fromState,
		ToState:    input.ToState,
		Actor:      user.ID,
		Reason:     input.Reason,
		OccurredAt: now,
	}
	if err := repo.AppendTransition(ctx, input.TenantID, record); err != nil {
		return nil, err
	}

	return c, nil
}
