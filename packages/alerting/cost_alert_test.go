package alerting_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/accounting"
	"github.com/YASSERRMD/verdex/packages/alerting"
)

func intPtr(v int) *int           { return &v }
func floatPtr(v float64) *float64 { return &v }

func TestEvaluateCostAlert_FiresWarningOnThresholdCrossing(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	now := time.Now()

	checker := accounting.NewInMemoryBudgetChecker([]accounting.BudgetConfig{
		{
			TenantID:          tenantID,
			DailyTokenLimit:   intPtr(1000),
			AlertThresholdPct: 80,
			HardStop:          false,
		},
	})
	rule := alerting.CostAlertRule{
		RuleName: "daily-token-budget",
		Checker:  checker,
		Severity: alerting.SeverityWarning,
	}

	// 850 of 1000 daily tokens = 85%, above the 80% alert threshold but
	// below 100% (not exceeded).
	usage := accounting.TokenUsage{DailyTokens: 850}

	event, fired, err := alerting.EvaluateCostAlert(t.Context(), tenantID, rule, usage, now)
	if err != nil {
		t.Fatalf("EvaluateCostAlert: %v", err)
	}
	if !fired {
		t.Fatal("EvaluateCostAlert did not fire when usage crossed the alert threshold")
	}
	if event.Severity != alerting.SeverityWarning {
		t.Errorf("event.Severity = %v, want SeverityWarning", event.Severity)
	}
	if event.ConditionKind != alerting.ConditionCostThreshold {
		t.Errorf("event.ConditionKind = %v, want ConditionCostThreshold", event.ConditionKind)
	}
}

func TestEvaluateCostAlert_FiresCriticalOnHardStopExceeded(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	now := time.Now()

	checker := accounting.NewInMemoryBudgetChecker([]accounting.BudgetConfig{
		{
			TenantID:          tenantID,
			DailyTokenLimit:   intPtr(1000),
			AlertThresholdPct: 80,
			HardStop:          true,
		},
	})
	rule := alerting.CostAlertRule{
		RuleName: "daily-token-budget",
		Checker:  checker,
		Severity: alerting.SeverityWarning,
		// ExceededSeverity left at zero value -> defaults to Critical.
	}

	usage := accounting.TokenUsage{DailyTokens: 1200} // over the 1000 limit, HardStop true

	event, fired, err := alerting.EvaluateCostAlert(t.Context(), tenantID, rule, usage, now)
	if err != nil {
		t.Fatalf("EvaluateCostAlert: %v", err)
	}
	if !fired {
		t.Fatal("EvaluateCostAlert did not fire for a hard-stop-exceeded budget")
	}
	if event.Severity != alerting.SeverityCritical {
		t.Errorf("event.Severity = %v, want SeverityCritical (default ExceededSeverity)", event.Severity)
	}
}

func TestEvaluateCostAlert_DoesNotFireWithinBudget(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	now := time.Now()

	checker := accounting.NewInMemoryBudgetChecker([]accounting.BudgetConfig{
		{
			TenantID:          tenantID,
			DailyTokenLimit:   intPtr(1000),
			DailyCostLimitUSD: floatPtr(10),
			AlertThresholdPct: 80,
		},
	})
	rule := alerting.CostAlertRule{
		RuleName: "daily-token-budget",
		Checker:  checker,
		Severity: alerting.SeverityWarning,
	}

	usage := accounting.TokenUsage{DailyTokens: 100, DailyCostUSD: 1} // well within budget

	_, fired, err := alerting.EvaluateCostAlert(t.Context(), tenantID, rule, usage, now)
	if err != nil {
		t.Fatalf("EvaluateCostAlert: %v", err)
	}
	if fired {
		t.Fatal("EvaluateCostAlert fired for usage well within budget")
	}
}

func TestEvaluateCostAlert_NoConfigNeverFires(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	now := time.Now()

	// No BudgetConfig registered for this tenant at all.
	checker := accounting.NewInMemoryBudgetChecker(nil)
	rule := alerting.CostAlertRule{
		RuleName: "daily-token-budget",
		Checker:  checker,
		Severity: alerting.SeverityWarning,
	}

	_, fired, err := alerting.EvaluateCostAlert(t.Context(), tenantID, rule, accounting.TokenUsage{DailyTokens: 999999}, now)
	if err != nil {
		t.Fatalf("EvaluateCostAlert: %v", err)
	}
	if fired {
		t.Fatal("EvaluateCostAlert fired for a tenant with no configured budget")
	}
}

func TestEvaluateCostAlert_InvalidRule(t *testing.T) {
	t.Parallel()
	_, _, err := alerting.EvaluateCostAlert(t.Context(), uuid.New(), alerting.CostAlertRule{}, accounting.TokenUsage{}, time.Now())
	if !errors.Is(err, alerting.ErrInvalidRule) {
		t.Fatalf("EvaluateCostAlert with nil Checker error = %v, want ErrInvalidRule", err)
	}
}

// fakeBudgetChecker is a minimal accounting.BudgetChecker fake for
// testing EvaluateCostAlert's translation logic independent of
// accounting's own threshold percentage math.
type fakeBudgetChecker struct {
	allowed bool
	alert   bool
	err     error
}

func (f fakeBudgetChecker) Check(_ context.Context, _ uuid.UUID, _ accounting.TokenUsage) (bool, bool, error) {
	return f.allowed, f.alert, f.err
}

func TestEvaluateCostAlert_WithFakeChecker(t *testing.T) {
	t.Parallel()

	rule := alerting.CostAlertRule{
		RuleName: "fake-budget",
		Checker:  fakeBudgetChecker{allowed: false, alert: true, err: accounting.ErrBudgetExceeded},
		Severity: alerting.SeverityWarning,
	}
	event, fired, err := alerting.EvaluateCostAlert(t.Context(), uuid.New(), rule, accounting.TokenUsage{}, time.Now())
	if err != nil {
		t.Fatalf("EvaluateCostAlert: %v", err)
	}
	if !fired {
		t.Fatal("EvaluateCostAlert did not fire when Checker reports allowed=false")
	}
	if event.Severity != alerting.SeverityCritical {
		t.Errorf("event.Severity = %v, want SeverityCritical", event.Severity)
	}
}
