package threatmodel

import (
	"errors"
	"fmt"
)

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrUnauthenticated is returned when an operation requiring an
	// actor is called with a context carrying no authenticated
	// identity.User.
	ErrUnauthenticated = errors.New("threatmodel: unauthenticated request")

	// ErrForbidden is returned when the authenticated actor lacks the
	// identity.Permission a threatmodel operation requires.
	ErrForbidden = errors.New("threatmodel: actor lacks required permission")

	// ErrCrossTenantAccess is returned when an operation would record
	// or query a mitigation-transition audit event under a tenant
	// scope that does not match the caller's own tenant, mirroring
	// packages/privacy.ErrCrossTenantAccess and
	// packages/compliance.ErrCrossTenantAccess.
	ErrCrossTenantAccess = errors.New("threatmodel: cross-tenant access denied")

	// ErrEmptyTenantID is returned when an operation is called with a
	// zero tenant ID.
	ErrEmptyTenantID = errors.New("threatmodel: tenant id is required")

	// ErrNilAuditSink is returned by constructors that require a
	// non-nil audit sink composing with packages/auditlog.Store.
	ErrNilAuditSink = errors.New("threatmodel: audit sink must not be nil")

	// ErrNilRepository is returned by constructors that require a
	// non-nil backing catalogue used to look up a Mitigation before
	// transitioning its status.
	ErrNilRepository = errors.New("threatmodel: repository must not be nil")

	// ErrInvalidComponent is returned when a Component fails structural
	// validation.
	ErrInvalidComponent = errors.New("threatmodel: invalid component")

	// ErrInvalidThreat is returned when a Threat fails structural
	// validation.
	ErrInvalidThreat = errors.New("threatmodel: invalid threat")

	// ErrInvalidMitigation is returned when a Mitigation fails
	// structural validation.
	ErrInvalidMitigation = errors.New("threatmodel: invalid mitigation")

	// ErrInvalidThreatModel is returned when a ThreatModel fails
	// structural validation.
	ErrInvalidThreatModel = errors.New("threatmodel: invalid threat model")

	// ErrMitigationNotFound is returned when a referenced Mitigation ID
	// does not resolve to any catalogued mitigation.
	ErrMitigationNotFound = errors.New("threatmodel: mitigation not found")

	// ErrIllegalStatusTransition is returned when a mitigation status
	// change is not a permitted forward transition (see
	// CanTransitionMitigation).
	ErrIllegalStatusTransition = errors.New("threatmodel: illegal mitigation status transition")

	// ErrInputTooLarge is returned by Validator/Sanitize functions when
	// input exceeds a configured size limit.
	ErrInputTooLarge = errors.New("threatmodel: input exceeds size limit")

	// ErrInputInvalidCharset is returned when input contains
	// disallowed control characters or otherwise fails a charset
	// check.
	ErrInputInvalidCharset = errors.New("threatmodel: input contains disallowed characters")

	// ErrInputInvalidStructure is returned when input fails a
	// structural well-formedness check (e.g. blank after trimming).
	ErrInputInvalidStructure = errors.New("threatmodel: input fails structural validation")

	// ErrInvalidZone is returned when a Zone fails structural
	// validation.
	ErrInvalidZone = errors.New("threatmodel: invalid zone")

	// ErrInvalidSegmentationPolicy is returned when a
	// SegmentationPolicy fails structural validation.
	ErrInvalidSegmentationPolicy = errors.New("threatmodel: invalid segmentation policy")

	// ErrZoneNotFound is returned when a SegmentationPolicy is
	// evaluated against a zone name it does not define.
	ErrZoneNotFound = errors.New("threatmodel: zone not found in policy")
)

// wrapf mirrors the fmt.Errorf("pkg: fn: %w", err) convention used
// throughout this repository's packages.
func wrapf(fn string, err error) error {
	return fmt.Errorf("threatmodel: %s: %w", fn, err)
}
