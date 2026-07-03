package signoff

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrNotFound is returned by Repository.Get/GetLatest when no
	// sign-off record matches the requested case (or the tenant scope
	// hides it — see ErrCrossTenantAccess).
	ErrNotFound = errors.New("signoff: record not found")

	// ErrCrossTenantAccess is returned by Repository methods when
	// asked to operate on a SignoffRecord whose TenantID does not
	// match the scope's tenantID, mirroring
	// packages/caselifecycle.ErrCrossTenantAccess.
	ErrCrossTenantAccess = errors.New("signoff: cross-tenant access denied")

	// ErrEmptyCaseID is returned when an operation is called with an
	// empty/nil case ID.
	ErrEmptyCaseID = errors.New("signoff: case id is required")

	// ErrUnauthenticated is returned when an operation requiring an
	// actor is called with a context carrying no authenticated
	// identity.User.
	ErrUnauthenticated = errors.New("signoff: unauthenticated request")

	// ErrForbidden is returned when the authenticated actor lacks
	// identity.PermSignOff.
	ErrForbidden = errors.New("signoff: actor lacks required permission")

	// ErrAcknowledgementRequired is returned by Approve/Reject when the
	// caller does not supply the explicit acknowledgement confirmation
	// string. Sign-off must never happen implicitly.
	ErrAcknowledgementRequired = errors.New("signoff: explicit acknowledgement is required")

	// ErrNotesRequired is returned by Reject when called with blank
	// notes: a rejection must always be self-documenting.
	ErrNotesRequired = errors.New("signoff: notes are required to reject a case")

	// ErrNilRepository is returned by constructors/functions that
	// require a non-nil Repository.
	ErrNilRepository = errors.New("signoff: repository must not be nil")

	// ErrNilCaseReader is returned by constructors/functions that
	// require a non-nil CaseVersionReader.
	ErrNilCaseReader = errors.New("signoff: case version reader must not be nil")

	// ErrCaseVersionMismatch is returned by Approve/Reject when the
	// caller-supplied CaseVersion does not match the case's current
	// version as reported by CaseVersionReader, signaling the reviewer
	// was looking at stale case content.
	ErrCaseVersionMismatch = errors.New("signoff: case version has changed since review began")
)
