package evidenceweighing

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrEmptyCaseID is returned when a case ID is required but empty.
	ErrEmptyCaseID = errors.New("evidenceweighing: case id is required")

	// ErrNoArgumentSets is returned when Weigh is called with both parties'
	// ArgumentSets empty (no arguments to weigh evidence against at all).
	ErrNoArgumentSets = errors.New("evidenceweighing: no argument sets supplied")

	// ErrCaseIDMismatch is returned when the supplied ArgumentSets or tree
	// nodes reference a case ID other than the one Weigh was asked to
	// weigh evidence for.
	ErrCaseIDMismatch = errors.New("evidenceweighing: case id mismatch across inputs")

	// ErrResultNotFound is returned by a Repository's Get when no
	// EvidenceWeighingResult was ever saved for the requested case.
	ErrResultNotFound = errors.New("evidenceweighing: no weighing result found for case")
)
