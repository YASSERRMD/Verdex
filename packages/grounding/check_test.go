package grounding_test

import (
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/grounding"
	"github.com/YASSERRMD/verdex/packages/synthesisagent"
)

func TestCheck_FullyGroundedOpinionPassesClean(t *testing.T) {
	store := seededStore(t)
	opinion := groundedOpinion()

	report, err := grounding.Check(authedContext(), testCaseID, store, opinion)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	if report.HasCritical() {
		t.Fatalf("expected no critical findings, got: %s", report.Summary())
	}
	if len(report.Conclusions) != 1 {
		t.Fatalf("expected 1 conclusion result, got %d", len(report.Conclusions))
	}
	if report.OpinionScore != 1.0 {
		t.Fatalf("expected OpinionScore 1.0 for a fully grounded opinion, got %f", report.OpinionScore)
	}
	if report.Conclusions[0].ConfidenceScore != 1.0 {
		t.Fatalf("expected ConfidenceScore 1.0, got %f", report.Conclusions[0].ConfidenceScore)
	}

	ok, err := grounding.CanFinalize(&report)
	if !ok || err != nil {
		t.Fatalf("expected CanFinalize to allow a clean report, got ok=%v err=%v", ok, err)
	}
}

func TestCheck_FabricatedFactOpinionFlagged(t *testing.T) {
	store := seededStore(t)
	opinion := groundedOpinion()
	opinion.Conclusions[0].SupportingFactIDs = []string{"fact-1", "fact-does-not-exist"}

	report, err := grounding.Check(authedContext(), testCaseID, store, opinion)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	if !report.HasCritical() {
		t.Fatalf("expected a critical finding for a fabricated fact reference, got: %s", report.Summary())
	}

	found := false
	for _, f := range report.Findings {
		if f.Code == grounding.CodeFabricatedReference && f.Claim.Value == "fact-does-not-exist" {
			found = true
			if f.Severity != grounding.SeverityCritical {
				t.Fatalf("expected CodeFabricatedReference to be SeverityCritical, got %s", f.Severity)
			}
		}
	}
	if !found {
		t.Fatalf("expected a CodeFabricatedReference finding for fact-does-not-exist, findings: %+v", report.Findings)
	}

	ok, err := grounding.CanFinalize(&report)
	if ok || err == nil {
		t.Fatalf("expected CanFinalize to block on a fabricated reference, got ok=%v err=%v", ok, err)
	}
	if !errors.Is(err, grounding.ErrCriticalFindings) {
		t.Fatalf("expected ErrCriticalFindings, got %v", err)
	}
}

func TestCheck_BrokenCitationOpinionFlagged(t *testing.T) {
	store := seededStore(t)
	opinion := groundedOpinion()
	opinion.Conclusions[0].SupportingRuleIDs = []string{"rule-hallucinated"}

	report, err := grounding.Check(authedContext(), testCaseID, store, opinion)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	if !report.HasCritical() {
		t.Fatalf("expected a critical finding for a hallucinated citation, got: %s", report.Summary())
	}

	citeFindings := report.AllCitationFindings()
	if len(citeFindings) != 1 {
		t.Fatalf("expected 1 citation finding, got %d: %+v", len(citeFindings), citeFindings)
	}
	if citeFindings[0].NodeID != "rule-hallucinated" {
		t.Fatalf("expected finding for rule-hallucinated, got %+v", citeFindings[0])
	}

	ok, err := grounding.CanFinalize(&report)
	if ok || err == nil {
		t.Fatalf("expected CanFinalize to block on a hallucinated citation, got ok=%v err=%v", ok, err)
	}
}

func TestCheck_WrongCaseCitationFlagged(t *testing.T) {
	// rule-1 belongs to testCaseID (see seededStore); checking a different
	// case's opinion that nonetheless cites rule-1 must be flagged as a
	// cross-case citation leak, not treated as if rule-1 does not exist.
	store := seededStore(t)
	ctx := authedContext()

	otherCase := "different-case"
	opinion := synthesisagent.Opinion{
		CaseID: otherCase,
		Conclusions: []synthesisagent.TentativeConclusion{
			{
				IssueNodeID:       "issue-1",
				Text:              "Some other case's conclusion.",
				SupportingRuleIDs: []string{"rule-1"},
			},
		},
	}

	report, err := grounding.Check(ctx, otherCase, store, opinion)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !report.HasCritical() {
		t.Fatalf("expected a critical wrong-case citation finding, got: %s", report.Summary())
	}
}

