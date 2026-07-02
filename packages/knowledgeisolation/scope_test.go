package knowledgeisolation_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/knowledgeisolation"
)

func TestClassifyNodeType(t *testing.T) {
	t.Parallel()

	cases := []struct {
		nodeType irac.NodeType
		want     knowledgeisolation.NodeScope
	}{
		{irac.NodeRule, knowledgeisolation.ScopeSharedLaw},
		{irac.NodeIssue, knowledgeisolation.ScopeCaseFacts},
		{irac.NodeFact, knowledgeisolation.ScopeCaseFacts},
		{irac.NodeApplication, knowledgeisolation.ScopeCaseFacts},
		{irac.NodeConclusion, knowledgeisolation.ScopeCaseFacts},
		{irac.NodeType("unknown"), knowledgeisolation.ScopeCaseFacts},
	}

	for _, c := range cases {
		if got := knowledgeisolation.ClassifyNodeType(c.nodeType); got != c.want {
			t.Errorf("ClassifyNodeType(%q) = %v, want %v", c.nodeType, got, c.want)
		}
	}
}

func TestIsSharedLawNode(t *testing.T) {
	t.Parallel()

	rule := irac.Node{ID: "r1", Type: irac.NodeRule, CaseID: "case-a"}
	if !knowledgeisolation.IsSharedLawNode(rule) {
		t.Errorf("expected RuleNode to be shared-law")
	}

	fact := irac.Node{ID: "f1", Type: irac.NodeFact, CaseID: "case-a"}
	if knowledgeisolation.IsSharedLawNode(fact) {
		t.Errorf("expected FactNode to not be shared-law")
	}
}

func TestNodeScope_String(t *testing.T) {
	t.Parallel()

	if got := knowledgeisolation.ScopeCaseFacts.String(); got != "case_facts" {
		t.Errorf("ScopeCaseFacts.String() = %q, want case_facts", got)
	}
	if got := knowledgeisolation.ScopeSharedLaw.String(); got != "shared_law" {
		t.Errorf("ScopeSharedLaw.String() = %q, want shared_law", got)
	}
	if got := knowledgeisolation.NodeScope(99).String(); got != "unknown" {
		t.Errorf("NodeScope(99).String() = %q, want unknown", got)
	}
}
