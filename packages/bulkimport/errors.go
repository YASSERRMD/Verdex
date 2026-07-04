package bulkimport

import (
	"errors"
	"fmt"
)

// Sentinel errors that callers can test with errors.Is, following the
// exact convention established by packages/compliance/errors.go and
// packages/privacy/errors.go.
var (
	// ErrUnauthenticated is returned when an operation requiring an
	// actor is called with a context carrying no authenticated
	// identity.User.
	ErrUnauthenticated = errors.New("bulkimport: unauthenticated request")

	// ErrForbidden is returned when the authenticated actor lacks the
	// identity.Permission a bulk-import operation requires.
	ErrForbidden = errors.New("bulkimport: actor lacks required permission")

	// ErrCrossTenantAccess is returned when an operation would read or
	// write a record whose TenantID does not match the caller's scope.
	ErrCrossTenantAccess = errors.New("bulkimport: cross-tenant access denied")

	// ErrEmptyTenantID is returned when an operation is called with a
	// zero tenant ID.
	ErrEmptyTenantID = errors.New("bulkimport: tenant id is required")

	// ErrNilStore is returned by constructors that require a non-nil
	// backing store/repository.
	ErrNilStore = errors.New("bulkimport: store must not be nil")

	// ErrNilAuditSink is returned by constructors that require a
	// non-nil audit sink composing with packages/auditlog.Store.
	ErrNilAuditSink = errors.New("bulkimport: audit sink must not be nil")

	// ErrNilSource is returned when Engine.RunBatch is called with a
	// nil RecordSource.
	ErrNilSource = errors.New("bulkimport: record source must not be nil")

	// ErrInvalidJob is returned when an ImportJob fails structural
	// validation.
	ErrInvalidJob = errors.New("bulkimport: invalid import job")

	// ErrJobNotFound is returned when a referenced ImportJob ID does
	// not resolve to any stored record for the tenant.
	ErrJobNotFound = errors.New("bulkimport: import job not found")

	// ErrInvalidRecord is returned when an ImportRecord fails
	// structural validation.
	ErrInvalidRecord = errors.New("bulkimport: invalid import record")

	// ErrRecordNotFound is returned when a referenced ImportRecord ID
	// does not resolve to any stored record for the tenant/job.
	ErrRecordNotFound = errors.New("bulkimport: import record not found")

	// ErrInvalidTransition is returned when a requested ImportJob
	// status transition is not permitted from the job's current
	// status (see Status.CanTransitionTo).
	ErrInvalidTransition = errors.New("bulkimport: invalid job status transition")

	// ErrJobNotRunnable is returned when RunBatch is called on a job
	// whose Status is not Pending or Running.
	ErrJobNotRunnable = errors.New("bulkimport: import job is not runnable")

	// ErrJobNotRollbackable is returned when Rollback is called on a
	// job whose Status is not Completed or Failed.
	ErrJobNotRollbackable = errors.New("bulkimport: import job cannot be rolled back")

	// ErrInvalidBatchSize is returned when RunBatch is called with a
	// batchSize <= 0.
	ErrInvalidBatchSize = errors.New("bulkimport: batch size must be positive")
)

// wrapf mirrors the fmt.Errorf("pkg: fn: %w", err) convention used
// throughout this repository's packages.
func wrapf(fn string, err error) error {
	return fmt.Errorf("bulkimport: %s: %w", fn, err)
}
