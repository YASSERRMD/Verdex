package reasoningprofile

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrUnknownFamily is returned when a Family value is not one of the
	// four canonical legal families this package recognizes.
	ErrUnknownFamily = errors.New("reasoningprofile: unrecognized legal family")

	// ErrInvalidWeight is returned by Validate when a Weights field is
	// NaN, negative, or greater than 1.0.
	ErrInvalidWeight = errors.New("reasoningprofile: weight must be a finite value in [0, 1]")

	// ErrEmptyCaseID is returned by SetOverride and OverrideFor when
	// called with an empty case ID.
	ErrEmptyCaseID = errors.New("reasoningprofile: case id is required")
)
