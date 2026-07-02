package ontology_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/ontology"
)

func TestRelationType_IsValid(t *testing.T) {
	tests := []struct {
		name string
		rt   ontology.RelationType
		want bool
	}{
		{"is_a valid", ontology.RelIsA, true},
		{"part_of valid", ontology.RelPartOf, true},
		{"related_to valid", ontology.RelRelatedTo, true},
		{"contradicts valid", ontology.RelContradicts, true},
		{"unknown invalid", ontology.RelationType("bogus"), false},
		{"empty invalid", ontology.RelationType(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rt.IsValid(); got != tt.want {
				t.Fatalf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllRelationTypes(t *testing.T) {
	all := ontology.AllRelationTypes()
	if len(all) != 4 {
		t.Fatalf("expected 4 relation types, got %d", len(all))
	}
	for _, rt := range all {
		if !rt.IsValid() {
			t.Fatalf("AllRelationTypes() returned invalid type %q", rt)
		}
	}
}

func TestNewRelation(t *testing.T) {
	rel := ontology.NewRelation("gross-negligence", "negligence", ontology.RelIsA)

	if rel.FromConceptID != "gross-negligence" {
		t.Fatalf("FromConceptID = %q, want %q", rel.FromConceptID, "gross-negligence")
	}
	if rel.ToConceptID != "negligence" {
		t.Fatalf("ToConceptID = %q, want %q", rel.ToConceptID, "negligence")
	}
	if rel.Type != ontology.RelIsA {
		t.Fatalf("Type = %q, want %q", rel.Type, ontology.RelIsA)
	}
}
