package signoff_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/guardrail"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/signoff"
)

func TestService_ReReviewOnCaseUpdate_RevertsApprovedToPending(t *testing.T) {
	tenantID := uuid.New()
	caseID := uuid.New()
	svc, reader, notifier := newTestService(caseID)
	judge := newTestUser(tenantID, identity.RoleJudge)
	ctx := ctxWithUser(judge)

	if _, err := svc.Approve(ctx, signoff.DecisionInput{
		TenantID:        tenantID,
		CaseID:          caseID,
		CaseVersion:     1,
		Acknowledgement: signoff.AcknowledgementConfirmation,
	}); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	notifiedBeforeUpdate := notifier.count()

	// Simulate the case's content changing after approval (e.g. new
	// evidence ingested, metadata bumped).
	reader.set(caseID, 2)

	rec, reverted, err := svc.ReReviewOnCaseUpdate(ctx, tenantID, caseID)
	if err != nil {
		t.Fatalf("ReReviewOnCaseUpdate: %v", err)
	}
	if !reverted {
		t.Fatal("expected reverted = true")
	}
	if rec.Status != guardrail.SignoffPending {
		t.Fatalf("Status = %v, want Pending after re-review", rec.Status)
	}
	if rec.CaseVersion != 2 {
		t.Fatalf("CaseVersion = %d, want 2", rec.CaseVersion)
	}

	// An audit entry must explain the reversion.
	history, err := svc.History(ctx, tenantID, caseID)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	last := history[len(history)-1]
	if last.Source != signoff.DecisionSourceReReview {
		t.Fatalf("last entry Source = %v, want DecisionSourceReReview", last.Source)
	}
	if last.ToStatus != guardrail.SignoffPending || last.FromStatus != guardrail.SignoffApproved {
		t.Fatalf("last entry from/to = %v/%v, want Approved/Pending", last.FromStatus, last.ToStatus)
	}
	if last.Notes == "" {
		t.Fatal("expected re-review audit entry to explain why")
	}

	// A notification must have fired for the reversion.
	if notifier.count() <= notifiedBeforeUpdate {
		t.Fatal("expected a notification to fire on re-review reversion")
	}
}

func TestService_ReReviewOnCaseUpdate_NoOpWhenVersionUnchanged(t *testing.T) {
	tenantID := uuid.New()
	caseID := uuid.New()
	svc, _, _ := newTestService(caseID)
	judge := newTestUser(tenantID, identity.RoleJudge)
	ctx := ctxWithUser(judge)

	if _, err := svc.Approve(ctx, signoff.DecisionInput{
		TenantID:        tenantID,
		CaseID:          caseID,
		CaseVersion:     1,
		Acknowledgement: signoff.AcknowledgementConfirmation,
	}); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	rec, reverted, err := svc.ReReviewOnCaseUpdate(ctx, tenantID, caseID)
	if err != nil {
		t.Fatalf("ReReviewOnCaseUpdate: %v", err)
	}
	if reverted {
		t.Fatal("expected reverted = false when version unchanged")
	}
	if rec.Status != guardrail.SignoffApproved {
		t.Fatalf("Status = %v, want still Approved", rec.Status)
	}
}

func TestService_ReReviewOnCaseUpdate_NoOpWhenNeverReviewed(t *testing.T) {
	tenantID := uuid.New()
	caseID := uuid.New()
	svc, _, _ := newTestService(caseID)

	rec, reverted, err := svc.ReReviewOnCaseUpdate(context.Background(), tenantID, caseID)
	if err != nil {
		t.Fatalf("ReReviewOnCaseUpdate: %v", err)
	}
	if reverted {
		t.Fatal("expected reverted = false when no record exists yet")
	}
	if rec != nil {
		t.Fatalf("expected nil record, got %+v", rec)
	}
}

func TestService_ReReviewOnCaseUpdate_RejectedStaysRejected(t *testing.T) {
	tenantID := uuid.New()
	caseID := uuid.New()
	svc, reader, _ := newTestService(caseID)
	judge := newTestUser(tenantID, identity.RoleJudge)
	ctx := ctxWithUser(judge)

	if _, err := svc.Reject(ctx, signoff.DecisionInput{
		TenantID:        tenantID,
		CaseID:          caseID,
		CaseVersion:     1,
		Acknowledgement: signoff.AcknowledgementConfirmation,
		Notes:           "needs more work",
	}); err != nil {
		t.Fatalf("Reject: %v", err)
	}

	reader.set(caseID, 2)
	rec, reverted, err := svc.ReReviewOnCaseUpdate(ctx, tenantID, caseID)
	if err != nil {
		t.Fatalf("ReReviewOnCaseUpdate: %v", err)
	}
	if reverted {
		t.Fatal("expected reverted = false: only Approved records revert")
	}
	if rec.Status != guardrail.SignoffRejected {
		t.Fatalf("Status = %v, want still Rejected", rec.Status)
	}
}
