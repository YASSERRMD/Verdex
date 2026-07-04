package reliability

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// This file contains resilience tests (task 9) that combine multiple
// primitives from this package against small, in-package fake
// dependencies -- never against real external services -- mirroring
// how packages/perf's benchmark tests build fixtures directly rather
// than standing up infrastructure.

// fakeDatabase is a minimal, in-memory stand-in for a flaky dependency
// (e.g. Postgres, the graph store) whose failure behavior a test can
// script deterministically.
type fakeDatabase struct {
	mu              sync.Mutex
	failNextN       int
	permanentlyDown bool
	calls           int
}

func (f *fakeDatabase) Query(_ context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.permanentlyDown {
		return "", errBoom
	}
	if f.failNextN > 0 {
		f.failNextN--
		return "", errBoom
	}
	return "live-row", nil
}

func (f *fakeDatabase) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// TestResilience_RetryRecoversFromTransientDependencyFailure exercises
// Retry wrapping a fake dependency that fails twice, then succeeds --
// the realistic "transient blip" scenario Retry exists for.
func TestResilience_RetryRecoversFromTransientDependencyFailure(t *testing.T) {
	db := &fakeDatabase{failNextN: 2}

	var result string
	err := Retry(context.Background(), RetryPolicy{MaxAttempts: 5, BaseDelay: 5 * time.Millisecond}, func(ctx context.Context) error {
		v, qerr := db.Query(ctx)
		if qerr != nil {
			return qerr
		}
		result = v
		return nil
	})

	if err != nil {
		t.Fatalf("expected Retry to recover from the transient failures, got %v", err)
	}
	if result != "live-row" {
		t.Fatalf("expected the eventual successful query result, got %q", result)
	}
	if db.callCount() != 3 {
		t.Fatalf("expected exactly 3 attempts (2 failures + 1 success), got %d", db.callCount())
	}
}

