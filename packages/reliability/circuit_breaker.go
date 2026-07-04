package reliability

import (
	"sync"
	"time"
)

// State is the finite state of a CircuitBreaker.
type State int

const (
	// StateClosed is the normal operating state: calls are allowed
	// through and failures are counted toward the trip threshold.
	StateClosed State = iota

	// StateOpen means the failure threshold has been exceeded; all
	// calls are rejected immediately (ErrCircuitOpen) until the
	// cooldown elapses.
	StateOpen

	// StateHalfOpen means the cooldown has elapsed and exactly one
	// probe call is allowed through to test whether the dependency has
	// recovered.
	StateHalfOpen
)

// String satisfies fmt.Stringer.
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half_open"
	default:
		return "unknown"
	}
}

const (
	// DefaultFailureThreshold is the number of consecutive failures
	// that trip a CircuitBreaker from Closed to Open, when
	// CircuitBreakerConfig.FailureThreshold is left at its zero value.
	DefaultFailureThreshold = 5

	// DefaultCooldown is how long a CircuitBreaker waits in the Open
	// state before allowing a single half-open probe, when
	// CircuitBreakerConfig.Cooldown is left at its zero value.
	DefaultCooldown = 30 * time.Second
)

// CircuitBreakerConfig configures a CircuitBreaker's trip threshold and
// cooldown.
type CircuitBreakerConfig struct {
	// FailureThreshold is the number of consecutive failures that trip
	// the breaker from Closed to Open. Values <= 0 fall back to
	// DefaultFailureThreshold.
	FailureThreshold int

	// Cooldown is how long the breaker stays Open before allowing a
	// half-open probe. Values <= 0 fall back to DefaultCooldown.
	Cooldown time.Duration
}

func (c CircuitBreakerConfig) failureThreshold() int {
	if c.FailureThreshold <= 0 {
		return DefaultFailureThreshold
	}
	return c.FailureThreshold
}

func (c CircuitBreakerConfig) cooldown() time.Duration {
	if c.Cooldown <= 0 {
		return DefaultCooldown
	}
	return c.Cooldown
}

// CircuitBreaker is a generic, dependency-agnostic implementation of
// the standard Closed -> Open -> HalfOpen -> Closed state machine.
// Unlike packages/router.CircuitBreaker (Phase 012), which is
// hardcoded to gate a single LLM provider inside that package's
// routing logic, this type carries no notion of "provider" at all: a
// caller names what it is protecting (a Postgres connection pool, the
// Neo4j graph store, an external integration client from Phase 087)
// purely by which CircuitBreaker instance it holds, typically one per
// dependency obtained from a CircuitBreakerRegistry keyed by a
// caller-chosen string.
//
// packages/router's provider circuit breaker is untouched and remains
// the only breaker gating provider selection in that package -- see
// doc.go and doc/reliability.md for the full "reused, not duplicated"
// discussion.
//
// All methods are safe for concurrent use from multiple goroutines.
type CircuitBreaker struct {
	mu       sync.Mutex
	name     string
	cfg      CircuitBreakerConfig
	state    State
	failures int
	openedAt time.Time
	now      func() time.Time
}

// NewCircuitBreaker constructs a CircuitBreaker named name (used only
// for diagnostics/logging by callers; this package does not log
// itself) with the given config. A zero-valued CircuitBreakerConfig is
// valid and resolves to DefaultFailureThreshold/DefaultCooldown.
func NewCircuitBreaker(name string, cfg CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		name:  name,
		cfg:   cfg,
		state: StateClosed,
		now:   time.Now,
	}
}

// Name returns the name this breaker was constructed with.
func (cb *CircuitBreaker) Name() string { return cb.name }

