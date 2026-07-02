package issue_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/issue"
)

func TestDedup_MergesGenuineDuplicates(t *testing.T) {
	issues := []issue.CandidateIssue{
		{
			ID:          "a",
			Text:        "whether the contract was breached by late payment of rent",
			SourceSpans: []irac.SourceSpan{{Start: 0, End: 10}},
			Confidence:  0.6,
		},
		{
			ID:          "b",
			Text:        "whether the contract was breached by late payment of the rent",
			SourceSpans: []irac.SourceSpan{{Start: 20, End: 30}},
			Confidence:  0.8,
		},
	}

	merged := issue.Dedup(issues)

	if len(merged) != 1 {
		t.Fatalf("expected 2 near-duplicate issues to merge into 1, got %d: %+v", len(merged), merged)
	}
	if merged[0].Confidence != 0.8 {
		t.Errorf("expected merged confidence to be max(0.6, 0.8)=0.8, got %v", merged[0].Confidence)
	}
	if len(merged[0].SourceSpans) != 2 {
		t.Errorf("expected merged issue to carry union of 2 source spans, got %d", len(merged[0].SourceSpans))
	}
}

func TestDedup_DoesNotMergeDistinctIssues(t *testing.T) {
	issues := []issue.CandidateIssue{
		{ID: "a", Text: "whether the contract was breached", Confidence: 0.6},
		{ID: "b", Text: "whether custody should transfer to the grandparents", Confidence: 0.7},
	}

	merged := issue.Dedup(issues)

	if len(merged) != 2 {
		t.Fatalf("expected 2 distinct issues to remain unmerged, got %d: %+v", len(merged), merged)
	}
}

func TestDedup_EmptyInput(t *testing.T) {
	if got := issue.Dedup(nil); got != nil {
		t.Errorf("expected nil for empty input, got %+v", got)
	}
}

func TestDedup_TransitiveMergeChain(t *testing.T) {
	issues := []issue.CandidateIssue{
		{ID: "a", Text: "whether the tenant paid rent on time each month", Confidence: 0.5},
		{ID: "b", Text: "whether the tenant paid the rent on time every month", Confidence: 0.6},
		{ID: "c", Text: "whether custody should transfer", Confidence: 0.9},
	}

	merged := issue.Dedup(issues)

	if len(merged) != 2 {
		t.Fatalf("expected the two rent-related issues to merge and custody to remain separate, got %d: %+v", len(merged), merged)
	}
}
