package precedent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/graph"
)

func scoredPrecedentsFixture(t *testing.T) []ScoredPrecedent {
	t.Helper()
	rule := syntheticPrecedentRule(t)
	tagged := TagPrecedents([]PrecedentRule{rule}, TagOptions{CategoryCode: "tort"})
	hierarchy := ApplyCourtHierarchy(tagged, "")
	embedded, err := EmbedPrecedents(context.Background(), &fakeEmbeddingService{}, hierarchy, EmbedOptions{})
	if err != nil {
		t.Fatalf("EmbedPrecedents() error = %v", err)
	}
	return ScorePrecedents(embedded, time.Now())
}

func TestPersistPrecedents_RoundTrip(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	rules := scoredPrecedentsFixture(t)

	persisted, err := PersistPrecedents(context.Background(), store, "UK", rules)
	if err != nil {
		t.Fatalf("PersistPrecedents() error = %v", err)
	}
	if len(persisted) != len(rules) {
		t.Fatalf("len(persisted) = %d, want %d", len(persisted), len(rules))
	}

	for _, want := range persisted {
		got, err := store.GetNode(context.Background(), want.ID)
		if err != nil {
			t.Fatalf("GetNode(%q) error = %v", want.ID, err)
		}
		if got.ID != want.ID {
			t.Errorf("GetNode(%q).ID = %q, want %q", want.ID, got.ID, want.ID)
		}
		if got.Text != want.Text {
			t.Errorf("GetNode(%q).Text = %q, want %q", want.ID, got.Text, want.Text)
		}
		if got.Type != want.Type {
			t.Errorf("GetNode(%q).Type = %q, want %q", want.ID, got.Type, want.Type)
		}
	}
}

func TestLoadPrecedentsForJurisdiction(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	rules := scoredPrecedentsFixture(t)
	persisted, err := PersistPrecedents(context.Background(), store, "UK", rules)
	if err != nil {
		t.Fatalf("PersistPrecedents() error = %v", err)
	}

	ids := make([]string, len(persisted))
	for i, n := range persisted {
		ids[i] = n.ID
	}

	loaded, err := LoadPrecedentsForJurisdiction(context.Background(), store, ids)
	if err != nil {
		t.Fatalf("LoadPrecedentsForJurisdiction() error = %v", err)
	}
	if len(loaded) != len(ids) {
		t.Fatalf("len(loaded) = %d, want %d", len(loaded), len(ids))
	}
}

func TestLoadPrecedentsForJurisdiction_NotFound(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	_, err := LoadPrecedentsForJurisdiction(context.Background(), store, []string{"does-not-exist"})
	if err == nil {
		t.Fatal("LoadPrecedentsForJurisdiction() error = nil, want error")
	}
	if !errors.Is(err, ErrRuleNotFound) {
		t.Errorf("errors.Is(err, ErrRuleNotFound) = false, err = %v", err)
	}
}

func TestPersistPrecedents_WrapsError(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	rules := scoredPrecedentsFixture(t)
	// InMemoryGraphStore.CreateNode rejects an empty node ID
	// (graph.ErrEmptyNodeID); use that to confirm PersistPrecedents wraps
	// the underlying store error with ErrPersistFailed.
	rules[0].ID = ""

	_, err := PersistPrecedents(context.Background(), store, "UK", rules)
	if err == nil {
		t.Fatal("PersistPrecedents() error = nil, want error for empty node ID")
	}
	if !errors.Is(err, ErrPersistFailed) {
		t.Errorf("errors.Is(err, ErrPersistFailed) = false, err = %v", err)
	}
}
