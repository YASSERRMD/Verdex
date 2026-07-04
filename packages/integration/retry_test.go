package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/integration"
)

func TestRetryPolicyValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		policy  integration.RetryPolicy
		wantErr bool
	}{
		{"default policy", integration.DefaultRetryPolicy(), false},
		{"zero attempts", integration.RetryPolicy{MaxAttempts: 0, BaseDelay: time.Second, MaxDelay: time.Second}, true},
		{"zero base delay", integration.RetryPolicy{MaxAttempts: 1, BaseDelay: 0, MaxDelay: time.Second}, true},
		{"max less than base", integration.RetryPolicy{MaxAttempts: 1, BaseDelay: 2 * time.Second, MaxDelay: time.Second}, true},
		{"negative jitter", integration.RetryPolicy{MaxAttempts: 1, BaseDelay: time.Second, MaxDelay: time.Second, Jitter: -0.1}, true},
		{"jitter over one", integration.RetryPolicy{MaxAttempts: 1, BaseDelay: time.Second, MaxDelay: time.Second, Jitter: 1.1}, true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.policy.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("Validate() = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestWithRetrySucceedsFirstAttempt(t *testing.T) {
	t.Parallel()
	calls := 0
	policy := integration.RetryPolicy{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: 10 * time.Millisecond}

	err := integration.WithRetry(context.Background(), policy, nil, func(_ context.Context) error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("WithRetry() error = %v", err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestWithRetrySucceedsAfterFailures(t *testing.T) {
	t.Parallel()
	calls := 0
	policy := integration.RetryPolicy{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: 10 * time.Millisecond}

	err := integration.WithRetry(context.Background(), policy, nil, func(_ context.Context) error {
		calls++
		if calls < 3 {
			return errors.New("transient failure")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WithRetry() error = %v", err)
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}

func TestWithRetryExhaustsAttempts(t *testing.T) {
	t.Parallel()
	calls := 0
	policy := integration.RetryPolicy{MaxAttempts: 3, BaseDelay: time.Millisecond, MaxDelay: 10 * time.Millisecond}
	wantErr := errors.New("permanent failure")

	err := integration.WithRetry(context.Background(), policy, nil, func(_ context.Context) error {
		calls++
		return wantErr
	})
	if !errors.Is(err, integration.ErrRetriesExhausted) {
		t.Fatalf("WithRetry() error = %v, want ErrRetriesExhausted", err)
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("WithRetry() error = %v, want it to wrap %v", err, wantErr)
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3 (MaxAttempts)", calls)
	}
}

func TestWithRetryHonorsNonRetryable(t *testing.T) {
	t.Parallel()
	calls := 0
	policy := integration.RetryPolicy{MaxAttempts: 5, BaseDelay: time.Millisecond, MaxDelay: 10 * time.Millisecond}
	permanentErr := errors.New("do not retry me")

	shouldRetry := func(err error) bool {
		return !errors.Is(err, permanentErr)
	}

	err := integration.WithRetry(context.Background(), policy, shouldRetry, func(_ context.Context) error {
		calls++
		return permanentErr
	})
	if !errors.Is(err, permanentErr) {
		t.Fatalf("WithRetry() error = %v, want %v", err, permanentErr)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (should not retry a non-retryable error)", calls)
	}
}

func TestWithRetryHonorsContextCancellation(t *testing.T) {
	t.Parallel()
	policy := integration.RetryPolicy{MaxAttempts: 5, BaseDelay: 50 * time.Millisecond, MaxDelay: 200 * time.Millisecond}

	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	err := integration.WithRetry(ctx, policy, nil, func(_ context.Context) error {
		calls++
		return errors.New("always fails")
	})
	if err == nil {
		t.Fatal("WithRetry() error = nil, want context cancellation or retries exhausted")
	}
	if calls == 0 {
		t.Error("expected at least one call before cancellation")
	}
}

func TestWithRetryRejectsNilFunc(t *testing.T) {
	t.Parallel()
	policy := integration.DefaultRetryPolicy()
	err := integration.WithRetry(context.Background(), policy, nil, nil)
	if err == nil {
		t.Fatal("WithRetry() error = nil, want error for nil fn")
	}
}

func TestWithRetryRejectsInvalidPolicy(t *testing.T) {
	t.Parallel()
	badPolicy := integration.RetryPolicy{}
	err := integration.WithRetry(context.Background(), badPolicy, nil, func(_ context.Context) error {
		return nil
	})
	if err == nil {
		t.Fatal("WithRetry() error = nil, want error for invalid policy")
	}
}
