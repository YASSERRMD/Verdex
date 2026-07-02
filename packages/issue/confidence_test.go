package issue_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/issue"
)

func TestScoreConfidence_ClaimSupportRaisesScore(t *testing.T) {
	base := []issue.CandidateIssue{
		{ID: "a", Text: "whether the contract was breached", Confidence: 0.5},
		{ID: "b", Text: "whether damages are owed", Confidence: 0.5},
	}

	withoutSupport := issue.ScoreConfidence(base, nil)
	withSupport := issue.ScoreConfidence(base, []issue.ClaimLink{
		{IssueIndex: 0, SegmentID: "seg-1", Overlap: 0.9},
	})

	if withSupport[0].Confidence <= withoutSupport[0].Confidence {
		t.Errorf("expected claim support to raise confidence: without=%v with=%v",
			withoutSupport[0].Confidence, withSupport[0].Confidence)
	}
	if withSupport[1].Confidence != withoutSupport[1].Confidence {
		t.Errorf("expected unsupported issue's confidence to be unaffected by another issue's claim link")
	}
}

func TestScoreConfidence_DedupCorroborationRaisesScore(t *testing.T) {
	single := []issue.CandidateIssue{
		{ID: "a", Text: "whether the contract was breached", Confidence: 0.5, SourceSpans: []irac.SourceSpan{{Start: 0, End: 5}}},
	}
	corroborated := []issue.CandidateIssue{
		{ID: "a", Text: "whether the contract was breached", Confidence: 0.5, SourceSpans: []irac.SourceSpan{{Start: 0, End: 5}, {Start: 10, End: 15}}},
	}

	singleScored := issue.ScoreConfidence(single, nil)
	corroboratedScored := issue.ScoreConfidence(corroborated, nil)

	if corroboratedScored[0].Confidence <= singleScored[0].Confidence {
		t.Errorf("expected multi-span corroboration to raise confidence: single=%v corroborated=%v",
			singleScored[0].Confidence, corroboratedScored[0].Confidence)
	}
}

func TestScoreConfidence_ClampedToUnitInterval(t *testing.T) {
	issues := []issue.CandidateIssue{
		{ID: "a", Text: "whether the contract was breached", Confidence: 1.0, SourceSpans: []irac.SourceSpan{{Start: 0, End: 5}, {Start: 10, End: 15}}},
	}
	links := []issue.ClaimLink{{IssueIndex: 0, SegmentID: "seg-1", Overlap: 1.0}}

	scored := issue.ScoreConfidence(issues, links)

	if scored[0].Confidence < 0 || scored[0].Confidence > 1 {
		t.Fatalf("expected confidence in [0,1], got %v", scored[0].Confidence)
	}
}

func TestScoreConfidence_DoesNotMutateInput(t *testing.T) {
	issues := []issue.CandidateIssue{
		{ID: "a", Text: "whether the contract was breached", Confidence: 0.5},
	}
	_ = issue.ScoreConfidence(issues, []issue.ClaimLink{{IssueIndex: 0, Overlap: 1.0}})

	if issues[0].Confidence != 0.5 {
		t.Errorf("expected ScoreConfidence to not mutate its input slice, got %v", issues[0].Confidence)
	}
}
