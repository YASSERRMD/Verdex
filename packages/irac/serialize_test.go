package irac

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestMarshalUnmarshalTree_RoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	span := SourceSpan{Start: 0, End: 20, Page: 1}
	prov := Provenance{GeneratedBy: "extractor-v1", GeneratedAt: now, UpstreamNodeIDs: []string{"f1", "r1"}}

	issue := NewIssueNode("i1", "case-1", "was there a breach?", now, 0.9, prov, span)
	rule := NewRuleNode("r1", "case-1", "contracts require consideration", "US-NY", "common_law", now, 0.85, prov, span)
	fact := NewFactNode("f1", "case-1", "the contract was signed", now, 0.7, prov, span)
	application := NewApplicationNode("a1", "case-1", "applying r1 to f1", now, 0.6, prov, span)
	conclusion := NewConclusionNode("c1", "case-1", "the elements appear satisfied", now, 0.5, prov, span)

	nodes := []NodeLike{issue, rule, fact, application, conclusion}
	edges := []Edge{
		{FromID: rule.ID, ToID: issue.ID, Type: EdgeGoverns},
		{FromID: application.ID, ToID: fact.ID, Type: EdgeAppliesTo},
		{FromID: application.ID, ToID: rule.ID, Type: EdgeAppliesTo},
		{FromID: fact.ID, ToID: application.ID, Type: EdgeSupports},
		{FromID: conclusion.ID, ToID: application.ID, Type: EdgeConcludesFrom},
	}
	revision := NewInitialRevision("case-1", now)

	data, err := MarshalTree(nodes, edges, revision)
	if err != nil {
		t.Fatalf("MarshalTree() error = %v", err)
	}

	gotNodes, gotEdges, gotRevision, err := UnmarshalTree(data)
	if err != nil {
		t.Fatalf("UnmarshalTree() error = %v", err)
	}

	if !reflect.DeepEqual(gotEdges, edges) {
		t.Errorf("edges round-trip mismatch:\ngot  %+v\nwant %+v", gotEdges, edges)
	}
	if !reflect.DeepEqual(gotRevision, revision) {
		t.Errorf("revision round-trip mismatch:\ngot  %+v\nwant %+v", gotRevision, revision)
	}
	if len(gotNodes) != len(nodes) {
		t.Fatalf("node count round-trip mismatch: got %d, want %d", len(gotNodes), len(nodes))
	}

	gotIssue, ok := gotNodes[0].(IssueNode)
	if !ok {
		t.Fatalf("nodes[0] type = %T, want IssueNode", gotNodes[0])
	}
	if !reflect.DeepEqual(gotIssue, issue) {
		t.Errorf("IssueNode round-trip mismatch:\ngot  %+v\nwant %+v", gotIssue, issue)
	}

	gotRule, ok := gotNodes[1].(RuleNode)
	if !ok {
		t.Fatalf("nodes[1] type = %T, want RuleNode", gotNodes[1])
	}
	if !reflect.DeepEqual(gotRule, rule) {
		t.Errorf("RuleNode round-trip mismatch:\ngot  %+v\nwant %+v", gotRule, rule)
	}

	gotFact, ok := gotNodes[2].(FactNode)
	if !ok {
		t.Fatalf("nodes[2] type = %T, want FactNode", gotNodes[2])
	}
	if !reflect.DeepEqual(gotFact, fact) {
		t.Errorf("FactNode round-trip mismatch:\ngot  %+v\nwant %+v", gotFact, fact)
	}

	gotApplication, ok := gotNodes[3].(ApplicationNode)
	if !ok {
		t.Fatalf("nodes[3] type = %T, want ApplicationNode", gotNodes[3])
	}
	if !reflect.DeepEqual(gotApplication, application) {
		t.Errorf("ApplicationNode round-trip mismatch:\ngot  %+v\nwant %+v", gotApplication, application)
	}

	gotConclusion, ok := gotNodes[4].(ConclusionNode)
	if !ok {
		t.Fatalf("nodes[4] type = %T, want ConclusionNode", gotNodes[4])
	}
	if !reflect.DeepEqual(gotConclusion, conclusion) {
		t.Errorf("ConclusionNode round-trip mismatch:\ngot  %+v\nwant %+v", gotConclusion, conclusion)
	}
	if !gotConclusion.HasGuardrailLabel() {
		t.Errorf("round-tripped ConclusionNode lost its guardrail label")
	}
}

func TestMarshalTree_RejectsConclusionWithoutGuardrailLabel(t *testing.T) {
	now := time.Now().UTC()
	corrupted := ConclusionNode{
		Node: Node{ID: "c-bad", Type: NodeConclusion, CaseID: "case-1", Text: "x", CreatedAt: now},
	}

	_, err := MarshalTree([]NodeLike{corrupted}, nil, NewInitialRevision("case-1", now))
	if !errors.Is(err, ErrMissingGuardrailLabel) {
		t.Fatalf("MarshalTree() error = %v, want ErrMissingGuardrailLabel", err)
	}
}

func TestUnmarshalTree_RejectsConclusionWithoutGuardrailLabel(t *testing.T) {
	// Hand-craft an envelope whose conclusion node is missing its label,
	// simulating tampered or pre-guardrail data reaching UnmarshalTree.
	data := []byte(`{
		"version": 1,
		"revision": {"revision_number": 1, "case_id": "case-1", "created_at": "2024-01-01T00:00:00Z"},
		"nodes": [
			{"kind": "conclusion", "node": {"id": "c1", "type": "conclusion", "case_id": "case-1", "text": "x", "created_at": "2024-01-01T00:00:00Z", "confidence": 0.5, "provenance": {"generated_by": "t", "generated_at": "2024-01-01T00:00:00Z"}}}
		],
		"edges": []
	}`)

	_, _, _, err := UnmarshalTree(data)
	if !errors.Is(err, ErrMissingGuardrailLabel) {
		t.Fatalf("UnmarshalTree() error = %v, want ErrMissingGuardrailLabel", err)
	}
}

func TestUnmarshalTree_RejectsUnknownNodeKind(t *testing.T) {
	data := []byte(`{
		"version": 1,
		"revision": {"revision_number": 1, "case_id": "case-1", "created_at": "2024-01-01T00:00:00Z"},
		"nodes": [
			{"kind": "bogus", "node": {}}
		],
		"edges": []
	}`)

	_, _, _, err := UnmarshalTree(data)
	if !errors.Is(err, ErrUnknownNodeType) {
		t.Fatalf("UnmarshalTree() error = %v, want ErrUnknownNodeType", err)
	}
}

func TestUnmarshalTree_InvalidJSON(t *testing.T) {
	_, _, _, err := UnmarshalTree([]byte("not json"))
	if err == nil {
		t.Fatal("UnmarshalTree() error = nil, want a JSON decode error")
	}
}

func TestMarshalTree_EmptyTree(t *testing.T) {
	now := time.Now().UTC()
	revision := NewInitialRevision("case-empty", now)

	data, err := MarshalTree(nil, nil, revision)
	if err != nil {
		t.Fatalf("MarshalTree() error = %v", err)
	}

	nodes, edges, gotRevision, err := UnmarshalTree(data)
	if err != nil {
		t.Fatalf("UnmarshalTree() error = %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("nodes = %v, want empty", nodes)
	}
	if len(edges) != 0 {
		t.Errorf("edges = %v, want empty", edges)
	}
	if !reflect.DeepEqual(gotRevision, revision) {
		t.Errorf("revision round-trip mismatch:\ngot  %+v\nwant %+v", gotRevision, revision)
	}
}
