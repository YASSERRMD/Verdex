package alerting

import "strings"

// RunbookStep is a single ordered remediation step within a Runbook:
// what to do, and who (by role) owns doing it. Structurally identical
// to packages/backupdr.RunbookStep (Phase 085) -- see doc.go for why
// this package defines its own type rather than importing backupdr's.
type RunbookStep struct {
	// Order is this step's 1-based position within the Runbook. Steps
	// are executed in ascending Order.
	Order int `json:"order"`

	// Description says what to do at this step.
	Description string `json:"description"`

	// OwnerRole names the role responsible for executing this step
	// (e.g. "on-call engineer", "incident commander") -- a free-form
	// label, not an identity.Role constant, mirroring
	// packages/backupdr.RunbookStep.OwnerRole exactly: a remediation
	// step's owner is an operational responsibility, not necessarily a
	// role this platform's RBAC model grants permissions to.
	OwnerRole string `json:"owner_role"`
}

// Validate checks s for structural well-formedness.
func (s RunbookStep) Validate() error {
	if s.Order <= 0 {
		return wrapf("RunbookStep.Validate", ErrInvalidRunbook)
	}
	if strings.TrimSpace(s.Description) == "" {
		return wrapf("RunbookStep.Validate", ErrInvalidRunbook)
	}
	if strings.TrimSpace(s.OwnerRole) == "" {
		return wrapf("RunbookStep.Validate", ErrInvalidRunbook)
	}
	return nil
}

// Runbook is a structured, ordered remediation procedure (task 7)
// attached to an AlertRule by name (AlertRule.RunbookName): the
// data-model counterpart to a human-readable doc/runbooks/*.md
// artifact. Name identifies which alert scenario this Runbook
// addresses (e.g. "ingestion-slo-breach", "reasoning-quality-
// regression"); Steps is the ordered procedure, each with a
// Description and an OwnerRole.
type Runbook struct {
	// Name identifies the alert scenario this Runbook addresses.
	// Should match an AlertRule.RunbookName for the rules it applies
	// to.
	Name string `json:"name"`

	// Steps is the ordered procedure. Validate requires at least one
	// step and requires Order values to be unique and to appear in
	// ascending order.
	Steps []RunbookStep `json:"steps"`
}

// Validate checks r for structural well-formedness: a non-blank Name,
// at least one step, and Steps sorted by strictly ascending, unique
// Order values.
func (r Runbook) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return wrapf("Runbook.Validate", ErrInvalidRunbook)
	}
	if len(r.Steps) == 0 {
		return wrapf("Runbook.Validate", ErrInvalidRunbook)
	}
	prevOrder := 0
	for _, s := range r.Steps {
		if err := s.Validate(); err != nil {
			return err
		}
		if s.Order <= prevOrder {
			return wrapf("Runbook.Validate", ErrInvalidRunbook)
		}
		prevOrder = s.Order
	}
	return nil
}

// StepCount returns len(r.Steps), a small convenience for callers (a
// dashboard, a doc generator) that just want the procedure's length.
func (r Runbook) StepCount() int {
	return len(r.Steps)
}

// OwnerRoles returns the distinct set of OwnerRole values across every
// step, in first-seen order -- who needs to be involved for this
// Runbook to be executable end to end.
func (r Runbook) OwnerRoles() []string {
	seen := make(map[string]struct{}, len(r.Steps))
	out := make([]string, 0, len(r.Steps))
	for _, s := range r.Steps {
		if _, ok := seen[s.OwnerRole]; ok {
			continue
		}
		seen[s.OwnerRole] = struct{}{}
		out = append(out, s.OwnerRole)
	}
	return out
}

// SeededRunbooks returns this package's starter runbooks (task 7): the
// structured counterpart to doc/runbooks/*.md, kept in the same order
// as those documents' numbered procedures so the two never drift apart
// silently. Keyed by Runbook.Name for direct lookup by an
// AlertRule.RunbookName value.
func SeededRunbooks() map[string]Runbook {
	runbooks := []Runbook{
		sloBreachRunbook(),
		qualityRegressionRunbook(),
		costBudgetExceededRunbook(),
	}
	out := make(map[string]Runbook, len(runbooks))
	for _, rb := range runbooks {
		out[rb.Name] = rb
	}
	return out
}

