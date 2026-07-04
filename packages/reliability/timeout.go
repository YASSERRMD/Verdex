package reliability

import (
	"context"
	"time"
)

// WithTimeout runs fn against a derived context that is canceled after
// timeout elapses, returning fn's error, or the derived context's
// error (typically context.DeadlineExceeded) if fn has not returned by
// the time the timeout fires. A non-positive timeout means "no
// deadline": fn simply runs against ctx unchanged.
//
// This is a thin, dependency-free convenience wrapper over
// context.WithTimeout -- most call sites in this codebase can and do
// call context.WithTimeout directly; WithTimeout exists so
// Retry/CircuitBreaker/Degrader callers in this package (and its
// consumers) can attach a per-attempt timeout in one line:
//
//	err := Retry(ctx, policy, func(ctx context.Context) error {
//	    return WithTimeout(ctx, 2*time.Second, doWork)
//	})
//
// Note fn continues running in its own goroutine after WithTimeout
// returns on a timeout (Go has no mechanism to forcibly abort a
// running goroutine); callers whose fn performs I/O should still
// thread cctx through so the underlying call (an HTTP request, a DB
// query) itself observes cancellation and returns promptly.
func WithTimeout(ctx context.Context, timeout time.Duration, fn RetryableFunc) error {
	if fn == nil {
		return wrapf("WithTimeout", ErrNilFunc)
	}
	if timeout <= 0 {
		return fn(ctx)
	}

	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- fn(cctx)
	}()

	select {
	case err := <-done:
		return err
	case <-cctx.Done():
		return cctx.Err()
	}
}
