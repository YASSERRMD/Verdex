package router

import "errors"

// Sentinel errors returned by the router.  Callers should test for them with
// errors.Is.
var (
	// ErrNoProvidersAvailable is returned when the selector can find no
	// candidate providers for the given task and tenant.
	ErrNoProvidersAvailable = errors.New("router: no providers available")

	// ErrAllProvidersFailed is returned when every candidate provider was
	// tried (or skipped due to an open circuit breaker) and none succeeded.
	ErrAllProvidersFailed = errors.New("router: all providers failed")

	// ErrAirGappedViolation is returned when AirGappedOnly is set but no
	// provider with a "local:" prefix exists in the candidate chain.
	ErrAirGappedViolation = errors.New("router: air-gapped mode requires a local provider but none is available")

	// ErrBudgetExceeded is returned when a request would exceed the
	// MaxCostPerRequest budget defined in the RoutingPolicy.
	ErrBudgetExceeded = errors.New("router: request cost exceeds per-request budget")
)
