package issue

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrNoSegments is returned when an identification or extraction
	// operation is given an empty segment batch.
	ErrNoSegments = errors.New("issue: no segments provided")

	// ErrEmptyInput is returned when an operation is given empty (or
	// whitespace-only, after trimming) required text input.
	ErrEmptyInput = errors.New("issue: input is empty")

	// ErrIssueNotFound is returned when a lookup by CandidateIssue ID (or
	// an override targeting an unknown issue) finds no matching record.
	ErrIssueNotFound = errors.New("issue: issue not found")

	// ErrInvalidOverride is returned when a ManualOverride fails basic
	// validation (e.g. empty IssueID or empty corrected Text) before
	// being applied.
	ErrInvalidOverride = errors.New("issue: invalid manual override")

	// ErrPersistFailed is returned when IssueExtractionService fails to
	// persist one or more irac.IssueNode values via graph.GraphStore.
	ErrPersistFailed = errors.New("issue: failed to persist issue node")
)
