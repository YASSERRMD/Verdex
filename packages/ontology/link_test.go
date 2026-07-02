package ontology_test

import (
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/ontology"
)

func TestLinkConcept_CarriesCorrectNodeType(t *testing.T) {
	tests := []struct {
		name string
		node irac.NodeLike
		want irac.NodeType
	}{
		{
			name: "issue node",
			node: irac.NewIssueNode("issue-1", "case-1", "Was there negligence?", time.Now(), 0.9, irac.Provenance{}),
			want: irac.NodeIssue,
		},
		{
			name: "rule node",
			node: irac.NewRuleNode("rule-1", "case-1", "A duty of care is owed...", "US-CA", "common_law", time.Now(), 0.9, irac.Provenance{}),
			want: irac.NodeRule,
		},
		{
			name: "fact node",
			node: irac.NewFactNode("fact-1", "case-1", "The defendant ran a red light.", time.Now(), 0.9, irac.Provenance{}),
			want: irac.NodeFact,
		},
	}

	concept := ontology.Concept{ID: "civil:negligence", Name: "Negligence"}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			link := ontology.LinkConcept(concept, tt.node, 0.75)

			if link.ConceptID != concept.ID {
				t.Fatalf("ConceptID = %q, want %q", link.ConceptID, concept.ID)
			}
			if link.NodeID != tt.node.GetID() {
				t.Fatalf("NodeID = %q, want %q", link.NodeID, tt.node.GetID())
			}
			if link.NodeType != tt.want {
				t.Fatalf("NodeType = %q, want %q", link.NodeType, tt.want)
			}
			if link.Confidence != 0.75 {
				t.Fatalf("Confidence = %v, want 0.75", link.Confidence)
			}
		})
	}
}
