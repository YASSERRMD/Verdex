# Monitoring & alerting (Phase 096)

This phase draws together five earlier threads -- the structured
logging, metrics registry, tracing, and health/readiness endpoints
already built in Phase 003 (`packages/observability`), the SLO/
error-budget evaluation added in Phase 093 (`packages/reliability`),
the reasoning-quality regression detector added in Phase 062
(`packages/reasoningeval`), the token/cost budget-threshold machinery
added in Phase 017 (`packages/accounting`), and the notification
delivery/inbox mechanism added in Phase 072 (`packages/notifications`)
-- into a single alert-rule-evaluation, on-call-escalation, and
runbook layer: `packages/alerting`.

## Goal

Give this platform production visibility and on-call alerting: a
catalogue of business-relevant metrics, dashboards for the platform's
key flows, alert rules that fire from SLO breaches, reasoning-quality
regressions, and cost/usage overages, an escalation policy that routes
a firing alert to the right responder over time, runbooks telling that
responder what to do, and synthetic monitoring that proactively probes
whether the platform is even reachable.

## What this package composes from, versus what is new

| Existing piece | What it already provides | What this phase adds |
|---|---|---|
| `packages/observability` (Phase 003) | `Registry`/`Counter`/`Gauge`/`Histogram`; `Checker`/`NamedChecker`/`ReadinessHandler` | `RegisterBusinessMetrics` registers a named catalogue of business metrics *through* that `Registry` -- no parallel metrics backend. `SyntheticCheck`'s `Prober` follows `Checker`'s exact convention by reference |
| `packages/reliability` (Phase 093) | `SLO`/`Observation`/`SLOStatus`/`EvaluateSLO`; `ErrorBudget`/`ComputeErrorBudget`/`ErrorBudgetPolicy` | `SLOAlertRule`/`EvaluateSLOAlert` calls directly into these, converting a breach or exhausted budget into an `AlertEvent` -- no second SLO/budget concept |
| `packages/reasoningeval` (Phase 062) | `RegressionDetector`/`RegressionResult`; `Alert`/`AlertSink` (its own quality-alert sink) | `QualityAlertRule`/`EvaluateQualityAlert` runs the same `Compare` call and converts a `Regressed` result into this package's `AlertEvent`, reusing `NewRegressionAlert`'s message text verbatim |
| `packages/accounting` (Phase 017) | `BudgetConfig`/`BudgetChecker`/`TokenUsage`; its own `AlertEvent`/`AlertSink` | `CostAlertRule`/`EvaluateCostAlert` calls `BudgetChecker.Check` and converts the allowed/alert result into this package's `AlertEvent` -- no second budget tracker |
| `packages/notifications` (Phase 072) | `Service.Notify`; existing `ReasoningEvalAlertSink`/`AccountingAlertSink` adapters | `NotificationRecipientSink` is the hand-off interface this package defines for a downstream notifier (e.g. `packages/notifications`) to implement -- `Route`/`RouteAndDeliver` only *decide* who owns an alert right now, delivery stays `notifications.Service`'s job |
| `packages/backupdr` (Phase 085, by reference only) | `Runbook`/`RunbookStep`'s `Order`/`Description`/`OwnerRole` shape | This package's own `Runbook`/`RunbookStep` follows the identical shape without importing `backupdr` |
| `packages/compliance` (Phase 082, by reference only) | `Dashboard`/`BuildDashboard` aggregation shape | `DashboardDefinition`/`BuildDashboard` follows the identical named-panel report shape without importing `compliance` |
| `packages/identity` (Phase 006) | `Role`/`Permission`/`PermissionMatrix`/`HasPermission` | `PermViewAlerting`/`PermManageAlerting`: the two fine-grained permissions this package's `Engine` gates every operation on |

## Business metrics (task 1)

`RegisterBusinessMetrics(registry observability.Registry)` (metrics.go)
registers a small, named catalogue of counters/gauges/histogram through
the caller's existing `observability.Registry`:

