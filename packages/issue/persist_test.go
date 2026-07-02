package issue_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/issue"
)

func TestToIssueNode_ConvertsCandidate(t *testing.T) {
	candidate := issue.CandidateIssue{
		ID:          "issue-1",
		Text:        "whether the contract was breached",
		Confidence:  0.75,
		SourceSpans: []irac.SourceSpan{{Start: 0, End: 10}},
	}
	createdAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	node := issue.ToIssueNode(candidate, "case-1", createdAt, []string{"fact-1"})

	if node.ID != "issue-1" || node.CaseID != "case-1" || node.Type != irac.NodeIssue {
		t.Fatalf("unexpected node shape: %+v", node)
	}
	if node.Text != candidate.Text {
		t.Errorf("expected text %q, got %q", candidate.Text, node.Text)
	}
	if node.Confidence != candidate.Confidence {
		t.Errorf("expected confidence %v, got %v", candidate.Confidence, node.Confidence)
	}
	if !node.Provenance.IsValid() {
		t.Errorf("expected valid provenance, got %+v", node.Provenance)
	}
	if len(node.Spans) != 1 {
		t.Errorf("expected 1 span, got %d", len(node.Spans))
	}
}

func TestPersistIssues_RoundTripsViaGetNode(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	candidates := []issue.CandidateIssue{
		{ID: "issue-1", Text: "whether the contract was breached", Confidence: 0.8},
		{ID: "issue-2", Text: "whether damages are owed", Confidence: 0.6},
	}
	createdAt := time.Now()

	nodes, err := issue.PersistIssues(context.Background(), store, candidates, "case-1", createdAt, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("expected 2 persisted nodes, got %d", len(nodes))
	}

	for _, want := range nodes {
		got, err := store.GetNode(context.Background(), want.ID)
		if err != nil {
			t.Fatalf("GetNode(%q) failed: %v", want.ID, err)
		}
		if got.Text != want.Text || got.CaseID != want.CaseID || got.Type != irac.NodeIssue {
			t.Errorf("round-tripped node mismatch: got %+v want %+v", got, want.Node)
		}
	}
}

func TestPersistIssues_FailurePropagatesErrPersistFailed(t *testing.T) {
	candidates := []issue.CandidateIssue{
		{ID: "", Text: "whether the contract was breached", Confidence: 0.8},
	}

	_, err := issue.PersistIssues(context.Background(), graph.NewInMemoryGraphStore(), candidates, "case-1", time.Now(), nil)
	if !errors.Is(err, issue.ErrPersistFailed) {
		t.Fatalf("expected ErrPersistFailed for empty node id, got %v", err)
	}
}

func TestPersistIssues_UpstreamNodeIDsFromLinks(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	candidates := []issue.CandidateIssue{
		{ID: "issue-1", Text: "whether the contract was breached", Confidence: 0.8},
	}
	links := map[int]issue.IssueLink{
		0: {IssueIndex: 0, FactIDs: []string{"fact-1", "fact-2"}},
	}

	nodes, err := issue.PersistIssues(context.Background(), store, candidates, "case-1", time.Now(), links)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes[0].Provenance.UpstreamNodeIDs) != 2 {
		t.Errorf("expected 2 upstream node ids, got %+v", nodes[0].Provenance.UpstreamNodeIDs)
	}
}
