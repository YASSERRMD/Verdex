package reasoningeval

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrEmptyRubric is returned when Score is called with a Rubric that
	// has no Dimensions.
	ErrEmptyRubric = errors.New("reasoningeval: rubric has no dimensions")

	// ErrNilGroundingReport is returned when Score is called with a nil
	// grounding report input required by the Grounding dimension.
	ErrNilGroundingReport = errors.New("reasoningeval: grounding report is required")

	// ErrEmptyCaseID is returned when a function requiring a case ID is
	// called with an empty string.
	ErrEmptyCaseID = errors.New("reasoningeval: case id is required")

	// ErrEmptyJurisdiction is returned when a function requiring a
	// jurisdiction code is called with an empty string.
	ErrEmptyJurisdiction = errors.New("reasoningeval: jurisdiction code is required")

	// ErrScoreNotFound is returned when a store lookup for a QualityScore
	// finds no matching record.
	ErrScoreNotFound = errors.New("reasoningeval: quality score not found")

	// ErrReviewNotFound is returned when a store lookup for an
	// ExpertReview finds no matching record.
	ErrReviewNotFound = errors.New("reasoningeval: expert review not found")

	// ErrNoScores is returned when regression detection or aggregation is
	// attempted over an empty set of QualityScores.
	ErrNoScores = errors.New("reasoningeval: no quality scores available")

	// ErrRegressionDetected is returned by
	// RegressionDetector.CompareErr when the current run's average score
	// drops below the baseline by more than the configured threshold.
	ErrRegressionDetected = errors.New("reasoningeval: quality regression detected")

	// ErrUnauthenticated is returned by access-control checks when ctx
	// carries no authenticated identity.User.
	ErrUnauthenticated = errors.New("reasoningeval: unauthenticated request")

	// ErrForbidden is returned by access-control checks when the
	// authenticated actor lacks the required permission.
	ErrForbidden = errors.New("reasoningeval: actor lacks required permission")

	// errEmptyReviewerID is returned by ExpertReview.Validate when
	// ReviewerID is empty.
	errEmptyReviewerID = errors.New("reasoningeval: reviewer id is required")

	// errReviewScoreOutOfRange is returned by ExpertReview.Validate when
	// Score is outside [0, 1].
	errReviewScoreOutOfRange = errors.New("reasoningeval: review score must be in [0, 1]")
)
