package traversal_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/traversal"
)

func TestPath_StartAndEndID_Empty(t *testing.T) {
	var p traversal.Path
	if got := p.StartID(); got != "" {
		t.Errorf("expected empty StartID for empty path, got %q", got)
	}
	if got := p.EndID(); got != "" {
		t.Errorf("expected empty EndID for empty path, got %q", got)
	}
	if got := p.Depth(); got != 0 {
		t.Errorf("expected depth 0 for empty path, got %d", got)
	}
}

func TestPath_Explain_Empty(t *testing.T) {
	var p traversal.Path
	if got := p.Explain(); got != "" {
		t.Errorf("expected empty explanation for empty path, got %q", got)
	}
}

func TestPath_Explain_SingleHop(t *testing.T) {
	p := traversal.Path{
		Nodes: []traversal.PathNode{
			{ID: "issue-1", Type: irac.NodeIssue},
			{ID: "rule-1", Type: irac.NodeRule},
		},
		Hops: []traversal.TraversedHop{
			{FromIndex: 0, Kind: traversal.HopKindGoverningRule, EdgeType: irac.EdgeGoverns, Direction: traversal.Reverse},
		},
	}
	want := "issue-1 --governing_rule(reverse:governs)--> rule-1"
	if got := p.Explain(); got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestDirection_String(t *testing.T) {
	if got := traversal.Forward.String(); got != "forward" {
		t.Errorf("expected \"forward\", got %q", got)
	}
	if got := traversal.Reverse.String(); got != "reverse" {
		t.Errorf("expected \"reverse\", got %q", got)
	}
}
