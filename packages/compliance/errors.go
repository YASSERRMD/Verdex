package compliance

import (
	"errors"
	"fmt"
)

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrUnauthenticated is returned when an operation requiring an
	// actor is called with a context carrying no authenticated
	// identity.User.
	ErrUnauthenticated = errors.New("compliance: unauthenticated request")

	// ErrForbidden is returned when the authenticated actor lacks the
	// identity.Permission a compliance operation requires.
	ErrForbidden = errors.New("compliance: actor lacks required permission")

	// ErrCrossTenantAccess is returned when an operation would read or
	// write a record whose TenantID does not match the caller's scope,
	// mirroring packages/privacy.ErrCrossTenantAccess and
	// packages/accessgovernance.ErrCrossTenantAccess.
	ErrCrossTenantAccess = errors.New("compliance: cross-tenant access denied")

	// ErrEmptyTenantID is returned when an operation is called with a
	// zero tenant ID.
	ErrEmptyTenantID = errors.New("compliance: tenant id is required")

	// ErrNilStore is returned by constructors that require a non-nil
	// backing store/repository.
	ErrNilStore = errors.New("compliance: store must not be nil")

	// ErrNilAuditSink is returned by constructors that require a
	// non-nil audit sink composing with packages/auditlog.Store.
	ErrNilAuditSink = errors.New("compliance: audit sink must not be nil")

	// ErrInvalidFramework is returned when a Framework value fails
	// structural validation (blank, etc).
	ErrInvalidFramework = errors.New("compliance: invalid framework")

	// ErrInvalidControl is returned when a Control fails structural
	// validation.
	ErrInvalidControl = errors.New("compliance: invalid control")

	// ErrControlNotFound is returned when a referenced Control ID does
	// not resolve to any catalogued control.
	ErrControlNotFound = errors.New("compliance: control not found")

	// ErrDuplicateControl is returned when RegisterControl is called
	// with a Control.ID already present in the catalogue.
	ErrDuplicateControl = errors.New("compliance: control already registered")

	// ErrInvalidEvidence is returned when a ControlEvidence record
	// fails structural validation.
	ErrInvalidEvidence = errors.New("compliance: invalid control evidence")

	// ErrEvidenceNotFound is returned when a referenced ControlEvidence
	// ID does not resolve to any stored record for the tenant.
	ErrEvidenceNotFound = errors.New("compliance: control evidence not found")

	// ErrInvalidProfile is returned when a ComplianceProfile fails
	// structural validation.
	ErrInvalidProfile = errors.New("compliance: invalid compliance profile")

	// ErrProfileNotFound is returned when no ComplianceProfile is on
	// file for a tenant.
	ErrProfileNotFound = errors.New("compliance: compliance profile not found")
)

// wrapf mirrors the fmt.Errorf("pkg: fn: %w", err) convention used
// throughout this repository's packages.
func wrapf(fn string, err error) error {
	return fmt.Errorf("compliance: %s: %w", fn, err)
}
