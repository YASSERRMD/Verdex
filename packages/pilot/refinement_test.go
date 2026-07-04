package pilot_test

import (
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/pilot"
)

// setupTriagedFinding provisions an active deployment, assigns a case,
// submits feedback, records a finding, and triages it to
// FindingStatusTriaged -- the precondition every RecordRefinement test
// below needs met before it can exercise the interesting behavior.
func setupTriagedFinding(t *testing.T, te *testEngine) pilot.PilotFinding {
	t.Helper()
	d := provisionAndActivate(t, te)
	pc := assignTestCase(t, te, d.ID)
	entry := submitTestFeedback(t, te, pc.ID)
	f := recordTestFinding(t, te, d.ID, entry.ID)
	admin := adminUser(te.tenantID)

	triaged, err := te.engine.TriageFinding(ctxWithUser(admin), te.tenantID, f.ID, pilot.PriorityHigh, pilot.FindingStatusTriaged, "Reviewed and prioritized.")
	if err != nil {
		t.Fatalf("TriageFinding: %v", err)
	}
	return triaged
}

func TestEngine_RecordRefinement_RejectsUntriaggedFinding(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	d := provisionAndActivate(t, te)
	pc := assignTestCase(t, te, d.ID)
	entry := submitTestFeedback(t, te, pc.ID)
	f := recordTestFinding(t, te, d.ID, entry.ID) // still FindingStatusOpen
	admin := adminUser(te.tenantID)

	_, err := te.engine.RecordRefinement(ctxWithUser(admin), te.tenantID, pilot.RefinementRecord{
		FindingID:   f.ID,
		Description: "Should be rejected -- finding was never triaged.",
	})
	if !errors.Is(err, pilot.ErrFindingNotTriaged) {
		t.Fatalf("error = %v, want ErrFindingNotTriaged", err)
	}
}

func TestEngine_RecordRefinement_SucceedsOnTriagedFinding(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	triaged := setupTriagedFinding(t, te)
	admin := adminUser(te.tenantID)

	r, err := te.engine.RecordRefinement(ctxWithUser(admin), te.tenantID, pilot.RefinementRecord{
		FindingID:   triaged.ID,
		Description: "Tightened the grounding threshold for commercial-contract issues.",
	})
	if err != nil {
		t.Fatalf("RecordRefinement: %v", err)
	}
	if r.VerifiedFixed {
		t.Fatal("a freshly recorded refinement should never start VerifiedFixed")
	}

	// Recording a refinement against a Triaged finding should move it
	// to InProgress: real state-machine coupling, not a bare record
	// with no effect on the finding itself.
	updated, err := te.engine.GetFinding(ctxWithUser(admin), te.tenantID, triaged.ID)
	if err != nil {
		t.Fatalf("GetFinding: %v", err)
	}
	if updated.Status != pilot.FindingStatusInProgress {
		t.Fatalf("finding Status = %q, want %q after RecordRefinement", updated.Status, pilot.FindingStatusInProgress)
	}
}

func TestEngine_RecordRefinement_CannotPreVerify(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	triaged := setupTriagedFinding(t, te)
	admin := adminUser(te.tenantID)

	// A caller attempting to construct an already-verified record
	// should have that stripped -- verification only happens through
	// VerifyRefinement.
	r, err := te.engine.RecordRefinement(ctxWithUser(admin), te.tenantID, pilot.RefinementRecord{
		FindingID:        triaged.ID,
		Description:      "Attempting to skip verification.",
		VerifiedFixed:    true,
		VerificationNote: "should be stripped",
	})
	if err != nil {
		t.Fatalf("RecordRefinement: %v", err)
	}
	if r.VerifiedFixed || r.VerificationNote != "" {
		t.Fatalf("RecordRefinement should strip caller-supplied verification fields, got VerifiedFixed=%v VerificationNote=%q", r.VerifiedFixed, r.VerificationNote)
	}
}

func TestEngine_VerifyRefinement_RequiresNonBlankNote(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	triaged := setupTriagedFinding(t, te)
	admin := adminUser(te.tenantID)

	r, err := te.engine.RecordRefinement(ctxWithUser(admin), te.tenantID, pilot.RefinementRecord{
		FindingID:   triaged.ID,
		Description: "A refinement to verify.",
	})
	if err != nil {
		t.Fatalf("RecordRefinement: %v", err)
	}

	_, err = te.engine.VerifyRefinement(ctxWithUser(admin), te.tenantID, r.ID, "")
	if !errors.Is(err, pilot.ErrNotesRequired) {
		t.Fatalf("error = %v, want ErrNotesRequired", err)
	}
}

func TestEngine_VerifyRefinement_ResolvesTheFinding(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	triaged := setupTriagedFinding(t, te)
	admin := adminUser(te.tenantID)

	r, err := te.engine.RecordRefinement(ctxWithUser(admin), te.tenantID, pilot.RefinementRecord{
		FindingID:   triaged.ID,
		Description: "Tightened the grounding threshold.",
	})
	if err != nil {
		t.Fatalf("RecordRefinement: %v", err)
	}

	verified, err := te.engine.VerifyRefinement(ctxWithUser(admin), te.tenantID, r.ID, "Re-ran three fresh pilot cases with no recurrence.")
	if err != nil {
		t.Fatalf("VerifyRefinement: %v", err)
	}
	if !verified.VerifiedFixed {
		t.Fatal("VerifiedFixed should be true after VerifyRefinement")
	}
	if verified.VerifiedBy == nil || *verified.VerifiedBy != admin.ID {
		t.Fatalf("VerifiedBy = %v, want %v", verified.VerifiedBy, admin.ID)
	}
	if verified.VerifiedAt == nil {
		t.Fatal("VerifiedAt should be set")
	}

	finding, err := te.engine.GetFinding(ctxWithUser(admin), te.tenantID, triaged.ID)
	if err != nil {
		t.Fatalf("GetFinding: %v", err)
	}
	if finding.Status != pilot.FindingStatusResolved {
		t.Fatalf("finding Status = %q, want %q after VerifyRefinement", finding.Status, pilot.FindingStatusResolved)
	}
}
