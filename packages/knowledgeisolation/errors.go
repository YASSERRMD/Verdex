package knowledgeisolation

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrCrossCaseAccess is returned by CaseScopedStore (and the
	// retrieval-layer guards built on it) when a caller attempts to
	// read or write a case-scoped node/edge/record belonging to a case
	// other than the one the guard is scoped to. Shared-law nodes (see
	// IsSharedLawNode) never trigger this error, regardless of which
	// case originally indexed them.
	ErrCrossCaseAccess = errors.New("knowledgeisolation: cross-case access rejected")

	// ErrEmptyCaseID is returned when a case-scoped operation is
	// invoked with an empty CaseID, either as the guard's own
	// authorized case or as part of the request being validated.
	ErrEmptyCaseID = errors.New("knowledgeisolation: case id is required")

	// ErrNilStore is returned when a guard constructor is given a nil
	// inner store to wrap.
	ErrNilStore = errors.New("knowledgeisolation: inner store must not be nil")

	// ErrMissingAuthorization is returned by the cross-case analysis
	// escape hatch when no valid CrossCaseAuthorization is presented.
	ErrMissingAuthorization = errors.New("knowledgeisolation: missing cross-case authorization")

	// ErrAuthorizationExpired is returned by the cross-case analysis
	// escape hatch when a CrossCaseAuthorization's expiry has passed.
	ErrAuthorizationExpired = errors.New("knowledgeisolation: cross-case authorization expired")

	// ErrCaseNotAuthorized is returned by the cross-case analysis escape
	// hatch when a CrossCaseAuthorization does not cover one of the
	// requested cases.
	ErrCaseNotAuthorized = errors.New("knowledgeisolation: case not covered by authorization")
)
