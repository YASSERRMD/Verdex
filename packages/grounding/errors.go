package grounding

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrEmptyCaseID is returned when a function requiring a case ID is
	// called with an empty string.
	ErrEmptyCaseID = errors.New("grounding: case id is required")

	// ErrNilGraphStore is returned when Check is called with a nil
	// graph.GraphStore.
	ErrNilGraphStore = errors.New("grounding: graph store is required")

	// ErrOpinionCaseMismatch is returned when Check is called with an
	// Opinion whose CaseID does not match the caseID argument — a caller
	// error, not a grounding finding, since verifying one case's opinion
	// against a mismatched case ID would silently produce meaningless
	// results.
	ErrOpinionCaseMismatch = errors.New("grounding: opinion case id does not match requested case id")

	// ErrUnauthenticated is returned by RequireCheckPermission when ctx
	// carries no authenticated identity.User.
	ErrUnauthenticated = errors.New("grounding: unauthenticated request")

	// ErrForbidden is returned by RequireCheckPermission when the
	// authenticated actor lacks identity.PermViewCase.
	ErrForbidden = errors.New("grounding: actor lacks required permission")

	// ErrCriticalFindings is returned by CanFinalize when report contains
	// at least one SeverityCritical Finding.
	ErrCriticalFindings = errors.New("grounding: report has critical grounding finding(s)")

	// ErrNilReport is returned by CanFinalize when called with a nil
	// Report pointer, distinguishing "no report was ever computed" from a
	// zero-value Report that legitimately has no findings.
	ErrNilReport = errors.New("grounding: report is required")
)
