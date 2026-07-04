package accessgovernance

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ScheduleReview creates a Review entry (task 4) flagging subjectID
// (a CaseGrant or Grant ID, per subjectKind) as due for review by
// dueAt. requestedBy identifies the user the underlying grant was
// issued to/requested by, recorded on the Review so Attest can later
// enforce segregation of duties without a second lookup (task 5).
// Requires managePermission.
func (e *Engine) ScheduleReview(ctx context.Context, tenantID uuid.UUID, subjectKind GrantKind, subjectID, requestedBy uuid.UUID, dueAt time.Time) (Review, error) {
	if e.reviews == nil {
		return Review{}, ErrNilStore
	}
	user, err := authorizeManage(ctx)
	if err != nil {
		return Review{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return Review{}, err
	}

	rv := &Review{
		ID:          uuid.New(),
		TenantID:    tenantID,
		SubjectKind: subjectKind,
		SubjectID:   subjectID,
		RequestedBy: requestedBy,
		DueAt:       dueAt,
		CreatedAt:   e.now(),
	}
	if err := rv.Validate(); err != nil {
		return Review{}, err
	}
	if err := e.reviews.Create(ctx, tenantID, rv); err != nil {
		return Review{}, wrapf("ScheduleReview", err)
	}
	return *rv, nil
}

// ListDueReviews returns every pending Review for tenantID whose
// DueAt is at or before asOf (task 4's "list active grants due for
// review"). Requires reviewPermission.
func (e *Engine) ListDueReviews(ctx context.Context, tenantID uuid.UUID, asOf time.Time) ([]Review, error) {
	if e.reviews == nil {
		return nil, ErrNilStore
	}
	user, err := authorizeActor(ctx)
	if err != nil {
		return nil, err
	}
	if !user.HasPermission(reviewPermission) {
		return nil, ErrForbidden
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return nil, err
	}

	list, err := e.reviews.ListDue(ctx, tenantID, asOf)
	if err != nil {
		return nil, wrapf("ListDueReviews", err)
	}
	return list, nil
}

// Attest records actor's decision on reviewID (task 4): Approve
// leaves the underlying grant active until its own natural expiry;
// Revoke immediately revokes it. Attest enforces segregation of
// duties (task 5) before recording anything: the actor attesting
// cannot be the same user the underlying grant was requested by/
// issued to (RuleRequesterCannotApprove) -- ErrSegregationOfDuties is
// returned, and the rejection itself is still audited, if that check
// fails. Attest is idempotent-unsafe by design: a Review that has
// already recorded a Decision cannot be attested again
// (ErrReviewAlreadyDecided).
func (e *Engine) Attest(ctx context.Context, tenantID, reviewID uuid.UUID, decision AttestationDecision, notes string) (Review, error) {
	if e.reviews == nil {
		return Review{}, ErrNilStore
	}
	if !decision.IsValid() {
		return Review{}, ErrInvalidAttestationDecision
	}

	user, err := authorizeActor(ctx)
	if err != nil {
		return Review{}, err
	}
	if !user.HasPermission(reviewPermission) {
		if e.audit != nil {
			_, _ = e.audit.RecordAttest(ctx, tenantID, user.ID, reviewID, decision, ErrForbidden)
		}
		return Review{}, ErrForbidden
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordAttest(ctx, tenantID, user.ID, reviewID, decision, err)
		}
		return Review{}, err
	}

	rv, err := e.reviews.Get(ctx, tenantID, reviewID)
	if err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordAttest(ctx, tenantID, user.ID, reviewID, decision, err)
		}
		return Review{}, err
	}
	if !rv.IsPending() {
		if e.audit != nil {
			_, _ = e.audit.RecordAttest(ctx, tenantID, user.ID, reviewID, decision, ErrReviewAlreadyDecided)
		}
		return Review{}, ErrReviewAlreadyDecided
	}

	if violated := CheckConflict(ConflictCheck{RequestedBy: rv.RequestedBy, ActingUserID: user.ID}, nil); violated != "" {
		if e.audit != nil {
			_, _ = e.audit.RecordAttest(ctx, tenantID, user.ID, reviewID, decision, ErrSegregationOfDuties)
		}
		return Review{}, ErrSegregationOfDuties
	}

	now := e.now()
	rv.Decision = decision
	rv.AttestedBy = user.ID
	rv.AttestedAt = &now
	rv.Notes = notes

	if err := e.reviews.Update(ctx, tenantID, rv); err != nil {
		wrapped := wrapf("Attest", err)
		if e.audit != nil {
			_, _ = e.audit.RecordAttest(ctx, tenantID, user.ID, reviewID, decision, wrapped)
		}
		return Review{}, wrapped
	}

	if decision == AttestationRevoke {
		if revokeErr := e.revokeSubject(ctx, tenantID, rv.SubjectKind, rv.SubjectID, now); revokeErr != nil {
			wrapped := wrapf("Attest", revokeErr)
			if e.audit != nil {
				_, _ = e.audit.RecordAttest(ctx, tenantID, user.ID, reviewID, decision, wrapped)
			}
			return Review{}, wrapped
		}
	}

	if e.audit != nil {
		_, _ = e.audit.RecordAttest(ctx, tenantID, user.ID, reviewID, decision, nil)
	}
	return *rv, nil
}

func (e *Engine) revokeSubject(ctx context.Context, tenantID uuid.UUID, kind GrantKind, subjectID uuid.UUID, now time.Time) error {
	switch kind {
	case GrantKindCase:
		return e.grants.Revoke(ctx, tenantID, subjectID, now)
	case GrantKindElevation:
		return e.elevate.Revoke(ctx, tenantID, subjectID, now)
	default:
		return ErrInvalidGrant
	}
}
