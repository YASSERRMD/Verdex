package accessgovernance_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/accessgovernance"
	"github.com/YASSERRMD/verdex/packages/identity"
)

// TestEngine_ReviewAttestWorkflow_RoundTrips proves the full review
// workflow (task 4): schedule a review for a CaseGrant, list it as
// due, and attest it -- Approve leaves the grant active.
func TestEngine_ReviewAttestWorkflow_RoundTrips(t *testing.T) {
	engine, tenantID := newTestEngine(t)
	admin := newTestUser(tenantID, identity.RoleAdmin)
	auditor := newTestUser(tenantID, identity.RoleAuditor)
	reviewer := newTestUser(tenantID)
	caseID := uuid.New()

	grant, err := engine.GrantCaseAccess(ctxWithUser(admin), tenantID, accessgovernance.CaseGrant{
		CaseID:        caseID,
		GranteeUserID: reviewer.ID,
		Permissions:   []identity.Permission{identity.PermViewCase},
		ExpiresAt:     time.Now().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("GrantCaseAccess: %v", err)
	}

	rv, err := engine.ScheduleReview(ctxWithUser(admin), tenantID, accessgovernance.GrantKindCase, grant.ID, reviewer.ID, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("ScheduleReview: %v", err)
	}
	if !rv.IsPending() {
		t.Fatal("newly scheduled review should be pending")
	}

	due, err := engine.ListDueReviews(ctxWithUser(auditor), tenantID, time.Now())
	if err != nil {
		t.Fatalf("ListDueReviews: %v", err)
	}
	if len(due) != 1 || due[0].ID != rv.ID {
		t.Fatalf("ListDueReviews() = %v, want exactly the scheduled review", due)
	}

	attested, err := engine.Attest(ctxWithUser(auditor), tenantID, rv.ID, accessgovernance.AttestationApprove, "looks fine")
	if err != nil {
		t.Fatalf("Attest: %v", err)
	}
	if attested.IsPending() {
		t.Fatal("attested review should no longer be pending")
	}
	if attested.AttestedBy != auditor.ID {
		t.Fatalf("attested.AttestedBy = %v, want %v", attested.AttestedBy, auditor.ID)
	}

	// Approve must not revoke the underlying grant.
	dec, err := engine.Evaluate(ctxWithUser(reviewer), accessgovernance.Request{
		ActorUserID: reviewer.ID,
		TenantID:    tenantID,
		CaseID:      caseID,
		Action:      "case:view",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !dec.Allowed() {
		t.Fatal("Evaluate() after Approve attestation should still allow")
	}
}

// TestEngine_Attest_RevokeDecisionRevokesUnderlyingGrant proves
// AttestationRevoke immediately revokes the subject grant.
func TestEngine_Attest_RevokeDecisionRevokesUnderlyingGrant(t *testing.T) {
	engine, tenantID := newTestEngine(t)
	admin := newTestUser(tenantID, identity.RoleAdmin)
	auditor := newTestUser(tenantID, identity.RoleAuditor)
	reviewer := newTestUser(tenantID)
	caseID := uuid.New()

	grant, err := engine.GrantCaseAccess(ctxWithUser(admin), tenantID, accessgovernance.CaseGrant{
		CaseID:        caseID,
		GranteeUserID: reviewer.ID,
		Permissions:   []identity.Permission{identity.PermViewCase},
		ExpiresAt:     time.Now().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("GrantCaseAccess: %v", err)
	}
	rv, err := engine.ScheduleReview(ctxWithUser(admin), tenantID, accessgovernance.GrantKindCase, grant.ID, reviewer.ID, time.Now())
	if err != nil {
		t.Fatalf("ScheduleReview: %v", err)
	}

	if _, err := engine.Attest(ctxWithUser(auditor), tenantID, rv.ID, accessgovernance.AttestationRevoke, "no longer needed"); err != nil {
		t.Fatalf("Attest: %v", err)
	}

	dec, err := engine.Evaluate(ctxWithUser(reviewer), accessgovernance.Request{
		ActorUserID: reviewer.ID,
		TenantID:    tenantID,
		CaseID:      caseID,
		Action:      "case:view",
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if dec.Allowed() {
		t.Fatal("Evaluate() after Revoke attestation should deny")
	}
}

// TestEngine_Attest_SegregationOfDuties_RejectsSelfApproval proves
// task 5: the actor who requested/was granted access cannot also
// attest the review covering it.
func TestEngine_Attest_SegregationOfDuties_RejectsSelfApproval(t *testing.T) {
	engine, tenantID := newTestEngine(t)
	admin := newTestUser(tenantID, identity.RoleAdmin)
	reviewer := newTestUser(tenantID, identity.RoleAuditor) // holds reviewPermission
	caseID := uuid.New()

	grant, err := engine.GrantCaseAccess(ctxWithUser(admin), tenantID, accessgovernance.CaseGrant{
		CaseID:        caseID,
		GranteeUserID: reviewer.ID,
		Permissions:   []identity.Permission{identity.PermViewCase},
		ExpiresAt:     time.Now().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("GrantCaseAccess: %v", err)
	}
	rv, err := engine.ScheduleReview(ctxWithUser(admin), tenantID, accessgovernance.GrantKindCase, grant.ID, reviewer.ID, time.Now())
	if err != nil {
		t.Fatalf("ScheduleReview: %v", err)
	}

	// reviewer is both the grant's requested-by and the attester --
	// must be rejected.
	_, err = engine.Attest(ctxWithUser(reviewer), tenantID, rv.ID, accessgovernance.AttestationApprove, "self-approving")
	if !errors.Is(err, accessgovernance.ErrSegregationOfDuties) {
		t.Fatalf("Attest() self-approval error = %v, want ErrSegregationOfDuties", err)
	}
}

// TestEngine_Attest_AlreadyDecidedRejected proves a Review can be
// attested exactly once.
func TestEngine_Attest_AlreadyDecidedRejected(t *testing.T) {
	engine, tenantID := newTestEngine(t)
	admin := newTestUser(tenantID, identity.RoleAdmin)
	auditor := newTestUser(tenantID, identity.RoleAuditor)
	reviewer := newTestUser(tenantID)
	caseID := uuid.New()

	grant, err := engine.GrantCaseAccess(ctxWithUser(admin), tenantID, accessgovernance.CaseGrant{
		CaseID:        caseID,
		GranteeUserID: reviewer.ID,
		Permissions:   []identity.Permission{identity.PermViewCase},
		ExpiresAt:     time.Now().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("GrantCaseAccess: %v", err)
	}
	rv, err := engine.ScheduleReview(ctxWithUser(admin), tenantID, accessgovernance.GrantKindCase, grant.ID, reviewer.ID, time.Now())
	if err != nil {
		t.Fatalf("ScheduleReview: %v", err)
	}

	if _, err := engine.Attest(ctxWithUser(auditor), tenantID, rv.ID, accessgovernance.AttestationApprove, "first pass"); err != nil {
		t.Fatalf("Attest (first): %v", err)
	}
	if _, err := engine.Attest(ctxWithUser(auditor), tenantID, rv.ID, accessgovernance.AttestationApprove, "second pass"); !errors.Is(err, accessgovernance.ErrReviewAlreadyDecided) {
		t.Fatalf("Attest (second) error = %v, want ErrReviewAlreadyDecided", err)
	}
}

// TestCheckConflict_ApproverNotSoleAuthor proves the second built-in
// segregation-of-duties rule (task 5): a case's sole author cannot be
// the approving reviewer.
func TestCheckConflict_ApproverNotSoleAuthor(t *testing.T) {
	author := uuid.New()

	violated := accessgovernance.CheckConflict(accessgovernance.ConflictCheck{
		RequestedBy:  author,
		ActingUserID: author,
		SoleAuthor:   true,
	}, nil)
	if violated != accessgovernance.RuleRequesterCannotApprove {
		t.Fatalf("CheckConflict() = %q, want %q (requester rule fires first)", violated, accessgovernance.RuleRequesterCannotApprove)
	}

	// A different acting user is never in conflict.
	violated = accessgovernance.CheckConflict(accessgovernance.ConflictCheck{
		RequestedBy:  author,
		ActingUserID: uuid.New(),
		SoleAuthor:   true,
	}, nil)
	if violated != "" {
		t.Fatalf("CheckConflict() for a distinct approver = %q, want no violation", violated)
	}
}
