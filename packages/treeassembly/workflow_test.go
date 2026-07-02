package treeassembly

import (
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/irac"
)

func TestTree_FieldsRoundTrip(t *testing.T) {
	rev := irac.NewInitialRevision("case-1", time.Now())
	issue := irac.NewIssueNode("issue-1", "case-1", "text", time.Now(), 0.9, testProvenance())

	tree := &Tree{
		Nodes:    []irac.NodeLike{issue},
		Edges:    []irac.Edge{},
		Revision: rev,
	}

	if len(tree.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(tree.Nodes))
	}
	if tree.Revision.CaseID != "case-1" {
		t.Fatalf("expected case-1, got %q", tree.Revision.CaseID)
	}
}

func TestAssemblyInput_Fields(t *testing.T) {
	input := syntheticInput("case-1")

	tests := []struct {
		name string
		got  int
		want int
	}{
		{"issues", len(input.Issues), 1},
		{"rules", len(input.Rules), 1},
		{"facts", len(input.Facts), 1},
		{"applications", len(input.Applications), 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %d, want %d", tt.got, tt.want)
			}
		})
	}
}
