package issue_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/issue"
)

func TestDecompose_SplitsCompoundIssue(t *testing.T) {
	parent := issue.CandidateIssue{
		Text:       "whether the contract was breached and whether damages are owed to the plaintiff",
		Confidence: 0.7,
	}

	got := issue.Decompose(parent, "parent-1")

	if len(got) != 3 {
		t.Fatalf("expected parent + 2 sub-issues, got %d: %+v", len(got), got)
	}
	if got[0].ID != "parent-1" || got[0].ParentIssueID != nil {
		t.Errorf("expected first element to be the unmodified parent, got %+v", got[0])
	}
	for _, sub := range got[1:] {
		if sub.ParentIssueID == nil || *sub.ParentIssueID != "parent-1" {
			t.Errorf("expected sub-issue to link back to parent-1, got %+v", sub)
		}
	}
}

func TestDecompose_LeavesSimpleIssueUnsplit(t *testing.T) {
	parent := issue.CandidateIssue{
		Text:       "whether the contract was breached",
		Confidence: 0.7,
	}

	got := issue.Decompose(parent, "parent-2")

	if len(got) != 1 {
		t.Fatalf("expected a simple issue to remain a single candidate, got %d: %+v", len(got), got)
	}
	if got[0].ParentIssueID != nil {
		t.Errorf("expected no ParentIssueID on a non-decomposed issue")
	}
}

func TestDecompose_ShortFragmentNotSplit(t *testing.T) {
	parent := issue.CandidateIssue{
		Text:       "whether the tenant paid the deposit in full and fees",
		Confidence: 0.7,
	}

	got := issue.Decompose(parent, "parent-3")

	if len(got) != 1 {
		t.Fatalf("expected short trailing fragment ('fees') to prevent decomposition, got %d: %+v", len(got), got)
	}
}

func TestDecompose_PreservesSourceSpansAndConfidence(t *testing.T) {
	parent := issue.CandidateIssue{
		Text:       "whether the lease was terminated and whether rent remains outstanding",
		Confidence: 0.65,
	}

	got := issue.Decompose(parent, "parent-4")

	if len(got) < 2 {
		t.Fatalf("expected decomposition, got %d elements", len(got))
	}
	for _, sub := range got[1:] {
		if sub.Confidence != parent.Confidence {
			t.Errorf("expected sub-issue confidence to match parent, got %v want %v", sub.Confidence, parent.Confidence)
		}
	}
}
