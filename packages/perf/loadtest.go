package perf

import (
	"context"
	"sort"
	"sync"
	"time"
)

// WorkFunc is a caller-supplied operation LoadTest drives repeatedly. It
// reports the latency of a single call attempt and any error encountered;
// LoadTest itself never times the call (it trusts the reported latency),
// since some callers may want to measure a sub-span of their own work
// rather than wall-clock-from-LoadTest's-perspective.
type WorkFunc func(ctx context.Context) (latency time.Duration, err error)

// LoadTestConfig configures a LoadTest run.
type LoadTestConfig struct {
	// Concurrency is the number of goroutines concurrently calling
	// WorkFunc. Must be > 0.
	Concurrency int

	// Duration bounds the run by wall-clock time. If zero, Iterations must
	// be set instead.
	Duration time.Duration

	// Iterations bounds the run by total call count across every
	// goroutine combined. If zero, Duration must be set instead. If both
	// are set, whichever limit is reached first stops the run.
	Iterations int
}

// validate reports whether cfg is structurally runnable.
func (cfg LoadTestConfig) validate() error {
	if cfg.Concurrency <= 0 {
		return ErrInvalidLoadTestConfig
	}
	if cfg.Duration <= 0 && cfg.Iterations <= 0 {
		return ErrInvalidLoadTestConfig
	}
	return nil
}

// LoadTestResult aggregates the outcome of a LoadTest run.
type LoadTestResult struct {
	// TotalCalls is the total number of WorkFunc invocations attempted.
	TotalCalls int

	// ErrorCount is how many of those invocations returned a non-nil
	// error.
	ErrorCount int

	// ErrorRate is ErrorCount / TotalCalls, or 0 if TotalCalls is 0.
	ErrorRate float64

	// P50, P95, and P99 are latency percentiles computed over every
	// successful and failed call's reported latency (see
	// percentile's doc comment for the exact indexing convention).
	P50 time.Duration
	P95 time.Duration
	P99 time.Duration
}

// LoadTest drives fn with the given concurrency for the configured
// duration and/or iteration count, then aggregates latency percentiles and
// error rate across every call.
//
// Returns ErrInvalidLoadTestConfig if cfg fails validation, or ErrNoSamples
// if the run completed with zero recorded calls (e.g. ctx was already
// cancelled).
func LoadTest(ctx context.Context, cfg LoadTestConfig, fn WorkFunc) (LoadTestResult, error) {
	if err := cfg.validate(); err != nil {
		return LoadTestResult{}, wrapf("LoadTest", err)
	}

	runCtx := ctx
	var cancel context.CancelFunc
	if cfg.Duration > 0 {
		runCtx, cancel = context.WithTimeout(ctx, cfg.Duration)
		defer cancel()
	}

	var (
		mu         sync.Mutex
		latencies  []time.Duration
		errorCount int
		remaining  = cfg.Iterations // 0 means "no iteration cap"
	)

	var wg sync.WaitGroup
	for w := 0; w < cfg.Concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				if runCtx.Err() != nil {
					return
				}
				if cfg.Iterations > 0 {
					mu.Lock()
					if remaining <= 0 {
						mu.Unlock()
						return
					}
					remaining--
					mu.Unlock()
				}

				latency, err := fn(runCtx)

				mu.Lock()
				latencies = append(latencies, latency)
				if err != nil {
					errorCount++
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if len(latencies) == 0 {
		return LoadTestResult{}, wrapf("LoadTest", ErrNoSamples)
	}

	sorted := append([]time.Duration(nil), latencies...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	return LoadTestResult{
		TotalCalls: len(sorted),
		ErrorCount: errorCount,
		ErrorRate:  float64(errorCount) / float64(len(sorted)),
		P50:        percentile(sorted, 50),
		P95:        percentile(sorted, 95),
		P99:        percentile(sorted, 99),
	}, nil
}

// percentile returns the p-th percentile (0 <= p <= 100) of sorted, which
// must already be sorted ascending and non-empty.
//
// # Indexing convention
//
// This uses the nearest-rank method: index = ceil(p/100 * n) - 1, clamped
// to [0, n-1]. For a 100-element slice, p95 selects sorted[94] (the 95th
// smallest element, 0-indexed as 94) rather than interpolating between
// sorted[94] and sorted[95]. This is deterministic, requires no
// interpolation arithmetic, and matches the convention most load-testing
// tools (e.g. wrk, autocannon) use for "p95 latency": the smallest
// observed value at or above which 95% of the distribution falls at or
// below.
func percentile(sorted []time.Duration, p float64) time.Duration {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	rank := int(mathCeil(p / 100 * float64(n)))
	idx := rank - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	return sorted[idx]
}

// mathCeil is a tiny local ceil to avoid importing "math" for a single
// call site.
func mathCeil(f float64) float64 {
	i := float64(int64(f))
	if f > i {
		return i + 1
	}
	return i
}
