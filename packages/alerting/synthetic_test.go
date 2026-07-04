package alerting_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/alerting"
)

// fakeClock returns a func() time.Time that advances by step on each
// call, starting at start -- letting a test control exactly how much
// "time" a SyntheticCheck.Run observes without a real sleep.
func fakeClock(start time.Time, step time.Duration) func() time.Time {
	current := start
	first := true
	return func() time.Time {
		if first {
			first = false
			return current
		}
		current = current.Add(step)
		return current
	}
}

func TestSyntheticCheck_Run_Pass(t *testing.T) {
	t.Parallel()
	check := alerting.SyntheticCheck{
		Name: "health-endpoint",
		Prober: func(_ context.Context) error {
			return nil
		},
	}

	result, err := check.Run(t.Context(), fakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), 20*time.Millisecond))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !result.Passed {
		t.Error("result.Passed = false, want true")
	}
	if result.Error != "" {
		t.Errorf("result.Error = %q, want empty", result.Error)
	}
	if result.Latency != 20*time.Millisecond {
		t.Errorf("result.Latency = %v, want 20ms", result.Latency)
	}
	if result.CheckName != "health-endpoint" {
		t.Errorf("result.CheckName = %q, want health-endpoint", result.CheckName)
	}
}

func TestSyntheticCheck_Run_Fail(t *testing.T) {
	t.Parallel()
	probeErr := errors.New("connection refused")
	check := alerting.SyntheticCheck{
		Name: "ingestion-canary",
		Prober: func(_ context.Context) error {
			return probeErr
		},
	}

	result, err := check.Run(t.Context(), fakeClock(time.Now(), 5*time.Millisecond))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Passed {
		t.Error("result.Passed = true, want false")
	}
	if result.Error != probeErr.Error() {
		t.Errorf("result.Error = %q, want %q", result.Error, probeErr.Error())
	}
}

func TestSyntheticCheck_Run_NilProber(t *testing.T) {
	t.Parallel()
	check := alerting.SyntheticCheck{Name: "no-prober"}
	_, err := check.Run(t.Context(), nil)
	if !errors.Is(err, alerting.ErrNilProber) {
		t.Fatalf("Run with nil Prober error = %v, want ErrNilProber", err)
	}
}

func TestSyntheticCheck_Run_InvalidName(t *testing.T) {
	t.Parallel()
	check := alerting.SyntheticCheck{
		Prober: func(_ context.Context) error { return nil },
	}
	_, err := check.Run(t.Context(), nil)
	if !errors.Is(err, alerting.ErrInvalidCheck) {
		t.Fatalf("Run with blank Name error = %v, want ErrInvalidCheck", err)
	}
}

func TestSyntheticCheck_Run_TimeoutAppliedToProbeContext(t *testing.T) {
	t.Parallel()

	var sawDeadline bool
	check := alerting.SyntheticCheck{
		Name:    "slow-probe",
		Timeout: 10 * time.Millisecond,
		Prober: func(ctx context.Context) error {
			_, sawDeadline = ctx.Deadline()
			return nil
		},
	}

	if _, err := check.Run(t.Context(), nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !sawDeadline {
		t.Error("Prober's context had no deadline, want Timeout to apply one")
	}
}

func TestSyntheticCheck_Run_DefaultsToRealClock(t *testing.T) {
	t.Parallel()
	// Passing a nil `now` func should fall back to time.Now rather
	// than panicking.
	check := alerting.SyntheticCheck{
		Name:   "real-clock",
		Prober: func(_ context.Context) error { return nil },
	}
	result, err := check.Run(t.Context(), nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.RanAt.IsZero() {
		t.Error("result.RanAt is zero, want a real timestamp")
	}
}

func TestObservabilityProberFromChecker(t *testing.T) {
	t.Parallel()
	called := false
	checker := func(_ context.Context) error {
		called = true
		return nil
	}
	prober := alerting.ObservabilityProberFromChecker(checker)
	if err := prober(t.Context()); err != nil {
		t.Fatalf("prober: %v", err)
	}
	if !called {
		t.Error("wrapped checker was not called")
	}
}
