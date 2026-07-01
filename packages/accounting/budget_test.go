package accounting_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/accounting"
)

func intPtr(v int) *int       { return &v }
func f64Ptr(v float64) *float64 { return &v }

// TestHardStop_TokenLimit verifies that Check returns ErrBudgetExceeded when
// the daily token limit is breached with HardStop=true.
func TestHardStop_TokenLimit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tenantID := uuid.New()

	cfg := accounting.BudgetConfig{
		TenantID:          tenantID,
		DailyTokenLimit:   intPtr(1000),
		HardStop:          true,
		AlertThresholdPct: 90,
	}
	checker := accounting.NewInMemoryBudgetChecker([]accounting.BudgetConfig{cfg})

	// Under limit: 800 tokens → allowed, no alert yet.
	usage := accounting.TokenUsage{DailyTokens: 800}
	allowed, alert, err := checker.Check(ctx, tenantID, usage)
	if !allowed {
		t.Errorf("expected allowed=true for 800/1000, got false")
	}
	if alert {
		t.Errorf("expected no alert at 80%%, got alert=true")
	}
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// At threshold: 910 tokens → 91% → alert, but still allowed.
	usage.DailyTokens = 910
	allowed, alert, err = checker.Check(ctx, tenantID, usage)
	if !allowed {
		t.Errorf("expected allowed=true at 91%% without hard-stop breach")
	}
	if !alert {
		t.Errorf("expected alert=true at 91%%")
	}
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Over limit: 1100 tokens → ErrBudgetExceeded.
	usage.DailyTokens = 1100
	allowed, alert, err = checker.Check(ctx, tenantID, usage)
	if allowed {
		t.Errorf("expected allowed=false when over hard limit")
	}
	if err == nil {
		t.Fatal("expected ErrBudgetExceeded, got nil")
	}
	_ = alert // may be true or false, not tested here
}

// TestSoftAlert_NoHardStop verifies that exceeding a limit with HardStop=false
// fires an alert but does not block the request.
func TestSoftAlert_NoHardStop(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tenantID := uuid.New()

	cfg := accounting.BudgetConfig{
		TenantID:          tenantID,
		DailyTokenLimit:   intPtr(1000),
		HardStop:          false, // soft mode
		AlertThresholdPct: 80,
	}
	checker := accounting.NewInMemoryBudgetChecker([]accounting.BudgetConfig{cfg})

	// Over limit but HardStop=false → allowed=true, alert=true.
	usage := accounting.TokenUsage{DailyTokens: 1200}
	allowed, alert, err := checker.Check(ctx, tenantID, usage)
	if !allowed {
		t.Errorf("expected allowed=true in soft mode, got false")
	}
	if !alert {
		t.Errorf("expected alert=true when over limit")
	}
	if err != nil {
		t.Errorf("unexpected error in soft mode: %v", err)
	}
}

// TestCostLimit_HardStop verifies that the daily cost limit triggers a
// hard stop when HardStop=true.
func TestCostLimit_HardStop(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tenantID := uuid.New()

	cfg := accounting.BudgetConfig{
		TenantID:          tenantID,
		DailyCostLimitUSD: f64Ptr(1.00),
		HardStop:          true,
		AlertThresholdPct: 80,
	}
	checker := accounting.NewInMemoryBudgetChecker([]accounting.BudgetConfig{cfg})

	// Under cost limit.
	usage := accounting.TokenUsage{DailyCostUSD: 0.50}
	allowed, _, err := checker.Check(ctx, tenantID, usage)
	if !allowed || err != nil {
		t.Errorf("expected allowed for 0.50/1.00, got allowed=%v err=%v", allowed, err)
	}

	// At alert threshold (85% of $1.00).
	usage.DailyCostUSD = 0.85
	allowed, alert, err := checker.Check(ctx, tenantID, usage)
	if !allowed {
		t.Errorf("expected allowed=true at alert threshold")
	}
	if !alert {
		t.Errorf("expected alert=true at 85%% of cost limit")
	}
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Over cost limit.
	usage.DailyCostUSD = 1.50
	allowed, _, err = checker.Check(ctx, tenantID, usage)
	if allowed {
		t.Errorf("expected allowed=false over cost hard limit")
	}
	if err == nil {
		t.Fatal("expected ErrBudgetExceeded for cost overage")
	}
}

// TestMonthlyLimit_HardStop verifies that the monthly token limit triggers a
// hard stop.
func TestMonthlyLimit_HardStop(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tenantID := uuid.New()

	cfg := accounting.BudgetConfig{
		TenantID:          tenantID,
		MonthlyTokenLimit: intPtr(50_000),
		HardStop:          true,
		AlertThresholdPct: 90,
	}
	checker := accounting.NewInMemoryBudgetChecker([]accounting.BudgetConfig{cfg})

	// Under limit.
	usage := accounting.TokenUsage{MonthlyTokens: 40_000}
	allowed, alert, err := checker.Check(ctx, tenantID, usage)
	if !allowed || alert || err != nil {
		t.Errorf("expected allowed, no alert, no err: got %v %v %v", allowed, alert, err)
	}

	// At 95% → alert.
	usage.MonthlyTokens = 47_500
	allowed, alert, err = checker.Check(ctx, tenantID, usage)
	if !allowed {
		t.Errorf("expected allowed at 95%%")
	}
	if !alert {
		t.Errorf("expected alert at 95%%")
	}
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Over limit.
	usage.MonthlyTokens = 51_000
	allowed, _, err = checker.Check(ctx, tenantID, usage)
	if allowed {
		t.Errorf("expected blocked over monthly limit")
	}
	if err == nil {
		t.Fatal("expected ErrBudgetExceeded")
	}
}

// TestNoBudgetConfig_AlwaysAllowed verifies that tenants without a budget
// config are always allowed through.
func TestNoBudgetConfig_AlwaysAllowed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	checker := accounting.NewInMemoryBudgetChecker(nil)
	tenantID := uuid.New()

	usage := accounting.TokenUsage{DailyTokens: 999_999_999}
	allowed, alert, err := checker.Check(ctx, tenantID, usage)
	if !allowed {
		t.Errorf("expected allowed=true for unconfigured tenant")
	}
	if alert {
		t.Errorf("expected no alert for unconfigured tenant")
	}
	if err != nil {
		t.Errorf("expected no error for unconfigured tenant: %v", err)
	}
}
