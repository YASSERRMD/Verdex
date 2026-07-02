package category

import "time"

// CategoryAssignment is the result of categorizing a single case: the
// Category chosen, its source Suggestion (if any), and room for a human
// ManualOverride to take precedence over the suggestion while the original
// suggestion remains retained for audit (see Override.Previous below).
type CategoryAssignment struct {
	// CaseID identifies the case this assignment describes.
	CaseID string

	// Category is the final category assigned to the case (the override's
	// category when Override is non-nil, otherwise the top-ranked
	// suggestion's category).
	Category Category

	// Confidence is this assignment's confidence score, in the closed
	// interval [0, 1].
	Confidence float64

	// Suggestions carries every Suggestion produced by a Suggester for this
	// case, retained regardless of whether an override was later applied.
	Suggestions []Suggestion

	// Override, when non-nil, records a human correction that takes
	// precedence over the suggested Category — see ApplyOverride. The
	// suggestion-derived determination is preserved distinctly on
	// Override.Previous rather than being overwritten.
	Override *ManualOverride
}

// ManualOverride records a human correction to a case's suggested category,
// letting a reviewer's judgment take precedence over the automated
// suggestion while preserving the original determination for audit —
// mirroring packages/evidence's ManualOverride/ApplyOverride pattern.
type ManualOverride struct {
	// CaseID identifies the case being corrected. Must match the CaseID of
	// the CategoryAssignment it is applied to.
	CaseID string

	// Category is the corrected Category.
	Category Category

	// Reason optionally records why the reviewer overrode the suggestion
	// (free text, e.g. "misclassified as civil; is a consumer complaint").
	Reason string

	// ReviewedBy identifies the human reviewer who made the correction.
	ReviewedBy string

	// ReviewedAt records when the correction was made. If zero,
	// ApplyOverride sets it to time.Now().
	ReviewedAt time.Time

	// Previous carries the assignment's original CategoryAssignment, exactly
	// as it stood before this override was applied, so the override is
	// recorded distinctly rather than silently overwriting the automated
	// result. Populated by ApplyOverride; callers do not need to set it.
	Previous *CategoryAssignment
}

// Validate checks that o carries the minimum fields required to be applied:
// a non-empty CaseID and a non-empty Category.Code. Returns
// ErrInvalidOverride if any check fails.
func (o ManualOverride) Validate() error {
	if o.CaseID == "" {
		return ErrInvalidOverride
	}
	if o.Category.Code == "" {
		return ErrInvalidOverride
	}
	return nil
}

// ApplyOverride applies a validated ManualOverride to original, returning a
// new CategoryAssignment whose Category reflects the human correction,
// whose Confidence is 1.0 (a manual override is treated as fully
// confident), and whose Override field records the correction together
// with a copy of original so the suggestion-derived determination is
// preserved distinctly rather than being discarded. original.Suggestions is
// carried forward unchanged so both the override and the original
// suggestions remain retained.
//
// Returns ErrInvalidOverride if override fails Validate, or if
// override.CaseID does not match original.CaseID.
func ApplyOverride(original CategoryAssignment, override ManualOverride) (CategoryAssignment, error) {
	if err := override.Validate(); err != nil {
		return CategoryAssignment{}, err
	}
	if override.CaseID != original.CaseID {
		return CategoryAssignment{}, ErrInvalidOverride
	}

	reviewedAt := override.ReviewedAt
	if reviewedAt.IsZero() {
		reviewedAt = time.Now()
	}

	previous := original
	previous.Override = nil // the preserved snapshot never nests a prior override

	applied := override
	applied.ReviewedAt = reviewedAt
	applied.Previous = &previous

	return CategoryAssignment{
		CaseID:      original.CaseID,
		Category:    override.Category,
		Confidence:  1.0,
		Suggestions: original.Suggestions,
		Override:    &applied,
	}, nil
}
