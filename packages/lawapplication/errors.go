package lawapplication

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrEmptyCaseID is returned when a case ID is required but empty.
	ErrEmptyCaseID = errors.New("lawapplication: case id is required")

	// ErrNoIssues is returned when Apply is called with no FramedIssues
	// to analyze at all.
	ErrNoIssues = errors.New("lawapplication: no issues supplied")

	// ErrCaseIDMismatch is returned when a supplied ArgumentSet or
	// evidenceweighing.Result references a case ID other than the one
	// Apply was asked to analyze.
	ErrCaseIDMismatch = errors.New("lawapplication: case id mismatch across inputs")

	// ErrResultNotFound is returned by a Repository's Get when no
	// Result was ever saved for the requested case.
	ErrResultNotFound = errors.New("lawapplication: no law application result found for case")
)
