package reasoningeval

import "time"

// ExpertReview is a human reviewer's structured assessment of a sampled
// Opinion, distinct from and never merged with an automated QualityScore.
// Where QualityScore reflects what the rubric's mechanical dimension
// scorers measured, ExpertReview captures a legal expert's own judgment —
// the two are stored side by side (see Store) precisely so a caller can
// compare automated and human assessments of the same Opinion without
// either one silently overwriting the other.
type ExpertReview struct {
	// ReviewID is a stable, unique identifier for this review.
	ReviewID string

	// CaseID identifies the case the reviewed Opinion belongs to.
	CaseID string

	// JurisdictionCode is the jurisdiction this case was decided under,
	// mirroring QualityScore.JurisdictionCode for consistent aggregation.
	JurisdictionCode string

	// ReviewerID identifies the human expert who performed this review
	// (e.g. a judge or senior advocate's user ID).
	ReviewerID string

	// Score is the reviewer's own overall quality assessment in [0.0,
	// 1.0]. Unlike QualityScore.Overall, this is a subjective human
	// judgment, not a weighted rubric aggregate.
	Score float64

	// Comments is the reviewer's free-text feedback explaining the score
	// (e.g. "grounding solid but issue 3's conclusion is thin").
	Comments string

	// FlaggedIssues lists TentativeConclusion.IssueNodeID values the
	// reviewer specifically flagged as problematic, if any.
	FlaggedIssues []string

	// ReviewedAt records when this review was submitted.
	ReviewedAt time.Time
}

// Validate reports whether r has the minimum required fields to be
// persisted: a non-empty CaseID, ReviewerID, and a Score within [0,1].
func (r ExpertReview) Validate() error {
	if r.CaseID == "" {
		return ErrEmptyCaseID
	}
	if r.ReviewerID == "" {
		return errEmptyReviewerID
	}
	if r.Score < 0 || r.Score > 1 {
		return errReviewScoreOutOfRange
	}
	return nil
}
