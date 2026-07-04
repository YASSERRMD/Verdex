package perf

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestPercentile_KnownDistribution_100Values asserts the nearest-rank
// percentile convention documented on percentile against a fixed,
// hand-computed 100-element distribution: sorted[i] = i milliseconds for i
// in [0, 99]. rank = ceil(p/100*100), idx = rank-1:
//
//	p50: rank=50, idx=49  -> sorted[49]  == 49ms
//	p95: rank=95, idx=94  -> sorted[94]  == 94ms
//	p99: rank=99, idx=98  -> sorted[98]  == 98ms
func TestPercentile_KnownDistribution_100Values(t *testing.T) {
	sorted := make([]time.Duration, 100)
	for i := range sorted {
		sorted[i] = time.Duration(i) * time.Millisecond
	}

	cases := []struct {
		p    float64
		want time.Duration
	}{
		{50, 49 * time.Millisecond},
		{95, 94 * time.Millisecond},
		{99, 98 * time.Millisecond},
	}
	for _, c := range cases {
		got := percentile(sorted, c.p)
		if got != c.want {
			t.Errorf("percentile(100-value dist, p%.0f) = %v, want exactly %v", c.p, got, c.want)
		}
	}
}

// TestPercentile_KnownDistribution_1000Values is the same check scaled to
// 1000 elements (sorted[i] = i ms for i in [0, 999]):
//
//	p50: rank=500, idx=499 -> sorted[499] == 499ms
//	p95: rank=950, idx=949 -> sorted[949] == 949ms
//	p99: rank=990, idx=989 -> sorted[989] == 989ms
func TestPercentile_KnownDistribution_1000Values(t *testing.T) {
	sorted := make([]time.Duration, 1000)
	for i := range sorted {
		sorted[i] = time.Duration(i) * time.Millisecond
	}

	cases := []struct {
		p    float64
		want time.Duration
	}{
		{50, 499 * time.Millisecond},
		{95, 949 * time.Millisecond},
		{99, 989 * time.Millisecond},
	}
	for _, c := range cases {
		got := percentile(sorted, c.p)
		if got != c.want {
			t.Errorf("percentile(1000-value dist, p%.0f) = %v, want exactly %v", c.p, got, c.want)
		}
	}
}

// TestPercentile_SmallDistribution covers a hand-verifiable n=4 case:
// values 10,20,30,40 ms. p50: rank=ceil(2)=2, idx=1 -> 20ms. p95:
// rank=ceil(3.8)=4, idx=3 -> 40ms.
func TestPercentile_SmallDistribution(t *testing.T) {
	sorted := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
	}
	if got := percentile(sorted, 50); got != 20*time.Millisecond {
		t.Errorf("p50 = %v, want 20ms", got)
	}
	if got := percentile(sorted, 95); got != 40*time.Millisecond {
		t.Errorf("p95 = %v, want 40ms", got)
	}
}

func TestPercentile_SingleValue(t *testing.T) {
	sorted := []time.Duration{7 * time.Millisecond}
	for _, p := range []float64{1, 50, 95, 99, 100} {
		if got := percentile(sorted, p); got != 7*time.Millisecond {
			t.Errorf("percentile with single value at p%.0f = %v, want 7ms", p, got)
		}
	}
}

func TestLoadTest_AggregatesKnownLatencies(t *testing.T) {
	// fn deterministically returns latencies 0..99 ms in round-robin
	// order across a single goroutine (Concurrency: 1) so the resulting
	// distribution is exactly the 100-value fixture checked above.
	var next int
	fn := func(_ context.Context) (time.Duration, error) {
		lat := time.Duration(next) * time.Millisecond
		next++
		return lat, nil
	}

	result, err := LoadTest(context.Background(), LoadTestConfig{Concurrency: 1, Iterations: 100}, fn)
	if err != nil {
		t.Fatalf("LoadTest returned unexpected error: %v", err)
	}
	if result.TotalCalls != 100 {
		t.Fatalf("expected 100 total calls, got %d", result.TotalCalls)
	}
	if result.ErrorCount != 0 {
		t.Fatalf("expected 0 errors, got %d", result.ErrorCount)
	}
	if result.P50 != 49*time.Millisecond {
		t.Errorf("P50 = %v, want 49ms", result.P50)
	}
	if result.P95 != 94*time.Millisecond {
		t.Errorf("P95 = %v, want 94ms", result.P95)
	}
	if result.P99 != 98*time.Millisecond {
		t.Errorf("P99 = %v, want 98ms", result.P99)
	}
}

func TestLoadTest_ErrorRate(t *testing.T) {
	var calls int
	fn := func(_ context.Context) (time.Duration, error) {
		calls++
		if calls%2 == 0 {
			return time.Millisecond, errors.New("synthetic failure")
		}
		return time.Millisecond, nil
	}

	result, err := LoadTest(context.Background(), LoadTestConfig{Concurrency: 1, Iterations: 10}, fn)
	if err != nil {
		t.Fatalf("LoadTest returned unexpected error: %v", err)
	}
	if result.ErrorCount != 5 {
		t.Fatalf("expected 5 errors out of 10, got %d", result.ErrorCount)
	}
	if result.ErrorRate != 0.5 {
		t.Fatalf("expected ErrorRate 0.5, got %f", result.ErrorRate)
	}
}

func TestLoadTest_ConcurrencyRespectsIterationCap(t *testing.T) {
	fn := func(_ context.Context) (time.Duration, error) {
		return time.Microsecond, nil
	}

	result, err := LoadTest(context.Background(), LoadTestConfig{Concurrency: 8, Iterations: 200}, fn)
	if err != nil {
		t.Fatalf("LoadTest returned unexpected error: %v", err)
	}
	if result.TotalCalls != 200 {
		t.Fatalf("expected exactly 200 total calls across all goroutines, got %d", result.TotalCalls)
	}
}

func TestLoadTest_DurationBounded(t *testing.T) {
	fn := func(ctx context.Context) (time.Duration, error) {
		return time.Microsecond, nil
	}

	start := time.Now()
	result, err := LoadTest(context.Background(), LoadTestConfig{Concurrency: 4, Duration: 50 * time.Millisecond}, fn)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("LoadTest returned unexpected error: %v", err)
	}
	if result.TotalCalls == 0 {
		t.Fatal("expected at least one call within the duration window")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("expected LoadTest to respect its Duration bound, took %v", elapsed)
	}
}

func TestLoadTest_InvalidConfig(t *testing.T) {
	_, err := LoadTest(context.Background(), LoadTestConfig{Concurrency: 0, Iterations: 1}, func(context.Context) (time.Duration, error) {
		return 0, nil
	})
	if !errors.Is(err, ErrInvalidLoadTestConfig) {
		t.Fatalf("expected ErrInvalidLoadTestConfig for zero concurrency, got %v", err)
	}

	_, err = LoadTest(context.Background(), LoadTestConfig{Concurrency: 1}, func(context.Context) (time.Duration, error) {
		return 0, nil
	})
	if !errors.Is(err, ErrInvalidLoadTestConfig) {
		t.Fatalf("expected ErrInvalidLoadTestConfig when neither Duration nor Iterations is set, got %v", err)
	}
}
