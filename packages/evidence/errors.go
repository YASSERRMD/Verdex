package evidence

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrEmptyInput is returned when a classification operation is given a
	// segment with empty (or whitespace-only, after trimming) text.
	ErrEmptyInput = errors.New("evidence: segment text is empty")

	// ErrSegmentNotFound is returned when a lookup by segment ID (via
	// ClassificationStore.Get, or an override targeting an unknown segment)
	// finds no matching record.
	ErrSegmentNotFound = errors.New("evidence: segment not found")

	// ErrInvalidOverride is returned when a ManualOverride fails basic
	// validation (e.g. empty SegmentID, unrecognized EvidenceType, or
	// Confidence outside [0, 1]) before being applied.
	ErrInvalidOverride = errors.New("evidence: invalid manual override")
)
