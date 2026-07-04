// Package alerting's cost_alert.go implements task 5, cost and usage
// alerts, by composing directly with packages/accounting's existing
// BudgetChecker/BudgetConfig/TokenUsage (Phase 017) rather than
// redefining a second budget-tracking concept. This file never
// accumulates daily or monthly token/cost totals itself --
// accounting.InMemoryBudgetChecker (or any other BudgetChecker
// implementation) remains the only place that happens; this file only
// calls BudgetChecker.Check and translates its allowed/alert/err
// result into this package's own AlertEvent.
package alerting

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/accounting"
)

// CostAlertRule wraps an accounting.BudgetChecker plus this package's
// own Severity/RunbookName, so a cost/usage-budget rule can be
// catalogued alongside every other alert kind.
type CostAlertRule struct {
	// RuleName identifies this rule.
	RuleName string

	// Checker evaluates whether a tenant is within budget. Required.
	Checker accounting.BudgetChecker

	// Severity is the AlertEvent.Severity assigned for a soft
	// (warning-level) alert -- current usage crossed the configured
	// alert threshold but has not hard-exceeded a limit.
	Severity Severity

	// ExceededSeverity is the AlertEvent.Severity assigned when
	// accounting reports the budget as fully exceeded. Falls back to
	// SeverityCritical when left at the zero value, since a fully
	// exceeded budget is a more urgent signal than a threshold
	// warning.
	ExceededSeverity Severity

	// RunbookName, if non-empty, names the Runbook a responder should
	// follow for this rule.
	RunbookName string
}

// Validate checks r for structural well-formedness.
func (r CostAlertRule) Validate() error {
	if r.RuleName == "" {
		return wrapf("CostAlertRule.Validate", ErrInvalidRule)
	}
	if r.Checker == nil {
		return wrapf("CostAlertRule.Validate", ErrInvalidRule)
	}
	if !r.Severity.IsValid() {
		return wrapf("CostAlertRule.Validate", ErrInvalidSeverity)
	}
	return nil
}

func (r CostAlertRule) exceededSeverity() Severity {
	if r.ExceededSeverity.IsValid() {
		return r.ExceededSeverity
	}
	return SeverityCritical
}

// EvaluateCostAlert calls rule.Checker.Check(ctx, tenantID, usage) --
// the exact same evaluation packages/accounting's own
// AccountingService consults before allowing an LLM call through --
// and converts its result into this package's own AlertEvent:
//
//   - allowed=false (hard-stop exceeded): fires with
//     rule.exceededSeverity() and ConditionCostThreshold.
//   - allowed=true, alert=true (soft warning, e.g. threshold crossed
//     without HardStop, or exceeded without HardStop): fires with
//     rule.Severity.
//   - allowed=true, alert=false: does not fire.
//
// Real composition: this function never re-derives a percentage of a
// daily/monthly limit itself, it only calls into
// packages/accounting.BudgetChecker.Check and translates the result.
//
// Returns (AlertEvent{}, false, nil) when Check reports no alert
// needed -- not firing is not an error, even though Check's own
// ErrBudgetExceeded is surfaced as ok=true (Check's hard-stop error is
// domain signal, not an EvaluateCostAlert failure).
func EvaluateCostAlert(ctx context.Context, tenantID uuid.UUID, rule CostAlertRule, usage accounting.TokenUsage, now time.Time) (AlertEvent, bool, error) {
	if err := rule.Validate(); err != nil {
		return AlertEvent{}, false, wrapf("EvaluateCostAlert", err)
	}

	allowed, alertNeeded, checkErr := rule.Checker.Check(ctx, tenantID, usage)

	switch {
	case !allowed:
		return newCostExceededEvent(tenantID, rule, usage, checkErr, now), true, nil
	case alertNeeded:
		return newCostWarningEvent(tenantID, rule, usage, now), true, nil
	default:
		return AlertEvent{}, false, nil
	}
}

func newCostWarningEvent(tenantID uuid.UUID, rule CostAlertRule, usage accounting.TokenUsage, now time.Time) AlertEvent {
	detail := fmt.Sprintf(
		"budget warning: daily_tokens=%d monthly_tokens=%d daily_cost_usd=%.4f monthly_cost_usd=%.4f",
		usage.DailyTokens, usage.MonthlyTokens, usage.DailyCostUSD, usage.MonthlyCostUSD,
	)
	return AlertEvent{
		ID:            uuid.New(),
		TenantID:      tenantID,
		RuleName:      rule.RuleName,
		Severity:      rule.Severity,
		ConditionKind: ConditionCostThreshold,
		TriggerValue:  float64(usage.MonthlyTokens),
		Detail:        detail,
		CreatedAt:     now,
	}
}

func newCostExceededEvent(tenantID uuid.UUID, rule CostAlertRule, usage accounting.TokenUsage, checkErr error, now time.Time) AlertEvent {
	detail := fmt.Sprintf(
		"budget exceeded: daily_tokens=%d monthly_tokens=%d daily_cost_usd=%.4f monthly_cost_usd=%.4f",
		usage.DailyTokens, usage.MonthlyTokens, usage.DailyCostUSD, usage.MonthlyCostUSD,
	)
	if checkErr != nil {
		detail = fmt.Sprintf("%s error=%s", detail, checkErr.Error())
	}
	return AlertEvent{
		ID:            uuid.New(),
		TenantID:      tenantID,
		RuleName:      rule.RuleName,
		Severity:      rule.exceededSeverity(),
		ConditionKind: ConditionCostThreshold,
		TriggerValue:  float64(usage.MonthlyTokens),
		Detail:        detail,
		CreatedAt:     now,
	}
}
