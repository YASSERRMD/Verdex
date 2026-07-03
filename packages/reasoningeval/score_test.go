package reasoningeval_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/reasoningeval"
)

func TestScore_FullyGroundedWellCitedOpinionScoresHigh(t *testing.T) {
	input := scoreInput(wellGroundedOpinion(), fullyGroundedReport(), "AE-DXB")
	rubric := reasoningeval.DefaultRubric()

	got, err := reasoningeval.Score(input, rubric, "run-1")
	if err != nil {
		t.Fatalf("Score() error = %v", err)
	}

	if got.Overall < 0.85 {
		t.Errorf("Overall = %.4f, want >= 0.85 for a fully-grounded, well-cited, substantive opinion", got.Overall)
	}
	if got.CaseID != testCaseID {
		t.Errorf("CaseID = %q, want %q", got.CaseID, testCaseID)
	}
	if got.JurisdictionCode != "AE-DXB" {
		t.Errorf("JurisdictionCode = %q, want %q", got.JurisdictionCode, "AE-DXB")
	}
	if got.RubricName != rubric.Name {
		t.Errorf("RubricName = %q, want %q", got.RubricName, rubric.Name)
	}
	if got.RunID != "run-1" {
		t.Errorf("RunID = %q, want %q", got.RunID, "run-1")
	}
	if got.PerDimension[reasoningeval.DimensionGrounding] != 1.0 {
		t.Errorf("grounding dimension = %.4f, want 1.0", got.PerDimension[reasoningeval.DimensionGrounding])
	}
	if got.PerDimension[reasoningeval.DimensionCitation] != 1.0 {
		t.Errorf("citation dimension = %.4f, want 1.0 (no findings)", got.PerDimension[reasoningeval.DimensionCitation])
	}
}

func TestScore_FindingsLowerScoreProportionally(t *testing.T) {
	goodInput := scoreInput(wellGroundedOpinion(), fullyGroundedReport(), "AE-DXB")
	badInput := scoreInput(wellGroundedOpinion(), findingsReport(), "AE-DXB")
	rubric := reasoningeval.DefaultRubric()

	good, err := reasoningeval.Score(goodInput, rubric, "run-1")
	if err != nil {
		t.Fatalf("Score(good) error = %v", err)
	}
	bad, err := reasoningeval.Score(badInput, rubric, "run-1")
	if err != nil {
		t.Fatalf("Score(bad) error = %v", err)
	}

	if bad.Overall >= good.Overall {
		t.Errorf("Overall for findings-report = %.4f, want < fully-grounded Overall = %.4f", bad.Overall, good.Overall)
	}

	// findingsReport has OpinionScore 0.4, so the grounding dimension must
	// reflect that directly.
	if bad.PerDimension[reasoningeval.DimensionGrounding] != 0.4 {
		t.Errorf("grounding dimension = %.4f, want 0.4", bad.PerDimension[reasoningeval.DimensionGrounding])
	}
	// findingsReport has 1 critical out of 2 total citation findings, so
	// citation dimension = 1 - 1/2 = 0.5.
	if bad.PerDimension[reasoningeval.DimensionCitation] != 0.5 {
		t.Errorf("citation dimension = %.4f, want 0.5", bad.PerDimension[reasoningeval.DimensionCitation])
	}
}

func TestScore_ThinOpinionScoresLowOnCoherence(t *testing.T) {
	good := scoreInput(wellGroundedOpinion(), fullyGroundedReport(), "AE-DXB")
	thin := scoreInput(thinOpinion(), fullyGroundedReport(), "AE-DXB")
	rubric := reasoningeval.DefaultRubric()

	goodScore, err := reasoningeval.Score(good, rubric, "run-1")
	if err != nil {
		t.Fatalf("Score(good) error = %v", err)
	}
	thinScore, err := reasoningeval.Score(thin, rubric, "run-1")
	if err != nil {
		t.Fatalf("Score(thin) error = %v", err)
	}

	if thinScore.PerDimension[reasoningeval.DimensionCoherence] >= goodScore.PerDimension[reasoningeval.DimensionCoherence] {
		t.Errorf("thin coherence = %.4f, want < good coherence = %.4f",
			thinScore.PerDimension[reasoningeval.DimensionCoherence], goodScore.PerDimension[reasoningeval.DimensionCoherence])
	}
}

func TestScore_EmptyRubricReturnsError(t *testing.T) {
	input := scoreInput(wellGroundedOpinion(), fullyGroundedReport(), "AE-DXB")
	_, err := reasoningeval.Score(input, reasoningeval.Rubric{Name: "empty"}, "run-1")
	if err == nil {
		t.Fatal("Score() with empty rubric: want error, got nil")
	}
}

func TestScore_NilGroundingReportReturnsError(t *testing.T) {
	input := reasoningeval.ScoreInput{
		CaseID:  testCaseID,
		Opinion: reasoningeval.WrapOpinion(wellGroundedOpinion()),
		// GroundingReport intentionally left nil.
	}
	_, err := reasoningeval.Score(input, reasoningeval.DefaultRubric(), "run-1")
	if err == nil {
		t.Fatal("Score() with nil grounding report: want error, got nil")
	}
}

func TestScoreBatch_ScoresEveryInputInOrder(t *testing.T) {
	inputs := []reasoningeval.ScoreInput{
		scoreInput(wellGroundedOpinion(), fullyGroundedReport(), "AE-DXB"),
		scoreInput(thinOpinion(), findingsReport(), "US-NY"),
	}
	scores, err := reasoningeval.ScoreBatch(inputs, reasoningeval.DefaultRubric(), "run-1")
	if err != nil {
		t.Fatalf("ScoreBatch() error = %v", err)
	}
	if len(scores) != 2 {
		t.Fatalf("len(scores) = %d, want 2", len(scores))
	}
	if scores[0].JurisdictionCode != "AE-DXB" || scores[1].JurisdictionCode != "US-NY" {
		t.Errorf("ScoreBatch did not preserve input order: got %q, %q", scores[0].JurisdictionCode, scores[1].JurisdictionCode)
	}
}
