package reliability

import (
	"errors"
	"testing"
	"time"
)

func TestComputeErrorBudget_RejectsLatencySLO(t *testing.T) {
	status := SLOStatus{SLO: SLO{Name: "x", Kind: SLOKindLatency, Target: 1, Window: time.Hour}}
	_, err := ComputeErrorBudget(status)
	if !errors.Is(err, ErrInvalidSLO) {
		t.Fatalf("expected ErrInvalidSLO for a non-success-rate SLO, got %v", err)
	}
}

// TestComputeErrorBudget_WithinBudget: a 99% target SLO (1% allowed
// failure rate) observing a 99.5% success rate (0.5% failures) has used
// half its budget -- comfortably within budget.
func TestComputeErrorBudget_WithinBudget(t *testing.T) {
	status := SLOStatus{
		SLO:         SLO{Name: "ingestion", Kind: SLOKindSuccessRate, Target: 0.99, Window: 24 * time.Hour},
		Observed:    0.995,
		SampleCount: 1000,
	}

	budget, err := ComputeErrorBudget(status)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if budget.AllowedFailureRate < 0.0099 || budget.AllowedFailureRate > 0.0101 {
		t.Fatalf("expected AllowedFailureRate ~= 0.01, got %v", budget.AllowedFailureRate)
	}
	if budget.ObservedFailureRate < 0.0049 || budget.ObservedFailureRate > 0.0051 {
		t.Fatalf("expected ObservedFailureRate ~= 0.005, got %v", budget.ObservedFailureRate)
	}
	if budget.ConsumedFraction < 0.49 || budget.ConsumedFraction > 0.51 {
		t.Fatalf("expected ConsumedFraction ~= 0.5 (half the budget used), got %v", budget.ConsumedFraction)
	}
	if budget.RemainingFraction < 0.49 || budget.RemainingFraction > 0.51 {
		t.Fatalf("expected RemainingFraction ~= 0.5, got %v", budget.RemainingFraction)
	}

	policy := ErrorBudgetPolicy{}
	result, err := policy.Evaluate(budget)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if result.Exhausted {
		t.Fatal("expected budget NOT exhausted at 50% consumption")
	}
	if result.BlockRiskyDeploys {
		t.Fatal("expected BlockRiskyDeploys=false when budget is within bounds")
	}
}

// TestComputeErrorBudget_Exhausted: the same 99% target SLO observing
// only 97% success (3% failures, 3x the 1% allowed) has fully blown
// through its budget.
func TestComputeErrorBudget_Exhausted(t *testing.T) {
	status := SLOStatus{
		SLO:         SLO{Name: "ingestion", Kind: SLOKindSuccessRate, Target: 0.99, Window: 24 * time.Hour},
		Observed:    0.97,
		SampleCount: 1000,
	}

	budget, err := ComputeErrorBudget(status)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if budget.ConsumedFraction <= 1.0 {
		t.Fatalf("expected ConsumedFraction > 1.0 (3%% failures vs 1%% allowed), got %v", budget.ConsumedFraction)
	}
	if budget.RemainingFraction != 0 {
		t.Fatalf("expected RemainingFraction clamped to 0 once exhausted, got %v", budget.RemainingFraction)
	}

	policy := ErrorBudgetPolicy{}
	result, err := policy.Evaluate(budget)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if !result.Exhausted {
		t.Fatal("expected budget exhausted at 3x consumption")
	}
	if !result.BlockRiskyDeploys {
		t.Fatal("expected BlockRiskyDeploys=true once the budget is exhausted")
	}
}

func TestComputeErrorBudget_ExactlyAtTarget_FullyConsumedNotExceeded(t *testing.T) {
	status := SLOStatus{
		SLO:      SLO{Name: "x", Kind: SLOKindSuccessRate, Target: 0.99, Window: time.Hour},
		Observed: 0.99, // exactly at target: exactly 1% failures, exactly the allowed rate
	}
	budget, err := ComputeErrorBudget(status)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if budget.ConsumedFraction < 0.99 || budget.ConsumedFraction > 1.01 {
		t.Fatalf("expected ConsumedFraction ~= 1.0 exactly at target, got %v", budget.ConsumedFraction)
	}

	policy := ErrorBudgetPolicy{}
	result, _ := policy.Evaluate(budget)
	if !result.Exhausted {
		t.Fatal("expected exactly-at-threshold consumption to count as exhausted (>= comparison)")
	}
}

func TestComputeErrorBudget_PerfectSuccessRate_ZeroConsumption(t *testing.T) {
	status := SLOStatus{
		SLO:      SLO{Name: "x", Kind: SLOKindSuccessRate, Target: 0.99, Window: time.Hour},
		Observed: 1.0,
	}
	budget, err := ComputeErrorBudget(status)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if budget.ConsumedFraction != 0 {
		t.Fatalf("expected zero consumption at 100%% observed success, got %v", budget.ConsumedFraction)
	}
	if budget.RemainingFraction != 1 {
		t.Fatalf("expected full remaining budget, got %v", budget.RemainingFraction)
	}
}

