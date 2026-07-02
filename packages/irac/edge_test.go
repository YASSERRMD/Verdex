package irac

import "testing"

func TestEdgeType_IsValid(t *testing.T) {
	tests := []struct {
		name string
		et   EdgeType
		want bool
	}{
		{"governs", EdgeGoverns, true},
		{"applies_to", EdgeAppliesTo, true},
		{"supports", EdgeSupports, true},
		{"concludes_from", EdgeConcludesFrom, true},
		{"unknown", EdgeType("bogus"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.et.IsValid(); got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllEdgeTypes(t *testing.T) {
	got := AllEdgeTypes()
	want := []EdgeType{EdgeGoverns, EdgeAppliesTo, EdgeSupports, EdgeConcludesFrom}
	if len(got) != len(want) {
		t.Fatalf("AllEdgeTypes() len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("AllEdgeTypes()[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestIsLegalEdgeTriple(t *testing.T) {
	tests := []struct {
		name string
		from NodeType
		edge EdgeType
		to   NodeType
		want bool
	}{
		{"rule governs issue", NodeRule, EdgeGoverns, NodeIssue, true},
		{"application applies_to fact", NodeApplication, EdgeAppliesTo, NodeFact, true},
		{"application applies_to rule", NodeApplication, EdgeAppliesTo, NodeRule, true},
		{"fact supports application", NodeFact, EdgeSupports, NodeApplication, true},
		{"conclusion concludes_from application", NodeConclusion, EdgeConcludesFrom, NodeApplication, true},

		// Illegal triples: right nodes, wrong edge/direction.
		{"issue governs rule (reversed)", NodeIssue, EdgeGoverns, NodeRule, false},
		{"fact governs issue", NodeFact, EdgeGoverns, NodeIssue, false},
		{"rule applies_to fact", NodeRule, EdgeAppliesTo, NodeFact, false},
		{"application supports fact", NodeApplication, EdgeSupports, NodeFact, false},
		{"application concludes_from conclusion (reversed)", NodeApplication, EdgeConcludesFrom, NodeConclusion, false},
		{"issue concludes_from application", NodeIssue, EdgeConcludesFrom, NodeApplication, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsLegalEdgeTriple(tt.from, tt.edge, tt.to); got != tt.want {
				t.Errorf("IsLegalEdgeTriple(%v, %v, %v) = %v, want %v", tt.from, tt.edge, tt.to, got, tt.want)
			}
		})
	}
}

func TestLegalEdgeTriples(t *testing.T) {
	triples := LegalEdgeTriples()
	if len(triples) != 5 {
		t.Fatalf("LegalEdgeTriples() len = %d, want 5", len(triples))
	}
	for _, tr := range triples {
		if !IsLegalEdgeTriple(tr.From, tr.Edge, tr.To) {
			t.Errorf("triple %+v reported by LegalEdgeTriples() but IsLegalEdgeTriple disagrees", tr)
		}
	}
}

func TestEdge_Fields(t *testing.T) {
	e := Edge{FromID: "r1", ToID: "i1", Type: EdgeGoverns}
	if e.FromID != "r1" || e.ToID != "i1" || e.Type != EdgeGoverns {
		t.Errorf("unexpected edge fields: %+v", e)
	}
}
