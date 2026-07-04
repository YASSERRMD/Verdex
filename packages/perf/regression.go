package perf

import "time"

// BenchmarkRun is one historical benchmark execution's recorded outcome:
// measured latency percentiles and throughput for a named operation, plus
// enough metadata to distinguish runs across deployments and time.
// Mirrors packages/reasoningeval.QualityScore's role as the record type a
// Store persists and a regression detector compares across runs.
type BenchmarkRun struct {
	// RunID uniquely identifies this benchmark execution.
	RunID string

	// Operation names which benchmarked operation this run measured.
	Operation OperationName

	// Measurement is the observed latency/throughput for this run.
	Measurement Measurement

	// TenantID optionally scopes this run to a tenant, for deployments
	// that track per-tenant performance separately. Empty means
	// platform-wide.
	TenantID string

	// DeploymentTag identifies the deployment/environment this run was
	// captured in (e.g. "staging", "on-prem-uae-1").
	DeploymentTag string

	// RecordedAt is when this run was recorded.
	RecordedAt time.Time
}

// validate reports whether r has every field a well-formed BenchmarkRun
// requires.
func (r BenchmarkRun) validate() error {
	if r.RunID == "" || r.Operation == "" {
		return ErrInvalidBenchmarkRun
	}
	if err := r.Measurement.validate(); err != nil {
		return ErrInvalidBenchmarkRun
	}
	return nil
}

// RegressionThreshold is the maximum allowable fractional latency increase
// (or throughput decrease) between a historical baseline and a current
// run's P95 latency / throughput before DetectRegression flags a
// regression. A value of 0.20 permits up to a 20% degradation.
const RegressionThreshold = 0.20

// RegressionResult reports the outcome of comparing a current BenchmarkRun
// against a historical baseline for the same Operation, mirroring
// packages/reasoningeval.RegressionResult's shape (baseline/current
// averages, a Drop, a Regressed flag) applied to latency/throughput
// dimensions instead of quality-score dimensions.
type RegressionResult struct {
	// Operation is the operation compared.
	Operation OperationName

	// BaselineP95 and CurrentP95 are the compared P95 latencies.
	BaselineP95 time.Duration
	CurrentP95  time.Duration

	// P95IncreaseRatio is (CurrentP95-BaselineP95)/BaselineP95. Positive
	// means latency got worse.
	P95IncreaseRatio float64

	// BaselineThroughput and CurrentThroughput are the compared
	// throughputs.
	BaselineThroughput float64
	CurrentThroughput  float64

	// ThroughputDecreaseRatio is (BaselineThroughput-CurrentThroughput)/
	// BaselineThroughput. Positive means throughput got worse.
	ThroughputDecreaseRatio float64

	// Regressed is true when either ratio exceeds RegressionThreshold.
	Regressed bool
}

// CompareRuns computes a RegressionResult for current against the average
// of historical (which should be prior runs of the same Operation; runs
// with a different Operation are ignored). Returns a zero RegressionResult
// with Regressed false if historical is empty (nothing to regress
// against).
func CompareRuns(current BenchmarkRun, historical []BenchmarkRun) RegressionResult {
	var baselineP95Sum time.Duration
	var baselineThroughputSum float64
	var n int
	for _, h := range historical {
		if h.Operation != current.Operation {
			continue
		}
		baselineP95Sum += h.Measurement.P95
		baselineThroughputSum += h.Measurement.Throughput
		n++
	}

	if n == 0 {
		return RegressionResult{
			Operation:         current.Operation,
			CurrentP95:        current.Measurement.P95,
			CurrentThroughput: current.Measurement.Throughput,
		}
	}

	baselineP95 := baselineP95Sum / time.Duration(n)
	baselineThroughput := baselineThroughputSum / float64(n)

	p95Ratio := ratioIncrease(float64(baselineP95), float64(current.Measurement.P95))
	throughputRatio := ratioIncrease(current.Measurement.Throughput, baselineThroughput)

	return RegressionResult{
		Operation:               current.Operation,
		BaselineP95:             baselineP95,
		CurrentP95:              current.Measurement.P95,
		P95IncreaseRatio:        p95Ratio,
		BaselineThroughput:      baselineThroughput,
		CurrentThroughput:       current.Measurement.Throughput,
		ThroughputDecreaseRatio: throughputRatio,
		Regressed:               p95Ratio > RegressionThreshold || throughputRatio > RegressionThreshold,
	}
}

// ratioIncrease returns (to-from)/from, or 0 if from is 0 (avoids a
// division-by-zero when no historical baseline recorded any signal for a
// dimension).
func ratioIncrease(from, to float64) float64 {
	if from == 0 {
		return 0
	}
	return (to - from) / from
}

// DetectRegression reports whether current represents a performance
// regression relative to historical, per the brief's minimum bool-
// returning contract. It is a thin wrapper over CompareRuns's richer
// RegressionResult for callers that only need the yes/no answer.
func DetectRegression(current BenchmarkRun, historical []BenchmarkRun) bool {
	return CompareRuns(current, historical).Regressed
}
