package integration

import (
	"context"
	"errors"
	"math/rand"
	"time"
)

// RetryPolicy configures WithRetry's bounded, jittered exponential
// backoff (task 6). A zero-value RetryPolicy is invalid; use
// DefaultRetryPolicy for sensible defaults.
type RetryPolicy struct {
	// MaxAttempts is the maximum number of times the wrapped call is
	// invoked, including the first attempt. Must be at least 1.
	MaxAttempts int

	// BaseDelay is the delay before the second attempt. Each
	// subsequent attempt doubles the previous delay (exponential
	// backoff), before jitter is applied. Must be positive.
	BaseDelay time.Duration

	// MaxDelay caps the computed backoff delay before jitter, so
	// retries do not grow unbounded on a long-running failure. Must be
	// >= BaseDelay.
	MaxDelay time.Duration

	// Jitter is the maximum fraction (0.0-1.0) of the computed delay
	// added or subtracted at random, avoiding synchronized retry storms
	// across concurrent callers. Zero disables jitter (useful for
	// deterministic tests).
	Jitter float64
}

// DefaultRetryPolicy returns a RetryPolicy with reasonable defaults for
// calling an external court case-management system: 3 attempts, 500ms
// base delay, 10s cap, 20% jitter.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts: 3,
		BaseDelay:   500 * time.Millisecond,
		MaxDelay:    10 * time.Second,
		Jitter:      0.2,
	}
}

// Validate checks p for structural well-formedness.
func (p RetryPolicy) Validate() error {
	if p.MaxAttempts < 1 {
		return wrapf("RetryPolicy.Validate", errors.New("integration: MaxAttempts must be at least 1"))
	}
	if p.BaseDelay <= 0 {
		return wrapf("RetryPolicy.Validate", errors.New("integration: BaseDelay must be positive"))
	}
	if p.MaxDelay < p.BaseDelay {
		return wrapf("RetryPolicy.Validate", errors.New("integration: MaxDelay must be >= BaseDelay"))
	}
	if p.Jitter < 0 || p.Jitter > 1 {
		return wrapf("RetryPolicy.Validate", errors.New("integration: Jitter must be in [0,1]"))
	}
	return nil
}

// delayForAttempt returns the backoff delay before attempt (1-indexed;
// attempt 1 never delays, since it is the first try). rng supplies
// jitter; passing a nil rng disables jitter regardless of p.Jitter,
// used by deterministic tests.
func (p RetryPolicy) delayForAttempt(attempt int, rng *rand.Rand) time.Duration {
	if attempt <= 1 {
		return 0
	}
	// attempt 2 -> BaseDelay * 2^0, attempt 3 -> BaseDelay * 2^1, ...
	delay := p.BaseDelay << uint(attempt-2) //nolint:gosec // bounded by MaxAttempts, not overflow-prone in practice
	if delay > p.MaxDelay || delay <= 0 {
		delay = p.MaxDelay
	}
	if p.Jitter <= 0 || rng == nil {
		return delay
	}
	jitterRange := float64(delay) * p.Jitter
	offset := (rng.Float64()*2 - 1) * jitterRange // uniform in [-jitterRange, +jitterRange]
	jittered := time.Duration(float64(delay) + offset)
	if jittered < 0 {
		jittered = 0
	}
	return jittered
}

// RetryableFunc is the signature WithRetry wraps: a single attempt at
// a Connector-backed operation.
type RetryableFunc func(ctx context.Context) error

// IsRetryable, when non-nil, lets a caller mark certain errors as
// non-retryable (e.g. a structural validation failure that will never
// succeed on retry) even though attempts remain. A nil IsRetryable
// treats every error as retryable.
type IsRetryable func(err error) bool

// WithRetry invokes fn up to policy.MaxAttempts times, applying
// jittered exponential backoff between attempts, until fn succeeds,
// ctx is cancelled, shouldRetry (if non-nil) reports an error as
// non-retryable, or attempts are exhausted. This is the real
// retry-with-backoff wrapper task 6 requires around Connector calls
// (Ping/ImportCases/DeliverReport) -- bounded attempts, jittered
// backoff, not a bare loop.
//
// WithRetry returns the last error encountered, wrapped in
// ErrRetriesExhausted once MaxAttempts is reached without success.
func WithRetry(ctx context.Context, policy RetryPolicy, shouldRetry IsRetryable, fn RetryableFunc) error {
	if err := policy.Validate(); err != nil {
		return err
	}
	if fn == nil {
		return wrapf("WithRetry", errors.New("integration: fn must not be nil"))
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec // jitter timing only, not security-sensitive
	var lastErr error

	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		if delay := policy.delayForAttempt(attempt, rng); delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}

		if err := ctx.Err(); err != nil {
			return err
		}

		err := fn(ctx)
		if err == nil {
			return nil
		}
		lastErr = err

		if shouldRetry != nil && !shouldRetry(err) {
			return err
		}
	}

	return wrapf("WithRetry", errors.Join(ErrRetriesExhausted, lastErr))
}
