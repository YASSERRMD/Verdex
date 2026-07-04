# Runbook: SLO breach

This is the human-readable counterpart to `Runbook`/`RunbookStep`
(`runbook.go`) and `sloBreachRunbook()`'s structured procedure -- the
two are kept in the same order deliberately, so this document and the
data model never drift apart silently. If you change one, change the
other in the same commit.

Attach this runbook to an `AlertRule` by setting
`AlertRule.RunbookName = "slo-breach"`.

## Scope

This runbook covers an `AlertEvent` with
`ConditionKind == ConditionSLOBreached`, produced by
`EvaluateSLOAlert` (`slo_alert.go`) when either:

- a `reliability.SLO` is currently unmet (`SLOStatus.Met == false`), or
- a success-rate SLO's `reliability.ErrorBudget` has been exhausted per
  the wrapped `ErrorBudgetPolicy`.

Both cases compose directly with `packages/reliability` (Phase 093);
this runbook does not re-derive success rates or latency percentiles
itself -- it tells a responder where to look.

## Prerequisites

- The on-call engineer has access to the flow's `DashboardDefinition`
  (`dashboard.go`) to see the affected metric alongside related ones.
- The `AlertEvent.Detail` field already names the observed value,
  target, and (for a budget exhaustion) the consumed fraction -- read
  it before doing anything else.

## Procedure

1. **Acknowledge the page and open the flow's `DashboardDefinition` to
   confirm which SLO breached.**
   Owner: on-call engineer.
   `AlertEvent.RuleName` and `Detail` name the specific
   `reliability.SLO.Name` and observed/target values.

2. **Check `reliability.ErrorBudget.ConsumedFraction` to gauge
   severity.**
   Owner: on-call engineer.
   Below 1.0 is a developing risk (the SLO itself may still be met);
   at or above 1.0 means the objective is currently violated, or the
   allowed failure margin is fully spent.

3. **Inspect recent deploys and traffic-shifting decisions
   (`packages/reliability.TrafficShifter`) for a correlated change.**
   Owner: on-call engineer.
   Most SLO breaches correlate with a recent deploy, a traffic shift
   away from a degraded backend, or a dependency's own incident.

4. **If a recent deploy correlates, roll it back; if
   `PolicyResult.BlockRiskyDeploys` is true, pause further non-hotfix
   deploys until the budget recovers.**
   Owner: incident commander.
   `ErrorBudgetPolicy.Evaluate`'s `BlockRiskyDeploys` signal exists
   exactly for this decision -- this package computes the signal, the
   incident commander (and this deployment's release process) act on
   it.

5. **If no deploy correlates, check dependency health via the affected
   service's readiness endpoint and `packages/reliability.CircuitBreaker`
   state.**
   Owner: on-call engineer.
   A downstream dependency's own degradation is the next most common
   cause; `packages/reliability`'s circuit breaker and degradation
   machinery (Phase 093) are the first place to look.

6. **Once mitigated, continue monitoring the rolling window until
   `reliability.EvaluateSLO` reports `Met` again.**
   Owner: on-call engineer.
   An SLO evaluated over a rolling window does not recover
   instantaneously even after the underlying cause is fixed -- older,
   still-in-window failures keep the observed rate depressed until
   they age out.

7. **File a follow-up postmortem note if the error budget was fully
   exhausted (`ConsumedFraction >= 1.0`).**
   Owner: incident commander.
   A fully exhausted budget is worth a retrospective even after
   mitigation, per this deployment's incident-review process.

## Related seeded `AlertRule`

A tenant seeding a starter SLO-based rule set typically registers one
`AlertRule` per `reliability.SLO` it defines (e.g.
`"ingestion-availability"`, `"reasoning-orchestration-latency"`), each
with `RunbookName = "slo-breach"` and a `Severity` matching how urgently
that particular flow's breach needs a human response.
