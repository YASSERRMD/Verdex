package eval

import (
	"fmt"
)

// RegressionGate enforces that a new evaluation run does not fall below a
// quality threshold relative to a known-good baseline report.
//
// The gate compares per-provider average scores: if any provider's average
// score drops by more than Threshold (an absolute value in [0, 1]) compared
// to the same provider's baseline average, the check fails.
//
// Providers that were not present in the baseline are skipped (not treated as
// regressions).  Providers present in the baseline but absent from the current
// report are flagged as regressions with a score of 0.
type RegressionGate struct {
	// Baseline is the reference EvalReport to compare against.
	Baseline EvalReport

	// Threshold is the maximum allowable drop in average score for any single
	// provider.  A value of 0.05 means a 5-percentage-point drop is permitted;
	// any larger drop fails the gate.
	//
	// Must be in [0, 1].  Values outside this range are clamped.
	Threshold float64
}

// NewRegressionGate creates a RegressionGate with the given baseline and
// threshold.
func NewRegressionGate(baseline EvalReport, threshold float64) *RegressionGate {
	if threshold < 0 {
		threshold = 0
	}
	if threshold > 1 {
		threshold = 1
	}
	return &RegressionGate{
		Baseline:  baseline,
		Threshold: threshold,
	}
}

// Check compares current against the baseline and reports whether all providers
// are within the allowed threshold.
//
// Returns:
//   - passed == true and regressions == nil if every provider meets the gate.
//   - passed == false and regressions containing human-readable descriptions if
//     one or more providers regressed.
//   - err != nil (wrapping ErrRegressionDetected) when passed is false.
func (g *RegressionGate) Check(current EvalReport) (passed bool, regressions []string, err error) {
	threshold := g.Threshold
	if threshold < 0 {
		threshold = 0
	}
	if threshold > 1 {
		threshold = 1
	}

	var msgs []string

	for pid, baseSummary := range g.Baseline.Summary {
		curSummary, ok := current.Summary[pid]
		if !ok {
			// Provider was in baseline but missing from current — treat as 0 score.
			drop := baseSummary.AvgScore
			if drop > threshold {
				msgs = append(msgs, fmt.Sprintf(
					"provider %q: missing from current report (baseline avg=%.4f, drop=%.4f > threshold=%.4f)",
					pid, baseSummary.AvgScore, drop, threshold,
				))
			}
			continue
		}

		drop := baseSummary.AvgScore - curSummary.AvgScore
		if drop > threshold {
			msgs = append(msgs, fmt.Sprintf(
				"provider %q: avg score dropped from %.4f to %.4f (drop=%.4f > threshold=%.4f)",
				pid, baseSummary.AvgScore, curSummary.AvgScore, drop, threshold,
			))
		}
	}

	if len(msgs) > 0 {
		return false, msgs, fmt.Errorf("%w: %d provider(s) regressed", ErrRegressionDetected, len(msgs))
	}
	return true, nil, nil
}
