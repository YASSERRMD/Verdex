package eval

import "time"

// EvalResult captures every observable from a single (provider, task) run.
type EvalResult struct {
	// TaskID matches EvalTask.ID.
	TaskID string

	// ProviderID matches LLMProvider.ID().
	ProviderID string

	// ModelID is the specific model used (from provider Capabilities).
	ModelID string

	// Score is the weighted aggregate score across all rubric criteria,
	// normalised to [0.0, 1.0].
	Score float64

	// Rubric maps each RubricCriteria.Name to its raw (un-weighted) score so
	// that callers can inspect per-criterion performance.
	Rubric map[string]float64

	// Output is the verbatim text returned by the model.
	Output string

	// Latency is the wall-clock time for the Chat call.
	Latency time.Duration

	// InputTokens and OutputTokens are taken from the provider ChatResponse.
	InputTokens  int
	OutputTokens int

	// EvalAt is the UTC instant at which the result was recorded.
	EvalAt time.Time
}

// ProviderSummary aggregates all EvalResults for a single provider across an
// entire EvalReport run.
type ProviderSummary struct {
	// ProviderID identifies the provider this summary belongs to.
	ProviderID string

	// AvgScore is the arithmetic mean of EvalResult.Score across all tasks.
	AvgScore float64

	// P50Latency is the median wall-clock latency across all tasks.
	P50Latency time.Duration

	// P95Latency is the 95th-percentile wall-clock latency across all tasks.
	P95Latency time.Duration

	// TotalCost is a placeholder for future cost-accounting integration;
	// currently always 0.
	TotalCost float64
}

// EvalReport is the top-level artefact produced by a complete evaluation run.
type EvalReport struct {
	// GoldenVersion is the Version field of the GoldenSet used for this run.
	GoldenVersion string

	// Results contains every individual EvalResult from the run.
	Results []EvalResult

	// Summary maps ProviderID to an aggregated ProviderSummary.
	Summary map[string]ProviderSummary

	// RunAt is the UTC instant at which the run was initiated.
	RunAt time.Time
}
