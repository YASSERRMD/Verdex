package accessgovernance

import (
	"errors"
	"fmt"
)

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrUnauthenticated is returned when an operation requiring an
	// actor is called with a context carrying no authenticated
	// identity.User.
	ErrUnauthenticated = errors.New("accessgovernance: unauthenticated request")

	// ErrForbidden is returned when the authenticated actor lacks the
	// identity.Permission an access-governance operation requires.
	ErrForbidden = errors.New("accessgovernance: actor lacks required permission")

	// ErrCrossTenantAccess is returned when an operation would read or
	// write a record whose TenantID does not match the caller's scope,
	// mirroring packages/keymanagement.ErrCrossTenantAccess and
	// packages/auditlog's tenant-isolation guard.
	ErrCrossTenantAccess = errors.New("accessgovernance: cross-tenant access denied")

	// ErrEmptyTenantID is returned when an operation is called with a
	// zero tenant ID.
	ErrEmptyTenantID = errors.New("accessgovernance: tenant id is required")

	// ErrNilPolicy is returned when a nil *Policy is passed to a
	// function that requires one.
	ErrNilPolicy = errors.New("accessgovernance: policy must not be nil")

	// ErrInvalidPolicy is returned when a Policy fails structural
	// validation (see Policy.Validate).
	ErrInvalidPolicy = errors.New("accessgovernance: invalid policy")

	// ErrNilRequest is returned when Evaluate or Elevate is called with
	// a nil Request pointer -- unreachable given the current
	// method signatures (Request is passed by value), reserved for a
	// future pointer-based overload.
	ErrNilRequest = errors.New("accessgovernance: request must not be nil")

	// ErrPolicyNotFound is returned when a referenced Policy ID does
	// not resolve to any stored policy for the tenant.
	ErrPolicyNotFound = errors.New("accessgovernance: policy not found")

	// ErrGrantNotFound is returned when a referenced CaseGrant ID does
	// not resolve to any stored grant for the tenant.
	ErrGrantNotFound = errors.New("accessgovernance: case grant not found")

	// ErrGrantExpired is returned by Evaluate when a matching CaseGrant
	// or Elevate-produced Grant's ExpiresAt has already passed -- JIT
	// and per-case grants both fail closed once expired, with no
	// background job required to notice (task 3).
	ErrGrantExpired = errors.New("accessgovernance: grant has expired")

	// ErrInvalidGrant is returned when a CaseGrant or Grant fails
	// structural validation (missing CaseID/GranteeUserID, zero
	// ExpiresAt, etc).
	ErrInvalidGrant = errors.New("accessgovernance: invalid grant")

	// ErrJustificationRequired is returned by Elevate when no (or a
	// blank) justification is supplied -- mirroring
	// packages/keymanagement's break-glass justification requirement,
	// which this package's JIT elevation generalizes (task 3).
	ErrJustificationRequired = errors.New("accessgovernance: elevation requires a justification")

	// ErrReviewNotFound is returned when a referenced Review ID does
	// not resolve to any stored review for the tenant.
	ErrReviewNotFound = errors.New("accessgovernance: review not found")

	// ErrReviewAlreadyDecided is returned when Attest is called on a
	// Review that has already recorded a decision -- a review can be
	// attested exactly once.
	ErrReviewAlreadyDecided = errors.New("accessgovernance: review has already been attested")

	// ErrInvalidAttestationDecision is returned when Attest is called
	// with a AttestationDecision outside the closed enum.
	ErrInvalidAttestationDecision = errors.New("accessgovernance: invalid attestation decision")

	// ErrSegregationOfDuties is returned when a grant or attestation
	// would violate a registered ConflictRule -- e.g. the actor who
	// requested a grant attempting to also approve/attest it, or a
	// sole case author attempting to self-approve (task 5).
	ErrSegregationOfDuties = errors.New("accessgovernance: action violates segregation of duties")

	// ErrInvalidExportFormat is returned by Certify's export path when
	// asked to render a Report in a format other than
	// ExportFormatCSV/ExportFormatJSON.
	ErrInvalidExportFormat = errors.New("accessgovernance: invalid export format")

	// ErrInvalidPeriod is returned when Certify is called with a Period
	// whose End is not strictly after its Start.
	ErrInvalidPeriod = errors.New("accessgovernance: invalid certification period")

	// ErrNilStore is returned by constructors that require a non-nil
	// backing store/repository.
	ErrNilStore = errors.New("accessgovernance: store must not be nil")

	// ErrNilAuditSink is returned by constructors that require a
	// non-nil audit sink (task 6's composition with packages/auditlog).
	ErrNilAuditSink = errors.New("accessgovernance: audit sink must not be nil")
)

// wrapf mirrors the fmt.Errorf("pkg: fn: %w", err) convention used
// throughout this repository's packages.
func wrapf(fn string, err error) error {
	return fmt.Errorf("accessgovernance: %s: %w", fn, err)
}
