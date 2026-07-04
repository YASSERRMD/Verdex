package vulnmanagement

import (
	"errors"
	"fmt"
)

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrUnauthenticated is returned when an operation requiring an
	// actor is called with a context carrying no authenticated
	// identity.User.
	ErrUnauthenticated = errors.New("vulnmanagement: unauthenticated request")

	// ErrForbidden is returned when the authenticated actor lacks the
	// identity.Permission a vulnerability-management operation
	// requires.
	ErrForbidden = errors.New("vulnmanagement: actor lacks required permission")

	// ErrCrossTenantAccess is returned when an operation would read or
	// write a record whose TenantID does not match the caller's scope,
	// mirroring packages/compliance.ErrCrossTenantAccess and
	// packages/privacy.ErrCrossTenantAccess exactly.
	ErrCrossTenantAccess = errors.New("vulnmanagement: cross-tenant access denied")

	// ErrEmptyTenantID is returned when an operation is called with a
	// zero tenant ID.
	ErrEmptyTenantID = errors.New("vulnmanagement: tenant id is required")

	// ErrNilStore is returned by constructors that require a non-nil
	// backing store/repository.
	ErrNilStore = errors.New("vulnmanagement: store must not be nil")

	// ErrNilAuditSink is returned by constructors that require a
	// non-nil audit sink composing with packages/auditlog.Store.
	ErrNilAuditSink = errors.New("vulnmanagement: audit sink must not be nil")

	// ErrInvalidFinding is returned when a Finding fails structural
	// validation.
	ErrInvalidFinding = errors.New("vulnmanagement: invalid finding")

	// ErrFindingNotFound is returned when a referenced Finding ID does
	// not resolve to any stored record for the tenant.
	ErrFindingNotFound = errors.New("vulnmanagement: finding not found")

	// ErrIllegalStatusTransition is returned when a Finding status
	// change is not a permitted move (see CanTransition).
	ErrIllegalStatusTransition = errors.New("vulnmanagement: illegal finding status transition")

	// ErrInvalidTriageDecision is returned when a TriageDecision fails
	// structural validation (e.g. blank notes, invalid target status).
	ErrInvalidTriageDecision = errors.New("vulnmanagement: invalid triage decision")

	// ErrNotesRequired is returned when Triage is called with blank
	// notes -- every triage decision must explain itself, mirroring
	// packages/signoff.Reject's non-blank-Notes requirement.
	ErrNotesRequired = errors.New("vulnmanagement: triage notes are required")

	// ErrInvalidLicenseCheck is returned when a LicenseCheck fails
	// structural validation.
	ErrInvalidLicenseCheck = errors.New("vulnmanagement: invalid license check")
)

// wrapf mirrors the fmt.Errorf("pkg: fn: %w", err) convention used
// throughout this repository's packages.
func wrapf(fn string, err error) error {
	return fmt.Errorf("vulnmanagement: %s: %w", fn, err)
}
