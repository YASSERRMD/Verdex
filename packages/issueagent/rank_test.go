package issueagent

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/knowledgeapi"
)

func TestClamp01(t *testing.T) {
	cases := map[float64]float64{
		-0.5: 0,
		0:    0,
		0.5:  0.5,
		1:    1,
		1.5:  1,
	}
	for in, want := range cases {
		if got := clamp01(in); got != want {
			t.Errorf("clamp01(%f) = %f, want %f", in, got, want)
		}
	}
}

func TestBlendMateriality_NoModelScore_UsesHeuristic(t *testing.T) {
	got := blendMateriality(0.42, 0.99, false)
	if got != 0.42 {
		t.Fatalf("blendMateriality = %f, want 0.42 (heuristic only)", got)
	}
}

func TestBlendMateriality_WithModelScore_WeightsTowardModel(t *testing.T) {
	heuristic := 0.1
	model := 0.9
	got := blendMateriality(heuristic, model, true)
	// modelWeight is 0.7, so blended should lie strictly between heuristic
	// and model, closer to model.
	if got <= heuristic || got >= model {
		t.Fatalf("blendMateriality = %f, want strictly between %f and %f", got, heuristic, model)
	}
	if got < 0.5 {
		t.Fatalf("blendMateriality = %f, want weighted toward model (>0.5)", got)
	}
}

func TestRankIssues_OrdersByScoreDescending(t *testing.T) {
	issues := []FramedIssue{
		{SourceIssueNodeID: "b", MaterialityScore: 0.2},
		{SourceIssueNodeID: "a", MaterialityScore: 0.9},
		{SourceIssueNodeID: "c", MaterialityScore: 0.5},
	}
	rankIssues(issues)

	want := []string{"a", "c", "b"}
	for i, id := range want {
		if issues[i].SourceIssueNodeID != id {
			t.Fatalf("issues[%d].SourceIssueNodeID = %q, want %q", i, issues[i].SourceIssueNodeID, id)
		}
		if issues[i].MaterialityRank != i+1 {
			t.Fatalf("issues[%d].MaterialityRank = %d, want %d", i, issues[i].MaterialityRank, i+1)
		}
	}
}

func TestRankIssues_TiesBrokenByNodeID(t *testing.T) {
	issues := []FramedIssue{
		{SourceIssueNodeID: "zzz", MaterialityScore: 0.5},
		{SourceIssueNodeID: "aaa", MaterialityScore: 0.5},
	}
	rankIssues(issues)
	if issues[0].SourceIssueNodeID != "aaa" || issues[1].SourceIssueNodeID != "zzz" {
		t.Fatalf("tie-break order = [%s, %s], want [aaa, zzz]", issues[0].SourceIssueNodeID, issues[1].SourceIssueNodeID)
	}
}

func TestHeuristicMaterialityScore_MoreRulesScoreHigher(t *testing.T) {
	noRules := issueContext{}
	withRules := issueContext{GoverningRule: []knowledgeapi.NodeDTO{{ID: "r1"}, {ID: "r2"}}}

	scoreNoRules := heuristicMaterialityScore(noRules)
	if scoreNoRules < heuristicMaterialityFloor {
		t.Fatalf("heuristicMaterialityScore(no rules) = %f, want >= floor %f", scoreNoRules, heuristicMaterialityFloor)
	}

	scoreWithRules := heuristicMaterialityScore(withRules)
	if scoreWithRules <= scoreNoRules {
		t.Fatalf("heuristicMaterialityScore(with rules) = %f, want > %f (no rules)", scoreWithRules, scoreNoRules)
	}
}
