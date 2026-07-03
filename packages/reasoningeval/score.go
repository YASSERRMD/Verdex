package reasoningeval

import "time"

// nowFunc is overridden in tests for deterministic QualityScore.ScoredAt
// values, mirroring packages/grounding's nowFunc convention.
var nowFunc = time.Now

// Score runs rubric over a single ScoreInput and returns a QualityScore.
//
// runID identifies the evaluation/model/template run this score belongs
// to (e.g. a deployment version or batch identifier) and is copied
// verbatim onto the result — see RegressionDetector for how it is later
// used to compare two runs.
//
// Returns ErrEmptyRubric if rubric.Dimensions is empty, or any error
// returned by an individual Dimension.Scorer (e.g. ErrNilGroundingReport
// when GroundingReport is nil but a dimension requires it).
func Score(input ScoreInput, rubric Rubric, runID string) (QualityScore, error) {
	overall, perDimension, err := applyRubric(input, rubric)
	if err != nil {
		return QualityScore{}, err
	}

	caseID := input.CaseID
	if caseID == "" && input.Opinion != nil {
		caseID = input.Opinion.OpinionCaseID()
	}

	return QualityScore{
		CaseID:           caseID,
		JurisdictionCode: input.JurisdictionCode,
		LegalFamily:      input.LegalFamily,
		RubricName:       rubric.Name,
		RunID:            runID,
		Overall:          overall,
		PerDimension:     perDimension,
		ScoredAt:         nowFunc().UTC(),
	}, nil
}

// ScoreBatch runs Score over every input in inputs using the same rubric
// and runID, returning one QualityScore per input in the same order.
// Processing stops at the first error encountered, mirroring
// packages/eval.EvalRunner.RunAll's fail-fast-but-return-partial
// convention would be inappropriate here since a single bad input (e.g.
// missing GroundingReport) should not silently drop it from the batch —
// callers that want partial-success semantics should call Score
// individually per input and handle errors themselves.
func ScoreBatch(inputs []ScoreInput, rubric Rubric, runID string) ([]QualityScore, error) {
	scores := make([]QualityScore, 0, len(inputs))
	for _, input := range inputs {
		s, err := Score(input, rubric, runID)
		if err != nil {
			return scores, err
		}
		scores = append(scores, s)
	}
	return scores, nil
}
