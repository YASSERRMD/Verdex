package backupdr

import (
	"errors"
	"fmt"
)

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrUnauthenticated is returned when an operation requiring an
	// actor is called with a context carrying no authenticated
	// identity.User.
	ErrUnauthenticated = errors.New("backupdr: unauthenticated request")

	// ErrForbidden is returned when the authenticated actor lacks the
	// identity.Permission a backup/DR operation requires.
	ErrForbidden = errors.New("backupdr: actor lacks required permission")

	// ErrCrossTenantAccess is returned when an operation would read or
	// write a record whose TenantID does not match the caller's scope,
	// mirroring packages/privacy.ErrCrossTenantAccess and
	// packages/compliance.ErrCrossTenantAccess.
	ErrCrossTenantAccess = errors.New("backupdr: cross-tenant access denied")

	// ErrEmptyTenantID is returned when an operation is called with a
	// zero tenant ID.
	ErrEmptyTenantID = errors.New("backupdr: tenant id is required")

	// ErrNilStore is returned by constructors that require a non-nil
	// backing store/repository.
	ErrNilStore = errors.New("backupdr: store must not be nil")

	// ErrNilAuditSink is returned by constructors that require a
	// non-nil audit sink composing with packages/auditlog.Store.
	ErrNilAuditSink = errors.New("backupdr: audit sink must not be nil")

	// ErrInvalidDataClass is returned when a DataClass value fails
	// structural validation.
	ErrInvalidDataClass = errors.New("backupdr: invalid data class")

	// ErrInvalidPolicy is returned when a BackupPolicy fails structural
	// validation.
	ErrInvalidPolicy = errors.New("backupdr: invalid backup policy")

	// ErrPolicyNotFound is returned when no BackupPolicy is on file for
	// a tenant's DataClass.
	ErrPolicyNotFound = errors.New("backupdr: backup policy not found")

	// ErrInvalidRecord is returned when a BackupRecord fails structural
	// validation.
	ErrInvalidRecord = errors.New("backupdr: invalid backup record")

	// ErrRecordNotFound is returned when a referenced BackupRecord ID
	// does not resolve to any stored record for the tenant.
	ErrRecordNotFound = errors.New("backupdr: backup record not found")

	// ErrNoRecoveryPoint is returned by ResolveRecoveryPoint when no
	// BackupRecord in the supplied set is at-or-before the requested
	// time.
	ErrNoRecoveryPoint = errors.New("backupdr: no recovery point at or before requested time")

	// ErrInvalidDrill is returned when a RestoreDrill fails structural
	// validation.
	ErrInvalidDrill = errors.New("backupdr: invalid restore drill")

	// ErrDrillNotFound is returned when a referenced RestoreDrill ID
	// does not resolve to any stored record for the tenant.
	ErrDrillNotFound = errors.New("backupdr: restore drill not found")

	// ErrInvalidTarget is returned when a Target (RPO/RTO) fails
	// structural validation.
	ErrInvalidTarget = errors.New("backupdr: invalid RPO/RTO target")

	// ErrNoTarget is returned when EvaluateRPO/EvaluateRTO is asked to
	// evaluate a DataClass with no registered Target.
	ErrNoTarget = errors.New("backupdr: no RPO/RTO target registered for data class")

	// ErrIntegrityMismatch is returned by VerifyIntegrity when a
	// BackupRecord's stored hash does not match the freshly computed
	// hash.
	ErrIntegrityMismatch = errors.New("backupdr: integrity hash mismatch")

	// ErrInvalidRunbook is returned when a Runbook fails structural
	// validation.
	ErrInvalidRunbook = errors.New("backupdr: invalid DR runbook")
)

// wrapf mirrors the fmt.Errorf("pkg: fn: %w", err) convention used
// throughout this repository's packages.
func wrapf(fn string, err error) error {
	return fmt.Errorf("backupdr: %s: %w", fn, err)
}

// isNotFound reports whether err is target via errors.Is. Small helper
// so call sites like RunGapAnalysis-style permissive defaults can treat
// "nothing on file yet" as just that, without repeating the errors.Is
// call inline.
func isNotFound(err, target error) bool {
	return errors.Is(err, target)
}
