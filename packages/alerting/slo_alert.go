// Package alerting's slo_alert.go implements task 3, SLO-based alerts,
// by composing directly with packages/reliability's SLO/EvaluateSLO/
// ComputeErrorBudget/ErrorBudgetPolicy (Phase 093) rather than
// redefining a second SLO or error-budget concept. Every number this
// file compares against a threshold comes from a real call into
// packages/reliability; this file only decides when that
// already-computed reliability signal should become this package's
// own AlertEvent.
package alerting

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/reliability"
)

// SLOAlertRule wraps a reliability.SLO plus the policy this package
// uses to decide when its breach/budget-exhaustion status should
// raise an AlertEvent. RuleName/Severity/RunbookName follow the same
// AlertRule shape every other alert kind in this package uses, so an
// SLO-based rule can be catalogued and displayed alongside plain
// threshold rules.
type SLOAlertRule struct {
	// RuleName identifies this rule (typically matching the wrapped
	// SLO.Name for easy cross-reference).
	RuleName string

	// SLO is the reliability.SLO this rule evaluates. Required.
	SLO reliability.SLO

	// BudgetPolicy determines when the SLO's computed ErrorBudget is
	// considered exhausted. A zero-value ErrorBudgetPolicy falls back
	// to reliability.DefaultExhaustionThreshold (see
	// reliability.ErrorBudgetPolicy.Evaluate).
	BudgetPolicy reliability.ErrorBudgetPolicy

	// Severity is the AlertEvent.Severity assigned when this rule
	// fires.
	Severity Severity

	// RunbookName, if non-empty, names the Runbook a responder should
	// follow for this rule.
	RunbookName string
}

// Validate checks r for structural well-formedness.
func (r SLOAlertRule) Validate() error {
	if r.RuleName == "" {
		return wrapf("SLOAlertRule.Validate", ErrInvalidRule)
	}
	if err := r.SLO.Validate(); err != nil {
		return wrapf("SLOAlertRule.Validate", err)
	}
	if !r.Severity.IsValid() {
		return wrapf("SLOAlertRule.Validate", ErrInvalidSeverity)
	}
	return nil
}

// EvaluateSLOAlert feeds observations through
// reliability.EvaluateSLO, and -- for a success-rate SLO -- further
// through reliability.ComputeErrorBudget and rule.BudgetPolicy.Evaluate,
// producing an AlertEvent when either the SLO itself is currently
// unmet (SLOStatus.Met == false) or its error budget has been
// exhausted per rule.BudgetPolicy. Real composition: this function
// never recomputes a success rate or a P95 latency itself, it only
// calls into packages/reliability and translates the result.
//
// Returns (AlertEvent{}, false, nil) when the SLO is currently met and
// (for a success-rate SLO) its budget is not exhausted -- not firing
// is not an error.
func EvaluateSLOAlert(tenantID uuid.UUID, rule SLOAlertRule, observations []reliability.Observation, now time.Time) (AlertEvent, bool, error) {
	if err := rule.Validate(); err != nil {
		return AlertEvent{}, false, wrapf("EvaluateSLOAlert", err)
	}

	status, err := reliability.EvaluateSLO(rule.SLO, observations, now)
	if err != nil {
		return AlertEvent{}, false, wrapf("EvaluateSLOAlert", err)
	}

	if !status.Met {
		return newSLOBreachEvent(tenantID, rule, status, now), true, nil
	}

	// A latency SLO has no error-budget analogue (see
	// reliability/doc.go); only a success-rate SLO's budget is
	// evaluated further.
	if rule.SLO.Kind != reliability.SLOKindSuccessRate {
		return AlertEvent{}, false, nil
	}

	budget, err := reliability.ComputeErrorBudget(status)
	if err != nil {
		return AlertEvent{}, false, wrapf("EvaluateSLOAlert", err)
	}

	policyResult, err := rule.BudgetPolicy.Evaluate(budget)
	if err != nil {
		return AlertEvent{}, false, wrapf("EvaluateSLOAlert", err)
	}
	if !policyResult.Exhausted {
		return AlertEvent{}, false, nil
	}

	return newErrorBudgetExhaustedEvent(tenantID, rule, policyResult, now), true, nil
}

func newSLOBreachEvent(tenantID uuid.UUID, rule SLOAlertRule, status reliability.SLOStatus, now time.Time) AlertEvent {
	detail := fmt.Sprintf(
		"SLO %q breached: observed %.4f vs target %.4f over %s (samples=%d)",
		status.SLO.Name, status.Observed, status.SLO.Target, status.SLO.Window, status.SampleCount,
	)
	return AlertEvent{
		ID:            uuid.New(),
		TenantID:      tenantID,
		RuleName:      rule.RuleName,
		Severity:      rule.Severity,
		ConditionKind: ConditionSLOBreached,
		TriggerValue:  status.Observed,
		Threshold:     status.SLO.Target,
		Detail:        detail,
		CreatedAt:     now,
	}
}

func newErrorBudgetExhaustedEvent(tenantID uuid.UUID, rule SLOAlertRule, result reliability.PolicyResult, now time.Time) AlertEvent {
	detail := fmt.Sprintf(
		"SLO %q error budget exhausted: consumed %.2f%% of allowed failure margin (block_risky_deploys=%t)",
		result.Budget.SLO.Name, result.Budget.ConsumedFraction*100, result.BlockRiskyDeploys,
	)
	return AlertEvent{
		ID:            uuid.New(),
		TenantID:      tenantID,
		RuleName:      rule.RuleName,
		Severity:      rule.Severity,
		ConditionKind: ConditionSLOBreached,
		TriggerValue:  result.Budget.ConsumedFraction,
		Threshold:     1.0,
		Detail:        detail,
		CreatedAt:     now,
	}
}
