package signoff_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/guardrail"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/signoff"
)

func TestService_Approve_RequiresExplicitAcknowledgement(t *testing.T) {
	tenantID := uuid.New()
	caseID := uuid.New()
	svc, _, _ := newTestService(caseID)
	judge := newTestUser(tenantID, identity.RoleJudge)
	ctx := ctxWithUser(judge)

	_, err := svc.Approve(ctx, signoff.DecisionInput{
		TenantID:    tenantID,
		CaseID:      caseID,
		CaseVersion: 1,
		// Acknowledgement deliberately omitted.
	})
	if !errors.Is(err, signoff.ErrAcknowledgementRequired) {
		t.Fatalf("expected ErrAcknowledgementRequired, got %v", err)
	}

	_, err = svc.Approve(ctx, signoff.DecisionInput{
		TenantID:        tenantID,
		CaseID:          caseID,
		CaseVersion:     1,
		Acknowledgement: "yes",
	})
	if !errors.Is(err, signoff.ErrAcknowledgementRequired) {
		t.Fatalf("expected ErrAcknowledgementRequired for wrong string, got %v", err)
	}
}

func TestService_Approve_SucceedsWithAcknowledgement(t *testing.T) {
	tenantID := uuid.New()
	caseID := uuid.New()
	svc, _, _ := newTestService(caseID)
	judge := newTestUser(tenantID, identity.RoleJudge)
	ctx := ctxWithUser(judge)

	rec, err := svc.Approve(ctx, signoff.DecisionInput{
		TenantID:        tenantID,
		CaseID:          caseID,
		CaseVersion:     1,
		Acknowledgement: signoff.AcknowledgementConfirmation,
	})
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if rec.Status != guardrail.SignoffApproved {
		t.Fatalf("Status = %v, want Approved", rec.Status)
	}
	if rec.ReviewerID != judge.ID {
		t.Fatalf("ReviewerID = %v, want %v", rec.ReviewerID, judge.ID)
	}
	if rec.DecidedAt.IsZero() {
		t.Fatal("expected DecidedAt to be set")
	}
}

