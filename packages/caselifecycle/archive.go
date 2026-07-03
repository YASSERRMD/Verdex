package caselifecycle

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/identity"
)

// Archive moves the case identified by caseID from StateClosed to
// StateArchived, a terminal state distinct from StateClosed: unlike a
// closed case, an archived case can never be reopened (see
// State.IsTerminal and allowedTransitions, neither of which lists any
// outgoing transition for StateArchived). It stamps Case.ArchivedAt
// and records a TransitionRecord.
//
// reason is optional context for the archival decision (e.g. "case
// series consolidated", "retention policy"); unlike Reopen, Archive
// does not require a non-blank reason, since archiving a
// already-closed case is a lower-stakes, often-batch administrative
// action.
//
// Returns ErrUnauthenticated if ctx carries no authenticated user,
// ErrNotFound if the case does not exist or is not visible to
// tenantID, and ErrIllegalTransition if the case is not currently in
// StateClosed.
func Archive(ctx context.Context, repo Repository, tenantID, caseID uuid.UUID, reason string) (*Case, error) {
	if repo == nil {
		return nil, ErrNilRepository
	}
	user, ok := identity.UserFromContext(ctx)
	if !ok {
		return nil, ErrUnauthenticated
	}

	c, err := repo.Get(ctx, tenantID, caseID)
	if err != nil {
		return nil, err
	}
	if c.State != StateClosed {
		return nil, wrapf("Archive", ErrIllegalTransition)
	}

	now := time.Now().UTC()
	c.State = StateArchived
	c.UpdatedAt = now
	c.ArchivedAt = &now

	if err := repo.Update(ctx, tenantID, c); err != nil {
		return nil, err
	}

	record := &TransitionRecord{
		ID:         uuid.New(),
		CaseID:     c.ID,
		TenantID:   c.TenantID,
		FromState:  StateClosed,
		ToState:    StateArchived,
		Actor:      user.ID,
		Reason:     reason,
		OccurredAt: now,
	}
	if err := repo.AppendTransition(ctx, tenantID, record); err != nil {
		return nil, err
	}

	return c, nil
}
