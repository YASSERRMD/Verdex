package reliability

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestFailureInjector_NoneMode_RunsFnNormally(t *testing.T) {
	fi := NewFailureInjector(FailureInjectorConfig{Pattern: []FailureMode{FailureModeNone}})
	calls := 0
	err := fi.Execute(context.Background(), func(_ context.Context) error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected fn invoked once, got %d", calls)
	}
}

func TestFailureInjector_ErrorMode_ShortCircuitsWithoutInvokingFn(t *testing.T) {
	fi := NewFailureInjector(FailureInjectorConfig{Pattern: []FailureMode{FailureModeError}})
	calls := 0
	err := fi.Execute(context.Background(), func(_ context.Context) error {
		calls++
		return nil
	})
	if !errors.Is(err, ErrInjectedFailure) {
		t.Fatalf("expected ErrInjectedFailure, got %v", err)
	}
	if calls != 0 {
		t.Fatalf("expected fn NOT invoked in error mode, got %d calls", calls)
	}
}

func TestFailureInjector_ErrorMode_CustomError(t *testing.T) {
	custom := errors.New("custom injected failure")
	fi := NewFailureInjector(FailureInjectorConfig{
		Pattern: []FailureMode{FailureModeError},
		Err:     custom,
	})
	err := fi.Execute(context.Background(), func(_ context.Context) error { return nil })
	if !errors.Is(err, custom) {
		t.Fatalf("expected custom error, got %v", err)
	}
}

func TestFailureInjector_LatencyMode_DelaysBeforeInvokingFn(t *testing.T) {
	fi := NewFailureInjector(FailureInjectorConfig{
		Pattern: []FailureMode{FailureModeLatency},
		Latency: 40 * time.Millisecond,
	})
	calls := 0
	start := time.Now()
	err := fi.Execute(context.Background(), func(_ context.Context) error {
		calls++
		return nil
	})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected fn invoked once after the delay, got %d", calls)
	}
	if elapsed < 35*time.Millisecond {
		t.Fatalf("expected at least ~40ms injected latency, elapsed only %v", elapsed)
	}
}

func TestFailureInjector_LatencyMode_RespectsContextCancellation(t *testing.T) {
	fi := NewFailureInjector(FailureInjectorConfig{
		Pattern: []FailureMode{FailureModeLatency},
		Latency: time.Hour,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	calls := 0
	err := fi.Execute(ctx, func(_ context.Context) error {
		calls++
		return nil
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
	if calls != 0 {
		t.Fatalf("expected fn not invoked once ctx expires mid-delay, got %d", calls)
	}
}

func TestFailureInjector_PanicMode_RecoversAndReturnsError(t *testing.T) {
	fi := NewFailureInjector(FailureInjectorConfig{Pattern: []FailureMode{FailureModePanic}})
	calls := 0
	err := fi.Execute(context.Background(), func(_ context.Context) error {
		calls++
		return nil
	})
	if !errors.Is(err, ErrInjectedPanic) {
		t.Fatalf("expected ErrInjectedPanic, got %v", err)
	}
	if calls != 0 {
		t.Fatalf("expected fn not invoked when the injected panic preempts it, got %d", calls)
	}
	// Crucially: the test process itself must survive this call (the
	// panic must have been recovered, not propagated).
}

// TestFailureInjector_DeterministicPatternCycles verifies the injector
// cycles through a fixed Pattern deterministically and wraps around --
// no randomness, so this assertion is exact and never flaky.
func TestFailureInjector_DeterministicPatternCycles(t *testing.T) {
	pattern := []FailureMode{FailureModeNone, FailureModeError, FailureModeNone}
	fi := NewFailureInjector(FailureInjectorConfig{Pattern: pattern})

	wantResults := []bool{true, false, true, true, false, true, true, false} // true = success
	for i, wantSuccess := range wantResults {
		err := fi.Execute(context.Background(), func(_ context.Context) error { return nil })
		gotSuccess := err == nil
		if gotSuccess != wantSuccess {
			t.Fatalf("call %d: expected success=%v, got success=%v (err=%v)", i, wantSuccess, gotSuccess, err)
		}
	}
	if fi.Calls() != int64(len(wantResults)) {
		t.Fatalf("expected Calls()=%d, got %d", len(wantResults), fi.Calls())
	}
}

func TestFailureInjector_EmptyPatternDefaultsToNone(t *testing.T) {
	fi := NewFailureInjector(FailureInjectorConfig{})
	for i := 0; i < 5; i++ {
		err := fi.Execute(context.Background(), func(_ context.Context) error { return nil })
		if err != nil {
			t.Fatalf("call %d: expected nil error with empty Pattern (defaults to None), got %v", i, err)
		}
	}
}

func TestFailureInjector_NilFuncReturnsError(t *testing.T) {
	fi := NewFailureInjector(FailureInjectorConfig{})
	err := fi.Execute(context.Background(), nil)
	if !errors.Is(err, ErrNilFunc) {
		t.Fatalf("expected ErrNilFunc, got %v", err)
	}
}

func TestFailureInjector_DefaultLatencyWhenNonPositive(t *testing.T) {
	fi := NewFailureInjector(FailureInjectorConfig{})
	if got := fi.cfg.latency(); got != DefaultInjectedLatency {
		t.Fatalf("expected default latency %v, got %v", DefaultInjectedLatency, got)
	}
}

// TestFailureInjector_ConcurrentCallsAdvanceCallCounterSafely exercises
// the atomic call-counter/pattern-index under -race with many concurrent
// callers, using FailureModeNone throughout so timing is irrelevant --
// this test only cares that Calls() ends up exactly right with no data
// race, not which mode each individual call landed on.
func TestFailureInjector_ConcurrentCallsAdvanceCallCounterSafely(t *testing.T) {
	fi := NewFailureInjector(FailureInjectorConfig{Pattern: []FailureMode{FailureModeNone}})

	const goroutines = 50
	done := make(chan struct{}, goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			_ = fi.Execute(context.Background(), func(_ context.Context) error { return nil })
			done <- struct{}{}
		}()
	}
	for i := 0; i < goroutines; i++ {
		<-done
	}

	if fi.Calls() != goroutines {
		t.Fatalf("expected Calls()=%d, got %d", goroutines, fi.Calls())
	}
}
