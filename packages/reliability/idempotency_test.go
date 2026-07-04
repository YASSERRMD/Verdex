package reliability

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestIdempotencyGuard_FirstCallInvokesFn(t *testing.T) {
	g := NewIdempotencyGuard[string](time.Minute)
	calls := 0
	val, err := g.Execute(context.Background(), "key-1", func(_ context.Context) (string, error) {
		calls++
		return "result", nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if val != "result" {
		t.Fatalf("expected %q, got %q", "result", val)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

// TestIdempotencyGuard_SecondCallWithSameKeyReturnsCachedResult is the
// scenario the design brief calls out explicitly: a second Execute call
// with the same key, within the TTL window, must return the first
// call's cached result without invoking fn again.
func TestIdempotencyGuard_SecondCallWithSameKeyReturnsCachedResult(t *testing.T) {
	g := NewIdempotencyGuard[int](time.Minute)
	calls := 0
	fn := func(_ context.Context) (int, error) {
		calls++
		return 42, nil
	}

	v1, err1 := g.Execute(context.Background(), "same-key", fn)
	v2, err2 := g.Execute(context.Background(), "same-key", fn)

	if err1 != nil || err2 != nil {
		t.Fatalf("expected nil errors, got %v / %v", err1, err2)
	}
	if v1 != 42 || v2 != 42 {
		t.Fatalf("expected both calls to return 42, got %d / %d", v1, v2)
	}
	if calls != 1 {
		t.Fatalf("expected fn invoked exactly once across both Execute calls, got %d", calls)
	}
}

func TestIdempotencyGuard_DifferentKeysInvokeFnIndependently(t *testing.T) {
	g := NewIdempotencyGuard[int](time.Minute)
	calls := 0
	fn := func(_ context.Context) (int, error) {
		calls++
		return calls, nil
	}

	v1, _ := g.Execute(context.Background(), "key-a", fn)
	v2, _ := g.Execute(context.Background(), "key-b", fn)

	if v1 == v2 {
		t.Fatalf("expected distinct results for distinct keys, got %d and %d", v1, v2)
	}
	if calls != 2 {
		t.Fatalf("expected fn invoked once per distinct key (2 total), got %d", calls)
	}
}

func TestIdempotencyGuard_CachesErrorsToo(t *testing.T) {
	g := NewIdempotencyGuard[string](time.Minute)
	calls := 0
	fn := func(_ context.Context) (string, error) {
		calls++
		return "", errBoom
	}

	_, err1 := g.Execute(context.Background(), "err-key", fn)
	_, err2 := g.Execute(context.Background(), "err-key", fn)

	if !errors.Is(err1, errBoom) || !errors.Is(err2, errBoom) {
		t.Fatalf("expected both calls to surface errBoom, got %v / %v", err1, err2)
	}
	if calls != 1 {
		t.Fatalf("expected fn invoked exactly once even though it failed, got %d", calls)
	}
}

func TestIdempotencyGuard_ExpiredEntryInvokesFnAgain(t *testing.T) {
	g := NewIdempotencyGuard[int](time.Minute)
	fakeNow := time.Now()
	g.now = func() time.Time { return fakeNow }

	calls := 0
	fn := func(_ context.Context) (int, error) {
		calls++
		return calls, nil
	}

	v1, _ := g.Execute(context.Background(), "ttl-key", fn)
	if v1 != 1 {
		t.Fatalf("expected first call to return 1, got %d", v1)
	}

	// Advance the fake clock past the TTL.
	fakeNow = fakeNow.Add(2 * time.Minute)

	v2, _ := g.Execute(context.Background(), "ttl-key", fn)
	if v2 != 2 {
		t.Fatalf("expected fn re-invoked after TTL expiry (result 2), got %d", v2)
	}
	if calls != 2 {
		t.Fatalf("expected exactly 2 calls total, got %d", calls)
	}
}

func TestIdempotencyGuard_Forget_ForcesFreshCall(t *testing.T) {
	g := NewIdempotencyGuard[int](time.Minute)
	calls := 0
	fn := func(_ context.Context) (int, error) {
		calls++
		return calls, nil
	}

	if _, err := g.Execute(context.Background(), "k", fn); err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}
	g.Forget("k")
	v, _ := g.Execute(context.Background(), "k", fn)

	if v != 2 {
		t.Fatalf("expected fn re-invoked after Forget (result 2), got %d", v)
	}
	if calls != 2 {
		t.Fatalf("expected exactly 2 calls after Forget, got %d", calls)
	}
}

func TestIdempotencyGuard_DefaultTTLWhenNonPositive(t *testing.T) {
	g := NewIdempotencyGuard[int](0)
	if g.ttl != DefaultIdempotencyTTL {
		t.Fatalf("expected default TTL %v, got %v", DefaultIdempotencyTTL, g.ttl)
	}
}

func TestIdempotencyGuard_NilFuncOnUncachedKeyReturnsError(t *testing.T) {
	g := NewIdempotencyGuard[int](time.Minute)
	_, err := g.Execute(context.Background(), "no-cache", nil)
	if !errors.Is(err, ErrNilFunc) {
		t.Fatalf("expected ErrNilFunc, got %v", err)
	}
}

func TestIdempotencyGuard_NilFuncOnCachedKeyReplaysWithoutError(t *testing.T) {
	g := NewIdempotencyGuard[int](time.Minute)
	if _, err := g.Execute(context.Background(), "cached", func(_ context.Context) (int, error) { return 7, nil }); err != nil {
		t.Fatalf("unexpected error priming the cache: %v", err)
	}

	v, err := g.Execute(context.Background(), "cached", nil)
	if err != nil {
		t.Fatalf("expected cached replay to succeed even with nil fn, got %v", err)
	}
	if v != 7 {
		t.Fatalf("expected cached value 7, got %d", v)
	}
}

// TestIdempotencyGuard_ConcurrentCallsDedupeToSingleInvocation races many
// goroutines against the same key and asserts fn only ever runs once,
// exercising the in-flight-call waiting path under -race.
func TestIdempotencyGuard_ConcurrentCallsDedupeToSingleInvocation(t *testing.T) {
	g := NewIdempotencyGuard[int](time.Minute)
	var calls int32

	// fn deliberately always succeeds: this test's only concern is that
	// concurrent callers dedupe to a single invocation, not error handling.
	fn := func(_ context.Context) (int, error) { //nolint:unparam // error is intentionally always nil; see comment above
		atomic.AddInt32(&calls, 1)
		time.Sleep(20 * time.Millisecond) // widen the race window
		return 99, nil
	}

	const goroutines = 20
	var wg sync.WaitGroup
	results := make([]int, goroutines)
	errs := make([]error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			v, err := g.Execute(context.Background(), "race-key", fn)
			results[idx] = v
			errs[idx] = err
		}(i)
	}
	wg.Wait()

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected fn invoked exactly once across %d concurrent callers, got %d", goroutines, got)
	}
	for i, v := range results {
		if errs[i] != nil {
			t.Fatalf("goroutine %d: unexpected error %v", i, errs[i])
		}
		if v != 99 {
			t.Fatalf("goroutine %d: expected 99, got %d", i, v)
		}
	}
}

func TestIdempotencyGuard_ContextCancellationWhileWaitingOnInFlightCall(t *testing.T) {
	g := NewIdempotencyGuard[int](time.Minute)
	started := make(chan struct{})
	release := make(chan struct{})

	go func() {
		_, _ = g.Execute(context.Background(), "slow-key", func(_ context.Context) (int, error) {
			close(started)
			<-release
			return 1, nil
		})
	}()

	<-started // ensure the first call has registered its in-flight entry

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := g.Execute(ctx, "slow-key", func(_ context.Context) (int, error) {
		t.Error("fn must not be invoked a second time while the first call is in flight")
		return 0, nil
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded while waiting on the in-flight call, got %v", err)
	}

	close(release)
}