- `verdex_cases_ingested_total` (counter, by outcome)
- `verdex_opinions_signed_off_total` (counter, by disposition)
- `verdex_sar_requests_pending` (gauge, by tenant)
- `verdex_reasoning_runs_total` (counter, by outcome)
- `verdex_alerts_fired_total` (counter, by severity -- a meta-metric on
  this package's own output)
- `verdex_synthetic_check_latency_seconds` (histogram, by check and
  outcome)

This file never constructs a `*prometheus.Registry` or a competing
`Counter`/`Gauge` type -- every metric is a real handle obtained from
the caller's `Registry`, so a service that already exposes `/metrics`
picks these up automatically.

## Dashboards for key flows (task 2)

`DashboardDefinition`/`BuildDashboard(flowName, now)` (dashboard.go) is
a structured, named-panel data model -- not a UI -- for three named
key flows: `"ingestion"`, `"reasoning"`, and `"sign-off"`. Each `Panel`
names a `Catalogue` metric by string. `KnownFlows()` lists the
recognized flow names; `BuildDashboard` returns `ErrUnknownFlow` for
any other name. Mirrors `packages/compliance.Dashboard`/
`BuildDashboard`'s aggregation shape; this package does not import
`packages/compliance` or `packages/analytics`.

## Alert rules and the core evaluation mechanism

`AlertRule` (types.go) is a tenant-scoped, named rule: a `Condition`
(a `ConditionKind` plus a `MetricName`/`Threshold`) and a `Severity`.
Five `ConditionKind` values exist:

- `ConditionThresholdAbove`/`ConditionThresholdBelow`: evaluated by
  `Engine.Evaluate(ctx, tenantID, rule, currentValue)` (engine.go) --
  real threshold comparison, tested against both firing and
  non-firing inputs, including the exactly-at-threshold boundary case.
- `ConditionSLOBreached`, `ConditionQualityRegression`,
  `ConditionCostThreshold`: "externally evaluated" -- `Engine.Evaluate`
  itself rejects these (`ErrNilCondition`) since they are produced
  instead by the three dedicated functions below.

Every `ConditionKind` resolves to the same `AlertEvent` shape (task
3-5's shared mechanism) -- see "Alert vs AlertEvent" below for why this
package does not simply reuse `reasoningeval.Alert` or
`accounting.AlertEvent` directly.

## SLO-based alerts (task 3)

`SLOAlertRule`/`EvaluateSLOAlert` (slo_alert.go) wraps a
`reliability.SLO` plus a `reliability.ErrorBudgetPolicy`.
`EvaluateSLOAlert` feeds `Observation`s through
`reliability.EvaluateSLO`, and -- for a success-rate SLO -- further
through `reliability.ComputeErrorBudget` and the policy's `Evaluate`,
producing an `AlertEvent` when the SLO is currently unmet or its
error budget is exhausted. A latency SLO has no error-budget analogue
(per `packages/reliability`'s own doc.go) and is only checked against
`SLOStatus.Met`.

## Reasoning-quality alerts (task 4)

`QualityAlertRule`/`EvaluateQualityAlert` (quality_alert.go) wraps a
`reasoningeval.RegressionDetector`. `EvaluateQualityAlert` runs the
exact same `Compare(baseline, current)` call
`reasoningeval.QualityAlertChecker` itself performs, and -- only when
`Regressed` -- converts the resulting `RegressionResult` into this
package's `AlertEvent`, reusing `reasoningeval.NewRegressionAlert`'s
message text (including its non-binding-quality-signal suffix)
verbatim for `Detail`.

### Alert vs AlertEvent

`packages/reasoningeval.Alert` and `packages/accounting.AlertEvent`
are each scoped to exactly one alert kind and carry that kind's
domain-specific payload as first-class fields. This package's own
`AlertEvent` is deliberately more general -- a `RuleName`, `Severity`,
`TriggerValue`, and a free-form `Detail` string -- so an SLO breach, a
quality regression, a cost overage, and a plain metric-threshold
breach can all flow through the identical
`Engine.Evaluate`/`EvaluateSLOAlert`/`EvaluateQualityAlert`/
`EvaluateCostAlert` -> `AlertEvent` -> `Route` -> `Runbook` pipeline
uniformly. `QualityAlertRule` and `CostAlertRule` each *compose with*
(call into) `reasoningeval`'s and `accounting`'s existing detection
logic -- neither re-implements regression detection or budget-limit
math -- and then translate the already-computed domain result into
this package's uniform `AlertEvent` shape, exactly as
`packages/notifications`'s existing `ReasoningEvalAlertSink`/
`AccountingAlertSink` adapters translate those same upstream events
into a `Notification`. This package's `AlertEvent` is one further
translation step upstream of that.

## Cost and usage alerts (task 5)

`CostAlertRule`/`EvaluateCostAlert` (cost_alert.go) wraps an
`accounting.BudgetChecker`. `EvaluateCostAlert` calls
`BudgetChecker.Check` -- the same evaluation
`accounting.AccountingService` consults before allowing an LLM call
through -- and converts its `allowed`/`alert` result into this
package's `AlertEvent`: a hard-stop-exceeded budget fires with
`CostAlertRule.exceededSeverity()` (defaulting to `SeverityCritical`);
a soft warning (threshold crossed without a hard stop) fires with
`CostAlertRule.Severity`. Does not duplicate budget tracking or
threshold-percentage math; `accounting.InMemoryBudgetChecker` remains
the only place token/cost totals are actually accumulated.

## On-call routing (task 6)

`EscalationPolicy`/`EscalationTier`/`Route` (escalation.go): an ordered
list of named `Responder`s, each with a configured
`DelayBeforeNext` before the alert escalates to the next tier.
`Route(alert, policy, now)` walks the tier chain based on how long the
alert has been open (`now.Sub(alert.CreatedAt)`) -- real time-based
escalation logic, tested to confirm an alert escalates from tier 1 to
tier 2 exactly once tier 1's configured delay has elapsed, and remains
on the final tier indefinitely once every earlier tier's cumulative
delay has passed (there is nothing further to escalate to).
`NotificationRecipientSink` is the hand-off interface this package
defines (mirroring `reasoningeval.AlertSink`/`accounting.AlertSink`'s
interface-ownership direction) for a downstream notifier -- e.g.
`packages/notifications`, via an adapter mirroring its own
`ReasoningEvalAlertSink`/`AccountingAlertSink` -- to implement.
`RouteAndDeliver` composes `Route` with a `NotificationRecipientSink`
call in one step. This package does not import `packages/notifications`
to avoid a two-way module dependency.

## Alert runbooks (task 7)

`Runbook`/`RunbookStep` (runbook.go): an ordered remediation procedure
attached to an `AlertRule` by name (`AlertRule.RunbookName`),
structurally identical to `packages/backupdr.Runbook`/`RunbookStep`'s
`Order`/`Description`/`OwnerRole` shape (Phase 085) -- a remediation
procedure and a DR procedure are the same kind of artifact (an
ordered, role-assigned checklist) for a different trigger. This
package defines its own type rather than importing `backupdr`'s, since
the two attach to unrelated parent concepts and importing `backupdr`
solely for a two-field struct would add a dependency for no shared
behavior. `SeededRunbooks()` returns three concrete starter runbooks --
`slo-breach`, `quality-regression`, `cost-budget-exceeded` -- each with
a human-readable counterpart under `doc/runbooks/`, kept in the same
step order deliberately so the document and the data model never drift
apart silently.

## Synthetic monitoring (task 8)

`SyntheticCheck`/`Prober`/`Run` (synthetic.go): a named, on-demand (or
externally scheduled) probe wrapping a caller-supplied `Prober`
function -- `func(ctx) error`, the identical convention
`observability.Checker` and `reliability.TrafficShifter.HealthCheckFunc`
already use, named independently here rather than imported so a caller
can wrap its existing `/readyz` checker without this package depending
on `observability`'s HTTP types. `Run` records real pass/fail and
latency using an injectable clock, applying an optional per-check
`Timeout` on top of the caller's context -- tested with a fake probe
function for both outcomes, plus a fake clock proving latency
recording is real arithmetic, not a stub.

## Access control

Two new `identity.Permission` constants gate every `Engine` operation,
added following `permission.go`'s exact `PermViewBackupDR`/
`PermManageBackupDR` precedent from Phase 085:

- `alerting:view` (`identity.PermViewAlerting`): read-only access to
  the alert-rule catalogue, fired alert-event history, escalation
  policies, dashboards, and synthetic-check results.
- `alerting:manage` (`identity.PermManageAlerting`): register or
  update `AlertRule`s, set a tenant's `EscalationPolicy`, and run a
  `SyntheticCheck` on demand.

`RoleAdmin` holds both; `RoleAuditor` holds only the view permission,
consistent with its read-only, compliance-facing posture elsewhere in
the matrix (see `packages/identity/doc/rbac-matrix.md`).

## Persistence: why this package has a migration (unlike Phase 093)

`packages/reliability` (Phase 093) explicitly skips a migration because
every type it defines -- `CircuitBreaker` state, `IdempotencyGuard`'s
cache, `TrafficShifter`'s backend health, SLO `Observation`s -- is
live, in-process operational state with no tenant-facing historical
value (see `packages/reliability/doc.go`). This package is different:
an `AlertRule` definition, a fired `AlertEvent`, and an
`EscalationPolicy` are exactly the kind of tenant-facing,
queryable-after-the-fact record `packages/notifications.Notification`
and `packages/reasoningeval.Alert`-consuming dashboards already are --
an operator or auditor legitimately wants to ask "what alerts fired
last week" or "what is our current escalation policy" days or months
later. That is the same reasoning `packages/compliance` (082),
`packages/privacy` (081), and `packages/backupdr` (085) each gave for
adding tenant-scoped, RLS-backed storage, and this phase follows it.

Two new migration pairs (numbered 000042/000043 as the next available
slot at the time this phase was written -- expect a renumbering
collision with sibling phases 097/098 landing in parallel, to be
resolved centrally by a coordinator):

- `packages/persistence/migrations/000042_create_alerting.up.sql` /
  `.down.sql` create three tables: `alerting_rules`, `alerting_events`,
  and `alerting_escalation_policies`, all tenant-scoped.
- `packages/persistence/migrations/000043_enable_rls_alerting.up.sql` /
  `.down.sql` enable and force row-level security with the standard
  `tenant_isolation` policy on all three tables.

Each table follows the same `Repository` / `PostgresXRepository` /
`TenantScopedXRepository` three-layer pattern established by
`packages/compliance`/`packages/backupdr`, with Row-Level Security
enforcing tenant isolation at the database layer in addition to each
repository's own application-level `requireMatchingTenant` guard.

## What is explicitly reused, not duplicated

- `packages/observability.Registry`/`Counter`/`Gauge`/`Histogram`
  (Phase 003) remain the only metrics-registration machinery in this
  codebase; `RegisterBusinessMetrics` registers through a
  caller-supplied `Registry`.
- `packages/observability.Checker`/`NamedChecker`/`ReadinessHandler`
  (Phase 003) remain the only HTTP liveness/readiness probe machinery;
  `SyntheticCheck.Prober` follows the identical convention by
  reference, not import.
- `packages/reliability.SLO`/`Observation`/`SLOStatus`/`EvaluateSLO`/
  `ErrorBudget`/`ComputeErrorBudget`/`ErrorBudgetPolicy` (Phase 093)
  remain the only SLO and error-budget computation in this codebase;
  `SLOAlertRule`/`EvaluateSLOAlert` calls directly into these.
- `packages/reasoningeval.RegressionDetector`/`RegressionResult`/
  `QualityScore` (Phase 062) remain the only reasoning-quality
  regression detection in this codebase; `QualityAlertRule`/
  `EvaluateQualityAlert` calls directly into `RegressionDetector.Compare`.
- `packages/accounting.BudgetConfig`/`BudgetChecker`/`TokenUsage`
  (Phase 017) remain the only token/cost budget tracking in this
  codebase; `CostAlertRule`/`EvaluateCostAlert` calls directly into a
  `BudgetChecker.Check` result.
- `packages/notifications.Service`/`NotifyInput` (Phase 072) remains
  the only notification delivery and inbox mechanism in this codebase;
  `Route`/`RouteAndDeliver` only decide which `Responder` an
  `AlertEvent` should be handed to next, via the
  `NotificationRecipientSink` interface this package defines for a
  downstream notifier to implement.
- `packages/backupdr.Runbook`/`RunbookStep`'s
  `Order`/`Description`/`OwnerRole` shape (Phase 085) is the structural
  precedent this package's own `Runbook`/`RunbookStep` follows, not a
  dependency -- this package does not import `packages/backupdr`.
- `packages/compliance.Dashboard`/`BuildDashboard` (Phase 082) is the
  structural precedent `DashboardDefinition`/`BuildDashboard` follows,
  not a dependency -- this package does not import `packages/compliance`
  or `packages/analytics`.
- `identity.Role`/`identity.Permission`/`identity.HasPermission`
  (Phase 006) remain the coarse RBAC gate every `Engine` method calls
  through `authorizeManage`/`authorizeView` before doing anything
  alerting-specific.

See `doc/runbooks/*.md` for the human-readable procedures attached to
this phase's three seeded alert rules.