// sloBreachRunbook is the structured counterpart to
// doc/runbooks/slo-breach.md.
func sloBreachRunbook() Runbook {
	return Runbook{
		Name: "slo-breach",
		Steps: []RunbookStep{
			{Order: 1, Description: "Acknowledge the page and open the flow's DashboardDefinition to confirm which SLO breached.", OwnerRole: "on-call engineer"},
			{Order: 2, Description: "Check reliability.ErrorBudget.ConsumedFraction to gauge severity: below 1.0 is a developing risk, at/above 1.0 means the objective is currently violated.", OwnerRole: "on-call engineer"},
			{Order: 3, Description: "Inspect recent deploys and traffic-shifting decisions (packages/reliability.TrafficShifter) for a correlated change.", OwnerRole: "on-call engineer"},
			{Order: 4, Description: "If a recent deploy correlates, roll it back; if BlockRiskyDeploys is true, pause further non-hotfix deploys until the budget recovers.", OwnerRole: "incident commander"},
			{Order: 5, Description: "If no deploy correlates, check dependency health via the affected service's readiness endpoint and packages/reliability.CircuitBreaker state.", OwnerRole: "on-call engineer"},
			{Order: 6, Description: "Once mitigated, continue monitoring the rolling window until reliability.EvaluateSLO reports Met again.", OwnerRole: "on-call engineer"},
			{Order: 7, Description: "File a follow-up postmortem note if the error budget was fully exhausted (ConsumedFraction >= 1.0).", OwnerRole: "incident commander"},
		},
	}
}

// qualityRegressionRunbook is the structured counterpart to
// doc/runbooks/quality-regression.md.
func qualityRegressionRunbook() Runbook {
	return Runbook{
		Name: "quality-regression",
		Steps: []RunbookStep{
			{Order: 1, Description: "Acknowledge the alert and note the AlertEvent.Detail's baseline vs current run IDs and average scores.", OwnerRole: "on-call engineer"},
			{Order: 2, Description: "Pull the full reasoningeval.RegressionResult.PerDimensionDrop to identify which quality dimension drove the regression.", OwnerRole: "reasoning-quality reviewer"},
			{Order: 3, Description: "Check whether the current run followed a prompt-template, model, or provider change; if so, that is the prime suspect.", OwnerRole: "reasoning-quality reviewer"},
			{Order: 4, Description: "If a recent change is implicated, roll it back or gate it behind a flag until the regression is understood.", OwnerRole: "incident commander"},
			{Order: 5, Description: "Re-run the comparison against a fresh sample once a fix is in place to confirm Regressed no longer holds.", OwnerRole: "reasoning-quality reviewer"},
			{Order: 6, Description: "Remember every quality signal here is a non-binding monitoring artifact -- never treat it as a conclusion about any specific case's merits.", OwnerRole: "reasoning-quality reviewer"},
		},
	}
}

// costBudgetExceededRunbook is the structured counterpart to
// doc/runbooks/cost-budget-exceeded.md.
func costBudgetExceededRunbook() Runbook {
	return Runbook{
		Name: "cost-budget-exceeded",
		Steps: []RunbookStep{
			{Order: 1, Description: "Acknowledge the alert and note whether it is a soft warning (threshold crossed) or a hard-stop (budget exceeded, HardStop true).", OwnerRole: "on-call engineer"},
			{Order: 2, Description: "Pull the tenant's current accounting.TokenUsage (daily/monthly tokens and cost) from the AlertEvent.Detail.", OwnerRole: "on-call engineer"},
			{Order: 3, Description: "Identify the driving case/task via accounting's per-case UsageRecord query, if the spike is unusually concentrated.", OwnerRole: "on-call engineer"},
			{Order: 4, Description: "If the usage is legitimate, raise the tenant's BudgetConfig limit with tenant-administrator sign-off; if not, investigate for a runaway retry loop or misconfigured batch job.", OwnerRole: "tenant administrator liaison"},
			{Order: 5, Description: "If HardStop blocked live traffic, confirm with the tenant before any temporary limit increase, since this affects billing.", OwnerRole: "incident commander"},
		},
	}
}
