package lawapplication_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/lawapplication"
)

func TestComputeConfidence_NoControllingRulesIsZero(t *testing.T) {
	confidence, steps := lawapplication.ComputeConfidence(nil, nil, nil, nil, nil, lawapplication.CommonLawFamily)
	if confidence != 0 {
		t.Errorf("confidence = %v, want 0", confidence)
	}
	if len(steps) == 0 {
		t.Errorf("steps should be non-empty explaining the zero confidence")
	}
}

func TestComputeConfidence_HigherWithVerifiedCitationsAndStrongFacts(t *testing.T) {
	rules := []lawapplication.RuleRef{{ID: "rule-1", OriginHint: lawapplication.OriginStatute}}

	strongCitations := []lawapplication.AppliedCitation{{RuleID: "rule-1", Resolved: true, Verified: true}}
	weakCitations := []lawapplication.AppliedCitation{{RuleID: "rule-1", Resolved: false, Verified: false}}

	strongFacts := []lawapplication.ElementFactEntry{{RuleID: "rule-1", FactNodeID: "fact-1", FactWeight: 0.9}}
	weakFacts := []lawapplication.ElementFactEntry{{RuleID: "rule-1", FactNodeID: "fact-1", FactWeight: 0.1}}

	strong, _ := lawapplication.ComputeConfidence([]string{"rule-1"}, rules, strongCitations, strongFacts, nil, lawapplication.CommonLawFamily)
	weak, _ := lawapplication.ComputeConfidence([]string{"rule-1"}, rules, weakCitations, weakFacts, nil, lawapplication.CommonLawFamily)

	if strong <= weak {
		t.Errorf("strong confidence (%v) should exceed weak confidence (%v)", strong, weak)
	}
}

func TestComputeConfidence_ConflictsReduceConfidence(t *testing.T) {
	rules := []lawapplication.RuleRef{{ID: "rule-1", OriginHint: lawapplication.OriginStatute}}
	citations := []lawapplication.AppliedCitation{{RuleID: "rule-1", Resolved: true, Verified: true}}
	facts := []lawapplication.ElementFactEntry{{RuleID: "rule-1", FactNodeID: "fact-1", FactWeight: 0.8}}

	withoutConflict, _ := lawapplication.ComputeConfidence([]string{"rule-1"}, rules, citations, facts, nil, lawapplication.CommonLawFamily)

	conflicts := []lawapplication.ConflictingAuthority{{IssueNodeID: "issue-1", FirstRuleID: "rule-1", SecondRuleID: "rule-2"}}
	withConflict, steps := lawapplication.ComputeConfidence([]string{"rule-1"}, rules, citations, facts, conflicts, lawapplication.CommonLawFamily)

	if withConflict >= withoutConflict {
		t.Errorf("confidence with conflict (%v) should be lower than without (%v)", withConflict, withoutConflict)
	}
	if len(steps) == 0 {
		t.Errorf("steps should be non-empty")
	}
}

func TestComputeConfidence_AlwaysInUnitRange(t *testing.T) {
	rules := []lawapplication.RuleRef{{ID: "rule-1"}}
	citations := []lawapplication.AppliedCitation{{RuleID: "rule-1"}}
	facts := []lawapplication.ElementFactEntry{{RuleID: "rule-1", FactWeight: 1.0}}
	manyConflicts := make([]lawapplication.ConflictingAuthority, 10)

	confidence, _ := lawapplication.ComputeConfidence([]string{"rule-1"}, rules, citations, facts, manyConflicts, lawapplication.CommonLawFamily)
	if confidence < 0 || confidence > 1 {
		t.Errorf("confidence = %v, want in [0,1]", confidence)
	}
}
