package reliability

import (
	"errors"
	"testing"
	"time"
)

func TestCircuitBreaker_StartsClosedAndAllows(t *testing.T) {
	cb := NewCircuitBreaker("db", CircuitBreakerConfig{})
	if got := cb.State(); got != StateClosed {
		t.Fatalf("expected initial state Closed, got %v", got)
	}
	if !cb.Allow() {
		t.Fatal("expected Allow() == true in Closed state")
	}
}

func TestCircuitBreaker_TripsOpenAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker("graph-store", CircuitBreakerConfig{FailureThreshold: 3})

	cb.RecordFailure()
	if cb.State() != StateClosed {
		t.Fatal("expected still Closed after 1 of 3 failures")
	}
	cb.RecordFailure()
	if cb.State() != StateClosed {
		t.Fatal("expected still Closed after 2 of 3 failures")
	}
	cb.RecordFailure()
	if cb.State() != StateOpen {
		t.Fatal("expected Open after 3rd consecutive failure")
	}

	if cb.Allow() {
		t.Fatal("expected Allow() == false immediately after tripping open")
	}
}

func TestCircuitBreaker_SuccessResetsFailureCounterInClosed(t *testing.T) {
	cb := NewCircuitBreaker("db", CircuitBreakerConfig{FailureThreshold: 3})

	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess() // resets counter
	cb.RecordFailure()
	cb.RecordFailure()

	if cb.State() != StateClosed {
		t.Fatalf("expected Closed (counter reset by intervening success), got %v", cb.State())
	}
}

