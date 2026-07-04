package pilot

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// DimensionName identifies one named axis of reasoning quality a
// FeedbackEntry rates. Deliberately mirrors
// packages/reasoningeval.DimensionName's identical string-type shape
// and its three built-in axes (grounding, citation, coherence) rather
// than importing that type directly: packages/reasoningeval's own
// go.mod pulls in the full reasoning stack (packages/grounding,
// packages/synthesisagent, packages/treeassembly,
// packages/vectorindex, the neo4j driver, and more) purely to define a
// three-field scoring struct, and this package's expert-feedback
// collection has no need for any of that machinery -- see doc/pilot.md
// ("Why FeedbackEntry mirrors reasoningeval.DimensionName instead of
// importing it") for the full rationale. The two types' Name values
// are kept string-identical by convention so a caller that already
// holds a reasoningeval.DimensionName can pass its String() value
// here unchanged.
type DimensionName string

const (
	// DimensionGrounding rates how well the reviewed opinion's
	// assertions are grounded in the case's own facts and law, mirroring
	// packages/reasoningeval.DimensionGrounding by name.
	DimensionGrounding DimensionName = "grounding"

	// DimensionCitation rates citation fidelity, mirroring
	// packages/reasoningeval.DimensionCitation by name.
	DimensionCitation DimensionName = "citation"

	// DimensionCoherence rates structural coherence of the opinion's own
	// text, mirroring packages/reasoningeval.DimensionCoherence by name.
	DimensionCoherence DimensionName = "coherence"

	// DimensionUsefulness rates how useful the reviewer found the
	// analysis for their actual supervisory work -- a pilot-specific
	// axis with no reasoningeval counterpart, since "was this useful to
	// a human supervisor in practice" is a question only a live pilot
	// can answer.
	DimensionUsefulness DimensionName = "usefulness"
)

