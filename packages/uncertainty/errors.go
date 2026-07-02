package uncertainty

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrEmptyCaseID is returned when a case ID is required but empty.
	ErrEmptyCaseID = errors.New("uncertainty: case id is required")

	// ErrCaseIDMismatch is returned when one of the four upstream results
	// supplied to Surface carries a CaseID other than the one requested.
	ErrCaseIDMismatch = errors.New("uncertainty: case id mismatch across inputs")

	// ErrNoFramedIssues is returned when the supplied
	// issueagent.IssueAnalysisResult has no issues, leaving no
	// materiality context to rank uncertainties against.
	ErrNoFramedIssues = errors.New("uncertainty: no framed issues supplied")
)
