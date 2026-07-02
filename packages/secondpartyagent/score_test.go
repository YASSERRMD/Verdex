package secondpartyagent

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/knowledgeapi"
)

func TestStrengthScore_HigherFactConfidenceAndVerifiedCitation_ScoresHigher(t *testing.T) {
	facts := map[string]knowledgeapi.NodeDTO{
		"fact-1": {ID: "fact-1", Confidence: 0.9},
	}
	weak := Argument{
		SupportingFactIDs: []string{"fact-1"},
		SupportingRuleIDs: []string{"rule-1"},
		Citations:         []CitationRef{{NodeID: "rule-1", Verified: false, ConfidenceScore: 0.1}},
	}
	strong := Argument{
		SupportingFactIDs: []string{"fact-1"},
		SupportingRuleIDs: []string{"rule-1"},
		Citations:         []CitationRef{{NodeID: "rule-1", Verified: true, ConfidenceScore: 0.95}},
	}

	weakScore := strengthScore(weak, facts)
	strongScore := strengthScore(strong, facts)
	if strongScore <= weakScore {
		t.Fatalf("strongScore = %f, weakScore = %f, want strongScore > weakScore", strongScore, weakScore)
	}
	if strongScore < 0 || strongScore > 1 {
		t.Fatalf("strongScore = %f, want in [0,1]", strongScore)
	}
}

func TestStrengthScore_NoCitations_StillProducesDefinedScore(t *testing.T) {
	facts := map[string]knowledgeapi.NodeDTO{
		"fact-1": {ID: "fact-1", Confidence: 0.6},
	}
	arg := Argument{
		SupportingFactIDs: []string{"fact-1"},
	}

	score := strengthScore(arg, facts)
	if score <= 0 {
		t.Fatalf("score = %f, want > 0 (fact confidence alone should contribute)", score)
	}
	if score > 1 {
		t.Fatalf("score = %f, want <= 1", score)
	}
}

func TestStrengthScore_NoEvidenceAtAll_ScoresZero(t *testing.T) {
	score := strengthScore(Argument{}, map[string]knowledgeapi.NodeDTO{})
	if score != 0 {
		t.Fatalf("score = %f, want 0", score)
	}
}

func TestStrengthScore_RicherRuleLinkage_ScoresHigher(t *testing.T) {
	facts := map[string]knowledgeapi.NodeDTO{}
	thin := Argument{SupportingRuleIDs: []string{"rule-1"}}
	rich := Argument{SupportingRuleIDs: []string{"rule-1", "rule-2", "rule-3"}}

	thinScore := strengthScore(thin, facts)
	richScore := strengthScore(rich, facts)
	if richScore <= thinScore {
		t.Fatalf("richScore = %f, thinScore = %f, want richScore > thinScore", richScore, thinScore)
	}
}
