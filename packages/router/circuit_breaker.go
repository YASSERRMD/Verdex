package router

import (
	"sync"
	"time"
)

// cbState represents the finite state of a circuit breaker.
type cbState int

const (
	// cbClosed is the normal operating state: requests are forwarded.
	cbClosed cbState = iota
	// cbOpen means the provider has exceeded its failure threshold; all
	// requests are rejected immediately.
	cbOpen
	// cbHalfOpen means the recovery timeout has elapsed; exactly one probe
	// request is allowed through to test the provider.
	cbHalfOpen
)

const (
	// defaultFailureThreshold is the number of consecutive failures that trip
	// the circuit breaker from closed to open.
	defaultFailureThreshold = 5
	// defaultRecoveryTimeout is how long the breaker waits in the open state
	// before allowing a single probe (transitioning to half-open).
	defaultRecoveryTimeout = 30 * time.Second
)

// CircuitBreaker tracks failure counts for a single provider and implements
// the standard closed → open → half-open → closed state machine.
//
// All methods are safe for concurrent use from multiple goroutines.
type CircuitBreaker struct {
	mu               sync.Mutex
	providerID       string
	state            cbState
	failures         int
	failureThreshold int
	recoveryTimeout  time.Duration
	openedAt         time.Time
}

// newCircuitBreaker creates a CircuitBreaker with the given provider ID and
// default thresholds.
func newCircuitBreaker(providerID string) *CircuitBreaker {
	return &CircuitBreaker{
		providerID:       providerID,
		state:            cbClosed,
		failureThreshold: defaultFailureThreshold,
		recoveryTimeout:  defaultRecoveryTimeout,
	}
}

// Allow reports whether a request should be forwarded to the provider.
//
//   - Closed: always returns true.
//   - Open: returns true only if the recovery timeout has elapsed (which also
//     transitions the breaker to half-open).
//   - Half-open: returns true exactly once; subsequent calls return false
//     until Reset() or Trip() is called.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case cbClosed:
		return true
	case cbOpen:
		if time.Since(cb.openedAt) >= cb.recoveryTimeout {
			cb.state = cbHalfOpen
			return true
		}
		return false
	case cbHalfOpen:
		// Only one probe is allowed; deny subsequent calls until the probe
		// result is registered via Trip() or Reset().
		return false
	}
	return false
}

// Trip records a failure.  Once the failure count reaches the threshold the
// breaker transitions to the open state.  Calling Trip() in the half-open
// state also transitions back to open (the probe failed).
func (cb *CircuitBreaker) Trip() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case cbClosed:
		cb.failures++
		if cb.failures >= cb.failureThreshold {
			cb.state = cbOpen
			cb.openedAt = time.Now()
		}
	case cbHalfOpen:
		// Probe failed — reopen.
		cb.state = cbOpen
		cb.openedAt = time.Now()
	case cbOpen:
		// Already open; nothing to change.
	}
}

// Reset records a success and returns the breaker to the closed state,
// clearing the failure counter.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = cbClosed
	cb.failures = 0
}

// State returns the current cbState.  Exposed primarily for testing.
func (cb *CircuitBreaker) State() cbState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// CircuitBreakerRegistry manages one CircuitBreaker per provider ID.
//
// All methods are safe for concurrent use from multiple goroutines.
type CircuitBreakerRegistry struct {
	mu       sync.RWMutex
	breakers map[string]*CircuitBreaker
}

// NewCircuitBreakerRegistry returns an empty registry.
func NewCircuitBreakerRegistry() *CircuitBreakerRegistry {
	return &CircuitBreakerRegistry{
		breakers: make(map[string]*CircuitBreaker),
	}
}

// Get returns the CircuitBreaker for providerID, creating it lazily if it
// does not yet exist.
func (r *CircuitBreakerRegistry) Get(providerID string) *CircuitBreaker {
	// Fast path: breaker already exists.
	r.mu.RLock()
	cb, ok := r.breakers[providerID]
	r.mu.RUnlock()
	if ok {
		return cb
	}

	// Slow path: create under write lock.
	r.mu.Lock()
	defer r.mu.Unlock()
	// Double-check in case another goroutine created it between the two locks.
	if cb, ok = r.breakers[providerID]; ok {
		return cb
	}
	cb = newCircuitBreaker(providerID)
	r.breakers[providerID] = cb
	return cb
}