// Allow reports whether a call should be permitted:
//
//   - Closed: always true.
//   - Open: true only once the cooldown has elapsed, which also
//     transitions the breaker to HalfOpen as a side effect.
//   - HalfOpen: true exactly once per probe cycle; subsequent calls
//     return false until RecordSuccess or RecordFailure resolves the
//     outstanding probe.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		if cb.now().Sub(cb.openedAt) >= cb.cfg.cooldown() {
			cb.state = StateHalfOpen
			return true
		}
		return false
	case StateHalfOpen:
		return false
	default:
		return false
	}
}

// RecordSuccess reports a successful call. In Closed, it resets the
// consecutive-failure counter. In HalfOpen, the probe succeeded, so the
// breaker closes and the counter resets. It has no effect in Open
// (a success cannot be observed for a call that was never allowed
// through).
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		cb.failures = 0
	case StateHalfOpen:
		cb.state = StateClosed
		cb.failures = 0
	case StateOpen:
		// No outstanding probe to resolve.
	}
}

// RecordFailure reports a failed call. In Closed, it increments the
// consecutive-failure counter, tripping to Open once the configured
// threshold is reached. In HalfOpen, the probe failed, so the breaker
// reopens (and its cooldown timer restarts). It has no effect in Open.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		cb.failures++
		if cb.failures >= cb.cfg.failureThreshold() {
			cb.trip()
		}
	case StateHalfOpen:
		cb.trip()
	case StateOpen:
		// Already open.
	}
}

// trip transitions to Open and records the opening time. Callers must
// hold cb.mu.
func (cb *CircuitBreaker) trip() {
	cb.state = StateOpen
	cb.openedAt = cb.now()
}

// State returns the breaker's current State. Exposed primarily for
// diagnostics and tests; production call sites should prefer Allow.
func (cb *CircuitBreaker) State() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

// Execute is a convenience wrapper: it calls Allow, and if permitted,
// invokes fn and records the outcome via RecordSuccess/RecordFailure.
// Returns ErrCircuitOpen without invoking fn if the breaker currently
// denies the call, or a wrapped ErrNilFunc if fn is nil.
func (cb *CircuitBreaker) Execute(fn RetryableFuncNoCtx) error {
	if fn == nil {
		return wrapf("CircuitBreaker.Execute", ErrNilFunc)
	}
	if !cb.Allow() {
		return wrapf("CircuitBreaker.Execute", ErrCircuitOpen)
	}

	err := fn()
	if err != nil {
		cb.RecordFailure()
		return err
	}
	cb.RecordSuccess()
	return nil
}

// RetryableFuncNoCtx is a context-free unit of work, used by
// CircuitBreaker.Execute for callers whose protected operation does
// not need a context threaded through the breaker itself (it can
// still close over one).
type RetryableFuncNoCtx func() error

// CircuitBreakerRegistry manages one CircuitBreaker per named
// dependency, lazily constructed on first use -- mirroring
// packages/router.CircuitBreakerRegistry's per-provider registry shape
// (Phase 012), generalized to any dependency name rather than a
// provider ID.
//
// All methods are safe for concurrent use from multiple goroutines.
type CircuitBreakerRegistry struct {
	mu       sync.RWMutex
	cfg      CircuitBreakerConfig
	breakers map[string]*CircuitBreaker
}

// NewCircuitBreakerRegistry returns an empty registry that lazily
// constructs breakers using cfg.
func NewCircuitBreakerRegistry(cfg CircuitBreakerConfig) *CircuitBreakerRegistry {
	return &CircuitBreakerRegistry{
		cfg:      cfg,
		breakers: make(map[string]*CircuitBreaker),
	}
}

// Get returns the CircuitBreaker for name, creating it lazily (using
// the registry's configured CircuitBreakerConfig) if it does not yet
// exist.
func (r *CircuitBreakerRegistry) Get(name string) *CircuitBreaker {
	r.mu.RLock()
	cb, ok := r.breakers[name]
	r.mu.RUnlock()
	if ok {
		return cb
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if cb, ok = r.breakers[name]; ok {
		return cb
	}
	cb = NewCircuitBreaker(name, r.cfg)
	r.breakers[name] = cb
	return cb
}
