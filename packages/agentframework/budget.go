package agentframework

import (
	"context"
	"time"
)

// DefaultMaxSteps is the MaxSteps a zero-valued Budget falls back to.
const DefaultMaxSteps = 12

// DefaultMaxWallClock is the MaxWallClock a zero-valued Budget falls back
// to.
const DefaultMaxWallClock = 2 * time.Minute

// DefaultStepTimeout is the StepTimeout a zero-valued Budget falls back
// to.
const DefaultStepTimeout = 30 * time.Second

// Budget bounds how much work a single Runner.Run may perform: how many
// steps it may take, how much wall-clock time the whole run may spend,
// how long any single step may run before being cancelled, and
// (best-effort) how many tokens it may consume in total. A Runner
// enforces every limit independently — whichever is reached first ends
// the run — so a run that would otherwise loop indefinitely degrades to
// a graceful TerminationBudgetExhausted result instead of blocking
// forever or panicking.
//
// The zero value is not directly usable; use DefaultBudget for sensible
// defaults, or construct a Budget literal — Runner treats zero fields as
// "use the default", mirroring adaptiveretrieval.BuildBudget's own
// zero-means-default convention.
type Budget struct {
	// MaxSteps caps how many Agent.BuildRequest/Interpret iterations a
	// run may perform. Zero or negative falls back to DefaultMaxSteps.
	MaxSteps int

	// MaxWallClock caps the total wall-clock duration of a run, measured
	// from the first call to Runner.Run. Zero or negative falls back to
	// DefaultMaxWallClock.
	MaxWallClock time.Duration

	// StepTimeout bounds a single step's model call plus any tool calls
	// it triggers. Zero or negative falls back to DefaultStepTimeout.
	StepTimeout time.Duration

	// MaxTokens caps the cumulative provider.TokenUsage.TotalTokens
	// across every step's ChatResponse. Zero (or negative) means no
	// token ceiling is enforced — this is a best-effort limit: it is
	// checked only after each step completes (a provider does not report
	// token usage before generating a response), so a single step can
	// still overshoot it by that step's own usage before the run stops
	// on the next iteration.
	MaxTokens int
}

// DefaultBudget returns a Budget using DefaultMaxSteps, DefaultMaxWallClock,
// and DefaultStepTimeout, with no token ceiling.
func DefaultBudget() Budget {
	return Budget{
		MaxSteps:     DefaultMaxSteps,
		MaxWallClock: DefaultMaxWallClock,
		StepTimeout:  DefaultStepTimeout,
	}
}

// withDefaults returns a copy of b with zero-or-negative fields replaced
// by their documented defaults. MaxTokens is left as-is (zero legitimately
// means "unbounded").
func (b Budget) withDefaults() Budget {
	out := b
	if out.MaxSteps <= 0 {
		out.MaxSteps = DefaultMaxSteps
	}
	if out.MaxWallClock <= 0 {
		out.MaxWallClock = DefaultMaxWallClock
	}
	if out.StepTimeout <= 0 {
		out.StepTimeout = DefaultStepTimeout
	}
	return out
}

// budgetTracker turns a Budget into concrete run-time bookkeeping: a
// wall-clock deadline, a step counter, and a running token total.
type budgetTracker struct {
	budget     Budget
	deadline   time.Time
	stepsTaken int
	tokensUsed int
}

// newBudgetTracker starts a budgetTracker for budget, measured from now.
func newBudgetTracker(budget Budget) *budgetTracker {
	budget = budget.withDefaults()
	return &budgetTracker{
		budget:   budget,
		deadline: time.Now().Add(budget.MaxWallClock),
	}
}

// exhausted reports whether any Budget limit has been reached, and if so,
// which one triggered it (for telemetry/error messages).
func (t *budgetTracker) exhausted() (bool, string) {
	if t.stepsTaken >= t.budget.MaxSteps {
		return true, "max steps reached"
	}
	if time.Now().After(t.deadline) {
		return true, "wall-clock budget exceeded"
	}
	if t.budget.MaxTokens > 0 && t.tokensUsed >= t.budget.MaxTokens {
		return true, "token budget exceeded"
	}
	return false, ""
}

// recordStep increments the step counter and adds tokens to the running
// total.
func (t *budgetTracker) recordStep(tokens int) {
	t.stepsTaken++
	t.tokensUsed += tokens
}

// stepContext returns a context bounded by both parent's own deadline (if
// any), t's tracked overall run deadline, and the Budget's per-step
// timeout — whichever is soonest — plus the resulting cancel function.
// Callers must call the returned cancel function.
func (t *budgetTracker) stepContext(parent context.Context) (context.Context, context.CancelFunc) {
	stepDeadline := time.Now().Add(t.budget.StepTimeout)
	if t.deadline.Before(stepDeadline) {
		stepDeadline = t.deadline
	}
	return context.WithDeadline(parent, stepDeadline)
}
