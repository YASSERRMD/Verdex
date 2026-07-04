package alerting

import (
	"errors"
	"fmt"
)

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrUnauthenticated is returned when an operation requiring an
	// actor is called with a context carrying no authenticated
	// identity.User.
	ErrUnauthenticated = errors.New("alerting: unauthenticated request")

	// ErrForbidden is returned when the authenticated actor lacks the
	// identity.Permission an alerting operation requires.
	ErrForbidden = errors.New("alerting: actor lacks required permission")

	// ErrCrossTenantAccess is returned when an operation would read or
	// write a record whose TenantID does not match the caller's scope,
	// mirroring packages/compliance.ErrCrossTenantAccess and
	// packages/backupdr.ErrCrossTenantAccess.
	ErrCrossTenantAccess = errors.New("alerting: cross-tenant access denied")

	// ErrEmptyTenantID is returned when an operation is called with a
	// zero tenant ID.
	ErrEmptyTenantID = errors.New("alerting: tenant id is required")

	// ErrNilStore is returned by constructors that require a non-nil
	// backing store/repository.
	ErrNilStore = errors.New("alerting: store must not be nil")

	// ErrInvalidRule is returned when an AlertRule fails structural
	// validation.
	ErrInvalidRule = errors.New("alerting: invalid alert rule")

	// ErrRuleNotFound is returned when a referenced AlertRule name/ID
	// does not resolve to any stored rule.
	ErrRuleNotFound = errors.New("alerting: alert rule not found")

	// ErrDuplicateRule is returned when RegisterRule is called with a
	// rule Name already registered for the tenant.
	ErrDuplicateRule = errors.New("alerting: alert rule already registered")

	// ErrInvalidSeverity is returned when a Severity value is not one
	// of the recognized constants.
	ErrInvalidSeverity = errors.New("alerting: invalid severity")

	// ErrInvalidEvent is returned when an AlertEvent fails structural
	// validation.
	ErrInvalidEvent = errors.New("alerting: invalid alert event")

	// ErrEventNotFound is returned when a referenced AlertEvent ID does
	// not resolve to any stored record for the tenant.
	ErrEventNotFound = errors.New("alerting: alert event not found")

	// ErrInvalidPolicy is returned when an EscalationPolicy fails
	// structural validation (no tiers, non-monotonic delays, etc).
	ErrInvalidPolicy = errors.New("alerting: invalid escalation policy")

	// ErrPolicyNotFound is returned when no EscalationPolicy is on
	// file for a tenant/name.
	ErrPolicyNotFound = errors.New("alerting: escalation policy not found")

	// ErrNoTiers is returned by Route when a policy has no tiers to
	// escalate through.
	ErrNoTiers = errors.New("alerting: escalation policy has no tiers")

	// ErrInvalidRunbook is returned when a Runbook or RunbookStep fails
	// structural validation.
	ErrInvalidRunbook = errors.New("alerting: invalid runbook")

	// ErrInvalidCheck is returned when a SyntheticCheck fails
	// structural validation.
	ErrInvalidCheck = errors.New("alerting: invalid synthetic check")

	// ErrNilProber is returned when a SyntheticCheck is run with no
	// Prober function configured.
	ErrNilProber = errors.New("alerting: synthetic check has no prober")

	// ErrNilCondition is returned when Engine.Evaluate is called with
	// an AlertRule carrying a nil/empty Condition.
	ErrNilCondition = errors.New("alerting: alert rule has no condition")
)

// wrapf mirrors the fmt.Errorf("pkg: fn: %w", err) convention used
// throughout this repository's packages.
func wrapf(fn string, err error) error {
	return fmt.Errorf("alerting: %s: %w", fn, err)
}
