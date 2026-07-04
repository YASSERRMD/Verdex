package reliability

import (
	"context"
	"errors"
	"math/rand"
	"testing"
	"time"
)

var errBoom = errors.New("boom")

func TestRetry_SucceedsOnFirstAttempt(t *testing.T) {
	calls := 0
	err := Retry(context.Background(), RetryPolicy{MaxAttempts: 3}, func(_ context.Context) error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected exactly 1 call, got %d", calls)
	}
}

func TestRetry_SucceedsAfterTransientFailures(t *testing.T) {
	calls := 0
	err := Retry(context.Background(), RetryPolicy{MaxAttempts: 5}, func(_ context.Context) error {
		calls++
		if calls < 3 {
			return errBoom
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected eventual success, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected exactly 3 calls, got %d", calls)
	}
}

func TestRetry_ExhaustsAttemptsAndWrapsLastError(t *testing.T) {
	calls := 0
	err := Retry(context.Background(), RetryPolicy{MaxAttempts: 3}, func(_ context.Context) error {
		calls++
		return errBoom
	})
	if err == nil {
		t.Fatal("expected an error after exhausting retries")
	}
	if !errors.Is(err, ErrRetriesExhausted) {
		t.Fatalf("expected errors.Is(err, ErrRetriesExhausted), got %v", err)
	}
	if !errors.Is(err, errBoom) {
		t.Fatalf("expected the last underlying error to remain in the chain, got %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected exactly 3 calls (MaxAttempts), got %d", calls)
	}
}

func TestRetry_DefaultMaxAttemptsWhenUnset(t *testing.T) {
	calls := 0
	err := Retry(context.Background(), RetryPolicy{}, func(_ context.Context) error {
		calls++
		return errBoom
	})
	if !errors.Is(err, ErrRetriesExhausted) {
		t.Fatalf("expected ErrRetriesExhausted, got %v", err)
	}
	if calls != DefaultMaxAttempts {
		t.Fatalf("expected %d calls (DefaultMaxAttempts), got %d", DefaultMaxAttempts, calls)
	}
}

func TestRetry_StopsOnContextCancellationBeforeAttempt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already canceled

	calls := 0
	err := Retry(ctx, RetryPolicy{MaxAttempts: 5}, func(_ context.Context) error {
		calls++
		return errBoom
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if calls != 0 {
		t.Fatalf("expected zero calls once ctx is already canceled, got %d", calls)
	}
}

func TestRetry_StopsOnContextCancellationDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	policy := RetryPolicy{
		MaxAttempts: 10,
		BaseDelay:   500 * time.Millisecond,
	}

	calls := 0
	start := time.Now()
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := Retry(ctx, policy, func(_ context.Context) error {
		calls++
		return errBoom
	})
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected exactly 1 call before cancellation during backoff sleep, got %d", calls)
	}
	// Must not have waited anywhere near the full 500ms backoff.
	if elapsed > 400*time.Millisecond {
		t.Fatalf("expected Retry to stop promptly once canceled, took %v", elapsed)
	}
}

func TestRetry_NilFuncReturnsError(t *testing.T) {
	err := Retry(context.Background(), RetryPolicy{}, nil)
	if !errors.Is(err, ErrNilFunc) {
		t.Fatalf("expected ErrNilFunc, got %v", err)
	}
}

func TestRetry_InvalidPolicyReturnsError(t *testing.T) {
	err := Retry(context.Background(), RetryPolicy{BaseDelay: -1}, func(_ context.Context) error {
		return nil
	})
	if !errors.Is(err, ErrInvalidRetryPolicy) {
		t.Fatalf("expected ErrInvalidRetryPolicy, got %v", err)
	}
}

func TestRetryPolicy_DelayForAttempt_ExponentialBackoffNoJitter(t *testing.T) {
	policy := RetryPolicy{BaseDelay: 100 * time.Millisecond}

	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{1, 100 * time.Millisecond},
		{2, 200 * time.Millisecond},
		{3, 400 * time.Millisecond},
		{4, 800 * time.Millisecond},
	}
	for _, c := range cases {
		got := policy.delayForAttempt(c.attempt)
		if got != c.want {
			t.Errorf("delayForAttempt(%d) = %v, want %v", c.attempt, got, c.want)
		}
	}
}