// TestResilience_CircuitBreakerProtectsAgainstPermanentlyDownDependency
// scripts a dependency that never recovers and confirms the breaker
// stops hammering it once tripped, then allows exactly one probe after
// cooldown -- combining CircuitBreaker with a fake dependency rather
// than a real one.
func TestResilience_CircuitBreakerProtectsAgainstPermanentlyDownDependency(t *testing.T) {
	db := &fakeDatabase{permanentlyDown: true}
	cb := NewCircuitBreaker("fake-db", CircuitBreakerConfig{FailureThreshold: 3, Cooldown: 20 * time.Millisecond})
	fakeNow := time.Now()
	cb.now = func() time.Time { return fakeNow }

	// Drive 3 failures to trip the breaker.
	for i := 0; i < 3; i++ {
		err := cb.Execute(func() error {
			_, qerr := db.Query(context.Background())
			return qerr
		})
		if !errors.Is(err, errBoom) {
			t.Fatalf("call %d: expected errBoom passed through while closed, got %v", i, err)
		}
	}
	if cb.State() != StateOpen {
		t.Fatalf("expected breaker Open after 3 failures, got %v", cb.State())
	}

	callsBeforeCooldown := db.callCount()

	// Further calls while open must be rejected without reaching the DB.
	for i := 0; i < 5; i++ {
		err := cb.Execute(func() error {
			_, qerr := db.Query(context.Background())
			return qerr
		})
		if !errors.Is(err, ErrCircuitOpen) {
			t.Fatalf("call %d: expected ErrCircuitOpen while breaker is open, got %v", i, err)
		}
	}
	if db.callCount() != callsBeforeCooldown {
		t.Fatalf("expected the fake DB to receive zero additional calls while the breaker is open, got %d more", db.callCount()-callsBeforeCooldown)
	}

	// After cooldown, exactly one probe reaches the DB (and fails, since
	// it's permanently down, reopening the breaker).
	fakeNow = fakeNow.Add(30 * time.Millisecond)
	err := cb.Execute(func() error {
		_, qerr := db.Query(context.Background())
		return qerr
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("expected the probe's real failure surfaced, got %v", err)
	}
	if cb.State() != StateOpen {
		t.Fatalf("expected breaker reopened after the failed probe, got %v", cb.State())
	}
	if db.callCount() != callsBeforeCooldown+1 {
		t.Fatalf("expected exactly one additional call for the half-open probe, got %d", db.callCount()-callsBeforeCooldown)
	}
}

// TestResilience_DegraderFallsBackWhenPrimaryDependencyDown combines
// Degrader with a fake dependency and a fake in-memory cache fallback,
// the "serve cached/stale result" scenario the design brief calls out.
func TestResilience_DegraderFallsBackWhenPrimaryDependencyDown(t *testing.T) {
	db := &fakeDatabase{permanentlyDown: true}
	staleCache := map[string]string{"case-123": "cached-case-summary"}

	degrader := NewDegrader(ModeServeStale,
		func(ctx context.Context) (string, error) {
			return db.Query(ctx)
		},
		func(_ context.Context, _ error) (string, error) {
			v, ok := staleCache["case-123"]
			if !ok {
				return "", errors.New("no cached value available")
			}
			return v, nil
		},
	)

	result, err := degrader.Run(context.Background())
	if err != nil {
		t.Fatalf("expected the fallback to succeed, got %v", err)
	}
	if !result.Degraded {
		t.Fatal("expected the result marked degraded")
	}
	if result.Mode != ModeServeStale {
		t.Fatalf("expected Mode=%q, got %q", ModeServeStale, result.Mode)
	}
	if result.Value != "cached-case-summary" {
		t.Fatalf("expected the stale cached value, got %q", result.Value)
	}
}

// TestResilience_RetryPlusCircuitBreakerPlusDegrader_FullChain composes
// all three: Retry attempts the primary a few times, a CircuitBreaker
// gates whether the primary is even attempted, and once both give up,
// Degrader supplies the reduced-service fallback -- the realistic
// layered defense this package's primitives are meant to compose into.
func TestResilience_RetryPlusCircuitBreakerPlusDegrader_FullChain(t *testing.T) {
	db := &fakeDatabase{permanentlyDown: true}
	cb := NewCircuitBreaker("fake-db", CircuitBreakerConfig{FailureThreshold: 10}) // high threshold: won't trip mid-test

	primary := func(ctx context.Context) (string, error) {
		var result string
		err := Retry(ctx, RetryPolicy{MaxAttempts: 2, BaseDelay: time.Millisecond}, func(c context.Context) error {
			return cb.Execute(func() error {
				v, qerr := db.Query(c)
				if qerr != nil {
					return qerr
				}
				result = v
				return nil
			})
		})
		return result, err
	}

	degrader := NewDegrader(ModeStaticDefault,
		primary,
		func(_ context.Context, _ error) (string, error) {
			return "default-safe-value", nil
		},
	)

	result, err := degrader.Run(context.Background())
	if err != nil {
		t.Fatalf("expected the static-default fallback to succeed, got %v", err)
	}
	if !result.Degraded || result.Value != "default-safe-value" {
		t.Fatalf("expected a degraded static-default result, got %+v", result)
	}
	// Retry made 2 attempts against the permanently-down DB.
	if db.callCount() != 2 {
		t.Fatalf("expected exactly 2 attempts via Retry before Degrader's fallback kicked in, got %d", db.callCount())
	}
}

// TestResilience_IdempotentRetrySafeUnderConcurrentDuplicateRequests
// combines IdempotencyGuard with Retry to model a duplicate-request
// scenario (e.g. a client retrying a POST after a timeout): the
// underlying side-effecting operation must run at most once even
// though Retry-wrapped calls race in from two different logical
// callers using the same idempotency key.
func TestResilience_IdempotentRetrySafeUnderConcurrentDuplicateRequests(t *testing.T) {
	guard := NewIdempotencyGuard[string](time.Minute)
	sideEffects := 0
	var mu sync.Mutex

	performSideEffectingCall := func(ctx context.Context, key string) (string, error) {
		return guard.Execute(ctx, key, func(c context.Context) (string, error) {
			var out string
			err := Retry(c, RetryPolicy{MaxAttempts: 3, BaseDelay: time.Millisecond}, func(_ context.Context) error {
				mu.Lock()
				sideEffects++
				mu.Unlock()
				out = "charge-succeeded"
				return nil
			})
			return out, err
		})
	}

	var wg sync.WaitGroup
	results := make([]string, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			v, err := performSideEffectingCall(context.Background(), "payment-abc-123")
			if err != nil {
				t.Errorf("goroutine %d: unexpected error %v", idx, err)
				return
			}
			results[idx] = v
		}(i)
	}
	wg.Wait()

	if sideEffects != 1 {
		t.Fatalf("expected the underlying side-effecting call to run exactly once across 5 duplicate requests, got %d", sideEffects)
	}
	for i, r := range results {
		if r != "charge-succeeded" {
			t.Errorf("goroutine %d: expected the shared cached result, got %q", i, r)
		}
	}
}

