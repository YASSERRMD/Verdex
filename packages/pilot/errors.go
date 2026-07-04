package pilot

import (
	"errors"
	"fmt"
)

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrUnauthenticated is returned when an operation requiring an
	// actor is called with a context carrying no authenticated
	// identity.User.
	ErrUnauthenticated = errors.New("pilot: unauthenticated request")

	// ErrForbidden is returned when the authenticated actor lacks the
	// identity.Permission a pilot operation requires.
	ErrForbidden = errors.New("pilot: actor lacks required permission")

	// ErrCrossTenantAccess is returned when an operation would read or
	// write a record whose TenantID does not match the caller's scope,
	// mirroring packages/compliance.ErrCrossTenantAccess and
	// packages/vulnmanagement's equivalent guard.
	ErrCrossTenantAccess = errors.New("pilot: cross-tenant access denied")

	// ErrEmptyTenantID is returned when an operation is called with a
	// zero tenant ID.
	ErrEmptyTenantID = errors.New("pilot: tenant id is required")

	// ErrNilStore is returned by constructors that require a non-nil
	// backing store/repository.
	ErrNilStore = errors.New("pilot: store must not be nil")

	// ErrNilAuditSink is returned by constructors that require a
	// non-nil audit sink composing with packages/auditlog.Store.
	ErrNilAuditSink = errors.New("pilot: audit sink must not be nil")

	// ErrInvalidDeployment is returned when a PilotDeployment fails
	// structural validation.
	ErrInvalidDeployment = errors.New("pilot: invalid pilot deployment")

	// ErrDeploymentNotFound is returned when a referenced
	// PilotDeployment ID does not resolve to any stored record for the
	// tenant.
	ErrDeploymentNotFound = errors.New("pilot: pilot deployment not found")

	// ErrIllegalStatusTransition is returned when a requested
	// PilotDeployment status change is not a legal move per
	// CanTransitionDeployment.
	ErrIllegalStatusTransition = errors.New("pilot: illegal deployment status transition")

	// ErrInvalidCase is returned when a PilotCase fails structural
	// validation.
	ErrInvalidCase = errors.New("pilot: invalid pilot case")

	// ErrCaseNotFound is returned when a referenced PilotCase ID does
	// not resolve to any stored record for the tenant.
	ErrCaseNotFound = errors.New("pilot: pilot case not found")

	// ErrInvalidFeedback is returned when a FeedbackEntry fails
	// structural validation.
	ErrInvalidFeedback = errors.New("pilot: invalid feedback entry")

	// ErrFeedbackNotFound is returned when a referenced FeedbackEntry
	// ID does not resolve to any stored record for the tenant.
	ErrFeedbackNotFound = errors.New("pilot: feedback entry not found")

	// ErrInvalidFinding is returned when a PilotFinding fails structural
	// validation.
	ErrInvalidFinding = errors.New("pilot: invalid pilot finding")

	// ErrFindingNotFound is returned when a referenced PilotFinding ID
	// does not resolve to any stored record for the tenant.
	ErrFindingNotFound = errors.New("pilot: pilot finding not found")

	// ErrFindingNotTriaged is returned when RecordRefinement is called
	// against a PilotFinding that has not yet been moved to
	// StatusTriaged (or later) -- a refinement cannot reference a
	// finding no human has ever reviewed.
	ErrFindingNotTriaged = errors.New("pilot: finding must be triaged before a refinement can reference it")

	// ErrInvalidRefinement is returned when a RefinementRecord fails
	// structural validation.
	ErrInvalidRefinement = errors.New("pilot: invalid refinement record")

	// ErrRefinementNotFound is returned when a referenced
	// RefinementRecord ID does not resolve to any stored record for the
	// tenant.
	ErrRefinementNotFound = errors.New("pilot: refinement record not found")

	// ErrNotesRequired is returned when a triage or refinement
	// operation is called with blank notes, mirroring
	// packages/vulnmanagement.ErrNotesRequired's non-blank-accountability
	// requirement.
	ErrNotesRequired = errors.New("pilot: notes are required")

	// ErrEmptyOpinionText is returned when ValidateNonBindingCompliance
	// is called with blank opinion text.
	ErrEmptyOpinionText = errors.New("pilot: opinion text is required")

	// ErrInvalidPeriod is returned when a Period fails structural
	// validation (End before Start).
	ErrInvalidPeriod = errors.New("pilot: invalid reporting period")

	// ErrInvalidExportFormat is returned when ExportReport is called
	// with an unrecognized ExportFormat.
	ErrInvalidExportFormat = errors.New("pilot: invalid export format")
)

// wrapf mirrors the fmt.Errorf("pkg: fn: %w", err) convention used
// throughout this repository's packages.
func wrapf(fn string, err error) error {
	return fmt.Errorf("pilot: %s: %w", fn, err)
}
