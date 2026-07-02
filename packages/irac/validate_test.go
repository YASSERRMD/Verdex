package irac

import (
	"errors"
	"testing"
	"time"
)

func testTreeNodes(t *testing.T, now time.Time) (issue IssueNode, rule RuleNode, fact FactNode, application ApplicationNode, conclusion ConclusionNode) {
	t.Helper()
	prov := Provenance{GeneratedBy: "test", GeneratedAt: now}
	issue = NewIssueNode("i1", "case-1", "issue text", now, 0.9, prov)
	rule = NewRuleNode("r1", "case-1", "rule text", "US", "common_law", now, 0.9, prov)
	fact = NewFactNode("f1", "case-1", "fact text", now, 0.9, prov)
	application = NewApplicationNode("a1", "case-1", "application text", now, 0.9, prov)
	conclusion = NewConclusionNode("c1", "case-1", "conclusion text", now, 0.9, prov)
	return
}

func TestValidateTree_ValidTreeHasNoIssues(t *testing.T) {
	now := time.Now().UTC()
	issue, rule, fact, application, conclusion := testTreeNodes(t, now)

	nodes := []NodeLike{issue, rule, fact, application, conclusion}
	edges := []Edge{
		{FromID: rule.ID, ToID: issue.ID, Type: EdgeGoverns},
		{FromID: application.ID, ToID: fact.ID, Type: EdgeAppliesTo},
		{FromID: application.ID, ToID: rule.ID, Type: EdgeAppliesTo},
		{FromID: fact.ID, ToID: application.ID, Type: EdgeSupports},
		{FromID: conclusion.ID, ToID: application.ID, Type: EdgeConcludesFrom},
	}

	issues := ValidateTree(nodes, edges)
	if len(issues) != 0 {
		t.Fatalf("ValidateTree() = %v issues, want 0: %+v", len(issues), issues)
	}
}

func TestValidateTree_IllegalEdgeTriple(t *testing.T) {
	now := time.Now().UTC()
	issue, rule, _, _, _ := testTreeNodes(t, now)

	nodes := []NodeLike{issue, rule}
	edges := []Edge{
		// Reversed: Issue --governs--> Rule is illegal (only Rule
		// --governs--> Issue is legal).
		{FromID: issue.ID, ToID: rule.ID, Type: EdgeGoverns},
	}

	issues := ValidateTree(nodes, edges)
	if len(issues) != 1 {
		t.Fatalf("ValidateTree() = %d issues, want 1: %+v", len(issues), issues)
	}
	if !errors.Is(issues[0].Err, ErrIllegalEdgeTriple) {
		t.Errorf("issues[0].Err = %v, want ErrIllegalEdgeTriple", issues[0].Err)
	}
}

func TestValidateTree_DanglingEdge(t *testing.T) {
	now := time.Now().UTC()
	issue, _, _, _, _ := testTreeNodes(t, now)

	nodes := []NodeLike{issue}
	edges := []Edge{
		{FromID: "nonexistent-rule", ToID: issue.ID, Type: EdgeGoverns},
	}

	issues := ValidateTree(nodes, edges)
	if len(issues) == 0 {
		t.Fatalf("ValidateTree() = 0 issues, want at least 1")
	}
	found := false
	for _, iss := range issues {
		if errors.Is(iss.Err, ErrDanglingEdge) {
			found = true
			if iss.NodeID != "nonexistent-rule" {
				t.Errorf("dangling issue NodeID = %q, want %q", iss.NodeID, "nonexistent-rule")
			}
		}
	}
	if !found {
		t.Errorf("expected an ErrDanglingEdge issue, got %+v", issues)
	}
}

func TestValidateTree_DanglingBothEndpoints(t *testing.T) {
	edges := []Edge{
		{FromID: "ghost-1", ToID: "ghost-2", Type: EdgeGoverns},
	}
	issues := ValidateTree(nil, edges)

	danglingCount := 0
	for _, iss := range issues {
		if errors.Is(iss.Err, ErrDanglingEdge) {
			danglingCount++
		}
	}
	if danglingCount != 2 {
		t.Errorf("dangling issue count = %d, want 2 (one per missing endpoint): %+v", danglingCount, issues)
	}
}

