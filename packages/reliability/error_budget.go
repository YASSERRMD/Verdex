package reliability

// ErrorBudget tracks how much of an SLO's allowed failure margin has
// been consumed by observed failures over the same rolling window the
// SLO itself is evaluated against. An SLO of 99.5% success over a
// window implicitly allows up to 0.5% of calls in that window to fail
// -- that 0.5% is the error budget; ErrorBudget computes how much of
// it remains.
//
// Only meaningful for an SLOKindSuccessRate SLO: a latency SLO has no
// analogous "budget" in this model (see doc.go).
type ErrorBudget struct {
	// SLO is the objective this budget is computed against.
	SLO SLO

	// AllowedFailureRate is 1 - SLO.Target: the maximum fraction of
	// calls in the window permitted to fail while still meeting the
	// objective.
	AllowedFailureRate float64

	// ObservedFailureRate is the actual fraction of calls in the
	// window that failed.
	ObservedFailureRate float64

	// ConsumedFraction is ObservedFailureRate / AllowedFailureRate,
	// i.e. what fraction of the allowed budget has been used. A value
	// of 1.0 means the budget is exactly exhausted; > 1.0 means the
	// SLO is currently being violated (more failures observed than the
	// budget allows).
	ConsumedFraction float64

	// RemainingFraction is 1 - ConsumedFraction, clamped to 0 (never
	// negative -- once exhausted, "remaining" reads as zero, not a
	// negative number).
	RemainingFraction float64

	// SampleCount is how many observations (within the SLO's window)
	// this computation is based on.
	SampleCount int
}

// ComputeErrorBudget derives an ErrorBudget from status, which must
// have been produced by EvaluateSLO for an SLOKindSuccessRate SLO.
// Returns a wrapped ErrInvalidSLO if status.SLO is not a success-rate
// SLO.
//
// When AllowedFailureRate is exactly zero (a 100%-success-rate SLO,
// which permits zero failures), any observed failure at all fully
// exhausts the budget (ConsumedFraction is reported as +Inf-avoiding
// 1.0 when ObservedFailureRate is also zero, or a large finite ratio
// otherwise) -- callers evaluating a 100% SLO should treat any
// ObservedFailureRate > 0 as an immediate, total violation regardless
// of the exact ConsumedFraction value.
func ComputeErrorBudget(status SLOStatus) (ErrorBudget, error) {
	if status.SLO.Kind != SLOKindSuccessRate {
		return ErrorBudget{}, wrapf("ComputeErrorBudget", ErrInvalidSLO)
	}

	allowed := 1 - status.SLO.Target
	observedFailureRate := 1 - status.Observed

	var consumed float64
	switch {
	case allowed <= 0 && observedFailureRate <= 0:
		consumed = 0
	case allowed <= 0:
		// Zero-tolerance SLO with any observed failure: fully (and
		// then some) exhausted. Report a large but finite ratio rather
		// than dividing by zero.
		consumed = observedFailureRate / smallestPositive
	default:
		consumed = observedFailureRate / allowed
	}

	remaining := 1 - consumed
	if remaining < 0 {
		remaining = 0
	}

	return ErrorBudget{
		SLO:                 status.SLO,
		AllowedFailureRate:  allowed,
		ObservedFailureRate: observedFailureRate,
		ConsumedFraction:    consumed,
		RemainingFraction:   remaining,
		SampleCount:         status.SampleCount,
	}, nil
}

// smallestPositive is the divisor ComputeErrorBudget falls back to
// when AllowedFailureRate is exactly zero, so a zero-tolerance SLO
// with any observed failure produces a very large (but finite and
// well-defined) ConsumedFraction instead of dividing by zero.
const smallestPositive = 0.0001

// ErrorBudgetPolicy evaluates an ErrorBudget against a configured
// exhaustion threshold and reports a policy signal -- e.g. "budget
// exhausted, block risky deploys" -- as a plain boolean, per the
// brief's scope: this package computes the signal, it does not itself
// gate any deploy pipeline.
type ErrorBudgetPolicy struct {
	// ExhaustionThreshold is the ConsumedFraction at or above which
	// this policy considers the budget exhausted. Must be in (0,
	// +Inf); a value of 1.0 (the common convention) means "exhausted
	// once 100% of the allowed budget is used". Values <= 0 fall back
	// to DefaultExhaustionThreshold.
	ExhaustionThreshold float64
}

// DefaultExhaustionThreshold is the ConsumedFraction ErrorBudgetPolicy
// treats as exhausted when ExhaustionThreshold is left at its zero
// value: 1.0, meaning the budget is considered exhausted once the
// full allowed failure margin has been consumed.
const DefaultExhaustionThreshold = 1.0

func (p ErrorBudgetPolicy) threshold() float64 {
	if p.ExhaustionThreshold <= 0 {
		return DefaultExhaustionThreshold
	}
	return p.ExhaustionThreshold
}

// Validate checks p for structural well-formedness.
func (p ErrorBudgetPolicy) Validate() error {
	if p.ExhaustionThreshold < 0 {
		return ErrInvalidBudgetPolicy
	}
	return nil
}

// PolicyResult is the outcome of evaluating an ErrorBudgetPolicy
// against an ErrorBudget.
type PolicyResult struct {
	// Budget is the ErrorBudget this result was computed from.
	Budget ErrorBudget

	// Exhausted reports whether Budget.ConsumedFraction has reached
	// the policy's threshold.
	Exhausted bool

	// BlockRiskyDeploys is the policy signal a deploy pipeline could
	// consult: true when Exhausted, meaning risky (non-hotfix)
	// deploys should be paused until the budget recovers. This package
	// does not integrate with any actual deploy gate -- it only
	// computes this boolean per the brief's scope.
	BlockRiskyDeploys bool
}

// Evaluate reports whether budget has exhausted policy's threshold,
// and if so, signals that risky deploys should be blocked. Returns a
// wrapped ErrInvalidBudgetPolicy if p fails structural validation.
func (p ErrorBudgetPolicy) Evaluate(budget ErrorBudget) (PolicyResult, error) {
	if err := p.Validate(); err != nil {
		return PolicyResult{}, wrapf("ErrorBudgetPolicy.Evaluate", err)
	}

	exhausted := budget.ConsumedFraction >= p.threshold()
	return PolicyResult{
		Budget:            budget,
		Exhausted:         exhausted,
		BlockRiskyDeploys: exhausted,
	}, nil
}
