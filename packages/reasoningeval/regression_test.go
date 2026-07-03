package reasoningeval_test

import (
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/reasoningeval"
)

func scoresWithOverall(runID string, overalls ...float64) []reasoningeval.QualityScore {
	out := make([]reasoningeval.QualityScore, 0, len(overalls))
	for _, o := range overalls {
		out = append(out, reasoningeval.QualityScore{
			RunID:   runID,
			Overall: o,
			PerDimension: map[reasoningeval.DimensionName]float64{
				reasoningeval.DimensionGrounding: o,
				reasoningeval.DimensionCitation:  o,
				reasoningeval.DimensionCoherence: o,
			},
		})
	}
	return out
}

func TestRegressionDetector_FlagsRealDrop(t *testing.T) {
	baseline := scoresWithOverall("v1", 0.9, 0.92, 0.88)
	current := scoresWithOverall("v2", 0.6, 0.62, 0.58)

	det := reasoningeval.NewRegressionDetector(0.05)
	result, err := det.Compare(baseline, current)
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	if !result.Regressed {
		t.Errorf("Regressed = false, want true for a %.2f -> %.2f drop", result.BaselineAvg, result.CurrentAvg)
	}

	_, err = det.CompareErr(baseline, current)
	if !errors.Is(err, reasoningeval.ErrRegressionDetected) {
		t.Errorf("CompareErr() error = %v, want ErrRegressionDetected", err)
	}
}

func TestRegressionDetector_DoesNotFalsePositiveOnNoise(t *testing.T) {
	baseline := scoresWithOverall("v1", 0.90, 0.91, 0.89)
	current := scoresWithOverall("v2", 0.895, 0.905, 0.885) // tiny noise, ~0.01 drop

	det := reasoningeval.NewRegressionDetector(0.05)
	result, err := det.Compare(baseline, current)
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	if result.Regressed {
		t.Errorf("Regressed = true, want false for a noise-level drop of %.4f (threshold 0.05)", result.Drop)
	}

	_, err = det.CompareErr(baseline, current)
	if err != nil {
		t.Errorf("CompareErr() error = %v, want nil", err)
	}
}

func TestRegressionDetector_EmptyInputsReturnError(t *testing.T) {
	det := reasoningeval.NewRegressionDetector(0.05)
	if _, err := det.Compare(nil, scoresWithOverall("v2", 0.9)); !errors.Is(err, reasoningeval.ErrNoScores) {
		t.Errorf("Compare(nil baseline) error = %v, want ErrNoScores", err)
	}
	if _, err := det.Compare(scoresWithOverall("v1", 0.9), nil); !errors.Is(err, reasoningeval.ErrNoScores) {
		t.Errorf("Compare(nil current) error = %v, want ErrNoScores", err)
	}
}

func TestRegressionDetector_PerDimensionDropReflectsTheDroppedAxis(t *testing.T) {
	baseline := []reasoningeval.QualityScore{{
		RunID:   "v1",
		Overall: 0.9,
		PerDimension: map[reasoningeval.DimensionName]float64{
			reasoningeval.DimensionGrounding: 0.95,
			reasoningeval.DimensionCitation:  0.90,
			reasoningeval.DimensionCoherence: 0.85,
		},
	}}
	current := []reasoningeval.QualityScore{{
		RunID:   "v2",
		Overall: 0.7,
		PerDimension: map[reasoningeval.DimensionName]float64{
			reasoningeval.DimensionGrounding: 0.50, // this is the axis that dropped
			reasoningeval.DimensionCitation:  0.90,
			reasoningeval.DimensionCoherence: 0.85,
		},
	}}

	det := reasoningeval.NewRegressionDetector(0.1)
	result, err := det.Compare(baseline, current)
	if err != nil {
		t.Fatalf("Compare() error = %v", err)
	}
	if drop := result.PerDimensionDrop[reasoningeval.DimensionGrounding]; drop < 0.4 {
		t.Errorf("grounding drop = %.4f, want >= 0.4", drop)
	}
	if drop := result.PerDimensionDrop[reasoningeval.DimensionCitation]; drop != 0 {
		t.Errorf("citation drop = %.4f, want 0", drop)
	}
}
