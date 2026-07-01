package eval

// Category classifies an EvalTask by the type of legal reasoning it tests.
type Category string

const (
	// CategoryRetrieval tests whether the model can surface relevant facts or
	// provisions from a given set of materials.
	CategoryRetrieval Category = "retrieval"

	// CategoryReasoning tests multi-step legal analysis: IRAC-style reasoning,
	// applying rules to facts, weighing competing considerations.
	CategoryReasoning Category = "reasoning"

	// CategoryCitationFidelity tests whether the model accurately cites
	// statutes, case names, and docket numbers.
	CategoryCitationFidelity Category = "citation_fidelity"

	// CategoryJurisdictionAccuracy tests whether the model correctly identifies
	// and applies the governing jurisdiction.
	CategoryJurisdictionAccuracy Category = "jurisdiction_accuracy"
)

// ScorerFn is a pure scoring function.
//
// got is the raw text produced by the model; expected is the golden answer.
// The function must return a value in [0.0, 1.0] where 1.0 means a perfect
// match and 0.0 means no credit.
//
// ScorerFn implementations must be safe for concurrent use from multiple
// goroutines and must not modify their arguments.
type ScorerFn func(got, expected string) float64

// RubricCriteria pairs a named criterion with its relative weight and scorer.
//
// During scoring the raw score from Fn is multiplied by Weight.  Weights
// across all criteria in a rubric do not have to sum to 1; EvalRunner
// normalises them before computing the final task score.
type RubricCriteria struct {
	// Name is a short human-readable label for this criterion (e.g.
	// "keyword_presence", "citation_accuracy").
	Name string

	// Weight is the relative importance of this criterion.  Must be > 0.
	Weight float64

	// Fn is the scorer that evaluates the model output for this criterion.
	Fn ScorerFn
}

// EvalTask is a single evaluation scenario used to probe a model's legal
// reasoning ability.
//
// Tasks are immutable once constructed; EvalRunner never mutates them.
type EvalTask struct {
	// ID is a stable, unique identifier for this task (e.g. "negligence-001").
	ID string

	// Name is a short human-readable title (e.g. "Negligence issue spotting").
	Name string

	// Category classifies the cognitive skill being tested.
	Category Category

	// Prompt is the full text sent to the model as the user turn.
	Prompt string

	// GoldenAnswer is the ideal model response used as the comparison baseline
	// by all ScorerFns in ScoringRubric.
	GoldenAnswer string

	// ScoringRubric is an ordered list of weighted criteria applied to the
	// model output.  Must contain at least one entry.
	ScoringRubric []RubricCriteria

	// Seed is used for any stochastic elements in task generation; it is
	// recorded in EvalResult for reproducibility.  EvalRunner always calls
	// providers with Temperature=0 regardless of Seed.
	Seed int64
}
