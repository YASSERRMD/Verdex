package scalability

import "sync"

// BackpressureConfig configures a BackpressureController (task 6).
type BackpressureConfig struct {
	// MaxInFlight is the maximum number of concurrently admitted
	// requests before Admit starts shedding load. Must be > 0.
	MaxInFlight int
}

// Validate reports whether cfg is structurally well-formed.
func (cfg BackpressureConfig) Validate() error {
	if cfg.MaxInFlight <= 0 {
		return wrapf("Validate", ErrInvalidBackpressureConfig)
	}
	return nil
}

// BackpressureController accepts, rejects, or sheds load based on a
// configurable in-flight-request threshold (task 6). It is a real,
// concurrency-safe counter, not a rate limiter over time -- Admit
// succeeds as long as fewer than MaxInFlight admitted requests are
// currently outstanding (i.e. have not yet called Release), and fails
// with ErrLoadShed the instant that threshold would be exceeded,
// recovering automatically as soon as enough in-flight requests
// Release.
//
// Safe for concurrent use by multiple goroutines.
type BackpressureController struct {
	mu          sync.Mutex
	maxInFlight int
	inFlight    int
	closed      bool
}

// NewBackpressureController constructs a BackpressureController from
// cfg. Returns ErrInvalidBackpressureConfig if cfg fails validation.
func NewBackpressureController(cfg BackpressureConfig) (*BackpressureController, error) {
	if err := cfg.Validate(); err != nil {
		return nil, wrapf("NewBackpressureController", err)
	}
	return &BackpressureController{maxInFlight: cfg.MaxInFlight}, nil
}

// Admit attempts to admit one unit of work. On success, the caller
// must call Release exactly once when that work completes (typically
// via defer immediately after a successful Admit). Returns
// ErrLoadShed if MaxInFlight admitted-but-not-yet-released requests
// are already outstanding, or ErrControllerClosed if Close has been
// called.
func (c *BackpressureController) Admit() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return wrapf("Admit", ErrControllerClosed)
	}
	if c.inFlight >= c.maxInFlight {
		return wrapf("Admit", ErrLoadShed)
	}
	c.inFlight++
	return nil
}

// Release frees one in-flight slot, allowing a subsequently blocked
// (shed) caller to succeed on its next Admit call. Callers must pair
// every successful Admit with exactly one Release. Calling Release
// without a matching successful Admit undercounts in-flight work and
// will let more requests through than MaxInFlight permits -- this is
// a caller-discipline contract, mirrored by every semaphore-style
// primitive in this codebase (see perf.Limiter.Release's identical
// contract).
func (c *BackpressureController) Release() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.inFlight > 0 {
		c.inFlight--
	}
}

// InFlight returns the current number of admitted-but-not-yet-released
// requests. Exposed primarily for observability/testing.
func (c *BackpressureController) InFlight() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.inFlight
}

// Close marks the controller closed: every subsequent Admit call
// returns ErrControllerClosed regardless of in-flight count. Close is
// idempotent and safe to call concurrently with Admit/Release.
func (c *BackpressureController) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
}
