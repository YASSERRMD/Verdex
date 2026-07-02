package ontology_test

import (
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/ontology"
)

func TestAliasRegistry_AddAndResolve_RoundTrips(t *testing.T) {
	registry := ontology.NewAliasRegistry()

	if err := registry.AddAlias("civil:negligence", "carelessness"); err != nil {
		t.Fatalf("AddAlias returned error: %v", err)
	}

	conceptID, ok := registry.ResolveAlias("carelessness")
	if !ok {
		t.Fatalf("expected alias to resolve")
	}
	if conceptID != "civil:negligence" {
		t.Fatalf("conceptID = %q, want %q", conceptID, "civil:negligence")
	}
}

func TestAliasRegistry_ResolveAlias_UnknownReturnsNotOK(t *testing.T) {
	registry := ontology.NewAliasRegistry()

	_, ok := registry.ResolveAlias("nonexistent")
	if ok {
		t.Fatalf("expected ok = false for unregistered alias")
	}
}

func TestAliasRegistry_AddAlias_EmptyInputRejected(t *testing.T) {
	registry := ontology.NewAliasRegistry()

	tests := []struct {
		name      string
		conceptID string
		alias     string
	}{
		{"empty concept id", "", "carelessness"},
		{"empty alias", "civil:negligence", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := registry.AddAlias(tt.conceptID, tt.alias)
			if !errors.Is(err, ontology.ErrEmptyInput) {
				t.Fatalf("expected ErrEmptyInput, got %v", err)
			}
		})
	}
}

func TestAliasRegistry_AddAlias_DuplicateRejected(t *testing.T) {
	registry := ontology.NewAliasRegistry()

	if err := registry.AddAlias("civil:negligence", "carelessness"); err != nil {
		t.Fatalf("first AddAlias returned error: %v", err)
	}

	err := registry.AddAlias("civil:breach-of-contract", "carelessness")
	if !errors.Is(err, ontology.ErrDuplicateAlias) {
		t.Fatalf("expected ErrDuplicateAlias, got %v", err)
	}
}

func TestAliasRegistry_AddAlias_SameConceptReRegisterIsNoOp(t *testing.T) {
	registry := ontology.NewAliasRegistry()

	if err := registry.AddAlias("civil:negligence", "carelessness"); err != nil {
		t.Fatalf("first AddAlias returned error: %v", err)
	}
	if err := registry.AddAlias("civil:negligence", "carelessness"); err != nil {
		t.Fatalf("re-registering same alias/concept should be a no-op, got error: %v", err)
	}
}

func TestAliasRegistry_Aliases_ReturnsAllForConcept(t *testing.T) {
	registry := ontology.NewAliasRegistry()
	_ = registry.AddAlias("civil:negligence", "carelessness")
	_ = registry.AddAlias("civil:negligence", "duty-of-care-breach")
	_ = registry.AddAlias("civil:breach-of-contract", "breach")

	aliases := registry.Aliases("civil:negligence")
	if len(aliases) != 2 {
		t.Fatalf("expected 2 aliases, got %d: %v", len(aliases), aliases)
	}
}

func TestAliasRegistry_RemoveAlias(t *testing.T) {
	registry := ontology.NewAliasRegistry()
	_ = registry.AddAlias("civil:negligence", "carelessness")

	registry.RemoveAlias("carelessness")

	_, ok := registry.ResolveAlias("carelessness")
	if ok {
		t.Fatalf("expected alias to be removed")
	}
}
