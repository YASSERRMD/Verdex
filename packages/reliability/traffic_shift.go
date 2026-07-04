package reliability

import (
	"context"
	"sort"
	"sync"
	"sync/atomic"
)

// HealthStatus reports whether a single named Backend is currently fit
// to receive traffic. This mirrors the boolean shape of
// packages/observability.Checker (Phase 003, "nil error == healthy")
// by reference -- HealthStatus is a plain enum rather than an
// error-returning func because TrafficShifter needs to hold the last
// known status for many backends at once (a readiness Checker is
// invoked fresh per HTTP request; a TrafficShifter is consulted on
// every routed call and cannot afford to re-run a potentially slow
// dependency check that often). A caller typically drives
// Backend.Status from the same underlying check a
// packages/observability.NamedChecker already wraps, via
// TrafficShifter.SetStatus, rather than this package importing
// packages/observability just to reuse its func type.
type HealthStatus int

const (
	// HealthUnknown means no status has been reported yet. Treated the
	// same as HealthUnhealthy for selection purposes (fail closed: an
	// unreported backend is not assumed healthy).
	HealthUnknown HealthStatus = iota

	// HealthHealthy means the backend's last reported check succeeded.
	HealthHealthy

	// HealthUnhealthy means the backend's last reported check failed.
	HealthUnhealthy
)

// String satisfies fmt.Stringer.
func (h HealthStatus) String() string {
	switch h {
	case HealthHealthy:
		return "healthy"
	case HealthUnhealthy:
		return "unhealthy"
	case HealthUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}

// Backend names one traffic destination TrafficShifter can route to,
// alongside its last-known HealthStatus.
type Backend struct {
	// Name uniquely identifies this backend (e.g. "postgres-primary",
	// "postgres-replica-eu", "graph-store-neo4j-1").
	Name string

	// Status is the backend's last-known health, as reported via
	// TrafficShifter.SetStatus.
	Status HealthStatus
}

// TrafficShifter selects which currently-healthy Backend(s) should
// receive traffic, given each registered backend's last-reported
// HealthStatus:
//
//   - All backends healthy: every healthy backend is eligible,
//     selected in round-robin order (Select rotates which backend is
//     returned first across successive calls).
//   - Some unhealthy: traffic degrades to the remaining healthy set
//     (round-robin over just those).
//   - None healthy: Select fails closed, returning ErrNoHealthyBackends
//     rather than routing to a known-bad backend.
//
// All methods are safe for concurrent use from multiple goroutines.
type TrafficShifter struct {
	mu       sync.RWMutex
	backends map[string]HealthStatus
	order    []string // registration order, for stable round-robin
	rrCursor int64    // atomic
}

// NewTrafficShifter constructs a TrafficShifter with the given initial
// backend names, all starting at HealthUnknown until SetStatus reports
// otherwise.
func NewTrafficShifter(backendNames ...string) *TrafficShifter {
	ts := &TrafficShifter{backends: make(map[string]HealthStatus, len(backendNames))}
	for _, name := range backendNames {
		ts.backends[name] = HealthUnknown
		ts.order = append(ts.order, name)
	}
	return ts
}

// SetStatus records status for the named backend, registering it if
// not already known.
func (ts *TrafficShifter) SetStatus(name string, status HealthStatus) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if _, known := ts.backends[name]; !known {
		ts.order = append(ts.order, name)
	}
	ts.backends[name] = status
}

// Backends returns every registered backend and its current status,
// in registration order.
func (ts *TrafficShifter) Backends() []Backend {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	out := make([]Backend, 0, len(ts.order))
	for _, name := range ts.order {
		out = append(out, Backend{Name: name, Status: ts.backends[name]})
	}
	return out
}

// healthyNamesLocked returns the sorted names of every backend
// currently HealthHealthy. Callers must hold ts.mu (read or write).
func (ts *TrafficShifter) healthyNamesLocked() []string {
	healthy := make([]string, 0, len(ts.order))
	for _, name := range ts.order {
		if ts.backends[name] == HealthHealthy {
			healthy = append(healthy, name)
		}
	}
	sort.Strings(healthy) // deterministic order regardless of map iteration/registration order
	return healthy
}

// Select returns the single backend that should receive the next
// unit of traffic: round-robin across every currently healthy
// backend. Returns ErrNoHealthyBackends if none are healthy (fail
// closed).
func (ts *TrafficShifter) Select() (string, error) {
	ts.mu.RLock()
	healthy := ts.healthyNamesLocked()
	ts.mu.RUnlock()

	if len(healthy) == 0 {
		return "", wrapf("TrafficShifter.Select", ErrNoHealthyBackends)
	}

	idx := atomic.AddInt64(&ts.rrCursor, 1) - 1
	return healthy[int(idx)%len(healthy)], nil
}

// SelectAll returns every currently healthy backend name, sorted, for
// callers that fan out to all healthy backends rather than
// round-robining across them one at a time (e.g. a broadcast write).
// Returns ErrNoHealthyBackends if none are healthy.
func (ts *TrafficShifter) SelectAll() ([]string, error) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	healthy := ts.healthyNamesLocked()
	if len(healthy) == 0 {
		return nil, wrapf("TrafficShifter.SelectAll", ErrNoHealthyBackends)
	}
	return healthy, nil
}

// HealthyCount returns the number of currently healthy backends.
func (ts *TrafficShifter) HealthyCount() int {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return len(ts.healthyNamesLocked())
}

// RefreshFromCheckers runs every given HealthCheckFunc against ctx and
// updates this TrafficShifter's status for the matching backend name,
// mirroring how packages/observability.ReadinessHandler (Phase 003)
// runs a set of NamedChecker before responding -- the same
// "func(ctx) error, nil means healthy" convention, so a caller
// wiring both together can share one underlying check function per
// dependency between its /readyz handler and its TrafficShifter.
func (ts *TrafficShifter) RefreshFromCheckers(ctx context.Context, checkers map[string]HealthCheckFunc) {
	for name, check := range checkers {
		if check == nil {
			ts.SetStatus(name, HealthUnknown)
			continue
		}
		if err := check(ctx); err != nil {
			ts.SetStatus(name, HealthUnhealthy)
		} else {
			ts.SetStatus(name, HealthHealthy)
		}
	}
}

// HealthCheckFunc reports whether a single dependency is currently
// healthy: nil means healthy, any error means unhealthy. This is the
// exact same shape as packages/observability.Checker (Phase 003),
// named independently here so this package does not import
// packages/observability solely to reuse a one-line func type -- see
// doc.go and doc/reliability.md.
type HealthCheckFunc func(ctx context.Context) error
