package reliability

import (
	"context"
	"math/rand"
	"time"
)

// DefaultMaxAttempts is the default number of attempts (including the
// first) Retry allows before giving up, when RetryPolicy.MaxAttempts is
// left at its zero value.
const DefaultMaxAttempts = 3

// RetryPolicy configures Retry's attempt count and exponential
// backoff-with-jitter schedule. This generalizes the same
// bounded-attempt shape packages/ingestion.RetryPolicy already applies
// per pipeline stage (Phase 029), adding a real, timed backoff
// component -- ingestion's own RetryPolicy intentionally has no
// backoff/sleep of its own (see packages/ingestion/retry.go), so this
// is additive, not a replacement.
type RetryPolicy struct {
	// MaxAttempts is the maximum number of attempts (including the
	// first). Values <= 0 fall back to DefaultMaxAttempts.
	MaxAttempts int

	// BaseDelay is the backoff delay before the second attempt. Each
	// subsequent attempt doubles the previous delay (exponential
	// backoff), before jitter is applied. Zero disables backoff
	// entirely (Retry then loops with no delay between attempts).
	BaseDelay time.Duration

	// MaxDelay caps the computed backoff delay before jitter is
	// applied. Zero means uncapped.
	MaxDelay time.Duration

	// Jitter is the fraction (0.0-1.0) of the computed backoff delay
	// that is randomized: the actual delay is chosen uniformly from
	// [(1-Jitter)*d, (1+Jitter)*d]. Zero means no jitter (the exact
	// computed delay is used every time). Values are clamped to
	// [0,1].
	Jitter float64

	// rng is the random source used for jitter. Nil means
	// math/rand's package-level source. Unexported and only set by
	// tests via withRNG, so production callers get real randomness by
	// default while tests can inject a seeded, deterministic source.
	rng *rand.Rand
}

// maxAttempts returns p.MaxAttempts, or DefaultMaxAttempts if unset.
func (p RetryPolicy) maxAttempts() int {
	if p.MaxAttempts <= 0 {
		return DefaultMaxAttempts
	}
	return p.MaxAttempts
}

// validate reports whether p is structurally well-formed.
func (p RetryPolicy) validate() error {
	if p.BaseDelay < 0 || p.MaxDelay < 0 {
		return ErrInvalidRetryPolicy
	}
	return nil
}

// jitterFraction clamps p.Jitter to [0,1].
func (p RetryPolicy) jitterFraction() float64 {
	switch {
	case p.Jitter < 0:
		return 0
	case p.Jitter > 1:
		return 1
	default:
		return p.Jitter
	}
}

// delayForAttempt computes the backoff delay before the given attempt
// number (2-indexed: the delay before the 2nd attempt, 3rd attempt,
// etc -- there is never a delay before the 1st). attempt is 1-indexed
// matching the attempt that just failed, so delayForAttempt(1)
// computes the wait before attempt 2.
func (p RetryPolicy) delayForAttempt(attempt int) time.Duration {
	if p.BaseDelay <= 0 {
		return 0
	}

	// Exponential backoff: BaseDelay * 2^(attempt-1).
	delay := p.BaseDelay
	for i := 1; i < attempt; i++ {
		delay *= 2
		if p.MaxDelay > 0 && delay >= p.MaxDelay {
			delay = p.MaxDelay
			break
		}
	}
	if p.MaxDelay > 0 && delay > p.MaxDelay {
		delay = p.MaxDelay
	}

	jitter := p.jitterFraction()
	if jitter == 0 {
		return delay
	}

	r := p.rng
	if r == nil {
		r = rand.New(rand.NewSource(time.Now().UnixNano())) // #nosec G404 -- jitter timing, not a security-sensitive value //nolint:gosec
	}
	// Uniformly distributed in [(1-jitter)*delay, (1+jitter)*delay].
	lower := float64(delay) * (1 - jitter)
	spread := float64(delay) * 2 * jitter
	jittered := lower + r.Float64()*spread
	if jittered < 0 {
		jittered = 0
	}
	return time.Duration(jittered)
}

// RetryableFunc is the shape of work Retry attempts. It receives the
// same ctx passed to Retry (or a derived one), so it can honor
// cancellation/timeouts.
type RetryableFunc func(ctx context.Context) error

// Retry invokes fn up to policy.maxAttempts() times, sleeping between
// attempts per policy's exponential-backoff-with-jitter schedule,
// until fn succeeds (returns nil), ctx is canceled/expires, or attempts
// are exhausted.
//
//   - On success, Retry returns nil immediately.
//   - If ctx is done (canceled or deadline exceeded) either before an
//     attempt or while sleeping between attempts, Retry stops
//     retrying and returns ctx.Err() -- it never sleeps past a
//     canceled context.
//   - If every attempt fails without ctx ever becoming done, Retry
//     returns the last error wrapped in ErrRetriesExhausted.
//
// Returns ErrNilFunc if fn is nil, or a wrapped ErrInvalidRetryPolicy
// if policy fails structural validation.
func Retry(ctx context.Context, policy RetryPolicy, fn RetryableFunc) error {
	if fn == nil {
		return wrapf("Retry", ErrNilFunc)
	}
	if err := policy.validate(); err != nil {
		return wrapf("Retry", err)
	}

	max := policy.maxAttempts()
	var lastErr error
	for attempt := 1; attempt <= max; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}

		if attempt == max {
			break
		}

		delay := policy.delayForAttempt(attempt)
		if delay <= 0 {
			continue
		}

		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		}
	}

	return wrapf("Retry", fmtRetriesExhausted(max, lastErr))
}

// fmtRetriesExhausted wraps lastErr with ErrRetriesExhausted while
// preserving it in the error chain (errors.Is/As continue to work
// against both ErrRetriesExhausted and lastErr).
func fmtRetriesExhausted(attempts int, lastErr error) error {
	return &retriesExhaustedError{attempts: attempts, cause: lastErr}
}

// retriesExhaustedError implements error and Unwrap, chaining both
// ErrRetriesExhausted (via Is) and the last underlying failure (via
// Unwrap), so callers can test either with errors.Is/errors.As.
type retriesExhaustedError struct {
	attempts int
	cause    error
}

func (e *retriesExhaustedError) Error() string {
	if e.cause == nil {
		return ErrRetriesExhausted.Error()
	}
	return ErrRetriesExhausted.Error() + ": " + e.cause.Error()
}

func (e *retriesExhaustedError) Unwrap() error { return e.cause }

func (e *retriesExhaustedError) Is(target error) bool {
	return target == ErrRetriesExhausted
}
