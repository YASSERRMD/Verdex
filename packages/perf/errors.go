package perf

import (
	"errors"
	"fmt"
)

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrUnknownOperation is returned by Budget.Evaluate when asked to
	// evaluate a Measurement against an operation name with no registered
	// budget.
	ErrUnknownOperation = errors.New("perf: unknown operation")

	// ErrInvalidBudget is returned when a Budget entry fails structural
	// validation (negative latency targets, zero/negative throughput
	// floor).
	ErrInvalidBudget = errors.New("perf: invalid budget")

	// ErrInvalidMeasurement is returned when a Measurement fails structural
	// validation (negative latency or throughput values).
	ErrInvalidMeasurement = errors.New("perf: invalid measurement")

	// ErrCacheMiss is returned by Cache helpers that offer an error-
	// returning variant of Get for callers preferring that idiom over the
	// (V, bool) form.
	ErrCacheMiss = errors.New("perf: cache miss")

	// ErrBatcherClosed is returned when Add is called on a Batcher that has
	// already been stopped.
	ErrBatcherClosed = errors.New("perf: batcher is closed")

	// ErrNoSamples is returned by LoadTest result aggregation when no
	// latency samples were recorded.
	ErrNoSamples = errors.New("perf: no samples recorded")

	// ErrInvalidLoadTestConfig is returned when a LoadTestConfig fails
	// structural validation (non-positive concurrency, neither duration nor
	// iteration count set).
	ErrInvalidLoadTestConfig = errors.New("perf: invalid load test config")

	// ErrInvalidRecommendation is returned when a Recommendation fails
	// structural validation (a required field left blank).
	ErrInvalidRecommendation = errors.New("perf: invalid recommendation")

	// ErrInvalidBenchmarkRun is returned when a BenchmarkRun fails
	// structural validation.
	ErrInvalidBenchmarkRun = errors.New("perf: invalid benchmark run")

	// ErrLimiterClosed is returned when Acquire is called on a Limiter
	// whose ResourceLimits named class does not exist.
	ErrLimiterClosed = errors.New("perf: unknown resource class")
)

// wrapf mirrors the fmt.Errorf("pkg: fn: %w", err) convention used
// throughout this repository's packages.
func wrapf(fn string, err error) error {
	return fmt.Errorf("perf: %s: %w", fn, err)
}