func TestCircuitBreaker_TransitionsToHalfOpenAfterCooldown(t *testing.T) {
	cb := NewCircuitBreaker("db", CircuitBreakerConfig{FailureThreshold: 1, Cooldown: 30 * time.Millisecond})
	fakeNow := time.Now()
	cb.now = func() time.Time { return fakeNow }

	cb.RecordFailure() // trips open at fakeNow
	if cb.State() != StateOpen {
		t.Fatal("expected Open after single failure with threshold 1")
	}

	// Before cooldown elapses: still denied, still Open.
	if cb.Allow() {
		t.Fatal("expected Allow() == false before cooldown elapses")
	}

	// Advance the fake clock past the cooldown.
	fakeNow = fakeNow.Add(31 * time.Millisecond)
	if !cb.Allow() {
		t.Fatal("expected Allow() == true once cooldown has elapsed")
	}
	if cb.State() != StateHalfOpen {
		t.Fatalf("expected HalfOpen after cooldown, got %v", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenAllowsExactlyOneProbe(t *testing.T) {
	cb := NewCircuitBreaker("db", CircuitBreakerConfig{FailureThreshold: 1, Cooldown: 10 * time.Millisecond})
	fakeNow := time.Now()
	cb.now = func() time.Time { return fakeNow }

	cb.RecordFailure()
	fakeNow = fakeNow.Add(20 * time.Millisecond)

	if !cb.Allow() {
		t.Fatal("expected first Allow() to succeed transitioning into HalfOpen")
	}
	if cb.State() != StateHalfOpen {
		t.Fatalf("expected HalfOpen, got %v", cb.State())
	}

	// A second probe attempt must be denied while the first is outstanding.
	if cb.Allow() {
		t.Fatal("expected second Allow() to be denied during an outstanding half-open probe")
	}
}

func TestCircuitBreaker_SuccessfulProbeClosesBreaker(t *testing.T) {
	cb := NewCircuitBreaker("db", CircuitBreakerConfig{FailureThreshold: 1, Cooldown: 10 * time.Millisecond})
	fakeNow := time.Now()
	cb.now = func() time.Time { return fakeNow }

	cb.RecordFailure()
	fakeNow = fakeNow.Add(20 * time.Millisecond)
	cb.Allow() // -> HalfOpen, probe outstanding

	cb.RecordSuccess()
	if cb.State() != StateClosed {
		t.Fatalf("expected Closed after successful probe, got %v", cb.State())
	}
	if !cb.Allow() {
		t.Fatal("expected Allow() == true after closing")
	}
}

func TestCircuitBreaker_FailedProbeReopensBreaker(t *testing.T) {
	cb := NewCircuitBreaker("db", CircuitBreakerConfig{FailureThreshold: 1, Cooldown: 10 * time.Millisecond})
	fakeNow := time.Now()
	cb.now = func() time.Time { return fakeNow }

	cb.RecordFailure()
	fakeNow = fakeNow.Add(20 * time.Millisecond)
	cb.Allow() // -> HalfOpen

	cb.RecordFailure() // probe failed
	if cb.State() != StateOpen {
		t.Fatalf("expected Open after failed probe, got %v", cb.State())
	}

	// And it must wait out a fresh cooldown again before allowing another probe.
	if cb.Allow() {
		t.Fatal("expected Allow() == false immediately after a failed probe reopens the breaker")
	}
}

func TestCircuitBreaker_DefaultsApplyWhenConfigZeroValued(t *testing.T) {
	cb := NewCircuitBreaker("db", CircuitBreakerConfig{})
	if got := cb.cfg.failureThreshold(); got != DefaultFailureThreshold {
		t.Fatalf("expected default failure threshold %d, got %d", DefaultFailureThreshold, got)
	}
	if got := cb.cfg.cooldown(); got != DefaultCooldown {
		t.Fatalf("expected default cooldown %v, got %v", DefaultCooldown, got)
	}
}

func TestCircuitBreaker_Execute_SuccessPassesThrough(t *testing.T) {
	cb := NewCircuitBreaker("db", CircuitBreakerConfig{})
	calls := 0
	err := cb.Execute(func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected fn invoked once, got %d", calls)
	}
	if cb.State() != StateClosed {
		t.Fatal("expected still Closed after success")
	}
}

func TestCircuitBreaker_Execute_FailureRecordedAndReturned(t *testing.T) {
	cb := NewCircuitBreaker("db", CircuitBreakerConfig{FailureThreshold: 1})
	err := cb.Execute(func() error { return errBoom })
	if !errors.Is(err, errBoom) {
		t.Fatalf("expected errBoom returned, got %v", err)
	}
	if cb.State() != StateOpen {
		t.Fatal("expected Open after Execute's single failure trips the threshold")
	}
}

func TestCircuitBreaker_Execute_RejectsWhenOpen(t *testing.T) {
	cb := NewCircuitBreaker("db", CircuitBreakerConfig{FailureThreshold: 1, Cooldown: time.Hour})
	_ = cb.Execute(func() error { return errBoom }) // trips open

	calls := 0
	err := cb.Execute(func() error {
		calls++
		return nil
	})
	if !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
	if calls != 0 {
		t.Fatal("expected fn not invoked while breaker is open")
	}
}

func TestCircuitBreaker_Execute_NilFuncReturnsError(t *testing.T) {
	cb := NewCircuitBreaker("db", CircuitBreakerConfig{})
	if err := cb.Execute(nil); !errors.Is(err, ErrNilFunc) {
		t.Fatalf("expected ErrNilFunc, got %v", err)
	}
}

func TestCircuitBreakerRegistry_LazyCreationReturnsSameInstance(t *testing.T) {
	reg := NewCircuitBreakerRegistry(CircuitBreakerConfig{})

	cb1 := reg.Get("postgres")
	cb2 := reg.Get("postgres")
	if cb1 != cb2 {
		t.Fatal("expected the same CircuitBreaker instance for the same name")
	}

	cb3 := reg.Get("neo4j")
	if cb1 == cb3 {
		t.Fatal("expected different CircuitBreaker instances for different names")
	}
	if cb3.Name() != "neo4j" {
		t.Fatalf("expected breaker name %q, got %q", "neo4j", cb3.Name())
	}
}

func TestState_String(t *testing.T) {
	cases := map[State]string{
		StateClosed:   "closed",
		StateOpen:     "open",
		StateHalfOpen: "half_open",
		State(99):     "unknown",
	}
	for state, want := range cases {
		if got := state.String(); got != want {
			t.Errorf("State(%d).String() = %q, want %q", state, got, want)
		}
	}
}
