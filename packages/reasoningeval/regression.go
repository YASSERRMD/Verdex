package reasoningeval

import "fmt"

// RegressionResult reports the outcome of comparing two runs' quality
// score distributions.
type RegressionResult struct {
	// BaselineRunID and CurrentRunID identify the two runs compared.
	BaselineRunID string
	CurrentRunID  string

	// BaselineAvg and CurrentAvg are the arithmetic mean Overall scores
	// for each run.
	BaselineAvg float64
	CurrentAvg  float64

	// Drop is BaselineAvg - CurrentAvg. Positive means quality fell.
	Drop float64

	// Regressed is true when Drop exceeds the configured threshold.
	Regressed bool

	// PerDimensionDrop maps each DimensionName to (baseline avg - current
	// avg) for that dimension, for callers who want to see which
	// dimension drove a regression.
	PerDimensionDrop map[DimensionName]float64
}

// RegressionDetector compares QualityScore distributions across two runs
// (e.g. before/after a model or prompt-template change) and flags a
// threshold-meaningful drop, mirroring packages/eval.RegressionGate's
// baseline/threshold convention — but operating over reasoningeval's own
// QualityScore distributions instead of packages/eval.EvalReport's
// per-provider summaries, since this package has no notion of
// "provider", only "run".
type RegressionDetector struct {
	// Threshold is the maximum allowable drop in average Overall score
	// between baseline and current runs. A value of 0.05 permits up to a
	// 5-percentage-point drop; anything larger is flagged as a
	// regression. Must be in [0, 1]; values outside this range are
	// clamped.
	Threshold float64
}

// NewRegressionDetector creates a RegressionDetector with the given
// threshold.
func NewRegressionDetector(threshold float64) *RegressionDetector {
	return &RegressionDetector{Threshold: clamp01(threshold)}
}

// Compare computes a RegressionResult for baseline vs current.
//
// Returns ErrNoScores if either slice is empty.
func (d *RegressionDetector) Compare(baseline, current []QualityScore) (RegressionResult, error) {
	if len(baseline) == 0 || len(current) == 0 {
		return RegressionResult{}, ErrNoScores
	}

	threshold := clamp01(d.Threshold)

	baseAvg := averageOverall(baseline)
	curAvg := averageOverall(current)
	drop := baseAvg - curAvg

	result := RegressionResult{
		BaselineRunID:    baseline[0].RunID,
		CurrentRunID:     current[0].RunID,
		BaselineAvg:      baseAvg,
		CurrentAvg:       curAvg,
		Drop:             drop,
		Regressed:        drop > threshold,
		PerDimensionDrop: perDimensionDrop(baseline, current),
	}
	return result, nil
}

// CompareErr behaves like Compare but returns a non-nil error wrapping
// ErrRegressionDetected when a regression is flagged, for callers that
// prefer the "check returns an error" idiom used by
// packages/eval.RegressionGate.Check.
func (d *RegressionDetector) CompareErr(baseline, current []QualityScore) (RegressionResult, error) {
	result, err := d.Compare(baseline, current)
	if err != nil {
		return result, err
	}
	if result.Regressed {
		return result, fmt.Errorf(
			"%w: avg score dropped from %.4f to %.4f (drop=%.4f > threshold=%.4f)",
			ErrRegressionDetected, result.BaselineAvg, result.CurrentAvg, result.Drop, d.Threshold,
		)
	}
	return result, nil
}

func averageOverall(scores []QualityScore) float64 {
	if len(scores) == 0 {
		return 0
	}
	var sum float64
	for _, s := range scores {
		sum += s.Overall
	}
	return sum / float64(len(scores))
}

func perDimensionDrop(baseline, current []QualityScore) map[DimensionName]float64 {
	baseAvgs := averagePerDimension(baseline)
	curAvgs := averagePerDimension(current)

	drops := make(map[DimensionName]float64, len(baseAvgs))
	for name, baseAvg := range baseAvgs {
		curAvg, ok := curAvgs[name]
		if !ok {
			continue
		}
		drops[name] = baseAvg - curAvg
	}
	return drops
}

func averagePerDimension(scores []QualityScore) map[DimensionName]float64 {
	sums := make(map[DimensionName]float64)
	counts := make(map[DimensionName]int)
	for _, s := range scores {
		for name, v := range s.PerDimension {
			sums[name] += v
			counts[name]++
		}
	}
	avgs := make(map[DimensionName]float64, len(sums))
	for name, sum := range sums {
		avgs[name] = sum / float64(counts[name])
	}
	return avgs
}