// TestResilience_TrafficShiftsAwayFromFailingBackendUnderSLOPressure
// models a two-backend deployment where one backend degrades, combining
// TrafficShifter (to stop routing to it) with SLO/ErrorBudget tracking
// (to quantify how bad things got) -- the two task-6/7/8 primitives
// working together against a shared simulated traffic scenario.
func TestResilience_TrafficShiftsAwayFromFailingBackendUnderSLOPressure(t *testing.T) {
	ts := NewTrafficShifter("primary", "replica")
	ts.SetStatus("primary", HealthHealthy)
	ts.SetStatus("replica", HealthHealthy)

	slo := SLO{Name: "read-availability", Kind: SLOKindSuccessRate, Target: 0.99, Window: time.Hour}
	now := time.Now()
	var observations []Observation

	// Simulate 100 requests while both backends are healthy: all succeed.
	for i := 0; i < 100; i++ {
		backend, err := ts.Select()
		if err != nil {
			t.Fatalf("request %d: unexpected error %v", i, err)
		}
		observations = append(observations, Observation{Success: true, At: now})
		_ = backend
	}

	// "primary" starts failing; traffic shifts to the remaining healthy
	// backend, but a burst of requests before detection still failed.
	for i := 0; i < 20; i++ {
		observations = append(observations, Observation{Success: false, At: now})
	}
	ts.SetStatus("primary", HealthUnhealthy)

	// After detection, the next 50 requests all land on the healthy
	// replica and succeed.
	for i := 0; i < 50; i++ {
		backend, err := ts.Select()
		if err != nil {
			t.Fatalf("post-detection request %d: unexpected error %v", i, err)
		}
		if backend != "replica" {
			t.Fatalf("expected all post-detection traffic on 'replica', got %q", backend)
		}
		observations = append(observations, Observation{Success: true, At: now})
	}

	status, err := EvaluateSLO(slo, observations, now)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	// 170 total, 20 failed => ~88.2% success, well under the 99% target.
	if status.Met {
		t.Fatalf("expected the SLO violated by the 20-request failure burst, got Observed=%v", status.Observed)
	}

	budget, err := ComputeErrorBudget(status)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	policyResult, err := (ErrorBudgetPolicy{}).Evaluate(budget)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if !policyResult.Exhausted {
		t.Fatal("expected the error budget exhausted after the failure burst")
	}
	if !policyResult.BlockRiskyDeploys {
		t.Fatal("expected BlockRiskyDeploys=true after the failure burst")
	}

	// But traffic shifting itself already recovered: no more traffic is
	// routed to the unhealthy backend going forward.
	if ts.HealthyCount() != 1 {
		t.Fatalf("expected exactly 1 healthy backend remaining, got %d", ts.HealthyCount())
	}
}

// TestResilience_FailureInjectorDrivesCircuitBreakerThroughAllStates
// uses FailureInjector as the fault source (per its own documented
// purpose: "for use in other packages' tests") to drive a
// CircuitBreaker deterministically through Closed -> Open -> HalfOpen
// -> Closed, rather than hand-scripting failures inline.
func TestResilience_FailureInjectorDrivesCircuitBreakerThroughAllStates(t *testing.T) {
	// Pattern: 2 failures (trips a threshold-2 breaker), then recovers.
	fi := NewFailureInjector(FailureInjectorConfig{
		Pattern: []FailureMode{FailureModeError, FailureModeError, FailureModeNone, FailureModeNone},
	})
	cb := NewCircuitBreaker("injected-dep", CircuitBreakerConfig{FailureThreshold: 2, Cooldown: 15 * time.Millisecond})
	fakeNow := time.Now()
	cb.now = func() time.Time { return fakeNow }

	call := func() error {
		return cb.Execute(func() error {
			return fi.Execute(context.Background(), func(_ context.Context) error { return nil })
		})
	}

	if err := call(); err == nil {
		t.Fatal("expected the first injected failure to surface")
	}
	if cb.State() != StateClosed {
		t.Fatalf("expected still Closed after 1 of 2 failures, got %v", cb.State())
	}

	if err := call(); err == nil {
		t.Fatal("expected the second injected failure to surface")
	}
	if cb.State() != StateOpen {
		t.Fatalf("expected Open after the 2nd consecutive failure, got %v", cb.State())
	}

	// Calls while open are rejected without consuming a FailureInjector
	// slot's outcome being observed by the breaker (Allow() denies before
	// fi.Execute is ever invoked).
	if err := call(); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen while breaker is open, got %v", err)
	}

	fakeNow = fakeNow.Add(20 * time.Millisecond)

	// Half-open probe consumes the injector's 3rd pattern entry (None):
	// succeeds, closing the breaker.
	if err := call(); err != nil {
		t.Fatalf("expected the half-open probe to succeed (3rd pattern entry is None), got %v", err)
	}
	if cb.State() != StateClosed {
		t.Fatalf("expected Closed after the successful probe, got %v", cb.State())
	}
}