func TestCheck_NumericMismatchFlagged(t *testing.T) {
	store := seededStore(t)
	opinion := groundedOpinion()
	opinion.Conclusions[0].Text = "The parties signed a written memorandum on 2024-03-15 for $9,999.00, satisfying the writing requirement."

	report, err := grounding.Check(authedContext(), testCaseID, store, opinion)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	if !report.HasCritical() {
		t.Fatalf("expected a critical numeric mismatch finding, got: %s", report.Summary())
	}

	found := false
	for _, f := range report.Findings {
		if f.Code == grounding.CodeNumericMismatch {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a CodeNumericMismatch finding, findings: %+v", report.Findings)
	}
}

func TestCheck_DateMismatchFlagged(t *testing.T) {
	store := seededStore(t)
	opinion := groundedOpinion()
	opinion.Conclusions[0].Text = "The parties signed a written memorandum on 2099-01-01 for $4,500.00, satisfying the writing requirement."

	report, err := grounding.Check(authedContext(), testCaseID, store, opinion)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	if !report.HasCritical() {
		t.Fatalf("expected a critical date mismatch finding, got: %s", report.Summary())
	}

	found := false
	for _, f := range report.Findings {
		if f.Code == grounding.CodeDateMismatch {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a CodeDateMismatch finding, findings: %+v", report.Findings)
	}
}

func TestCheck_UnverifiableWhenNoSupportingFacts(t *testing.T) {
	store := seededStore(t)
	opinion := groundedOpinion()
	opinion.Conclusions[0].SupportingFactIDs = nil

	report, err := grounding.Check(authedContext(), testCaseID, store, opinion)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}

	if report.HasCritical() {
		t.Fatalf("expected no critical finding for unverifiable claims, got: %s", report.Summary())
	}

	found := false
	for _, f := range report.Findings {
		if f.Code == grounding.CodeUnverifiableClaim {
			found = true
			if f.Severity != grounding.SeverityWarning {
				t.Fatalf("expected CodeUnverifiableClaim to be SeverityWarning, got %s", f.Severity)
			}
		}
	}
	if !found {
		t.Fatalf("expected a CodeUnverifiableClaim finding, findings: %+v", report.Findings)
	}

	ok, err := grounding.CanFinalize(&report)
	if !ok || err != nil {
		t.Fatalf("expected CanFinalize to allow (warnings only), got ok=%v err=%v", ok, err)
	}
}

func TestCheck_RequiresAuthentication(t *testing.T) {
	store := seededStore(t)
	opinion := groundedOpinion()

	_, err := grounding.Check(unauthedContext(), testCaseID, store, opinion)
	if !errors.Is(err, grounding.ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestCheck_RejectsEmptyCaseID(t *testing.T) {
	store := seededStore(t)
	_, err := grounding.Check(authedContext(), "", store, groundedOpinion())
	if !errors.Is(err, grounding.ErrEmptyCaseID) {
		t.Fatalf("expected ErrEmptyCaseID, got %v", err)
	}
}

func TestCheck_RejectsNilGraphStore(t *testing.T) {
	_, err := grounding.Check(authedContext(), testCaseID, nil, groundedOpinion())
	if !errors.Is(err, grounding.ErrNilGraphStore) {
		t.Fatalf("expected ErrNilGraphStore, got %v", err)
	}
}

func TestCheck_RejectsMismatchedOpinionCaseID(t *testing.T) {
	store := seededStore(t)
	opinion := groundedOpinion()
	opinion.CaseID = "some-other-case"

	_, err := grounding.Check(authedContext(), testCaseID, store, opinion)
	if !errors.Is(err, grounding.ErrOpinionCaseMismatch) {
		t.Fatalf("expected ErrOpinionCaseMismatch, got %v", err)
	}
}

func TestCanFinalize_RejectsNilReport(t *testing.T) {
	ok, err := grounding.CanFinalize(nil)
	if ok || !errors.Is(err, grounding.ErrNilReport) {
		t.Fatalf("expected ErrNilReport, got ok=%v err=%v", ok, err)
	}
}
