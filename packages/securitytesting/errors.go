package securitytesting

import (
	"errors"
	"fmt"
)

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrUnauthenticated is returned when an operation requiring an
	// actor is called with a context carrying no authenticated
	// identity.User.
	ErrUnauthenticated = errors.New("securitytesting: unauthenticated request")

	// ErrForbidden is returned when the authenticated actor lacks the
	// identity.Permission a securitytesting operation requires.
	ErrForbidden = errors.New("securitytesting: actor lacks required permission")

	// ErrCrossTenantAccess is returned when an operation would read or
	// write a record whose TenantID does not match the caller's scope,
	// mirroring packages/privacy.ErrCrossTenantAccess,
	// packages/compliance.ErrCrossTenantAccess, and
	// packages/threatmodel.ErrCrossTenantAccess.
	ErrCrossTenantAccess = errors.New("securitytesting: cross-tenant access denied")

	// ErrEmptyTenantID is returned when an operation is called with a
	// zero tenant ID.
	ErrEmptyTenantID = errors.New("securitytesting: tenant id is required")

	// ErrNilStore is returned by constructors that require a non-nil
	// backing store/repository.
	ErrNilStore = errors.New("securitytesting: store must not be nil")

	// ErrNilAuditSink is returned by constructors that require a
	// non-nil audit sink composing with packages/auditlog.Store.
	ErrNilAuditSink = errors.New("securitytesting: audit sink must not be nil")

	// ErrInvalidScope is returned when a ScopeDocument fails structural
	// validation.
	ErrInvalidScope = errors.New("securitytesting: invalid scope document")

	// ErrInvalidScenario is returned when a Scenario's static
	// description (name, category) fails structural validation.
	ErrInvalidScenario = errors.New("securitytesting: invalid scenario")

	// ErrOutOfScope is returned when a Scenario or RunRequest targets a
	// component the active ScopeDocument marks out-of-scope, or names
	// no component the ScopeDocument recognizes as in-scope.
	ErrOutOfScope = errors.New("securitytesting: target is out of scope")

	// ErrInvalidFinding is returned when a Finding fails structural
	// validation.
	ErrInvalidFinding = errors.New("securitytesting: invalid finding")

	// ErrFindingNotFound is returned when a referenced Finding ID does
	// not resolve to any stored record for the tenant.
	ErrFindingNotFound = errors.New("securitytesting: finding not found")

	// ErrIllegalStatusTransition is returned when a Finding status
	// change is not a permitted forward transition (see
	// CanTransitionFinding).
	ErrIllegalStatusTransition = errors.New("securitytesting: illegal finding status transition")

	// ErrRemediationNotVerified is returned by
	// Engine.VerifyRemediation when a re-run of the Finding's
	// originating Scenario still reproduces the vulnerability -- the
	// Finding's status is deliberately left unchanged in this case (see
	// Engine.VerifyRemediation).
	ErrRemediationNotVerified = errors.New("securitytesting: remediation re-run still reproduces the finding")

	// ErrInvalidRunRecord is returned when a RunRecord fails structural
	// validation.
	ErrInvalidRunRecord = errors.New("securitytesting: invalid run record")

	// ErrDuplicateRunRecord is returned when Create is called with a
	// RunRecord.ID that already exists -- RunRecords are append-only
	// (see RunRecordRepository's doc comment), so a repeated ID is
	// always a replay/retry bug in the caller, never a legitimate
	// update. This guard is what makes the append-only guarantee real
	// rather than aspirational: see the abuse-case suite's
	// ScenarioAuditReplayRejected.
	ErrDuplicateRunRecord = errors.New("securitytesting: run record already exists")
)

// wrapf mirrors the fmt.Errorf("pkg: fn: %w", err) convention used
// throughout this repository's packages.
func wrapf(fn string, err error) error {
	return fmt.Errorf("securitytesting: %s: %w", fn, err)
}
