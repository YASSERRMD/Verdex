package issueagent

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/knowledgeapi"
)

func TestHeuristicAmbiguities_FlagsMissingRuleLinkage(t *testing.T) {
	ic := issueContext{Node: knowledgeapi.NodeDTO{Confidence: 0.9}}
	got := heuristicAmbiguities(ic)
	if len(got) != 1 {
		t.Fatalf("heuristicAmbiguities = %v, want exactly 1 flag (missing rule linkage)", got)
	}
}

func TestHeuristicAmbiguities_FlagsLowConfidence(t *testing.T) {
	ic := issueContext{
		Node:          knowledgeapi.NodeDTO{Confidence: 0.1},
		GoverningRule: []knowledgeapi.NodeDTO{{ID: "r1"}},
	}
	got := heuristicAmbiguities(ic)
	if len(got) != 1 {
		t.Fatalf("heuristicAmbiguities = %v, want exactly 1 flag (low confidence)", got)
	}
}

func TestHeuristicAmbiguities_CleanIssue_NoFlags(t *testing.T) {
	ic := issueContext{
		Node:          knowledgeapi.NodeDTO{Confidence: 0.9},
		GoverningRule: []knowledgeapi.NodeDTO{{ID: "r1"}},
	}
	got := heuristicAmbiguities(ic)
	if len(got) != 0 {
		t.Fatalf("heuristicAmbiguities = %v, want none", got)
	}
}

func TestRuleLinkageConfidenceSignal_Saturates(t *testing.T) {
	if got := ruleLinkageConfidenceSignal(0); got != 0 {
		t.Fatalf("ruleLinkageConfidenceSignal(0) = %f, want 0", got)
	}
	if got := ruleLinkageConfidenceSignal(materialityRuleSaturation * 10); got != 1 {
		t.Fatalf("ruleLinkageConfidenceSignal(large) = %f, want 1 (clamped)", got)
	}
}

func TestBlendConfidence_NoModelConfidence_UsesHeuristicOnly(t *testing.T) {
	got := blendConfidence(0, 0.9, false)
	if got != 0.5 {
		t.Fatalf("blendConfidence(no model) = %f, want 0.5 (0.5 + 0.5*0 signal)", got)
	}
}

func TestBlendConfidence_WithModelConfidence_Blends(t *testing.T) {
	got := blendConfidence(1.0, 0.0, true)
	if got <= 0 || got >= 1 {
		t.Fatalf("blendConfidence = %f, want strictly between 0 and 1", got)
	}
}
