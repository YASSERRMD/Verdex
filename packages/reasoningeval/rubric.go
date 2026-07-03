package reasoningeval

// DefaultRubric returns the standard three-dimension quality rubric:
// grounding, citation, and coherence, weighted evenly by default. Callers
// needing different weights should construct their own Rubric from the
// same Dimension scorers (GroundingDimension, CitationDimension,
// CoherenceDimension) rather than mutating the result of this function.
func DefaultRubric() Rubric {
	return Rubric{
		Name: "default-v1",
		Dimensions: []Dimension{
			GroundingDimension(0.4),
			CitationDimension(0.3),
			CoherenceDimension(0.3),
		},
	}
}

// GroundingDimension returns a Dimension that scores
// ScoreInput.GroundingReport.OpinionScoreValue() directly: this package
// never re-derives grounding confidence, it only folds
// packages/grounding's own verdict into the weighted rubric.
func GroundingDimension(weight float64) Dimension {
	return Dimension{
		Name:   DimensionGrounding,
		Weight: weight,
		Scorer: func(input ScoreInput) (float64, error) {
			if input.GroundingReport == nil {
				return 0, ErrNilGroundingReport
			}
			return clamp01(input.GroundingReport.OpinionScoreValue()), nil
		},
	}
}

// CitationDimension returns a Dimension that scores the fraction of
// citation findings (drawn from ScoreInput.GroundingReport, which already
// carries packages/citation.Finding values via
// grounding.Report.AllCitationFindings) that are NOT critical.
//
// Score = 1.0 when there are no citation findings at all (nothing to
// flag). Otherwise score = 1.0 - (critical findings / total findings),
// so an opinion whose only citation findings are non-critical warnings
// still scores well, while any citation with a critical finding
// (hallucinated, wrong-case) drags the score down proportionally to how
// much of the opinion's citation surface is affected.
func CitationDimension(weight float64) Dimension {
	return Dimension{
		Name:   DimensionCitation,
		Weight: weight,
		Scorer: func(input ScoreInput) (float64, error) {
			if input.GroundingReport == nil {
				return 0, ErrNilGroundingReport
			}
			total := input.GroundingReport.CitationFindingCount()
			if total == 0 {
				return 1.0, nil
			}
			critical := input.GroundingReport.CriticalCitationFindingCount()
			return clamp01(1.0 - float64(critical)/float64(total)), nil
		},
	}
}

// clamp01 clamps v into [0.0, 1.0], mirroring packages/eval's
// applyRubric clamping convention.
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// applyRubric scores input against every Dimension in rubric and returns
// the weighted aggregate (normalised to [0,1]) plus per-dimension raw
// scores, mirroring packages/eval.applyRubric's algorithm exactly.
func applyRubric(input ScoreInput, rubric Rubric) (float64, map[DimensionName]float64, error) {
	if len(rubric.Dimensions) == 0 {
		return 0, nil, ErrEmptyRubric
	}

	perDimension := make(map[DimensionName]float64, len(rubric.Dimensions))
	var totalWeight, weightedSum float64

	for _, d := range rubric.Dimensions {
		raw, err := d.Scorer(input)
		if err != nil {
			return 0, nil, err
		}
		raw = clamp01(raw)
		perDimension[d.Name] = raw
		weightedSum += raw * d.Weight
		totalWeight += d.Weight
	}

	if totalWeight == 0 {
		return 0, perDimension, nil
	}
	return weightedSum / totalWeight, perDimension, nil
}
