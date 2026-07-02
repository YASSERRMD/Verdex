package citation_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/citation"
	"github.com/YASSERRMD/verdex/packages/graph"
)

func TestVerifyVerified(t *testing.T) {
	ctx := context.Background()
	store := graph.NewInMemoryGraphStore()
	mustCreateNode(t, store, ruleNode("rule-1", "case-1", "text"))

	result, err := citation.Verify(ctx, store, citation.CitedUnit{NodeID: "rule-1", CaseID: "case-1"})
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if result.Status != citation.StatusVerified {
		t.Errorf("Status = %q, want %q", result.Status, citation.StatusVerified)
	}
	if !result.Verified() {
		t.Error("Verified() = false, want true")
	}
}

func TestVerifyHallucinated(t *testing.T) {
	ctx := context.Background()
	store := graph.NewInMemoryGraphStore()

	result, err := citation.Verify(ctx, store, citation.CitedUnit{NodeID: "ghost", CaseID: "case-1"})
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if result.Status != citation.StatusHallucinated {
		t.Errorf("Status = %q, want %q", result.Status, citation.StatusHallucinated)
	}
	if result.Verified() {
		t.Error("Verified() = true, want false")
	}
}

func TestVerifyWrongCase(t *testing.T) {
	ctx := context.Background()
	store := graph.NewInMemoryGraphStore()
	mustCreateNode(t, store, ruleNode("rule-1", "case-A", "text"))

	result, err := citation.Verify(ctx, store, citation.CitedUnit{NodeID: "rule-1", CaseID: "case-B"})
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if result.Status != citation.StatusWrongCase {
		t.Errorf("Status = %q, want %q", result.Status, citation.StatusWrongCase)
	}
	if result.ActualCaseID != "case-A" {
		t.Errorf("ActualCaseID = %q, want case-A", result.ActualCaseID)
	}
}

func TestVerifyValidationErrors(t *testing.T) {
	ctx := context.Background()
	store := graph.NewInMemoryGraphStore()

	if _, err := citation.Verify(ctx, nil, citation.CitedUnit{NodeID: "n", CaseID: "c"}); !errors.Is(err, citation.ErrNilGraphStore) {
		t.Errorf("Verify(nil store) error = %v, want ErrNilGraphStore", err)
	}
	if _, err := citation.Verify(ctx, store, citation.CitedUnit{CaseID: "c"}); !errors.Is(err, citation.ErrEmptyNodeID) {
		t.Errorf("Verify(empty node id) error = %v, want ErrEmptyNodeID", err)
	}
	if _, err := citation.Verify(ctx, store, citation.CitedUnit{NodeID: "n"}); !errors.Is(err, citation.ErrEmptyCaseID) {
		t.Errorf("Verify(empty case id) error = %v, want ErrEmptyCaseID", err)
	}
}

func TestVerifyAll(t *testing.T) {
	ctx := context.Background()
	store := graph.NewInMemoryGraphStore()
	mustCreateNode(t, store, ruleNode("rule-1", "case-1", "text"))

	units := []citation.CitedUnit{
		{NodeID: "rule-1", CaseID: "case-1"},
		{NodeID: "ghost", CaseID: "case-1"},
	}

	results, err := citation.VerifyAll(ctx, store, units)
	if err != nil {
		t.Fatalf("VerifyAll() error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if results[0].Status != citation.StatusVerified {
		t.Errorf("results[0].Status = %q, want %q", results[0].Status, citation.StatusVerified)
	}
	if results[1].Status != citation.StatusHallucinated {
		t.Errorf("results[1].Status = %q, want %q", results[1].Status, citation.StatusHallucinated)
	}
}

func TestVerificationStatusIsValid(t *testing.T) {
	cases := []struct {
		s    citation.VerificationStatus
		want bool
	}{
		{citation.StatusVerified, true},
		{citation.StatusHallucinated, true},
		{citation.StatusWrongCase, true},
		{citation.StatusBroken, true},
		{citation.VerificationStatus("bogus"), false},
	}
	for _, tc := range cases {
		if got := tc.s.IsValid(); got != tc.want {
			t.Errorf("VerificationStatus(%q).IsValid() = %v, want %v", tc.s, got, tc.want)
		}
	}
}
