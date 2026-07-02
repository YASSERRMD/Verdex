package statute

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/graph"
)

func embeddedRulesFixture(t *testing.T) []EmbeddedRule {
	t.Helper()
	amended := amendedRulesFixture(t)
	embedded, err := EmbedRules(context.Background(), &fakeEmbeddingService{}, amended, EmbedOptions{})
	if err != nil {
		t.Fatalf("EmbedRules() error = %v", err)
	}
	return embedded
}

func TestPersistRules_RoundTrip(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	rules := embeddedRulesFixture(t)

	persisted, err := PersistRules(context.Background(), store, "AE", rules)
	if err != nil {
		t.Fatalf("PersistRules() error = %v", err)
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
		if got.Text != want.Node.Text {
			t.Errorf("GetNode(%q).Text = %q, want %q", want.ID, got.Text, want.Node.Text)
		}
		if got.Type != want.Node.Type {
			t.Errorf("GetNode(%q).Type = %q, want %q", want.ID, got.Type, want.Node.Type)
		}
	}
}

func TestLoadRulesForJurisdiction(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	rules := embeddedRulesFixture(t)
	persisted, err := PersistRules(context.Background(), store, "AE", rules)
	if err != nil {
		t.Fatalf("PersistRules() error = %v", err)
	}

	ids := make([]string, len(persisted))
	for i, n := range persisted {
		ids[i] = n.ID
	}

	loaded, err := LoadRulesForJurisdiction(context.Background(), store, ids)
	if err != nil {
		t.Fatalf("LoadRulesForJurisdiction() error = %v", err)
	}
	if len(loaded) != len(ids) {
		t.Fatalf("len(loaded) = %d, want %d", len(loaded), len(ids))
	}
}

func TestLoadRulesForJurisdiction_NotFound(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	_, err := LoadRulesForJurisdiction(context.Background(), store, []string{"does-not-exist"})
	if err == nil {
		t.Fatal("LoadRulesForJurisdiction() error = nil, want error")
	}
	if !errors.Is(err, ErrRuleNotFound) {
		t.Errorf("errors.Is(err, ErrRuleNotFound) = false, err = %v", err)
	}
}

func TestPersistRules_WrapsError(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	rules := embeddedRulesFixture(t)
	// InMemoryGraphStore.CreateNode rejects an empty node ID
	// (graph.ErrEmptyNodeID); use that to confirm PersistRules wraps the
	// underlying store error with ErrPersistFailed.
	rules[0].Node.ID = ""

	_, err := PersistRules(context.Background(), store, "AE", rules)
	if err == nil {
		t.Fatal("PersistRules() error = nil, want error for empty node ID")
	}
	if !errors.Is(err, ErrPersistFailed) {
		t.Errorf("errors.Is(err, ErrPersistFailed) = false, err = %v", err)
	}
}
