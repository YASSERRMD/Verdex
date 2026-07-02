package issue_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/evidence"
	"github.com/YASSERRMD/verdex/packages/issue"
)

func TestMapClaimsToIssues_MatchesRelevantTypes(t *testing.T) {
	issues := []issue.CandidateIssue{
		{ID: "issue-1", Text: "whether the contract was breached by late payment"},
		{ID: "issue-2", Text: "whether the notice period was satisfied"},
	}
	classifications := []evidence.Classification{
		{SegmentID: "seg-1", Type: evidence.TypeArgument},
		{SegmentID: "seg-2", Type: evidence.TypeWitnessStatement},
		{SegmentID: "seg-3", Type: evidence.TypeStatutoryCitation},
	}
	segmentText := map[string]string{
		"seg-1": "The defendant breached the contract through late payment.",
		"seg-2": "I witnessed the payment being made after the due date.",
		"seg-3": "Section 73 of the Contract Act governs damages.",
	}

	links := issue.MapClaimsToIssues(classifications, issues, segmentText)

	if len(links) == 0 {
		t.Fatalf("expected at least one claim link")
	}
	for _, l := range links {
		if l.SegmentID == "seg-3" {
			t.Errorf("expected statutory citation to be excluded from claim mapping")
		}
	}
}

func TestMapClaimsToIssues_NoMatchBelowThreshold(t *testing.T) {
	issues := []issue.CandidateIssue{
		{ID: "issue-1", Text: "whether the vehicle was properly registered"},
	}
	classifications := []evidence.Classification{
		{SegmentID: "seg-1", Type: evidence.TypeArgument},
	}
	segmentText := map[string]string{
		"seg-1": "The weather was unusually cold that winter.",
	}

	links := issue.MapClaimsToIssues(classifications, issues, segmentText)
	if len(links) != 0 {
		t.Errorf("expected no links for unrelated text, got %d", len(links))
	}
}

func TestMapClaimsToIssues_EmptySegmentTextSkipped(t *testing.T) {
	issues := []issue.CandidateIssue{
		{ID: "issue-1", Text: "whether damages are owed"},
	}
	classifications := []evidence.Classification{
		{SegmentID: "seg-missing", Type: evidence.TypeArgument},
	}
	links := issue.MapClaimsToIssues(classifications, issues, map[string]string{})
	if len(links) != 0 {
		t.Errorf("expected no links when segment text is missing, got %d", len(links))
	}
}
