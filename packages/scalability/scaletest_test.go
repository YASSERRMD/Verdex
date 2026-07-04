package scalability

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestScaleTestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     ScaleTestConfig
		wantErr bool
	}{
		{"valid", ScaleTestConfig{ConcurrencyLevels: []int{1, 5}, DurationPerLevel: 10 * time.Millisecond}, false},
		{"empty levels", ScaleTestConfig{ConcurrencyLevels: nil, DurationPerLevel: 10 * time.Millisecond}, true},
		{"zero level", ScaleTestConfig{ConcurrencyLevels: []int{1, 0}, DurationPerLevel: 10 * time.Millisecond}, true},
		{"negative level", ScaleTestConfig{ConcurrencyLevels: []int{-1}, DurationPerLevel: 10 * time.Millisecond}, true},
		{"zero duration", ScaleTestConfig{ConcurrencyLevels: []int{1}, DurationPerLevel: 0}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestScaleTestInvalidConfig(t *testing.T) {
	_, err := ScaleTest(context.Background(), ScaleTestConfig{}, func(context.Context) (time.Duration, error) { return 0, nil })
	if !errors.Is(err, ErrInvalidScaleTestConfig) {
		t.Fatalf("expected ErrInvalidScaleTestConfig, got %v", err)
	}
}

func TestScaleTestNilOperation(t *testing.T) {
	cfg := ScaleTestConfig{ConcurrencyLevels: []int{1}, DurationPerLevel: 10 * time.Millisecond}
	_, err := ScaleTest(context.Background(), cfg, nil)
	if !errors.Is(err, ErrNilOperation) {
		t.Fatalf("expected ErrNilOperation, got %v", err)
	}
}

// TestScaleTestRampsThroughAllLevels confirms every configured
// concurrency level produces exactly one StageResult, in order, with
// the Concurrency field correctly recorded.
func TestScaleTestRampsThroughAllLevels(t *testing.T) {
	levels := []int{1, 2, 4}
	cfg := ScaleTestConfig{
		ConcurrencyLevels: levels,
		DurationPerLevel:  15 * time.Millisecond,
	}

	op := func(ctx context.Context) (time.Duration, error) {
		return time.Millisecond, nil
	}

	result, err := ScaleTest(context.Background(), cfg, op)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Stages) != len(levels) {
		t.Fatalf("expected %d stages, got %d", len(levels), len(result.Stages))
	}
	for i, level := range levels {
		if result.Stages[i].Concurrency != level {
			t.Fatalf("stage %d: expected Concurrency=%d, got %d", i, level, result.Stages[i].Concurrency)
		}
		if result.Stages[i].TotalCalls == 0 {
			t.Fatalf("stage %d: expected at least one call to have run", i)
		}
	}
}

// TestScaleTestHigherConcurrencyYieldsHigherThroughput confirms the
// harness actually measures throughput scaling with concurrency for a
// cheap, non-contended operation: more concurrent goroutines calling
// a fast no-op function should complete more total calls in the same
// wall-clock duration.
func TestScaleTestHigherConcurrencyYieldsHigherThroughput(t *testing.T) {
	cfg := ScaleTestConfig{
		ConcurrencyLevels: []int{1, 20},
		DurationPerLevel:  30 * time.Millisecond,
	}

	op := func(ctx context.Context) (time.Duration, error) {
		// A tiny amount of real work so the loop doesn't spin purely
		// on context checks.
		time.Sleep(100 * time.Microsecond)
		return 100 * time.Microsecond, nil
	}

	result, err := ScaleTest(context.Background(), cfg, op)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Stages) != 2 {
		t.Fatalf("expected 2 stages, got %d", len(result.Stages))
	}

	low, high := result.Stages[0], result.Stages[1]
	if high.TotalCalls <= low.TotalCalls {
		t.Fatalf("expected concurrency=20 stage to complete more calls than concurrency=1 stage; low=%d high=%d",
			low.TotalCalls, high.TotalCalls)
	}
	if high.ThroughputPerSecond <= low.ThroughputPerSecond {
		t.Fatalf("expected concurrency=20 stage to show higher throughput than concurrency=1 stage; low=%v high=%v",
			low.ThroughputPerSecond, high.ThroughputPerSecond)
	}
}

// TestScaleTestErrorRateRisesUnderContention demonstrates the
// "records how ... error-rate changes as concurrency increases"
// requirement using a real dependency: a ScaleOperation backed by a
// BackpressureController with a small MaxInFlight. At low
// concurrency, every call is admitted (error rate 0); at high
// concurrency far exceeding MaxInFlight, a meaningful fraction of
// calls are shed (error rate > 0). This also demonstrates
// BackpressureController and ScaleTest composing together, not just
// ScaleTest in isolation.
func TestScaleTestErrorRateRisesUnderContention(t *testing.T) {
	controller, err := NewBackpressureController(BackpressureConfig{MaxInFlight: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	op := func(ctx context.Context) (time.Duration, error) {
		if admitErr := controller.Admit(); admitErr != nil {
			return 0, admitErr
		}
		defer controller.Release()
		time.Sleep(2 * time.Millisecond)
		return 2 * time.Millisecond, nil
	}

	cfg := ScaleTestConfig{
		ConcurrencyLevels: []int{1, 50},
		DurationPerLevel:  40 * time.Millisecond,
	}

	result, err := ScaleTest(context.Background(), cfg, op)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	low, high := result.Stages[0], result.Stages[1]
	if low.ErrorRate != 0 {
		t.Fatalf("expected zero error rate at concurrency=1 (well under MaxInFlight=3), got %v", low.ErrorRate)
	}
	if high.ErrorRate <= 0 {
		t.Fatalf("expected positive error rate at concurrency=50 against MaxInFlight=3, got %v", high.ErrorRate)
	}
	if high.ErrorRate <= low.ErrorRate {
		t.Fatalf("expected error rate to rise with concurrency: low=%v high=%v", low.ErrorRate, high.ErrorRate)
	}
}

// TestScaleTestRespectsContextCancellation confirms a cancelled
// context stops the ramp early rather than continuing to run further
// concurrency-level stages.
func TestScaleTestRespectsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	callCount := 0
	op := func(ctx context.Context) (time.Duration, error) {
		callCount++
		if callCount == 1 {
			cancel()
		}
		return time.Microsecond, nil
	}

	cfg := ScaleTestConfig{
		ConcurrencyLevels: []int{1, 1, 1, 1, 1},
		DurationPerLevel:  50 * time.Millisecond,
	}

	result, err := ScaleTest(ctx, cfg, op)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The first stage still runs (it observes cancellation mid-flight
	// and stops), but subsequent stages should not run once ctx is
	// already done.
	if len(result.Stages) >= len(cfg.ConcurrencyLevels) {
		t.Fatalf("expected fewer than %d stages after cancellation, got %d", len(cfg.ConcurrencyLevels), len(result.Stages))
	}
}
