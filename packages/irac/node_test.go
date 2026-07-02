package irac

import (
	"testing"
	"time"
)

func TestNodeType_IsValid(t *testing.T) {
	tests := []struct {
		name string
		nt   NodeType
		want bool
	}{
		{"issue", NodeIssue, true},
		{"rule", NodeRule, true},
		{"fact", NodeFact, true},
		{"application", NodeApplication, true},
		{"conclusion", NodeConclusion, true},
		{"unknown", NodeType("bogus"), false},
		{"empty", NodeType(""), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.nt.IsValid(); got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllNodeTypes(t *testing.T) {
	got := AllNodeTypes()
	want := []NodeType{NodeIssue, NodeRule, NodeFact, NodeApplication, NodeConclusion}
	if len(got) != len(want) {
		t.Fatalf("AllNodeTypes() len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("AllNodeTypes()[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestNewIssueNode(t *testing.T) {
	now := time.Now().UTC()
	span := SourceSpan{Start: 0, End: 10}
	prov := Provenance{GeneratedBy: "extractor", GeneratedAt: now}
	n := NewIssueNode("i1", "case-1", "was there a breach?", now, 0.8, prov, span)

	if n.ID != "i1" || n.Type != NodeIssue || n.CaseID != "case-1" {
		t.Fatalf("unexpected node fields: %+v", n)
	}
	if n.Confidence != 0.8 {
		t.Errorf("Confidence = %v, want 0.8", n.Confidence)
	}
	if len(n.Spans) != 1 || n.Spans[0] != span {
		t.Errorf("Spans = %+v, want [%+v]", n.Spans, span)
	}
	if n.GetID() != "i1" || n.GetType() != NodeIssue {
		t.Errorf("NodeLike accessors mismatch: %+v", n)
	}
}

func TestNewRuleNode(t *testing.T) {
	now := time.Now().UTC()
	prov := Provenance{GeneratedBy: "rule-extractor", GeneratedAt: now}
	n := NewRuleNode("r1", "case-1", "contracts require consideration", "US-NY", "common_law", now, 0.9, prov)

	if n.Type != NodeRule {
		t.Fatalf("Type = %v, want NodeRule", n.Type)
	}
	if n.JurisdictionCode != "US-NY" || n.LegalFamily != "common_law" {
		t.Errorf("unexpected jurisdiction tagging: %+v", n)
	}
}

func TestNewFactNode(t *testing.T) {
	now := time.Now().UTC()
	prov := Provenance{GeneratedBy: "fact-extractor", GeneratedAt: now}
	n := NewFactNode("f1", "case-1", "the contract was signed on March 1", now, 0.7, prov)
	if n.Type != NodeFact {
		t.Fatalf("Type = %v, want NodeFact", n.Type)
	}
}

func TestNewApplicationNode(t *testing.T) {
	now := time.Now().UTC()
	prov := Provenance{GeneratedBy: "application-engine", GeneratedAt: now, UpstreamNodeIDs: []string{"f1", "r1"}}
	n := NewApplicationNode("a1", "case-1", "applying rule r1 to fact f1", now, 0.6, prov)
	if n.Type != NodeApplication {
		t.Fatalf("Type = %v, want NodeApplication", n.Type)
	}
	if len(n.Provenance.UpstreamNodeIDs) != 2 {
		t.Errorf("UpstreamNodeIDs = %v, want 2 entries", n.Provenance.UpstreamNodeIDs)
	}
}

func TestNode_GetIDGetType(t *testing.T) {
	n := Node{ID: "x1", Type: NodeFact}
	if n.GetID() != "x1" {
		t.Errorf("GetID() = %q, want %q", n.GetID(), "x1")
	}
	if n.GetType() != NodeFact {
		t.Errorf("GetType() = %v, want %v", n.GetType(), NodeFact)
	}
}

// TestNodeLike_Heterogeneous verifies every concrete node type implements
// NodeLike so a caller can hold a mixed-type slice of tree nodes.
func TestNodeLike_Heterogeneous(t *testing.T) {
	now := time.Now().UTC()
	prov := Provenance{GeneratedBy: "test", GeneratedAt: now}

	nodes := []NodeLike{
		NewIssueNode("i1", "case-1", "issue text", now, 0.5, prov),
		NewRuleNode("r1", "case-1", "rule text", "US", "common_law", now, 0.5, prov),
		NewFactNode("f1", "case-1", "fact text", now, 0.5, prov),
		NewApplicationNode("a1", "case-1", "application text", now, 0.5, prov),
		NewConclusionNode("c1", "case-1", "conclusion text", now, 0.5, prov),
	}

	wantTypes := []NodeType{NodeIssue, NodeRule, NodeFact, NodeApplication, NodeConclusion}
	for i, n := range nodes {
		if n.GetType() != wantTypes[i] {
			t.Errorf("nodes[%d].GetType() = %v, want %v", i, n.GetType(), wantTypes[i])
		}
	}
}
