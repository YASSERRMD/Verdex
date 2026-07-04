# Runbook: cost/usage budget exceeded

This is the human-readable counterpart to `Runbook`/`RunbookStep`
(`runbook.go`) and `costBudgetExceededRunbook()`'s structured
procedure -- the two are kept in the same order deliberately, so this
document and the data model never drift apart silently. If you change
one, change the other in the same commit.

Attach this runbook to an `AlertRule` by setting
`AlertRule.RunbookName = "cost-budget-exceeded"`.

## Scope

This runbook covers an `AlertEvent` with
`ConditionKind == ConditionCostThreshold`, produced by
`EvaluateCostAlert` (`cost_alert.go`) when
`packages/accounting.BudgetChecker.Check` (Phase 017) reports either:

- a soft warning (`alert == true`, usage crossed the tenant's
  configured `AlertThresholdPct` without a hard stop), or
- a hard-stop breach (`allowed == false`, the tenant's
  `BudgetConfig.HardStop` is true and a limit was fully exceeded).

`EvaluateCostAlert` assigns `AlertEvent.Severity` from
`CostAlertRule.Severity` for the soft-warning case and
`CostAlertRule.exceededSeverity()` (defaulting to `SeverityCritical`)
for the hard-stop case -- check `event.Severity` to know which
situation you are in before reading further.

## Prerequisites

- The on-call engineer has access to the tenant's current
  `accounting.TokenUsage` (daily/monthly tokens and cost) --
  `AlertEvent.Detail` already carries this.
- The tenant-administrator liaison is reachable, since any limit change
  affects billing and requires the tenant's own sign-off.

## Procedure

1. **Acknowledge the alert and note whether it is a soft warning
   (threshold crossed) or a hard-stop (budget exceeded, `HardStop`
   true).**
   Owner: on-call engineer.
   `event.Severity` distinguishes these: `SeverityWarning` (or
   whichever `CostAlertRule.Severity` was configured) for a soft
   warning, `SeverityCritical` (or `CostAlertRule.ExceededSeverity`, if
   set) for a hard-stop.

2. **Pull the tenant's current `accounting.TokenUsage` (daily/monthly
   tokens and cost) from the `AlertEvent.Detail`.**
   Owner: on-call engineer.
   The detail string already carries
   `daily_tokens=%d monthly_tokens=%d daily_cost_usd=%.4f monthly_cost_usd=%.4f`.

3. **Identify the driving case/task via `packages/accounting`'s
   per-case `UsageRecord` query, if the spike is unusually
   concentrated.**
   Owner: on-call engineer.
   A single case or batch job driving most of the usage points to a
   runaway retry loop or an unusually large document set, not
   organic growth.

4. **If the usage is legitimate, raise the tenant's `BudgetConfig`
   limit with tenant-administrator sign-off; if not, investigate for a
   runaway retry loop or misconfigured batch job.**
   Owner: tenant administrator liaison.
   Never silently raise a limit without the tenant's own
   administrator confirming the spend is expected -- this affects
   their bill.

5. **If `HardStop` blocked live traffic, confirm with the tenant before
   any temporary limit increase, since this affects billing.**
   Owner: incident commander.
   A hard-stop is deliberately blocking; lifting it even temporarily is
   a billing-affecting decision that should not be made unilaterally
   under incident pressure.

## Related seeded `AlertRule`

A tenant seeding a starter cost-alert rule typically registers one
`AlertRule` per budget scope it tracks (e.g. per-tenant daily/monthly),
with `RunbookName = "cost-budget-exceeded"`, `Severity = SeverityWarning`
for the soft-threshold case, and lets `CostAlertRule.ExceededSeverity`
default to `SeverityCritical` for the hard-stop case.
