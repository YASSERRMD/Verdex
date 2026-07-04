package corpusupdater

import (
	"errors"
	"fmt"
)

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrUnauthenticated is returned when an operation requiring an
	// actor is called with a context carrying no authenticated
	// identity.User.
	ErrUnauthenticated = errors.New("corpusupdater: unauthenticated request")

	// ErrForbidden is returned when the authenticated actor lacks the
	// identity.Permission a corpus-update operation requires.
	ErrForbidden = errors.New("corpusupdater: actor lacks required permission")

	// ErrCrossTenantAccess is returned when an operation would read or
	// write a record whose TenantID does not match the caller's scope,
	// mirroring packages/compliance.ErrCrossTenantAccess and
	// packages/privacy.ErrCrossTenantAccess.
	ErrCrossTenantAccess = errors.New("corpusupdater: cross-tenant access denied")

	// ErrEmptyTenantID is returned when an operation is called with a
	// zero tenant ID.
	ErrEmptyTenantID = errors.New("corpusupdater: tenant id is required")

	// ErrNilStore is returned by constructors that require a non-nil
	// backing store/repository.
	ErrNilStore = errors.New("corpusupdater: store must not be nil")

	// ErrNilAuditSink is returned by constructors that require a
	// non-nil audit sink composing with packages/auditlog.Store.
	ErrNilAuditSink = errors.New("corpusupdater: audit sink must not be nil")

	// ErrInvalidJob is returned when a CorpusUpdateJob fails structural
	// validation.
	ErrInvalidJob = errors.New("corpusupdater: invalid corpus update job")

	// ErrJobNotFound is returned when a referenced CorpusUpdateJob ID
	// does not resolve to any stored record for the tenant.
	ErrJobNotFound = errors.New("corpusupdater: corpus update job not found")

	// ErrInvalidJobTransition is returned when a job status transition
	// is not permitted by the JobStatus state machine (see
	// IsValidTransition).
	ErrInvalidJobTransition = errors.New("corpusupdater: invalid job status transition")

	// ErrInvalidAmendment is returned when an Amendment fails
	// structural validation (see Validate).
	ErrInvalidAmendment = errors.New("corpusupdater: invalid amendment")

	// ErrAmendmentNotFound is returned when a referenced Amendment ID
	// does not resolve to any stored record for the tenant.
	ErrAmendmentNotFound = errors.New("corpusupdater: amendment not found")

	// ErrInvalidCorpusTarget is returned when a CorpusTarget value is
	// not one of the recognized constants.
	ErrInvalidCorpusTarget = errors.New("corpusupdater: invalid corpus target")

	// ErrInvalidChangeType is returned when a ChangeType value is not
	// one of the recognized constants.
	ErrInvalidChangeType = errors.New("corpusupdater: invalid change type")

	// ErrMissingCitation is returned when an Amendment carries a blank
	// Citation.
	ErrMissingCitation = errors.New("corpusupdater: amendment citation is required")

	// ErrMissingTargetID is returned when an Amendment with
	// ChangeTypeAmend or ChangeTypeRepeal carries a blank TargetID.
	ErrMissingTargetID = errors.New("corpusupdater: amendment target id is required for amend/repeal")

	// ErrEffectiveDateOutOfRange is returned when an Amendment's
	// EffectiveDate is implausibly far in the past or future relative
	// to the validation time (see Validate).
	ErrEffectiveDateOutOfRange = errors.New("corpusupdater: amendment effective date out of range")

	// ErrNilEmbedder is returned by constructors/methods that require a
	// non-nil Embedder.
	ErrNilEmbedder = errors.New("corpusupdater: embedder must not be nil")

	// ErrJobNotApplied is returned when Rollback is called on a job
	// whose Status is not StatusApplied.
	ErrJobNotApplied = errors.New("corpusupdater: job is not in an applied state")

	// ErrAmendmentNotApplied is returned when an internal rollback step
	// finds an Amendment that was never actually applied (no
	// PreviousText snapshot recorded).
	ErrAmendmentNotApplied = errors.New("corpusupdater: amendment was not applied")
)

// wrapf mirrors the fmt.Errorf("pkg: fn: %w", err) convention used
// throughout this repository's packages.
func wrapf(fn string, err error) error {
	return fmt.Errorf("corpusupdater: %s: %w", fn, err)
}
