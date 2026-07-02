package evidence

import "time"

// ManualOverride records a human correction to a Classifier's
// determination, letting a reviewer's judgment take precedence over the
// automated classification while preserving the original determination for
// audit (see Classification.Override and ApplyOverride).
type ManualOverride struct {
	// SegmentID identifies the segmentation.Segment being corrected. Must
	// match the SegmentID of the Classification it is applied to.
	SegmentID string

	// Type is the corrected EvidenceType.
	Type EvidenceType

	// Party is the corrected PartyRole.
	Party PartyRole

	// Reason optionally records why the reviewer overrode the classifier
	// (free text, e.g. "misclassified as argument; is sworn testimony").
	Reason string

	// ReviewedBy identifies the human reviewer who made the correction.
	ReviewedBy string

	// ReviewedAt records when the correction was made. If zero,
	// ApplyOverride sets it to time.Now().
	ReviewedAt time.Time

	// Previous carries the classifier's original Classification, exactly as
	// it stood before this override was applied, so the override is
	// recorded distinctly rather than silently overwriting the automated
	// result. Populated by ApplyOverride; callers do not need to set it.
	Previous *Classification
}

// Validate checks that o carries the minimum fields required to be applied:
// a non-empty SegmentID, a recognized EvidenceType, and a Confidence-free
// sanity check deferred to the caller. Returns ErrInvalidOverride if any
// check fails.
func (o ManualOverride) Validate() error {
	if o.SegmentID == "" {
		return ErrInvalidOverride
	}
	if !isKnownEvidenceType(o.Type) {
		return ErrInvalidOverride
	}
	switch o.Party {
	case "", PartyFirst, PartySecond, PartyUnattributed:
		// ok; empty Party is allowed and defaults to PartyUnattributed in
		// ApplyOverride.
	default:
		return ErrInvalidOverride
	}
	return nil
}

// isKnownEvidenceType reports whether t is one of the constants declared in
// taxonomy.go.
func isKnownEvidenceType(t EvidenceType) bool {
	_, ok := evidenceTypeDescriptions[t]
	return ok
}

// ApplyOverride applies a validated ManualOverride to original, returning a
// new Classification whose Type/Party reflect the human correction, whose
// Confidence is 1.0 (a manual override is treated as fully confident), and
// whose Override field records the correction together with a copy of
// original so the classifier's own determination is preserved distinctly
// rather than being discarded.
//
// Returns ErrInvalidOverride if override fails Validate, or if
// override.SegmentID does not match original.SegmentID.
func ApplyOverride(original Classification, override ManualOverride) (Classification, error) {
	if err := override.Validate(); err != nil {
		return Classification{}, err
	}
	if override.SegmentID != original.SegmentID {
		return Classification{}, ErrInvalidOverride
	}

	party := override.Party
	if party == "" {
		party = PartyUnattributed
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

	return Classification{
		SegmentID:  original.SegmentID,
		Type:       override.Type,
		Party:      party,
		Confidence: 1.0,
		Override:   &applied,
	}, nil
}