func TestValidateTree_SelfLoop(t *testing.T) {
	now := time.Now().UTC()
	issue, _, _, _, _ := testTreeNodes(t, now)

	nodes := []NodeLike{issue}
	edges := []Edge{
		{FromID: issue.ID, ToID: issue.ID, Type: EdgeGoverns},
	}

	issues := ValidateTree(nodes, edges)
	found := false
	for _, iss := range issues {
		if errors.Is(iss.Err, ErrSelfLoop) {
			found = true
		}
	}
	if !found {
		t.Errorf("expected an ErrSelfLoop issue, got %+v", issues)
	}
}

func TestValidateTree_MissingGuardrailLabel(t *testing.T) {
	now := time.Now().UTC()
	issue, _, _, application, _ := testTreeNodes(t, now)

	// Construct a ConclusionNode without going through NewConclusionNode,
	// simulating a corrupted/decoded value that lost its guardrail label.
	corrupted := ConclusionNode{
		Node: Node{ID: "c-bad", Type: NodeConclusion, CaseID: "case-1", Text: "x", CreatedAt: now},
		// Label intentionally left empty.
	}

	nodes := []NodeLike{issue, application, corrupted}
	issues := ValidateTree(nodes, nil)

	found := false
	for _, iss := range issues {
		if errors.Is(iss.Err, ErrMissingGuardrailLabel) {
			found = true
			if iss.NodeID != "c-bad" {
				t.Errorf("guardrail issue NodeID = %q, want c-bad", iss.NodeID)
			}
		}
	}
	if !found {
		t.Fatalf("expected an ErrMissingGuardrailLabel issue, got %+v", issues)
	}
}

func TestValidateTree_UnknownNodeType(t *testing.T) {
	bad := Node{ID: "x1", Type: NodeType("bogus"), CaseID: "case-1"}
	issues := ValidateTree([]NodeLike{bad}, nil)

	found := false
	for _, iss := range issues {
		if errors.Is(iss.Err, ErrUnknownNodeType) {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected an ErrUnknownNodeType issue, got %+v", issues)
	}
}

func TestValidateTree_UnknownEdgeType(t *testing.T) {
	now := time.Now().UTC()
	issue, rule, _, _, _ := testTreeNodes(t, now)

	nodes := []NodeLike{issue, rule}
	edges := []Edge{
		{FromID: rule.ID, ToID: issue.ID, Type: EdgeType("bogus")},
	}
	issues := ValidateTree(nodes, edges)

	found := false
	for _, iss := range issues {
		if errors.Is(iss.Err, ErrUnknownEdgeType) {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected an ErrUnknownEdgeType issue, got %+v", issues)
	}
}

// TestValidateTree_CollectsMultipleIssues proves ValidateTree does not
// fail fast: it returns every issue found across multiple bad edges in a
// single pass.
func TestValidateTree_CollectsMultipleIssues(t *testing.T) {
	now := time.Now().UTC()
	issue, rule, _, _, _ := testTreeNodes(t, now)

	nodes := []NodeLike{issue, rule}
	edges := []Edge{
		{FromID: issue.ID, ToID: rule.ID, Type: EdgeGoverns}, // illegal triple
		{FromID: "ghost", ToID: issue.ID, Type: EdgeGoverns}, // dangling
		{FromID: rule.ID, ToID: rule.ID, Type: EdgeGoverns},  // self-loop
	}

	issues := ValidateTree(nodes, edges)
	if len(issues) < 3 {
		t.Fatalf("ValidateTree() = %d issues, want at least 3: %+v", len(issues), issues)
	}
}

func TestValidationIssue_Error(t *testing.T) {
	vi := ValidationIssue{Err: ErrSelfLoop, Message: "edge 0: self-loop"}
	if vi.Error() != "edge 0: self-loop" {
		t.Errorf("Error() = %q, want %q", vi.Error(), "edge 0: self-loop")
	}
}
