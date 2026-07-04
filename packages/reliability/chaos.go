package reliability

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

// FailureMode names a kind of fault a FailureInjector can inject.
type FailureMode string

const (
	// FailureModeNone injects no fault: the wrapped function runs
	// normally. Useful as an explicit "control" entry in a Pattern.
	FailureModeNone FailureMode = "none"

	// FailureModeLatency injects extra latency (see
	// FailureInjectorConfig.Latency) before invoking the wrapped
	// function.
	FailureModeLatency FailureMode = "latency"

	// FailureModeError short-circuits the wrapped function entirely,
	// returning FailureInjectorConfig.Err (or a default injected error
	// if unset) without invoking it.
	FailureModeError FailureMode = "error"

	// FailureModePanic invokes the wrapped function inside a recover
	// guard after triggering a panic, so callers can exercise their own
	// panic-recovery paths deterministically. The panic value is
	// recovered inside Execute and surfaced as an error (never
	// propagated to the caller's goroutine), so a test using this mode
	// cannot itself crash from an injected panic.
	FailureModePanic FailureMode = "panic"
)

// DefaultInjectedLatency is the latency FailureInjectorConfig.Latency
// falls back to for FailureModeLatency when left at its zero value.
const DefaultInjectedLatency = 100 * time.Millisecond

// ErrInjectedFailure is the default error returned by FailureModeError
// when FailureInjectorConfig.Err is nil.
var ErrInjectedFailure = fmt.Errorf("reliability: injected failure")

// ErrInjectedPanic wraps a recovered FailureModePanic panic value.
var ErrInjectedPanic = fmt.Errorf("reliability: injected panic")

// FailureInjectorConfig configures a FailureInjector's injected fault
// parameters.
type FailureInjectorConfig struct {
	// Pattern is the deterministic, seeded sequence of FailureMode
	// values to cycle through, one per Execute call, wrapping around
	// once exhausted. A nil or empty Pattern is equivalent to
	// []FailureMode{FailureModeNone} (never inject anything).
	//
	// A fixed Pattern slice (rather than a random rate) is
	// deliberate: it makes FailureInjector's own tests -- and any
	// other package's tests that use it as a fixture -- deterministic
	// and reproducible, never flaky from true randomness.
	Pattern []FailureMode

	// Latency is the extra delay injected by FailureModeLatency.
	// Non-positive falls back to DefaultInjectedLatency.
	Latency time.Duration

	// Err is the error returned by FailureModeError. Nil falls back to
	// ErrInjectedFailure.
	Err error
}

func (c FailureInjectorConfig) latency() time.Duration {
	if c.Latency <= 0 {
		return DefaultInjectedLatency
	}
	return c.Latency
}

func (c FailureInjectorConfig) err() error {
	if c.Err == nil {
		return ErrInjectedFailure
	}
	return c.Err
}

func (c FailureInjectorConfig) pattern() []FailureMode {
	if len(c.Pattern) == 0 {
		return []FailureMode{FailureModeNone}
	}
	return c.Pattern
}

// FailureInjector wraps a function and deterministically injects one
// of a configured sequence of failure modes per call, for use in other
// packages' tests exercising Retry/CircuitBreaker/Degrader against a
// dependency that misbehaves in controlled, reproducible ways --
// rather than every package hand-rolling its own flaky "fail every
// Nth call" test fixture.
//
// All methods are safe for concurrent use from multiple goroutines.
type FailureInjector struct {
	cfg   FailureInjectorConfig
	calls int64 // atomic
}

// NewFailureInjector constructs a FailureInjector with cfg.
func NewFailureInjector(cfg FailureInjectorConfig) *FailureInjector {
	return &FailureInjector{cfg: cfg}
}

// Calls returns the number of times Execute has been invoked so far.
func (fi *FailureInjector) Calls() int64 {
	return atomic.LoadInt64(&fi.calls)
}

// nextMode advances and returns this call's FailureMode from the
// configured Pattern, cycling deterministically.
func (fi *FailureInjector) nextMode() FailureMode {
	pattern := fi.cfg.pattern()
	idx := atomic.AddInt64(&fi.calls, 1) - 1
	return pattern[int(idx)%len(pattern)]
}

// Execute runs fn under this call's injected FailureMode:
//
//   - FailureModeNone: fn runs unmodified.
//   - FailureModeLatency: Execute sleeps for the configured latency
//     (honoring ctx cancellation) before running fn.
//   - FailureModeError: fn is not invoked at all; the configured error
//     is returned immediately.
//   - FailureModePanic: fn is invoked inside a recover guard after a
//     panic is triggered; the recovered value is wrapped in
//     ErrInjectedPanic and returned as an error rather than crashing
//     the calling goroutine.
//
// Returns ErrNilFunc if fn is nil.
func (fi *FailureInjector) Execute(ctx context.Context, fn RetryableFunc) error {
	if fn == nil {
		return wrapf("FailureInjector.Execute", ErrNilFunc)
	}

	switch fi.nextMode() {
	case FailureModeError:
		return fi.cfg.err()

	case FailureModeLatency:
		timer := time.NewTimer(fi.cfg.latency())
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return ctx.Err()
		}
		return fn(ctx)

	case FailureModePanic:
		return runWithPanicInjected(fn, ctx)

	case FailureModeNone:
		fallthrough
	default:
		return fn(ctx)
	}
}

// runWithPanicInjected simulates a dependency that panics instead of
// returning normally: it triggers a panic before fn would run (mirroring
// how FailureModeError short-circuits without invoking fn) and recovers
// it within this same call stack, so a caller exercising "does my code
// survive a panicking dependency" scenario observes a real, recovered
// failure signal (wrapped in ErrInjectedPanic) rather than a crashed
// goroutine.
func runWithPanicInjected(fn RetryableFunc, ctx context.Context) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%w: %v", ErrInjectedPanic, r)
		}
	}()

	panicInjectedDependency(ctx, fn)
	return nil
}

// panicInjectedDependency always panics; it exists as a separate
// function (rather than an inline panic() in runWithPanicInjected) so
// fn and ctx are visibly part of the simulated call even though the
// panic preempts fn ever actually running -- matching this file's other
// FailureMode branches, which all accept and are scoped to a single
// (ctx, fn) call.
func panicInjectedDependency(_ context.Context, _ RetryableFunc) {
	panic("reliability: injected chaos panic")
}