// IsValid reports whether n is one of the named DimensionName
// constants. Unlike packages/reasoningeval.DimensionName (deliberately
// open to caller-registered custom dimensions), this package's
// dimension set is closed: a FeedbackEntry is filled out by a human
// through a fixed structured form, not assembled programmatically from
// a caller-supplied Rubric, so there is no extensibility requirement
// to preserve.
func (n DimensionName) IsValid() bool {
	switch n {
	case DimensionGrounding, DimensionCitation, DimensionCoherence, DimensionUsefulness:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (n DimensionName) String() string { return string(n) }

// TrustRating is a reviewer's overall trust assessment of the reviewed
// opinion, on a discrete five-point scale distinct from the continuous
// [0,1] per-dimension scores below -- a human supervisor answering
// "would you trust this enough to build on it" reasons in a small
// number of discrete buckets, not a floating-point score.
type TrustRating int

const (
	// TrustVeryLow means the reviewer would not rely on this output at
	// all.
	TrustVeryLow TrustRating = 1

	// TrustLow means the reviewer has significant reservations.
	TrustLow TrustRating = 2

	// TrustModerate means the reviewer would use this output with
	// careful independent verification.
	TrustModerate TrustRating = 3

	// TrustHigh means the reviewer would rely on this output with only
	// light verification.
	TrustHigh TrustRating = 4

	// TrustVeryHigh means the reviewer would rely on this output
	// directly.
	TrustVeryHigh TrustRating = 5
)

// IsValid reports whether r falls within the named 1-5 TrustRating
// scale.
func (r TrustRating) IsValid() bool {
	return r >= TrustVeryLow && r <= TrustVeryHigh
}

// DimensionRating pairs a DimensionName with the reviewer's [0.0, 1.0]
// score on that axis, mirroring
// packages/reasoningeval.QualityScore.PerDimension's per-dimension map
// shape but as a slice of named pairs, since a FeedbackEntry's
// dimension ratings are entered once by a human reviewer through a
// fixed form rather than computed and re-aggregated programmatically.
type DimensionRating struct {
	// Dimension is the rated quality axis.
	Dimension DimensionName `json:"dimension"`

	// Score is the reviewer's [0.0, 1.0] rating on Dimension, where 1.0
	// is the best possible score.
	Score float64 `json:"score"`
}

// IsValid reports whether d has a recognized Dimension and a Score
// within [0,1].
func (d DimensionRating) IsValid() bool {
	return d.Dimension.IsValid() && d.Score >= 0 && d.Score <= 1
}

// FeedbackEntry is a reviewer's structured feedback on a single
// PilotCase (task 4): reviewer identity, structured per-dimension
// ratings, an overall TrustRating, and free-text comments. Distinct
// from and never merged with an automated
// packages/reasoningeval.QualityScore -- AggregateQuality
// (aggregate.go) combines the two side by side rather than either one
// silently overwriting the other, mirroring
// packages/reasoningeval.ExpertReview's own "stored side by side, never
// merged" doc comment.
type FeedbackEntry struct {
	// ID uniquely identifies this feedback entry.
	ID uuid.UUID `json:"id"`

	// TenantID is the tenant this feedback entry belongs to.
	TenantID uuid.UUID `json:"tenant_id"`

	// PilotCaseID identifies the PilotCase this feedback was collected
	// for.
	PilotCaseID uuid.UUID `json:"pilot_case_id"`

	// ReviewerUserID is the identity.User who submitted this feedback.
	ReviewerUserID uuid.UUID `json:"reviewer_user_id"`

	// Ratings is the structured per-dimension rating set. Must contain
	// at least one entry, each individually valid per
	// DimensionRating.IsValid.
	Ratings []DimensionRating `json:"ratings"`

	// Trust is the reviewer's overall trust assessment.
	Trust TrustRating `json:"trust"`

	// Comments is the reviewer's free-text feedback.
	Comments string `json:"comments,omitempty"`

	// SubmittedAt is when this feedback was recorded.
	SubmittedAt time.Time `json:"submitted_at"`

	// CreatedAt and UpdatedAt are bookkeeping timestamps.
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Validate checks f for structural well-formedness.
func (f *FeedbackEntry) Validate() error {
	if f == nil {
		return ErrInvalidFeedback
	}
	if f.TenantID == uuid.Nil {
		return ErrEmptyTenantID
	}
	if f.PilotCaseID == uuid.Nil {
		return wrapf("FeedbackEntry.Validate", ErrInvalidFeedback)
	}
	if f.ReviewerUserID == uuid.Nil {
		return wrapf("FeedbackEntry.Validate", ErrInvalidFeedback)
	}
	if len(f.Ratings) == 0 {
		return wrapf("FeedbackEntry.Validate", ErrInvalidFeedback)
	}
	for _, r := range f.Ratings {
		if !r.IsValid() {
			return wrapf("FeedbackEntry.Validate", ErrInvalidFeedback)
		}
	}
	if !f.Trust.IsValid() {
		return wrapf("FeedbackEntry.Validate", ErrInvalidFeedback)
	}
	if f.SubmittedAt.IsZero() {
		return wrapf("FeedbackEntry.Validate", ErrInvalidFeedback)
	}
	return nil
}

// OverallScore returns the arithmetic mean of f's per-dimension
// Ratings scores, or 0 if f has no ratings.
func (f FeedbackEntry) OverallScore() float64 {
	if len(f.Ratings) == 0 {
		return 0
	}
	sum := 0.0
	for _, r := range f.Ratings {
		sum += r.Score
	}
	return sum / float64(len(f.Ratings))
}

// ScoreFor returns the Score recorded for dimension, and whether a
// rating for that dimension is present at all.
func (f FeedbackEntry) ScoreFor(dimension DimensionName) (float64, bool) {
	for _, r := range f.Ratings {
		if r.Dimension == dimension {
			return r.Score, true
		}
	}
	return 0, false
}

// trimmedComments returns f.Comments with surrounding whitespace
// removed, used by repositories/audit recording so a caller's stray
// whitespace never leaks into persisted or audited data.
func trimmedComments(s string) string {
	return strings.TrimSpace(s)
}
