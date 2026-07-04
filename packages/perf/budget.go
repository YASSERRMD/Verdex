package perf

import "time"

// OperationName identifies a named, benchmarked operation this package has
// a Budget for.
type OperationName string

const (
	// OpHybridRetrieval names packages/hybridretrieval.Retriever.Retrieve's
	// budgeted operation.
	OpHybridRetrieval OperationName = "hybrid_retrieval"

	// OpGraphTraversal names packages/traversal.Walker.Execute's budgeted
	// operation.
	OpGraphTraversal OperationName = "graph_traversal"

	// OpIngestionPipeline names
	// packages/ingestion.IngestionOrchestrator.Process's budgeted
	// operation.
	OpIngestionPipeline OperationName = "ingestion_pipeline"
)

// Budget states the target latency percentiles and minimum throughput
// an operation is expected to meet. Zero-valued fields are not evaluated
// (see Evaluate), so a Budget may deliberately leave a dimension
// unbounded.
type Budget struct {
	// Operation names the operation this budget governs.
	Operation OperationName

	// P50 is the target median latency: at least half of observed calls
	// should complete at or under this duration.
	P50 time.Duration

	// P95 is the target 95th-percentile latency.
	P95 time.Duration

	// P99 is the target 99th-percentile latency.
	P99 time.Duration

	// MinThroughput is the minimum acceptable sustained throughput, in
	// operations per second.
	MinThroughput float64
}

// validate reports whether b is structurally well-formed: no negative
// latency target and a positive MinThroughput.
func (b Budget) validate() error {
	if b.Operation == "" {
		return ErrInvalidBudget
	}
	if b.P50 < 0 || b.P95 < 0 || b.P99 < 0 {
		return ErrInvalidBudget
	}
	if b.MinThroughput <= 0 {
		return ErrInvalidBudget
	}
	return nil
}

// DefaultBudgets returns this platform's real, concrete performance budgets
// for the three phases this package benchmarks against: packages/
// hybridretrieval (Phase 044), packages/traversal (Phase 043), and
// packages/ingestion (Phase 029). Numbers reflect each operation's expected
// shape: hybrid retrieval fuses a fast vector-recall floor with a
// budget-sensitive graph-expansion enrichment (see
// packages/hybridretrieval/budget.go); graph traversal is a bounded, in-
// memory breadth-first walk and is expected to be the fastest of the three;
// ingestion is the heaviest end-to-end pipeline (intake, transcription/OCR,
// normalization, segmentation, classification chained in sequence) and is
// budgeted accordingly slower and lower-throughput.
func DefaultBudgets() []Budget {
	return []Budget{
		{
			Operation:     OpHybridRetrieval,
			P50:           150 * time.Millisecond,
			P95:           500 * time.Millisecond,
			P99:           900 * time.Millisecond,
			MinThroughput: 20,
		},
		{
			Operation:     OpGraphTraversal,
			P50:           80 * time.Millisecond,
			P95:           300 * time.Millisecond,
			P99:           600 * time.Millisecond,
			MinThroughput: 40,
		},
		{
			Operation:     OpIngestionPipeline,
			P50:           2 * time.Second,
			P95:           5 * time.Second,
			P99:           8 * time.Second,
			MinThroughput: 5,
		},
	}
}

// Measurement is an observed latency/throughput sample for a named
// operation, to be checked against its Budget via Evaluate.
type Measurement struct {
	// P50, P95, and P99 are the observed latency percentiles.
	P50 time.Duration
	P95 time.Duration
	P99 time.Duration

	// Throughput is the observed sustained throughput, in operations per
	// second.
	Throughput float64
}

// validate reports whether m is structurally well-formed: no negative
// latency or throughput value.
func (m Measurement) validate() error {
	if m.P50 < 0 || m.P95 < 0 || m.P99 < 0 || m.Throughput < 0 {
		return ErrInvalidMeasurement
	}
	return nil
}

// DimensionVerdict reports pass/fail for a single evaluated dimension
// (P50/P95/P99/Throughput) of a Verdict.
type DimensionVerdict struct {
	// Target is the Budget's value for this dimension.
	Target float64

	// Observed is the Measurement's value for this dimension.
	Observed float64

	// Passed reports whether Observed met Target for this dimension.
	Passed bool
}

// Verdict reports the outcome of evaluating a Measurement against a
// Budget: a pass/fail per dimension, plus an overall pass/fail that is
// true only when every dimension passed.
type Verdict struct {
	// Operation names the operation this Verdict was computed for.
	Operation OperationName

	// P50, P95, P99, and Throughput report the per-dimension outcome.
	// Latency DimensionVerdicts store durations as float64 nanoseconds in
	// Target/Observed so the type is uniform across all four dimensions.
	P50        DimensionVerdict
	P95        DimensionVerdict
	P99        DimensionVerdict
	Throughput DimensionVerdict

	// Passed is true only when every dimension above passed.
	Passed bool
}

// budgetsByOperation indexes DefaultBudgets by Operation for O(1) lookup.
func budgetsByOperation() map[OperationName]Budget {
	out := make(map[OperationName]Budget)
	for _, b := range DefaultBudgets() {
		out[b.Operation] = b
	}
	return out
}

// Evaluate compares observed against operationName's registered Budget
// (see DefaultBudgets) and returns a Verdict reporting pass/fail per
// dimension plus an overall pass/fail.
//
// Returns ErrUnknownOperation if operationName has no registered budget, or
// ErrInvalidMeasurement if observed fails structural validation.
func Evaluate(operationName OperationName, observed Measurement) (Verdict, error) {
	if err := observed.validate(); err != nil {
		return Verdict{}, wrapf("Evaluate", err)
	}

	budget, ok := budgetsByOperation()[operationName]
	if !ok {
		return Verdict{}, wrapf("Evaluate", ErrUnknownOperation)
	}

	p50 := DimensionVerdict{
		Target:   float64(budget.P50),
		Observed: float64(observed.P50),
		Passed:   observed.P50 <= budget.P50,
	}
	p95 := DimensionVerdict{
		Target:   float64(budget.P95),
		Observed: float64(observed.P95),
		Passed:   observed.P95 <= budget.P95,
	}
	p99 := DimensionVerdict{
		Target:   float64(budget.P99),
		Observed: float64(observed.P99),
		Passed:   observed.P99 <= budget.P99,
	}
	throughput := DimensionVerdict{
		Target:   budget.MinThroughput,
		Observed: observed.Throughput,
		Passed:   observed.Throughput >= budget.MinThroughput,
	}

	return Verdict{
		Operation:  operationName,
		P50:        p50,
		P95:        p95,
		P99:        p99,
		Throughput: throughput,
		Passed:     p50.Passed && p95.Passed && p99.Passed && throughput.Passed,
	}, nil
}
