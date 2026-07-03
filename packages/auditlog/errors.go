package auditlog

import (
	"errors"
	"fmt"
)

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrNilEvent is returned when a nil *Event is passed to Append or a
	// method that requires one.
	ErrNilEvent = errors.New("auditlog: event must not be nil")

	// ErrEmptyTenantID is returned when an operation is called with, or
	// an event carries, a zero tenant ID.
	ErrEmptyTenantID = errors.New("auditlog: tenant id is required")

	// ErrEmptyAction is returned when an Event has a blank Action.
	ErrEmptyAction = errors.New("auditlog: action is required")

	// ErrEmptyActor is returned when an Event has a blank Actor.
	ErrEmptyActor = errors.New("auditlog: actor is required")

	// ErrCrossTenantAccess is returned when an operation targets an
	// Event whose TenantID does not match the scope's tenantID,
	// mirroring packages/keymanagement.ErrCrossTenantAccess and
	// packages/caseversioning.ErrCrossTenantAccess exactly.
	ErrCrossTenantAccess = errors.New("auditlog: cross-tenant access denied")

	// ErrUnauthenticated is returned when an operation requiring an
	// actor is called with a context carrying no authenticated
	// identity.User.
	ErrUnauthenticated = errors.New("auditlog: unauthenticated request")

	// ErrForbidden is returned when the authenticated actor lacks the
	// identity.Permission an audit operation requires (identity.PermAuditRead
	// for Query/Export, a stricter admin-only check for Purge).
	ErrForbidden = errors.New("auditlog: actor lacks required permission")

	// ErrNilRepository is returned by constructors that require a
	// non-nil Store/Repository.
	ErrNilRepository = errors.New("auditlog: repository must not be nil")

	// ErrChainBroken is returned by VerifyChain (and wrapped with the
	// index/ID of the first tampered record) when a stored ChainHash
	// does not match the recomputed expected hash.
	ErrChainBroken = errors.New("auditlog: hash chain integrity check failed")

	// ErrInvalidRetention is returned when a RetentionPolicy's window is
	// zero or negative.
	ErrInvalidRetention = errors.New("auditlog: retention window must be positive")

	// ErrInvalidExportFormat is returned when Export is asked to render
	// a format other than ExportFormatCSV or ExportFormatJSON.
	ErrInvalidExportFormat = errors.New("auditlog: unsupported export format")
)

// wrapf mirrors the fmt.Errorf("pkg: fn: %w", err) convention used
// throughout this repository's packages.
func wrapf(fn string, err error) error {
	return fmt.Errorf("auditlog: %s: %w", fn, err)
}
