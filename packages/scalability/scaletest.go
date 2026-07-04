package scalability

import (
	"context"
	"sync"
	"time"
)

// ScaleOperation is a caller-supplied operation ScaleTest drives
// repeatedly at increasing concurrency levels. It reports the
// latency of a single call attempt and any error encountered,
// mirroring packages/perf.WorkFunc's contract exactly (ScaleTest does
// not time the call itself; it trusts the reported latency) so an
// operation written for perf.LoadTest can be reused verbatim against
// ScaleTest and vice versa.
type ScaleOperation func(ctx context.Context) (latency time.Duration, err error)

// ScaleTestConfig configures a ScaleTest run (task 8).
type ScaleTestConfig struct {
	// ConcurrencyLevels lists the concurrency values to ramp through,
	// in the order given (typically ascending, e.g. [1, 5, 10, 25,
	// 50]). Each level runs as an independent stage; ScaleTest does
	// not require levels to be sorted, but callers wanting a
	// monotonic ramp should list them ascending. Must be non-empty,
	// and every value must be > 0.
	ConcurrencyLevels []int

	// DurationPerLevel bounds how long each concurrency level's stage
	// runs by wall-clock time. Must be > 0.
	DurationPerLevel time.Duration
}

// Validate reports whether cfg is structurally runnable.
func (cfg ScaleTestConfig) Validate() error {
	if len(cfg.ConcurrencyLevels) == 0 {
		return wrapf("Validate", ErrInvalidScaleTestConfig)
	}
	for _, level := range cfg.ConcurrencyLevels {
		if level <= 0 {
			return wrapf("Validate", ErrInvalidScaleTestConfig)
		}
	}
	if cfg.DurationPerLevel <= 0 {
		return wrapf("Validate", ErrInvalidScaleTestConfig)
	}
	return nil
}

// StageResult is one concurrency level's aggregated outcome within a
// ScaleTest run.
type StageResult struct {
	// Concurrency is the number of concurrent goroutines that ran
	// ScaleOperation during this stage.
	Concurrency int

	// TotalCalls is the total number of ScaleOperation invocations
	// attempted during this stage, across all goroutines.
	TotalCalls int

	// ErrorCount is how many of those invocations returned a non-nil
	// error.
	ErrorCount int

	// ErrorRate is ErrorCount / TotalCalls, or 0 if TotalCalls is 0.
	ErrorRate float64

	// ThroughputPerSecond is TotalCalls divided by this stage's actual
	// elapsed wall-clock duration -- the realized throughput at this
	// concurrency level.
	ThroughputPerSecond float64

	// MeanLatency is the arithmetic mean of every reported latency
	// (successful and failed calls alike) during this stage.
	MeanLatency time.Duration
}

// ScaleTestResult aggregates every stage of a ScaleTest run, in the
// same order as ScaleTestConfig.ConcurrencyLevels, so a caller can
// plot or compare how throughput and error rate change as concurrency
// increases.
type ScaleTestResult struct {
	Stages []StageResult
}

// ScaleTest ramps simulated concurrent load against op at each
// configured concurrency level in turn, recording how throughput and
// error rate change as concurrency increases (task 8).
//
// # Relationship to packages/perf.LoadTest
//
// packages/perf.LoadTest (Phase 091) already drives a WorkFunc at one
// fixed concurrency for a duration-or-iteration bound and aggregates
// latency percentiles plus error rate. ScaleTest is a complementary,
// not competing, tool: it runs a *sequence* of LoadTest-shaped stages
// at increasing concurrency (a "ramp"), specifically to answer "how
// does throughput/error-rate change as concurrency increases" -- the
// scale-testing question this phase's brief asks for -- rather than
// LoadTest's single-concurrency-level question. Importing
// packages/perf directly was considered and rejected: perf's go.mod
// pulls in packages/graph, packages/hybridretrieval,
// packages/ingestion, packages/vectorindex, and their own transitive
// dependencies (a Neo4j driver, pgvector, testcontainers) because
// those are the operations perf benchmarks -- none of which
// packages/scalability needs or wants as transitive dependencies for
// a generic concurrency-ramp harness. ScaleTest's ScaleOperation type
// is deliberately signature-identical to perf.WorkFunc, so a caller
// with an existing perf.WorkFunc can pass the exact same function to
// both without adapting it. See doc/scalability.md for the full
// composition write-up.
//
// Returns ErrInvalidScaleTestConfig if cfg fails validation, or
// ErrNilOperation if op is nil.
func ScaleTest(ctx context.Context, cfg ScaleTestConfig, op ScaleOperation) (ScaleTestResult, error) {
	if err := cfg.Validate(); err != nil {
		return ScaleTestResult{}, wrapf("ScaleTest", err)
	}
	if op == nil {
		return ScaleTestResult{}, wrapf("ScaleTest", ErrNilOperation)
	}

	result := ScaleTestResult{
		Stages: make([]StageResult, 0, len(cfg.ConcurrencyLevels)),
	}

	for _, concurrency := range cfg.ConcurrencyLevels {
		stage, err := runStage(ctx, concurrency, cfg.DurationPerLevel, op)
		if err != nil {
			return ScaleTestResult{}, wrapf("ScaleTest", err)
		}
		result.Stages = append(result.Stages, stage)

		if ctx.Err() != nil {
			// Caller-cancelled context: stop ramping further levels
			// rather than running additional stages against a
			// context that is already done.
			break
		}
	}

	return result, nil
}

// runStage drives op with the given concurrency for duration and
// aggregates the resulting StageResult.
func runStage(ctx context.Context, concurrency int, duration time.Duration, op ScaleOperation) (StageResult, error) {
	stageCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	var (
		mu         sync.Mutex
		totalCalls int
		errorCount int
		latencySum time.Duration
	)

	start := time.Now()

	var wg sync.WaitGroup
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				if stageCtx.Err() != nil {
					return
				}

				latency, err := op(stageCtx)

				mu.Lock()
				totalCalls++
				latencySum += latency
				if err != nil {
					errorCount++
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	elapsed := time.Since(start)

	stage := StageResult{
		Concurrency: concurrency,
		TotalCalls:  totalCalls,
		ErrorCount:  errorCount,
	}
	if totalCalls > 0 {
		stage.ErrorRate = float64(errorCount) / float64(totalCalls)
		stage.MeanLatency = latencySum / time.Duration(totalCalls)
	}
	if elapsed > 0 {
		stage.ThroughputPerSecond = float64(totalCalls) / elapsed.Seconds()
	}

	return stage, nil
}
