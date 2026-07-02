package application_test

import (
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/application"
	"github.com/YASSERRMD/verdex/packages/irac"
)

func testIssue(t *testing.T, id, text string) irac.IssueNode {
	t.Helper()
	return irac.NewIssueNode(id, "case-1", text, time.Now(), 0.8, irac.Provenance{})
}

func testRule(t *testing.T, id, text, jurisdiction, family string) irac.RuleNode {
	t.Helper()
	return irac.NewRuleNode(id, "case-1", text, jurisdiction, family, time.Now(), 0.8, irac.Provenance{})
}

func testFact(t *testing.T, id, text string) irac.FactNode {
	t.Helper()
	return irac.NewFactNode(id, "case-1", text, time.Now(), 0.8, irac.Provenance{})
}

func TestMatchIssueToRules_RanksRelevantRulesHigher(t *testing.T) {
	issue := testIssue(t, "issue-1", "whether the landlord gave reasonable notice before eviction")

	relevant := application.OriginatedRule{
		Rule:   testRule(t, "rule-notice", "a landlord must give reasonable notice before eviction", "US-CA", "common_law"),
		Origin: application.OriginStatute,
	}
	irrelevant := application.OriginatedRule{
		Rule:   testRule(t, "rule-tax", "corporate tax filings are due quarterly", "US-CA", "common_law"),
		Origin: application.OriginStatute,
	}

	matches := application.MatchIssueToRules(issue, []application.OriginatedRule{irrelevant, relevant})

	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
	if matches[0].Rule.Rule.ID != "rule-notice" {
		t.Fatalf("expected rule-notice to rank first, got %s (score %f) vs %s (score %f)",
			matches[0].Rule.Rule.ID, matches[0].Score, matches[1].Rule.Rule.ID, matches[1].Score)
	}
	if matches[0].Score <= matches[1].Score {
		t.Fatalf("expected relevant rule score (%f) > irrelevant rule score (%f)", matches[0].Score, matches[1].Score)
	}
}

func TestMatchIssueToRules_EmptyInputs(t *testing.T) {
	issue := testIssue(t, "issue-1", "")
	rule := application.OriginatedRule{Rule: testRule(t, "rule-1", "some rule text", "US-CA", "common_law")}

	matches := application.MatchIssueToRules(issue, []application.OriginatedRule{rule})
	if len(matches) != 1 {
		t.Fatalf("expected 1 match even with blank issue text, got %d", len(matches))
	}
	if matches[0].Score != 0 {
		t.Fatalf("expected score 0 for blank issue text, got %f", matches[0].Score)
	}

	empty := application.MatchIssueToRules(testIssue(t, "issue-2", "text"), nil)
	if len(empty) != 0 {
		t.Fatalf("expected 0 matches for nil rules, got %d", len(empty))
	}
}

func TestMatchIssueToRules_NoOverlapScoresZero(t *testing.T) {
	issue := testIssue(t, "issue-1", "alpha beta gamma")
	rule := application.OriginatedRule{Rule: testRule(t, "rule-1", "delta epsilon zeta", "US-CA", "common_law")}

	matches := application.MatchIssueToRules(issue, []application.OriginatedRule{rule})
	if len(matches) != 1 || matches[0].Score != 0 {
		t.Fatalf("expected single zero-score match, got %+v", matches)
	}
}