func TestComputeErrorBudget_ZeroToleranceSLO_AnyFailureExhausts(t *testing.T) {
	status := SLOStatus{
		SLO:      SLO{Name: "x", Kind: SLOKindSuccessRate, Target: 1.0, Window: time.Hour}, // 100% required
		Observed: 0.999,                                                                    // any failure at all
	}
	budget, err := ComputeErrorBudget(status)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if budget.AllowedFailureRate != 0 {
		t.Fatalf("expected zero allowed failure rate for a 100%% SLO, got %v", budget.AllowedFailureRate)
	}
	if budget.ConsumedFraction <= 1.0 {
		t.Fatalf("expected any failure against a zero-tolerance SLO to read as fully exhausted, got %v", budget.ConsumedFraction)
	}

	policy := ErrorBudgetPolicy{}
	result, _ := policy.Evaluate(budget)
	if !result.Exhausted {
		t.Fatal("expected a zero-tolerance SLO with any failure to be reported exhausted")
	}
}

func TestComputeErrorBudget_ZeroToleranceSLO_PerfectSuccessNotExhausted(t *testing.T) {
	status := SLOStatus{
		SLO:      SLO{Name: "x", Kind: SLOKindSuccessRate, Target: 1.0, Window: time.Hour},
		Observed: 1.0,
	}
	budget, err := ComputeErrorBudget(status)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if budget.ConsumedFraction != 0 {
		t.Fatalf("expected zero consumption when a zero-tolerance SLO sees perfect success, got %v", budget.ConsumedFraction)
	}
}

func TestErrorBudgetPolicy_CustomThreshold(t *testing.T) {
	// A conservative policy that blocks deploys once 80% of the budget
	// (not the full 100%) is consumed.
	policy := ErrorBudgetPolicy{ExhaustionThreshold: 0.8}

	budget := ErrorBudget{ConsumedFraction: 0.85}
	result, err := policy.Evaluate(budget)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if !result.Exhausted {
		t.Fatal("expected exhausted at 85% consumption against an 80% threshold")
	}

	budget2 := ErrorBudget{ConsumedFraction: 0.5}
	result2, _ := policy.Evaluate(budget2)
	if result2.Exhausted {
		t.Fatal("expected not exhausted at 50% consumption against an 80% threshold")
	}
}

func TestErrorBudgetPolicy_DefaultThresholdWhenUnset(t *testing.T) {
	policy := ErrorBudgetPolicy{}
	if got := policy.threshold(); got != DefaultExhaustionThreshold {
		t.Fatalf("expected default threshold %v, got %v", DefaultExhaustionThreshold, got)
	}
}

func TestErrorBudgetPolicy_Validate(t *testing.T) {
	if err := (ErrorBudgetPolicy{ExhaustionThreshold: -1}).Validate(); !errors.Is(err, ErrInvalidBudgetPolicy) {
		t.Fatalf("expected ErrInvalidBudgetPolicy for a negative threshold, got %v", err)
	}
	if err := (ErrorBudgetPolicy{ExhaustionThreshold: 0.5}).Validate(); err != nil {
		t.Fatalf("expected a valid policy, got %v", err)
	}
}

func TestErrorBudgetPolicy_Evaluate_InvalidPolicyReturnsError(t *testing.T) {
	policy := ErrorBudgetPolicy{ExhaustionThreshold: -1}
	_, err := policy.Evaluate(ErrorBudget{})
	if !errors.Is(err, ErrInvalidBudgetPolicy) {
		t.Fatalf("expected ErrInvalidBudgetPolicy, got %v", err)
	}
}

// TestEndToEnd_SLOToErrorBudgetToPolicy exercises the full pipeline this
// package documents: observations -> EvaluateSLO -> ComputeErrorBudget ->
// ErrorBudgetPolicy.Evaluate, for both a within-budget and an exhausted
// scenario, using realistic Observation data rather than a hand-built
// SLOStatus.
func TestEndToEnd_SLOToErrorBudgetToPolicy(t *testing.T) {
	now := time.Now()
	slo := SLO{Name: "ingestion-availability", Kind: SLOKindSuccessRate, Target: 0.99, Window: time.Hour}

	t.Run("within budget", func(t *testing.T) {
		obs := make([]Observation, 0, 1000)
		for i := 0; i < 995; i++ {
			obs = append(obs, Observation{Success: true, At: now})
		}
		for i := 0; i < 5; i++ {
			obs = append(obs, Observation{Success: false, At: now})
		}

		status, err := EvaluateSLO(slo, obs, now)
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		budget, err := ComputeErrorBudget(status)
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		result, err := (ErrorBudgetPolicy{}).Evaluate(budget)
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		if result.Exhausted {
			t.Fatal("expected 0.5% failures against a 1% budget to NOT be exhausted")
		}
	})

	t.Run("exhausted", func(t *testing.T) {
		obs := make([]Observation, 0, 1000)
		for i := 0; i < 950; i++ {
			obs = append(obs, Observation{Success: true, At: now})
		}
		for i := 0; i < 50; i++ {
			obs = append(obs, Observation{Success: false, At: now})
		}

		status, err := EvaluateSLO(slo, obs, now)
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		budget, err := ComputeErrorBudget(status)
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		result, err := (ErrorBudgetPolicy{}).Evaluate(budget)
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		if !result.Exhausted {
			t.Fatal("expected 5% failures against a 1% budget to be exhausted")
		}
		if !result.BlockRiskyDeploys {
			t.Fatal("expected BlockRiskyDeploys=true")
		}
	})
}
