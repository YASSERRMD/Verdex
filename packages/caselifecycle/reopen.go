package caselifecycle

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/identity"
)

// Reopen moves the case identified by caseID from StateClosed back to
// StateActive, recording justification as the resulting
// TransitionRecord's Reason. This is intentionally a distinct
// operation from Transition(ctx, repo, TransitionInput{ToState:
// StateActive}): reopening a closed case always requires a non-blank
// justification, so the audit log never has to answer "why was this
// reopened?" with silence.
//
// Returns ErrUnauthenticated if ctx carries no authenticated user,
// ErrReasonRequired if justification is blank, ErrNotFound if the
// case does not exist or is not visible to tenantID, and
// ErrIllegalTransition if the case is not currently in StateClosed
// (in particular, an archived case can never be reopened — see
// State.IsTerminal).
func Reopen(ctx context.Context, repo Repository, tenantID, caseID uuid.UUID, justification string) (*Case, error) {
	if repo == nil {
		return nil, ErrNilRepository
	}
	user, ok := identity.UserFromContext(ctx)
	if !ok {
		return nil, ErrUnauthenticated
	}
	justification = strings.TrimSpace(justification)
	if justification == "" {
		return nil, ErrReasonRequired
	}

	c, err := repo.Get(ctx, tenantID, caseID)
	if err != nil {
		return nil, err
	}
	if c.State != StateClosed {
		return nil, wrapf("Reopen", ErrIllegalTransition)
	}

	now := time.Now().UTC()
	c.State = StateActive
	c.UpdatedAt = now

	if err := repo.Update(ctx, tenantID, c); err != nil {
		return nil, err
	}

	record := &TransitionRecord{
		ID:         uuid.New(),
		CaseID:     c.ID,
		TenantID:   c.TenantID,
		FromState:  StateClosed,
		ToState:    StateActive,
		Actor:      user.ID,
		Reason:     justification,
		OccurredAt: now,
	}
	if err := repo.AppendTransition(ctx, tenantID, record); err != nil {
		return nil, err
	}

	return c, nil
}
