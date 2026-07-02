package issue

import (
	"strings"
	"time"
)

// ManualOverride records a human correction to a CandidateIssue's text
// and/or materiality determination, letting a reviewer's judgment take
// precedence over the automated extraction while preserving the original
// candidate for audit — mirroring packages/evidence's ManualOverride/
// ApplyOverride pattern (see packages/evidence/override.go).
type ManualOverride struct {
	// IssueID identifies the CandidateIssue being corrected. Must match
	// the ID of the CandidateIssue it is applied to.
	IssueID string

	// Text is the corrected issue text. Required (non-empty).
	Text string

	// Material records the reviewer's determination of whether this issue
	// is material to the case's disposition. Overrides do not carry an
	// automated materiality signal today, so this field exists purely as
	// human-supplied metadata.
	Material bool

	// Reason optionally records why the reviewer overrode the extractor
	// (free text, e.g. "not a genuine dispute; both parties agree").
	Reason string

	// ReviewedBy identifies the human reviewer who made the correction.
	ReviewedBy string

	// ReviewedAt records when the correction was made. If zero,
	// ApplyOverride sets it to time.Now().
	ReviewedAt time.Time

	// Previous carries the extractor's original CandidateIssue, exactly
	// as it stood before this override was applied, so the override is
	// recorded distinctly rather than silently overwriting the automated
	// result. Populated by ApplyOverride; callers do not need to set it.
	Previous *CandidateIssue
}

// Validate checks that o carries the minimum fields required to be
// applied: a non-empty IssueID and non-empty (non-whitespace) Text.
// Returns ErrInvalidOverride if any check fails.
func (o ManualOverride) Validate() error {
	if strings.TrimSpace(o.IssueID) == "" {
		return ErrInvalidOverride
	}
	if strings.TrimSpace(o.Text) == "" {
		return ErrInvalidOverride
	}
	return nil
}

// ApplyOverride applies a validated ManualOverride to original, returning
// a new CandidateIssue whose Text reflects the human correction, whose
// Confidence is 1.0 (a manual override is treated as fully confident,
// mirroring packages/evidence.ApplyOverride), and which preserves
// original as a distinct snapshot on the returned issue's provenance via
// the override itself (see OverriddenIssue).
//
// Returns ErrInvalidOverride if override fails Validate, or if
// override.IssueID does not match original.ID.
func ApplyOverride(original CandidateIssue, override ManualOverride) (OverriddenIssue, error) {
	if err := override.Validate(); err != nil {
		return OverriddenIssue{}, err
	}
	if override.IssueID != original.ID {
		return OverriddenIssue{}, ErrInvalidOverride
	}

	reviewedAt := override.ReviewedAt
	if reviewedAt.IsZero() {
		reviewedAt = time.Now()
	}

	previous := original
	applied := override
	applied.ReviewedAt = reviewedAt
	applied.Previous = &previous

	corrected := original
	corrected.Text = override.Text
	corrected.Confidence = 1.0

	return OverriddenIssue{
		CandidateIssue: corrected,
		Override:       applied,
	}, nil
}

// OverriddenIssue is a CandidateIssue whose Text/Confidence reflect a
// human ManualOverride, with the override (and the original candidate it
// preserves via Override.Previous) retained alongside it rather than
// discarded.
type OverriddenIssue struct {
	CandidateIssue

	// Override records the human correction applied to produce this
	// issue's Text/Confidence, including the original candidate on
	// Override.Previous.
	Override ManualOverride
}
