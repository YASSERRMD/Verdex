package application_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/application"
	"github.com/YASSERRMD/verdex/packages/irac"
)

func TestBuildApplicationNode_Success(t *testing.T) {
	issue := testIssue(t, "issue-1", "whether notice was reasonable")
	rule := application.OriginatedRule{
		Rule:   testRule(t, "rule-1", "a landlord must give reasonable notice", "US-CA", "common_law"),
		Origin: application.OriginStatute,
	}
	facts := []irac.FactNode{
		testFact(t, "fact-1", "the landlord gave two days notice"),
	}

	node, err := application.BuildApplicationNode(issue, rule, facts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.Type != irac.NodeApplication {
		t.Fatalf("expected NodeApplication type, got %s", node.Type)
	}
	if node.CaseID != issue.CaseID {
		t.Fatalf("expected CaseID %s, got %s", issue.CaseID, node.CaseID)
	}
	if node.Text == "" {
		t.Fatal("expected non-empty application text")
	}
	if node.ID == "" {
		t.Fatal("expected non-empty application node ID")
	}
}

func TestBuildApplicationNode_Deterministic(t *testing.T) {
	issue := testIssue(t, "issue-1", "whether notice was reasonable")
	rule := application.OriginatedRule{Rule: testRule(t, "rule-1", "text", "US-CA", "common_law")}
	facts := []irac.FactNode{testFact(t, "fact-1", "fact text")}

	node1, err := application.BuildApplicationNode(issue, rule, facts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	node2, err := application.BuildApplicationNode(issue, rule, facts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node1.ID != node2.ID {
		t.Fatalf("expected deterministic ID, got %s vs %s", node1.ID, node2.ID)
	}
}

func TestBuildApplicationNode_RejectsEmptyInputs(t *testing.T) {
	validIssue := testIssue(t, "issue-1", "text")
	validRule := application.OriginatedRule{Rule: testRule(t, "rule-1", "rule text", "US-CA", "common_law")}
	validFacts := []irac.FactNode{testFact(t, "fact-1", "fact text")}

	cases := []struct {
		name  string
		issue irac.IssueNode
		rule  application.OriginatedRule
		facts []irac.FactNode
	}{
		{"blank issue text", testIssue(t, "issue-1", ""), validRule, validFacts},
		{"blank rule text", validIssue, application.OriginatedRule{Rule: testRule(t, "rule-1", "", "US-CA", "common_law")}, validFacts},
		{"no facts", validIssue, validRule, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := application.BuildApplicationNode(tc.issue, tc.rule, tc.facts)
			if err != application.ErrEmptyInput {
				t.Fatalf("expected ErrEmptyInput, got %v", err)
			}
		})
	}
}
