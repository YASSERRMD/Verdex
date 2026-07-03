package signoff

import (
	"context"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/guardrail"
)

// MarkAwaitingSignoff ensures caseID has a Pending SignoffRecord and
// fires a PendingSignoffEvent notification. Callers are expected to
// invoke this when a case enters the state that requires judicial
// review (e.g. packages/caselifecycle.StateUnderReview) — see
// doc/signoff-workflow.md for the wiring example.
//
// If a record already exists and is already Pending, this still fires
// the notification (a caller may use it to re-announce a
// still-outstanding review) but does not create a duplicate audit
// entry. If a record exists and is Approved or Rejected, it is left
// untouched — MarkAwaitingSignoff never overwrites an existing
// decision; use ReReviewOnCaseUpdate to revert an Approved record.
func (s *Service) MarkAwaitingSignoff(ctx context.Context, tenantID, caseID uuid.UUID) (*SignoffRecord, error) {
	if caseID == uuid.Nil {
		return nil, ErrEmptyCaseID
	}

	liveVersion, err := s.caseReader.CaseVersion(ctx, tenantID, caseID)
	if err != nil {
		return nil, wrapf("MarkAwaitingSignoff", err)
	}

	at := s.now()
	rec, err := s.repo.Get(ctx, tenantID, caseID)
	if err != nil {
		if err != ErrNotFound {
			return nil, wrapf("MarkAwaitingSignoff", err)
		}
		rec = &SignoffRecord{
			ID:          uuid.New(),
			CaseID:      caseID,
			TenantID:    tenantID,
			Status:      guardrail.SignoffPending,
			CaseVersion: liveVersion,
			Source:      DecisionSourceInitial,
			DecidedAt:   at,
			CreatedAt:   at,
		}
		if err := s.repo.Upsert(ctx, tenantID, rec); err != nil {
			return nil, wrapf("MarkAwaitingSignoff", err)
		}
		if err := s.repo.AppendAudit(ctx, tenantID, &AuditEntry{
			ID:          uuid.New(),
			CaseID:      caseID,
			TenantID:    tenantID,
			FromStatus:  guardrail.SignoffPending,
			ToStatus:    guardrail.SignoffPending,
			Source:      DecisionSourceInitial,
			Notes:       "case entered sign-off workflow",
			CaseVersion: liveVersion,
			OccurredAt:  at,
		}); err != nil {
			return nil, wrapf("MarkAwaitingSignoff", err)
		}
	}

	_ = s.notifier.Notify(ctx, PendingSignoffEvent{
		TenantID:    tenantID,
		CaseID:      caseID,
		Reason:      "case awaiting human sign-off",
		CaseVersion: liveVersion,
		CreatedAt:   at,
	})

	return rec, nil
}
