package e2e

import (
	"context"
	"strings"
	"time"

	"github.com/YASSERRMD/verdex/packages/category"
)

// ScenarioOutcome classifies the terminal result of one Scenario.Run
// call, mirroring packages/securitytesting.Outcome's three-value shape
// (passed / failed / errored) applied to a full-journey test instead of
// an adversarial probe.
type ScenarioOutcome string

// ScenarioOutcome values.
const (
	// OutcomePassed means every stage the scenario composed through
	// completed and every assertion the scenario itself makes about its
	// own journey held.
	OutcomePassed ScenarioOutcome = "passed"

	// OutcomeFailed means the scenario ran to completion but one of its
	// own assertions about the journey (e.g. "sign-off blocks
	// finalization", "case B's facts are unreachable from case A") did
	// not hold -- a genuine defect, not an execution error.
	OutcomeFailed ScenarioOutcome = "failed"

	// OutcomeErrored means the scenario itself could not be evaluated
	// (e.g. a dependency call returned an unexpected error unrelated to
	// the property under test).
	OutcomeErrored ScenarioOutcome = "errored"
)

// IsValid reports whether o is one of the three named ScenarioOutcome
// constants.
func (o ScenarioOutcome) IsValid() bool {
	switch o {
	case OutcomePassed, OutcomeFailed, OutcomeErrored:
		return true
	}
	return false
}

// String satisfies fmt.Stringer.
func (o ScenarioOutcome) String() string { return string(o) }

// ScenarioResult is what one Scenario.Run call produces: the outcome,
// a human-readable Detail explaining it, and the real, typed artifacts
// the journey actually threaded through packages/ingestion,
// packages/reasoningorchestration, and packages/signoff -- so a caller
// (or a failing test's t.Fatalf) can inspect exactly how far the
// journey got and what it produced, not just a pass/fail bit.
type ScenarioResult struct {
	// Outcome is the terminal classification of this run.
	Outcome ScenarioOutcome

	// Detail is a human-readable explanation, required whenever Outcome
	// is not OutcomePassed, optional (but encouraged) otherwise.
	Detail string

	// CaseID is the synthetic case identifier this run used, threading
	// packages/ingestion's Job.CaseID and
	// packages/reasoningorchestration's caseID argument together.
	CaseID string

	// IngestionStage is the final ingestion.Stage the journey's intake
	// phase reached (see packages/ingestion.WorkflowState.Stage).
	IngestionStage string

	// ReasoningTermination is the final
	// reasoningorchestration.TerminationReason the journey's reasoning
	// phase reached.
	ReasoningTermination string

	// CompletedReasoningStages counts how many reasoningorchestration
	// stages completed successfully during this run.
	CompletedReasoningStages int

	// GuardrailApproved mirrors
	// reasoningorchestration.Checkpoint.GuardrailApproved from the
	// journey's terminal StageGuardrailCheck checkpoint.
	GuardrailApproved bool

	// LegalFamily is the jurisdiction.LegalFamily (as a plain string)
	// this run resolved and drove the reasoning stage's profile
	// selection with.
	LegalFamily string

	// ResolvedWeights is the reasoningprofile.Weights this run resolved
	// for LegalFamily, recorded so a multi-jurisdiction variant test can
	// assert two different families genuinely produced two different
	// weight profiles rather than merely "ran without erroring."
	ResolvedWeights ScenarioWeights

	// StartedAt and FinishedAt bound this run's wall-clock duration.
	StartedAt  time.Time
	FinishedAt time.Time
}

// ScenarioWeights is a plain-data mirror of
// packages/reasoningprofile.Weights, copied field-by-field rather than
// aliased, so this package's public ScenarioResult never forces a
// caller to import packages/reasoningprofile just to read back which
// weight profile a run resolved.
type ScenarioWeights struct {
	TestimonyEmphasis   float64
	DocumentaryEmphasis float64
	StatuteEmphasis     float64
	PrecedentEmphasis   float64
}