func TestService_Approve_RequiresSignoffPermission(t *testing.T) {
	tenantID := uuid.New()
	caseID := uuid.New()
	svc, _, _ := newTestService(caseID)
	// Advocate does not hold identity.PermSignOff.
	advocate := newTestUser(tenantID, identity.RoleAdvocate)
	ctx := ctxWithUser(advocate)

	_, err := svc.Approve(ctx, signoff.DecisionInput{
		TenantID:        tenantID,
		CaseID:          caseID,
		CaseVersion:     1,
		Acknowledgement: signoff.AcknowledgementConfirmation,
	})
	if !errors.Is(err, signoff.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestService_Approve_RequiresAuthenticatedActor(t *testing.T) {
	tenantID := uuid.New()
	caseID := uuid.New()
	svc, _, _ := newTestService(caseID)

	_, err := svc.Approve(context.Background(), signoff.DecisionInput{
		TenantID:        tenantID,
		CaseID:          caseID,
		CaseVersion:     1,
		Acknowledgement: signoff.AcknowledgementConfirmation,
	})
	if !errors.Is(err, signoff.ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestService_Approve_RequiresMatchingCaseVersion(t *testing.T) {
	tenantID := uuid.New()
	caseID := uuid.New()
	svc, reader, _ := newTestService(caseID)
	reader.set(caseID, 3)
	judge := newTestUser(tenantID, identity.RoleJudge)
	ctx := ctxWithUser(judge)

	_, err := svc.Approve(ctx, signoff.DecisionInput{
		TenantID:        tenantID,
		CaseID:          caseID,
		CaseVersion:     1, // stale
		Acknowledgement: signoff.AcknowledgementConfirmation,
	})
	if !errors.Is(err, signoff.ErrCaseVersionMismatch) {
		t.Fatalf("expected ErrCaseVersionMismatch, got %v", err)
	}
}

func TestService_Approve_NotesOptional(t *testing.T) {
	tenantID := uuid.New()
	caseID := uuid.New()
	svc, _, _ := newTestService(caseID)
	judge := newTestUser(tenantID, identity.RoleJudge)
	ctx := ctxWithUser(judge)

	rec, err := svc.Approve(ctx, signoff.DecisionInput{
		TenantID:        tenantID,
		CaseID:          caseID,
		CaseVersion:     1,
		Acknowledgement: signoff.AcknowledgementConfirmation,
	})
	if err != nil {
		t.Fatalf("Approve without notes: %v", err)
	}
	if rec.Notes != "" {
		t.Fatalf("expected empty Notes, got %q", rec.Notes)
	}
}

func TestService_Reject_RequiresNotes(t *testing.T) {
	tenantID := uuid.New()
	caseID := uuid.New()
	svc, _, _ := newTestService(caseID)
	judge := newTestUser(tenantID, identity.RoleJudge)
	ctx := ctxWithUser(judge)

	_, err := svc.Reject(ctx, signoff.DecisionInput{
		TenantID:        tenantID,
		CaseID:          caseID,
		CaseVersion:     1,
		Acknowledgement: signoff.AcknowledgementConfirmation,
		Notes:           "   ", // blank after trim
	})
	if !errors.Is(err, signoff.ErrNotesRequired) {
		t.Fatalf("expected ErrNotesRequired, got %v", err)
	}
}

func TestService_Reject_SucceedsWithNotes(t *testing.T) {
	tenantID := uuid.New()
	caseID := uuid.New()
	svc, _, _ := newTestService(caseID)
	judge := newTestUser(tenantID, identity.RoleJudge)
	ctx := ctxWithUser(judge)

	rec, err := svc.Reject(ctx, signoff.DecisionInput{
		TenantID:        tenantID,
		CaseID:          caseID,
		CaseVersion:     1,
		Acknowledgement: signoff.AcknowledgementConfirmation,
		Notes:           "Evidence chain is incomplete for exhibit 4.",
	})
	if err != nil {
		t.Fatalf("Reject: %v", err)
	}
	if rec.Status != guardrail.SignoffRejected {
		t.Fatalf("Status = %v, want Rejected", rec.Status)
	}
	if rec.Notes == "" {
		t.Fatal("expected Notes to be persisted")
	}
}

func TestService_History_RecordsEveryDecision(t *testing.T) {
	tenantID := uuid.New()
	caseID := uuid.New()
	svc, _, _ := newTestService(caseID)
	judge := newTestUser(tenantID, identity.RoleJudge)
	ctx := ctxWithUser(judge)

	if _, err := svc.Reject(ctx, signoff.DecisionInput{
		TenantID:        tenantID,
		CaseID:          caseID,
		CaseVersion:     1,
		Acknowledgement: signoff.AcknowledgementConfirmation,
		Notes:           "not ready",
	}); err != nil {
		t.Fatalf("Reject: %v", err)
	}
	if _, err := svc.Approve(ctx, signoff.DecisionInput{
		TenantID:        tenantID,
		CaseID:          caseID,
		CaseVersion:     1,
		Acknowledgement: signoff.AcknowledgementConfirmation,
	}); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	history, err := svc.History(ctx, tenantID, caseID)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 audit entries, got %d", len(history))
	}
	if history[0].ToStatus != guardrail.SignoffRejected {
		t.Fatalf("history[0].ToStatus = %v, want Rejected", history[0].ToStatus)
	}
	if history[1].ToStatus != guardrail.SignoffApproved {
		t.Fatalf("history[1].ToStatus = %v, want Approved", history[1].ToStatus)
	}
}

func TestService_Get_ReturnsSyntheticPendingWhenNeverReviewed(t *testing.T) {
	tenantID := uuid.New()
	caseID := uuid.New()
	svc, _, _ := newTestService(caseID)
	judge := newTestUser(tenantID, identity.RoleJudge)
	ctx := ctxWithUser(judge)

	rec, err := svc.Get(ctx, tenantID, caseID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if rec.Status != guardrail.SignoffPending {
		t.Fatalf("Status = %v, want Pending", rec.Status)
	}
}
