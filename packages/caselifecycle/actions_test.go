package caselifecycle_test

import (
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/caselifecycle"
)

func TestCanPerform_DraftAndActivePermitIngestion(t *testing.T) {
	if !caselifecycle.CanPerform(caselifecycle.StateDraft, caselifecycle.ActionIngestEvidence) {
		t.Error("expected ActionIngestEvidence permitted in StateDraft")
	}
	if !caselifecycle.CanPerform(caselifecycle.StateActive, caselifecycle.ActionIngestEvidence) {
		t.Error("expected ActionIngestEvidence permitted in StateActive")
	}
}

func TestCanPerform_ClosedAndArchivedForbidAllActions(t *testing.T) {
	allActions := []caselifecycle.Action{
		caselifecycle.ActionIngestEvidence,
		caselifecycle.ActionEditCategory,
		caselifecycle.ActionEditTimeline,
		caselifecycle.ActionGenerateReasoning,
		caselifecycle.ActionReviewOpinion,
		caselifecycle.ActionEditMetadata,
	}
	for _, state := range []caselifecycle.State{caselifecycle.StateClosed, caselifecycle.StateArchived} {
		for _, action := range allActions {
			if caselifecycle.CanPerform(state, action) {
				t.Errorf("CanPerform(%s, %s) = true, want false", state, action)
			}
		}
	}
}

func TestCanPerform_UnderReviewFreezesEditsButAllowsReview(t *testing.T) {
	if caselifecycle.CanPerform(caselifecycle.StateUnderReview, caselifecycle.ActionIngestEvidence) {
		t.Error("expected ActionIngestEvidence forbidden in StateUnderReview")
	}
	if caselifecycle.CanPerform(caselifecycle.StateUnderReview, caselifecycle.ActionEditTimeline) {
		t.Error("expected ActionEditTimeline forbidden in StateUnderReview")
	}
	if !caselifecycle.CanPerform(caselifecycle.StateUnderReview, caselifecycle.ActionReviewOpinion) {
		t.Error("expected ActionReviewOpinion permitted in StateUnderReview")
	}
}

func TestRequireAction_ReturnsErrActionNotPermitted(t *testing.T) {
	err := caselifecycle.RequireAction(caselifecycle.StateClosed, caselifecycle.ActionIngestEvidence)
	if !errors.Is(err, caselifecycle.ErrActionNotPermitted) {
		t.Fatalf("expected ErrActionNotPermitted, got %v", err)
	}

	if err := caselifecycle.RequireAction(caselifecycle.StateActive, caselifecycle.ActionIngestEvidence); err != nil {
		t.Fatalf("expected nil error for a permitted action, got %v", err)
	}
}

func TestPermittedActions_ReturnsDeterministicOrderAndEmptyForTerminalStates(t *testing.T) {
	draftActions := caselifecycle.PermittedActions(caselifecycle.StateDraft)
	if len(draftActions) == 0 {
		t.Fatal("expected StateDraft to permit at least one action")
	}
	// calling twice should be identical (deterministic ordering, not map order)
	again := caselifecycle.PermittedActions(caselifecycle.StateDraft)
	if len(draftActions) != len(again) {
		t.Fatalf("expected stable length across calls, got %d then %d", len(draftActions), len(again))
	}
	for i := range draftActions {
		if draftActions[i] != again[i] {
			t.Fatalf("expected stable order across calls, got %v then %v", draftActions, again)
		}
	}

	archivedActions := caselifecycle.PermittedActions(caselifecycle.StateArchived)
	if len(archivedActions) != 0 {
		t.Errorf("expected 0 permitted actions for StateArchived, got %v", archivedActions)
	}
}
