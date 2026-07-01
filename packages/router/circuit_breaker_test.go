package router

import (
	"testing"
	"time"
)

// TestCircuitBreaker_OpenAfterFiveFailures verifies that the circuit breaker
// transitions from closed to open after exactly five consecutive failures.
func TestCircuitBreaker_OpenAfterFiveFailures(t *testing.T) {
	cb := newCircuitBreaker("test-provider")

	// Should start closed and allow requests.
	if got := cb.State(); got != cbClosed {
		t.Fatalf("expected initial state closed, got %v", got)
	}
	if !cb.Allow() {
		t.Fatal("expected Allow() == true in closed state")
	}

	// Trip four times — should stay closed.
	for i := 0; i < 4; i++ {
		cb.Trip()
		if cb.State() != cbClosed {
			t.Fatalf("after %d trips expected still closed", i+1)
		}
	}

	// Fifth trip must open the breaker.
	cb.Trip()
	if got := cb.State(); got != cbOpen {
		t.Fatalf("expected open after 5 failures, got %v", got)
	}

	// Allow() must return false when open (recovery timeout has not elapsed).
	if cb.Allow() {
		t.Fatal("expected Allow() == false in open state before timeout")
	}
}

// TestCircuitBreaker_RecoverAfterTimeout verifies the breaker transitions to
// half-open once the recovery timeout has elapsed.
func TestCircuitBreaker_RecoverAfterTimeout(t *testing.T) {
	cb := newCircuitBreaker("test-provider")
	cb.failureThreshold = 1
	cb.recoveryTimeout = 50 * time.Millisecond

	// Open the breaker.
	cb.Trip()
	if cb.State() != cbOpen {
		t.Fatal("expected open state after single trip")
	}

	// Allow() immediately should return false.
	if cb.Allow() {
		t.Fatal("expected Allow() == false right after opening")
	}

	// Wait for recovery timeout.
	time.Sleep(100 * time.Millisecond)

	// Allow() should now return true and transition to half-open.
	if !cb.Allow() {
		t.Fatal("expected Allow() == true after recovery timeout")
	}
	if cb.State() != cbHalfOpen {
		t.Fatalf("expected half-open after timeout, got %v", cb.State())
	}
}

// TestCircuitBreaker_HalfOpenAllowsOneProbe verifies that in the half-open
// state exactly one probe is allowed.
func TestCircuitBreaker_HalfOpenAllowsOneProbe(t *testing.T) {
	cb := newCircuitBreaker("test-provider")
	cb.failureThreshold = 1
	cb.recoveryTimeout = 50 * time.Millisecond

	// Open, wait, transition to half-open.
	cb.Trip()
	time.Sleep(100 * time.Millisecond)
	if !cb.Allow() {
		t.Fatal("expected first Allow() to succeed in half-open")
	}
	if cb.State() != cbHalfOpen {
		t.Fatalf("expected half-open state, got %v", cb.State())
	}

	// A second Allow() must be denied (still half-open, probe outstanding).
	if cb.Allow() {
		t.Fatal("expected second Allow() to be denied in half-open state")
	}

	// Successful probe: Reset() should close the breaker.
	cb.Reset()
	if cb.State() != cbClosed {
		t.Fatalf("expected closed after Reset(), got %v", cb.State())
	}
	if !cb.Allow() {
		t.Fatal("expected Allow() == true after closing")
	}
}

// TestCircuitBreaker_FailedProbeReopens verifies that a failed probe in the
// half-open state reopens the breaker.
func TestCircuitBreaker_FailedProbeReopens(t *testing.T) {
	cb := newCircuitBreaker("test-provider")
	cb.failureThreshold = 1
	cb.recoveryTimeout = 50 * time.Millisecond

	cb.Trip()
	time.Sleep(100 * time.Millisecond)
	cb.Allow() // transitions to half-open

	// Failed probe.
	cb.Trip()
	if cb.State() != cbOpen {
		t.Fatalf("expected open after failed probe, got %v", cb.State())
	}
}

// TestCircuitBreakerRegistry_LazyCreation checks that Get always returns the
// same instance for the same provider ID.
func TestCircuitBreakerRegistry_LazyCreation(t *testing.T) {
	reg := NewCircuitBreakerRegistry()

	cb1 := reg.Get("p1")
	cb2 := reg.Get("p1")
	if cb1 != cb2 {
		t.Fatal("expected the same CircuitBreaker instance for the same provider ID")
	}

	cb3 := reg.Get("p2")
	if cb1 == cb3 {
		t.Fatal("expected different CircuitBreaker instances for different provider IDs")
	}
}
