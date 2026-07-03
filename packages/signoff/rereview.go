package signoff

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/guardrail"
)

// ReReviewOnCaseUpdate checks whether caseID's live
// packages/caselifecycle Case.MetadataVersion has advanced past the
// version recorded on its current SignoffRecord. If the case is
// currently Approved and the version has changed, the sign-off
// automatically reverts to Pending: an AuditEntry records why
// (DecisionSourceReReview), and a PendingSignoffEvent notification
// fires.
//
// Callers are expected to invoke this after any operation that bumps
// a case's MetadataVersion (see packages/caselifecycle.SetMetadata /
// MergeMetadata), or on a schedule/webhook, so a stale approval can
// never survive a case content change. It is idempotent: calling it
// again with no version change is a no-op that returns the unchanged
// record.
//
// ReReviewOnCaseUpdate returns the SignoffRecord as it stands after
// the check (whether or not a reversion happened), and a bool
// reporting whether a reversion occurred.
func (s *Service) ReReviewOnCaseUpdate(ctx context.Context, tenantID, caseID uuid.UUID) (*SignoffRecord, bool, error) {
	if caseID == uuid.Nil {
		return nil, false, ErrEmptyCaseID
	}

	liveVersion, err := s.caseReader.CaseVersion(ctx, tenantID, caseID)
	if err != nil {
		return nil, false, wrapf("ReReviewOnCaseUpdate", err)
	}

	current, err := s.repo.Get(ctx, tenantID, caseID)
	if err != nil {
		if err == ErrNotFound {
			// No sign-off record exists yet at all: nothing to
			// revert. The case will get a fresh Pending record the
			// first time Get/Approve/Reject touches it.
			return nil, false, nil
		}
		return nil, false, wrapf("ReReviewOnCaseUpdate", err)
	}

	if current.Status != guardrail.SignoffApproved || current.CaseVersion == liveVersion {
		return current, false, nil
	}

	at := s.now()
	fromStatus := current.Status
	reverted := &SignoffRecord{
		ID:          current.ID,
		CaseID:      caseID,
		TenantID:    tenantID,
		Status:      guardrail.SignoffPending,
		ReviewerID:  current.ReviewerID,
		Notes:       "",
		CaseVersion: liveVersion,
		Source:      DecisionSourceReReview,
		DecidedAt:   at,
		CreatedAt:   current.CreatedAt,
	}

	if err := s.repo.Upsert(ctx, tenantID, reverted); err != nil {
		return nil, false, wrapf("ReReviewOnCaseUpdate", err)
	}

	reason := fmt.Sprintf("case metadata version changed from %d to %d after approval", current.CaseVersion, liveVersion)
	if err := s.repo.AppendAudit(ctx, tenantID, &AuditEntry{
		ID:          uuid.New(),
		CaseID:      caseID,
		TenantID:    tenantID,
		FromStatus:  fromStatus,
		ToStatus:    guardrail.SignoffPending,
		Source:      DecisionSourceReReview,
		Notes:       reason,
		CaseVersion: liveVersion,
		OccurredAt:  at,
	}); err != nil {
		return nil, false, wrapf("ReReviewOnCaseUpdate", err)
	}

	_ = s.notifier.Notify(ctx, PendingSignoffEvent{
		TenantID:    tenantID,
		CaseID:      caseID,
		Reason:      reason,
		CaseVersion: liveVersion,
		CreatedAt:   at,
	})

	return reverted, true, nil
}