// Duration returns FinishedAt.Sub(StartedAt), or zero if either is
// unset.
func (r ScenarioResult) Duration() time.Duration {
	if r.StartedAt.IsZero() || r.FinishedAt.IsZero() {
		return 0
	}
	return r.FinishedAt.Sub(r.StartedAt)
}

// Validate checks r for structural well-formedness: a recognized
// Outcome, and a non-blank Detail whenever Outcome is not
// OutcomePassed. Mirrors
// packages/securitytesting.Result.Validate's shape.
func (r ScenarioResult) Validate() error {
	if !r.Outcome.IsValid() {
		return wrapf("ScenarioResult.Validate", ErrInvalidCategory)
	}
	if r.Outcome != OutcomePassed && strings.TrimSpace(r.Detail) == "" {
		return wrapf("ScenarioResult.Validate", ErrEmptyScenarioName)
	}
	return nil
}

// Scenario is the single extension point every full-journey E2E test in
// this package implements (task 1): a named, case-category-classified
// journey composing real calls through packages/ingestion's
// orchestrator, packages/reasoningorchestration's pipeline, and
// packages/signoff -- an actual in-process, in-memory, no-Docker test,
// never a description of one. Mirrors
// packages/securitytesting.Scenario's interface shape exactly (Name /
// Category / Run), applied to full-journey composition instead of
// adversarial probing.
type Scenario interface {
	// Name is a short, stable, human-referenceable identifier for this
	// scenario (e.g. "civil/setup-to-opinion",
	// "criminal/multi-jurisdiction/civil-law").
	Name() string

	// CaseCategory classifies which packages/category taxonomy entry
	// this scenario's synthetic case belongs to.
	CaseCategory() category.CategoryCode

	// Run executes the scenario's full journey and reports what
	// happened. A non-nil error means the scenario itself could not be
	// evaluated (e.g. a dependency panic-equivalent or malformed
	// fixture) -- distinct from a ScenarioResult{Outcome: OutcomeFailed},
	// which means the journey ran and *found* that a property under test
	// did not hold.
	Run(ctx context.Context) (ScenarioResult, error)
}

// ScenarioFunc adapts a plain function to the Scenario interface,
// mirroring packages/securitytesting.ScenarioFunc's
// http.HandlerFunc-style adapter: most scenarios in this package are a
// name/category pair plus one function body and do not need a bespoke
// struct type.
type ScenarioFunc struct {
	name string
	cat  category.CategoryCode
	run  func(ctx context.Context) (ScenarioResult, error)
}

// NewScenarioFunc builds a Scenario from name, cat, and run. Returns
// ErrEmptyScenarioName if name is blank, or ErrNilScenarioFunc if run is
// nil. cat is not validated against any specific packages/category
// Taxonomy instance here (a Scenario's synthetic case category is
// illustrative, not looked up against a live taxonomy), but must be
// non-blank.
func NewScenarioFunc(name string, cat category.CategoryCode, run func(ctx context.Context) (ScenarioResult, error)) (ScenarioFunc, error) {
	if strings.TrimSpace(name) == "" {
		return ScenarioFunc{}, ErrEmptyScenarioName
	}
	if strings.TrimSpace(string(cat)) == "" {
		return ScenarioFunc{}, wrapf("NewScenarioFunc", ErrInvalidCategory)
	}
	if run == nil {
		return ScenarioFunc{}, ErrNilScenarioFunc
	}
	return ScenarioFunc{name: name, cat: cat, run: run}, nil
}

// Name implements Scenario.
func (f ScenarioFunc) Name() string { return f.name }

// CaseCategory implements Scenario.
func (f ScenarioFunc) CaseCategory() category.CategoryCode { return f.cat }

// Run implements Scenario.
func (f ScenarioFunc) Run(ctx context.Context) (ScenarioResult, error) { return f.run(ctx) }

var _ Scenario = ScenarioFunc{}
