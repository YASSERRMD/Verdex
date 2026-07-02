package ontology_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/ontology"
)

func TestConcept_HasCategory(t *testing.T) {
	tests := []struct {
		name    string
		concept ontology.Concept
		code    string
		want    bool
	}{
		{
			name:    "matching category",
			concept: ontology.Concept{ID: "c1", CategoryCodes: []string{"civil", "commercial"}},
			code:    "civil",
			want:    true,
		},
		{
			name:    "non-matching category",
			concept: ontology.Concept{ID: "c1", CategoryCodes: []string{"civil"}},
			code:    "criminal",
			want:    false,
		},
		{
			name:    "empty category list",
			concept: ontology.Concept{ID: "c1"},
			code:    "civil",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.concept.HasCategory(tt.code)
			if got != tt.want {
				t.Fatalf("HasCategory(%q) = %v, want %v", tt.code, got, tt.want)
			}
		})
	}
}
