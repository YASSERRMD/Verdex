package reliability

import (
	"errors"
	"fmt"
)

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrInvalidRetryPolicy is returned when a RetryPolicy fails structural
	// validation (non-positive MaxAttempts, negative backoff bounds).
	ErrInvalidRetryPolicy = errors.New("reliability: invalid retry policy")

	// ErrRetriesExhausted is returned by Retry when every attempt failed and
	// no more remain under the configured RetryPolicy.
	ErrRetriesExhausted = errors.New("reliability: retries exhausted")

	// ErrCircuitOpen is returned when a call is rejected because a
	// CircuitBreaker is in the Open state.
	ErrCircuitOpen = errors.New("reliability: circuit breaker is open")

	// ErrNilFunc is returned by any wrapper (Retry, Degrader, Idempotency,
	// FailureInjector) invoked with a nil function to call.
	ErrNilFunc = errors.New("reliability: function must not be nil")

	// ErrNoHealthyBackends is returned by TrafficShifter.Select when every
	// registered backend is unhealthy (fail-closed: no traffic is routed
	// rather than routing to a known-bad backend).
	ErrNoHealthyBackends = errors.New("reliability: no healthy backends available")

	// ErrUnknownBackend is returned when an operation names a backend not
	// registered with the TrafficShifter.
	ErrUnknownBackend = errors.New("reliability: unknown backend")

	// ErrInvalidSLO is returned when an SLO fails structural validation
	// (blank name, target outside [0,1] for a success-rate SLO, or a
	// non-positive rolling window).
	ErrInvalidSLO = errors.New("reliability: invalid SLO")

	// ErrInvalidBudgetPolicy is returned when an ErrorBudgetPolicy fails
	// structural validation.
	ErrInvalidBudgetPolicy = errors.New("reliability: invalid error budget policy")
)

// wrapf mirrors the fmt.Errorf("pkg: fn: %w", err) convention used
// throughout this repository's packages.
func wrapf(fn string, err error) error {
	return fmt.Errorf("reliability: %s: %w", fn, err)
}
