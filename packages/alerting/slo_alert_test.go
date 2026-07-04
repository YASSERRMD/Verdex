package alerting_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/alerting"
	"github.com/YASSERRMD/verdex/packages/reliability"
)

func successRateSLO() reliability.SLO {
	return reliability.SLO{
		Name:   "ingestion-availability",
		Kind:   reliability.SLOKindSuccessRate,
		Target: 0.99,
		Window: 24 * time.Hour,
	}
}

func TestEvaluateSLOAlert_FiresOnBreach(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	now := time.Now()
	rule := alerting.SLOAlertRule{
		RuleName: "ingestion-slo",
		SLO:      successRateSLO(),
		Severity: alerting.SeverityCritical,
	}

	// 10 observations, 5 failures: a 50% success rate, well below the
	// 99% target.
	observations := make([]reliability.Observation, 0, 10)
	for i := 0; i < 10; i++ {
		observations = append(observations, reliability.Observation{
			Success: i%2 == 0,
			At:      now.Add(-time.Duration(i) * time.Minute),
		})
	}

	event, fired, err := alerting.EvaluateSLOAlert(tenantID, rule, observations, now)
	if err != nil {
		t.Fatalf("EvaluateSLOAlert: %v", err)
	}
	if !fired {
		t.Fatal("EvaluateSLOAlert did not fire for a breached SLO")
	}
	if event.Severity != alerting.SeverityCritical {
		t.Errorf("event.Severity = %v, want SeverityCritical", event.Severity)
	}
	if event.ConditionKind != alerting.ConditionSLOBreached {
		t.Errorf("event.ConditionKind = %v, want ConditionSLOBreached", event.ConditionKind)
	}
	if event.TenantID != tenantID {
		t.Errorf("event.TenantID = %v, want %v", event.TenantID, tenantID)
	}
}

func TestEvaluateSLOAlert_DoesNotFireWhenMet(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	now := time.Now()
	rule := alerting.SLOAlertRule{
		RuleName: "ingestion-slo",
		SLO:      successRateSLO(),
		Severity: alerting.SeverityCritical,
	}

	// 100 observations, all successful: 100% success rate, comfortably
	// above the 99% target and its error budget.
	observations := make([]reliability.Observation, 0, 100)
	for i := 0; i < 100; i++ {
		observations = append(observations, reliability.Observation{
			Success: true,
			At:      now.Add(-time.Duration(i) * time.Minute),
		})
	}

	_, fired, err := alerting.EvaluateSLOAlert(tenantID, rule, observations, now)
	if err != nil {
		t.Fatalf("EvaluateSLOAlert: %v", err)
	}
	if fired {
		t.Fatal("EvaluateSLOAlert fired for a fully healthy SLO")
	}
}

func TestEvaluateSLOAlert_FiresOnErrorBudgetExhaustion(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	now := time.Now()
	rule := alerting.SLOAlertRule{
		RuleName:     "ingestion-slo-budget",
		SLO:          successRateSLO(), // target 0.99, allowed failure rate 0.01
		BudgetPolicy: reliability.ErrorBudgetPolicy{ExhaustionThreshold: 1.0},
		Severity:     alerting.SeverityWarning,
	}

	// 1000 observations, 15 failures: observed success rate is 0.985,
	// which is >= 0.99 minus a hair... construct precisely: target
	// 0.99 permits 1% failures. Use 100 obs, 2 failures -> success
	// rate 0.98, which is BELOW target (Met=false), not a budget-only
	// case. To hit "Met but budget exhausted" we need Observed >=
	// Target while ObservedFailureRate/AllowedFailureRate >= 1.
	// Since Met = (rate >= Target), and consumed = (1-rate)/(1-Target),
	// consumed >= 1 implies rate <= Target, which almost always means
	// Met is also false at exactly the boundary (rate == Target is
	// Met=true AND consumed==1, exhausted). Use exactly the boundary.
	total := 1000
	failures := 10 // exactly 1% => rate == 0.99 == Target: Met=true, ConsumedFraction==1.0
	observations := make([]reliability.Observation, 0, total)
	for i := 0; i < total; i++ {
		observations = append(observations, reliability.Observation{
			Success: i >= failures,
			At:      now.Add(-time.Duration(i) * time.Second),
		})
	}

	event, fired, err := alerting.EvaluateSLOAlert(tenantID, rule, observations, now)
	if err != nil {
		t.Fatalf("EvaluateSLOAlert: %v", err)
	}
	if !fired {
		t.Fatal("EvaluateSLOAlert did not fire when the error budget is exactly exhausted at the SLO boundary")
	}
	if event.ConditionKind != alerting.ConditionSLOBreached {
		t.Errorf("event.ConditionKind = %v, want ConditionSLOBreached", event.ConditionKind)
	}
}

func TestEvaluateSLOAlert_LatencySLO_NoBudgetCheck(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	now := time.Now()
	rule := alerting.SLOAlertRule{
		RuleName: "latency-slo",
		SLO: reliability.SLO{
			Name:   "hybrid-retrieval-latency",
			Kind:   reliability.SLOKindLatency,
			Target: float64(200 * time.Millisecond),
			Window: time.Hour,
		},
		Severity: alerting.SeverityWarning,
	}

	// All observations well within budget for a latency SLO.
	observations := []reliability.Observation{
		{Success: true, Latency: 50 * time.Millisecond, At: now},
		{Success: true, Latency: 60 * time.Millisecond, At: now},
	}

	_, fired, err := alerting.EvaluateSLOAlert(tenantID, rule, observations, now)
	if err != nil {
		t.Fatalf("EvaluateSLOAlert: %v", err)
	}
	if fired {
		t.Fatal("EvaluateSLOAlert fired for a healthy latency SLO (no error-budget concept applies to latency)")
	}
}

func TestEvaluateSLOAlert_InvalidRule(t *testing.T) {
	t.Parallel()
	_, _, err := alerting.EvaluateSLOAlert(uuid.New(), alerting.SLOAlertRule{}, nil, time.Now())
	if !errors.Is(err, alerting.ErrInvalidRule) {
		t.Fatalf("EvaluateSLOAlert with empty rule error = %v, want ErrInvalidRule", err)
	}
}
