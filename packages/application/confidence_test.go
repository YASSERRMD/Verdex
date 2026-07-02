package application_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/application"
	"github.com/YASSERRMD/verdex/packages/irac"
)

func TestComputeConfidence_InRange(t *testing.T) {
	rule := application.OriginatedRule{
		Rule:   testRule(t, "rule-1", "text", "US-CA", "common_law"),
		Origin: application.OriginPrecedent,
	}
	match := application.RuleMatch{Rule: rule, Score: 0.9}

	confidence := application.ComputeConfidence(match, "common_law")
	if confidence < 0 || confidence > 1 {
		t.Fatalf("expected confidence in [0,1], got %f", confidence)
	}
}

func TestComputeConfidence_HigherMatchScoreYieldsHigherConfidence(t *testing.T) {
	rule := application.OriginatedRule{
		Rule:   testRule(t, "rule-1", "text", "US-CA", "common_law"),
		Origin: application.OriginStatute,
	}

	low := application.ComputeConfidence(application.RuleMatch{Rule: rule, Score: 0.1}, "common_law")
	high := application.ComputeConfidence(application.RuleMatch{Rule: rule, Score: 0.9}, "common_law")

	if high <= low {
		t.Fatalf("expected higher match score to yield higher confidence: low=%f high=%f", low, high)
	}
}

func TestApplyConfidence_SetsNodeConfidence(t *testing.T) {
	issue := testIssue(t, "issue-1", "notice was reasonable")
	rule := application.OriginatedRule{
		Rule:   testRule(t, "rule-1", "reasonable notice", "US-CA", "common_law"),
		Origin: application.OriginPrecedent,
	}
	facts := []irac.FactNode{testFact(t, "fact-1", "notice period was two days")}

	node, err := application.BuildApplicationNode(issue, rule, facts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	match := application.RuleMatch{Rule: rule, Score: 0.8}
	updated := application.ApplyConfidence(node, match, "common_law")

	if updated.Confidence == 0 {
		t.Fatal("expected non-zero confidence after ApplyConfidence")
	}
	if updated.Confidence != application.ComputeConfidence(match, "common_law") {
		t.Fatalf("expected updated confidence to match ComputeConfidence result")
	}
}
