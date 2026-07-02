package issue_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/evidence"
	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/issue"
	"github.com/YASSERRMD/verdex/packages/segmentation"
	"github.com/YASSERRMD/verdex/packages/timeline"
)

func TestIssueExtractionService_ExtractIssues_EndToEnd(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	svc := &issue.IssueExtractionService{
		Identifier: issue.NewRuleBasedIdentifier(),
		Store:      store,
	}

	segments := []segmentation.Segment{
		{ID: "seg-1", Type: segmentation.SegmentParagraph, Text: "The court must decide whether Acme Corp breached the supply agreement."},
		{ID: "seg-2", Type: segmentation.SegmentParagraph, Text: "We submit that Acme Corp breached the supply agreement by late delivery."},
	}
	classifications := []evidence.Classification{
		{SegmentID: "seg-2", Type: evidence.TypeArgument},
	}
	parties := []timeline.Party{
		{ID: "party-1", Role: timeline.PartyFirst, Name: "Acme Corp"},
	}

	nodes, err := svc.ExtractIssues(context.Background(), issue.ExtractRequest{
		CaseID:          "case-1",
		Segments:        segments,
		Classifications: classifications,
		Parties:         parties,
		IDPrefix:        "case-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) == 0 {
		t.Fatalf("expected at least one extracted issue node")
	}

	for _, n := range nodes {
		if n.Type != irac.NodeIssue {
			t.Errorf("expected NodeIssue type, got %v", n.Type)
		}
		if n.CaseID != "case-1" {
			t.Errorf("expected case-1, got %v", n.CaseID)
		}
		got, err := store.GetNode(context.Background(), n.ID)
		if err != nil {
			t.Fatalf("expected persisted node to round-trip via GetNode: %v", err)
		}
		if got.ID != n.ID {
			t.Errorf("round-tripped node id mismatch: got %v want %v", got.ID, n.ID)
		}
	}
}

func TestIssueExtractionService_ExtractIssues_EmptyCaseID(t *testing.T) {
	svc := issue.NewIssueExtractionService()
	_, err := svc.ExtractIssues(context.Background(), issue.ExtractRequest{
		Segments: []segmentation.Segment{{ID: "seg-1", Text: "whether x"}},
	})
	if !errors.Is(err, issue.ErrEmptyInput) {
		t.Fatalf("expected ErrEmptyInput, got %v", err)
	}
}

func TestIssueExtractionService_ExtractIssues_NoSegments(t *testing.T) {
	svc := issue.NewIssueExtractionService()
	_, err := svc.ExtractIssues(context.Background(), issue.ExtractRequest{
		CaseID: "case-1",
	})
	if !errors.Is(err, issue.ErrNoSegments) {
		t.Fatalf("expected ErrNoSegments, got %v", err)
	}
}

func TestIssueExtractionService_ExtractIssues_DefaultsApplied(t *testing.T) {
	svc := &issue.IssueExtractionService{}
	segments := []segmentation.Segment{
		{ID: "seg-1", Type: segmentation.SegmentParagraph, Text: "The parties dispute whether the deadline was extended."},
	}

	nodes, err := svc.ExtractIssues(context.Background(), issue.ExtractRequest{
		CaseID:   "case-2",
		Segments: segments,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) == 0 {
		t.Fatalf("expected extraction to work with nil Identifier/Store defaults")
	}
}

func TestIssueExtractionService_ExtractIssues_WithOverride(t *testing.T) {
	svc := issue.NewIssueExtractionService()
	segments := []segmentation.Segment{
		{ID: "seg-1", Type: segmentation.SegmentParagraph, Text: "The court must decide whether the lease was validly terminated."},
	}

	// First pass without override to discover the generated candidate ID.
	firstPass, err := svc.ExtractIssues(context.Background(), issue.ExtractRequest{
		CaseID:   "case-3",
		Segments: segments,
		IDPrefix: "case-3",
	})
	if err != nil {
		t.Fatalf("unexpected error on first pass: %v", err)
	}
	if len(firstPass) == 0 {
		t.Fatalf("expected at least one issue on first pass")
	}

	targetID := firstPass[0].ID
	overrides := map[string]issue.ManualOverride{
		targetID: {
			IssueID:    targetID,
			Text:       "whether the lease was validly terminated for cause",
			Material:   true,
			ReviewedBy: "reviewer-1",
		},
	}

	secondPass, err := svc.ExtractIssues(context.Background(), issue.ExtractRequest{
		CaseID:    "case-3",
		Segments:  segments,
		IDPrefix:  "case-3",
		Overrides: overrides,
	})
	if err != nil {
		t.Fatalf("unexpected error on second pass: %v", err)
	}

	found := false
	for _, n := range secondPass {
		if n.ID == targetID {
			found = true
			if n.Text != overrides[targetID].Text {
				t.Errorf("expected overridden text %q, got %q", overrides[targetID].Text, n.Text)
			}
			if n.Confidence != 1.0 {
				t.Errorf("expected overridden confidence 1.0, got %v", n.Confidence)
			}
		}
	}
	if !found {
		t.Fatalf("expected overridden issue id %q to be present in second pass results", targetID)
	}
}
