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

// TestGateImpl_CanFinalize_EndToEnd proves, using the real
// guardrail.CanFinalize function (not a mock of it), that:
//  1. a case with no sign-off record at all is blocked;
//  2. a case explicitly Rejected is blocked;
//  3. a case is only allowed to finalize once Approve has actually
//     been called through this package's Service.
func TestGateImpl_CanFinalize_EndToEnd(t *testing.T) {
	tenantID := uuid.New()
	caseID := uuid.New()
	ctx := context.Background()

	repo := signoff.NewInMemoryRepository()
	reader := newFakeCaseVersionReader()
	reader.set(caseID, 1)
	svc, err := signoff.NewService(repo, reader, nil)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	gate, err := signoff.NewGate(repo, tenantID)
	if err != nil {
		t.Fatalf("NewGate: %v", err)
	}

	// 1. No record at all: blocked.
	ok, err := guardrail.CanFinalize(ctx, caseID.String(), gate)
	if ok {
		t.Fatal("expected CanFinalize to block with no sign-off record")
	}
	if !errors.Is(err, guardrail.ErrSignoffNotApproved) {
		t.Fatalf("expected ErrSignoffNotApproved, got %v", err)
	}

	// 2. Explicitly rejected: still blocked.
	judge := newTestUser(tenantID, identity.RoleJudge)
	reviewCtx := ctxWithUser(judge)
	if _, err := svc.Reject(reviewCtx, signoff.DecisionInput{
		TenantID:        tenantID,
		CaseID:          caseID,
		CaseVersion:     1,
		Acknowledgement: signoff.AcknowledgementConfirmation,
		Notes:           "insufficient grounding on issue 2",
	}); err != nil {
		t.Fatalf("Reject: %v", err)
	}

	ok, err = guardrail.CanFinalize(ctx, caseID.String(), gate)
	if ok {
		t.Fatal("expected CanFinalize to block after Reject")
	}
	if !errors.Is(err, guardrail.ErrSignoffNotApproved) {
		t.Fatalf("expected ErrSignoffNotApproved after Reject, got %v", err)
	}

	// 3. Approved: allowed.
	if _, err := svc.Approve(reviewCtx, signoff.DecisionInput{
		TenantID:        tenantID,
		CaseID:          caseID,
		CaseVersion:     1,
		Acknowledgement: signoff.AcknowledgementConfirmation,
	}); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	ok, err = guardrail.CanFinalize(ctx, caseID.String(), gate)
	if err != nil {
		t.Fatalf("CanFinalize after Approve: %v", err)
	}
	if !ok {
		t.Fatal("expected CanFinalize to allow after Approve")
	}
}

// TestGateImpl_MatchesNoSignoffRecordedGate_ForUnknownCase proves
// GateImpl reports the same fail-closed SignoffPending status as
// guardrail.NoSignoffRecordedGate for a case with no record, so
// swapping one for the other never changes behavior for
// never-reviewed cases.
func TestGateImpl_MatchesNoSignoffRecordedGate_ForUnknownCase(t *testing.T) {
	tenantID := uuid.New()
	caseID := uuid.New()
	ctx := context.Background()

	repo := signoff.NewInMemoryRepository()
	gate, err := signoff.NewGate(repo, tenantID)
	if err != nil {
		t.Fatalf("NewGate: %v", err)
	}

	gotStatus, err := gate.Status(ctx, caseID.String())
	if err != nil {
		t.Fatalf("GateImpl.Status: %v", err)
	}

	var fallback guardrail.NoSignoffRecordedGate
	wantStatus, err := fallback.Status(ctx, caseID.String())
	if err != nil {
		t.Fatalf("NoSignoffRecordedGate.Status: %v", err)
	}

	if gotStatus != wantStatus {
		t.Fatalf("GateImpl.Status = %v, want %v (matching NoSignoffRecordedGate)", gotStatus, wantStatus)
	}
}