func TestRetryPolicy_DelayForAttempt_RespectsMaxDelay(t *testing.T) {
	policy := RetryPolicy{BaseDelay: 100 * time.Millisecond, MaxDelay: 250 * time.Millisecond}

	got := policy.delayForAttempt(4) // would be 800ms uncapped
	if got != 250*time.Millisecond {
		t.Fatalf("expected delay capped at MaxDelay (250ms), got %v", got)
	}
}

func TestRetryPolicy_DelayForAttempt_ZeroBaseDelayMeansNoBackoff(t *testing.T) {
	policy := RetryPolicy{}
	if got := policy.delayForAttempt(3); got != 0 {
		t.Fatalf("expected zero delay when BaseDelay is unset, got %v", got)
	}
}

// TestRetryPolicy_DelayForAttempt_JitterStaysWithinBounds verifies jitter
// distributes within the documented [(1-j)*d, (1+j)*d] envelope, using a
// seeded rand.Rand so the test is deterministic, not flaky.
func TestRetryPolicy_DelayForAttempt_JitterStaysWithinBounds(t *testing.T) {
	base := 100 * time.Millisecond
	policy := RetryPolicy{
		BaseDelay: base,
		Jitter:    0.5,
		rng:       rand.New(rand.NewSource(42)),
	}

	lower := time.Duration(float64(base) * 0.5)
	upper := time.Duration(float64(base) * 1.5)

	for i := 0; i < 100; i++ {
		got := policy.delayForAttempt(1)
		if got < lower || got > upper {
			t.Fatalf("iteration %d: delay %v outside jitter envelope [%v, %v]", i, got, lower, upper)
		}
	}
}

func TestRetryPolicy_JitterFraction_Clamped(t *testing.T) {
	if got := (RetryPolicy{Jitter: -1}).jitterFraction(); got != 0 {
		t.Fatalf("expected negative jitter clamped to 0, got %v", got)
	}
	if got := (RetryPolicy{Jitter: 2}).jitterFraction(); got != 1 {
		t.Fatalf("expected jitter > 1 clamped to 1, got %v", got)
	}
}

// TestRetry_BackoffTimingRoughlyMatchesPolicy asserts the wall-clock time
// spent retrying a fixed number of failures roughly matches the sum of the
// policy's expected (unjittered) backoff delays, within a generous
// tolerance to avoid flaking on a loaded CI machine.
func TestRetry_BackoffTimingRoughlyMatchesPolicy(t *testing.T) {
	policy := RetryPolicy{
		MaxAttempts: 3,
		BaseDelay:   30 * time.Millisecond,
	}
	// Expected total sleep: delay before attempt 2 (30ms) + delay before
	// attempt 3 (60ms) = 90ms. No jitter configured, so this is exact.
	wantMin := 85 * time.Millisecond
	wantMax := 400 * time.Millisecond

	start := time.Now()
	_ = Retry(context.Background(), policy, func(_ context.Context) error {
		return errBoom
	})
	elapsed := time.Since(start)

	if elapsed < wantMin || elapsed > wantMax {
		t.Fatalf("elapsed = %v, want within [%v, %v]", elapsed, wantMin, wantMax)
	}
}

func TestWithTimeout_ReturnsFnErrorWhenFast(t *testing.T) {
	err := WithTimeout(context.Background(), 100*time.Millisecond, func(_ context.Context) error {
		return errBoom
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("expected errBoom, got %v", err)
	}
}

func TestWithTimeout_DeadlineExceededWhenSlow(t *testing.T) {
	err := WithTimeout(context.Background(), 20*time.Millisecond, func(ctx context.Context) error {
		select {
		case <-time.After(200 * time.Millisecond):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
}

func TestWithTimeout_NonPositiveTimeoutRunsUnbounded(t *testing.T) {
	called := false
	err := WithTimeout(context.Background(), 0, func(_ context.Context) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !called {
		t.Fatal("expected fn to be invoked with a non-positive timeout")
	}
}

func TestWithTimeout_NilFuncReturnsError(t *testing.T) {
	err := WithTimeout(context.Background(), time.Second, nil)
	if !errors.Is(err, ErrNilFunc) {
		t.Fatalf("expected ErrNilFunc, got %v", err)
	}
}
