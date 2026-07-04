package e2e

import (
	"errors"
	"fmt"
)

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrEmptyScenarioName is returned by NewScenario when name is blank.
	ErrEmptyScenarioName = errors.New("e2e: scenario name must not be blank")

	// ErrNilScenarioFunc is returned by NewScenario when the run function
	// is nil.
	ErrNilScenarioFunc = errors.New("e2e: scenario run function must not be nil")

	// ErrInvalidCategory is returned when a Scenario is registered with a
	// CategoryCode this package's Category type does not recognize as
	// IsValid.
	ErrInvalidCategory = errors.New("e2e: invalid case category")

	// ErrScenarioNotFound is returned by Suite.Run when asked to run a
	// scenario name that was never registered.
	ErrScenarioNotFound = errors.New("e2e: scenario not found")

	// ErrDiscardVerificationFailed is returned when a scenario's
	// post-ingestion discard-guarantee check finds a binary artifact
	// still reachable (see discard.go).
	ErrDiscardVerificationFailed = errors.New("e2e: discard guarantee verification failed")

	// ErrSignoffNotEnforced is returned when a scenario expected
	// finalization to be blocked absent an explicit sign-off
	// acknowledgement, but it was not (see signoff_scenario.go).
	ErrSignoffNotEnforced = errors.New("e2e: sign-off gate did not block an unapproved case")

	// ErrIsolationBreached is returned when a scenario proving
	// cross-tenant/cross-case isolation instead observes a leak (see
	// isolation_scenario.go).
	ErrIsolationBreached = errors.New("e2e: cross-tenant or cross-case isolation was breached")

	// ErrMaxAttemptsExhausted is returned by RetryOnFlake when fn never
	// succeeds within maxAttempts tries (see flaky.go).
	ErrMaxAttemptsExhausted = errors.New("e2e: max retry attempts exhausted")

	// ErrQuarantinedScenario is returned by Suite.Run (and recorded, not
	// swallowed) when a scenario name is present in the active
	// QuarantineList (see flaky.go).
	ErrQuarantinedScenario = errors.New("e2e: scenario is quarantined")
)

// wrapf mirrors the fmt.Errorf("pkg: fn: %w", err) convention used
// throughout this repository's packages.
func wrapf(fn string, err error) error {
	return fmt.Errorf("e2e: %s: %w", fn, err)
}
