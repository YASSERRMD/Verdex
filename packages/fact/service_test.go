package fact_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/evidence"
	"github.com/YASSERRMD/verdex/packages/fact"
	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/timeline"
)

func TestFactConstructionService_ConstructFacts_EndToEnd(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	svc := &fact.FactConstructionService{Store: store}

	segments := []fact.SegmentInput{
		{
			Classification: evidence.Classification{SegmentID: "seg-1", Type: evidence.TypeWitnessStatement, Party: evidence.PartyFirst, Confidence: 0.8},
			Text:           "The defendant did not pay the invoice on time.",
			Span:           fact.SourceSpan{Start: 0, End: 10},
		},
		{
			Classification: evidence.Classification{SegmentID: "seg-2", Type: evidence.TypeWitnessStatement, Party: evidence.PartySecond, Confidence: 0.7},
			Text:           "The defendant paid the invoice on time.",
			Span:           fact.SourceSpan{Start: 10, End: 20},
		},
	}
	parties := []timeline.Party{
		{ID: "party-1", Role: timeline.PartyFirst, Name: "Acme Corp"},
		{ID: "party-2", Role: timeline.PartySecond, Name: "Jane Doe"},
	}

	nodes, err := svc.ConstructFacts(context.Background(), fact.ConstructRequest{
		CaseID:   "case-1",
		Segments: segments,
		Parties:  parties,
		IDPrefix: "case-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("expected 2 fact nodes, got %d", len(nodes))
	}

	for _, n := range nodes {
		if n.Type != irac.NodeFact {
			t.Errorf("expected NodeFact type, got %v", n.Type)
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

func TestFactConstructionService_ConstructFactsDetailed_FlagsDisputeAndScoresReliability(t *testing.T) {
	svc := fact.NewFactConstructionService()

	segments := []fact.SegmentInput{
		{
			Classification: evidence.Classification{SegmentID: "seg-1", Type: evidence.TypeWitnessStatement, Party: evidence.PartyFirst, Confidence: 0.8},
			Text:           "The defendant did not pay the invoice.",
		},
		{
			Classification: evidence.Classification{SegmentID: "seg-2", Type: evidence.TypeWitnessStatement, Party: evidence.PartySecond, Confidence: 0.7},
			Text:           "The defendant paid the invoice in full.",
		},
	}
	parties := []timeline.Party{
		{ID: "party-1", Role: timeline.PartyFirst, Name: "Acme Corp"},
		{ID: "party-2", Role: timeline.PartySecond, Name: "Jane Doe"},
	}

	details, err := svc.ConstructFactsDetailed(context.Background(), fact.ConstructRequest{
		CaseID:   "case-1",
		Segments: segments,
		Parties:  parties,
		IDPrefix: "case-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(details) != 2 {
		t.Fatalf("expected 2 fact details, got %d", len(details))
	}

	foundDisputed := false
	for _, d := range details {
		if d.DisputeStatus == fact.Disputed {
			foundDisputed = true
		}
		if d.ReliabilityScore < 0 || d.ReliabilityScore > 1 {
			t.Errorf("expected reliability score in [0, 1], got %v", d.ReliabilityScore)
		}
	}
	if !foundDisputed {
		t.Errorf("expected at least one disputed fact among contradictory party statements")
	}
}

func TestFactConstructionService_ConstructFacts_EmptyCaseID(t *testing.T) {
	svc := fact.NewFactConstructionService()
	_, err := svc.ConstructFacts(context.Background(), fact.ConstructRequest{
		Segments: []fact.SegmentInput{{Classification: evidence.Classification{SegmentID: "seg-1"}, Text: "text"}},
	})
	if !errors.Is(err, fact.ErrEmptyInput) {
		t.Fatalf("expected ErrEmptyInput, got %v", err)
	}
}

func TestFactConstructionService_ConstructFacts_NoSegments(t *testing.T) {
	svc := fact.NewFactConstructionService()
	_, err := svc.ConstructFacts(context.Background(), fact.ConstructRequest{CaseID: "case-1"})
	if !errors.Is(err, fact.ErrClassificationInvalid) {
		t.Fatalf("expected ErrClassificationInvalid, got %v", err)
	}
}

func TestFactConstructionService_ConstructFacts_DefaultsStore(t *testing.T) {
	svc := &fact.FactConstructionService{}
	nodes, err := svc.ConstructFacts(context.Background(), fact.ConstructRequest{
		CaseID: "case-1",
		Segments: []fact.SegmentInput{
			{Classification: evidence.Classification{SegmentID: "seg-1", Confidence: 0.5}, Text: "some fact text"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 fact node with default store, got %d", len(nodes))
	}
}
