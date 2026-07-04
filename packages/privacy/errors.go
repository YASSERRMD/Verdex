package privacy

import (
	"errors"
	"fmt"
)

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrUnauthenticated is returned when an operation requiring an
	// actor is called with a context carrying no authenticated
	// identity.User.
	ErrUnauthenticated = errors.New("privacy: unauthenticated request")

	// ErrForbidden is returned when the authenticated actor lacks the
	// identity.Permission a privacy operation requires.
	ErrForbidden = errors.New("privacy: actor lacks required permission")

	// ErrCrossTenantAccess is returned when an operation would read or
	// write a record whose TenantID does not match the caller's scope,
	// mirroring packages/accessgovernance.ErrCrossTenantAccess and
	// packages/keymanagement's tenant-isolation guard.
	ErrCrossTenantAccess = errors.New("privacy: cross-tenant access denied")

	// ErrEmptyTenantID is returned when an operation is called with a
	// zero tenant ID.
	ErrEmptyTenantID = errors.New("privacy: tenant id is required")

	// ErrNilStore is returned by constructors that require a non-nil
	// backing store/repository.
	ErrNilStore = errors.New("privacy: store must not be nil")

	// ErrNilAuditSink is returned by constructors that require a
	// non-nil audit sink composing with packages/auditlog.Store.
	ErrNilAuditSink = errors.New("privacy: audit sink must not be nil")

	// ErrInvalidDataCategory is returned when a DataCategory value
	// outside the closed enum is supplied where one is required.
	ErrInvalidDataCategory = errors.New("privacy: invalid data category")

	// ErrInvalidSensitivity is returned when a Sensitivity value
	// outside the closed enum is supplied where one is required.
	ErrInvalidSensitivity = errors.New("privacy: invalid sensitivity level")

	// ErrInvalidInventoryEntry is returned when a DataInventoryEntry
	// fails structural validation.
	ErrInvalidInventoryEntry = errors.New("privacy: invalid data inventory entry")

	// ErrInventoryEntryNotFound is returned when a referenced
	// DataInventoryEntry ID does not resolve to any stored entry for
	// the tenant.
	ErrInventoryEntryNotFound = errors.New("privacy: data inventory entry not found")

	// ErrInvalidRetentionPolicy is returned when a RetentionPolicy
	// fails structural validation (non-positive Window, unrecognized
	// DeletionAction, etc).
	ErrInvalidRetentionPolicy = errors.New("privacy: invalid retention policy")

	// ErrNoRetentionPolicy is returned by EnforceRetention when no
	// RetentionPolicy is registered for a DataCategory.
	ErrNoRetentionPolicy = errors.New("privacy: no retention policy registered for category")

	// ErrInvalidConsentRecord is returned when a ConsentRecord fails
	// structural validation (missing SubjectID/Purpose, blank
	// LegalBasis, zero GrantedAt, etc).
	ErrInvalidConsentRecord = errors.New("privacy: invalid consent record")

	// ErrConsentNotFound is returned when a referenced ConsentRecord ID
	// does not resolve to any stored record for the tenant.
	ErrConsentNotFound = errors.New("privacy: consent record not found")

	// ErrConsentAlreadyWithdrawn is returned when WithdrawConsent is
	// called on a ConsentRecord that has already been withdrawn.
	ErrConsentAlreadyWithdrawn = errors.New("privacy: consent already withdrawn")

	// ErrInvalidSAR is returned when a SubjectAccessRequest fails
	// structural validation.
	ErrInvalidSAR = errors.New("privacy: invalid subject access request")

	// ErrSARNotFound is returned when a referenced
	// SubjectAccessRequest ID does not resolve to any stored request
	// for the tenant.
	ErrSARNotFound = errors.New("privacy: subject access request not found")

	// ErrIllegalSARTransition is returned when a requested
	// SARStatus move is not permitted by CanTransitionSAR from the
	// request's current status, mirroring
	// packages/caselifecycle.ErrIllegalTransition's guarded-transition
	// discipline.
	ErrIllegalSARTransition = errors.New("privacy: illegal subject access request status transition")

	// ErrInvalidErasureRequest is returned when an ErasureRequest fails
	// structural validation.
	ErrInvalidErasureRequest = errors.New("privacy: invalid erasure request")

	// ErrErasureNotFound is returned when a referenced ErasureRequest
	// ID does not resolve to any stored request for the tenant.
	ErrErasureNotFound = errors.New("privacy: erasure request not found")

	// ErrProvenanceHashRequired is returned by ExecuteErasure when no
	// ProvenanceHash is supplied for the record being erased -- an
	// erasure that cannot point at the chain-of-custody hash it
	// preserved would silently defeat the non-negotiable
	// provenance-preservation constraint (see doc/privacy.md).
	ErrProvenanceHashRequired = errors.New("privacy: erasure requires a provenance hash to preserve")

	// ErrAlreadyErased is returned when ExecuteErasure is called twice
	// for the same ErasureRequest.
	ErrAlreadyErased = errors.New("privacy: erasure request has already been executed")
)

// wrapf mirrors the fmt.Errorf("pkg: fn: %w", err) convention used
// throughout this repository's packages.
func wrapf(fn string, err error) error {
	return fmt.Errorf("privacy: %s: %w", fn, err)
}
