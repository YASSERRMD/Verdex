# Operations runbooks

This document is the operator-facing companion to
[`packages/reliability`](../../packages/reliability) (Phase 093) and
[`packages/iac`](../../packages/iac) (Phase 094). It covers routine
operational procedures: reading the platform's reliability posture,
running post-deploy verification, and the promotion/rollout gates a
deploy goes through. For a live incident, use
[`docs/operations/incident-response.md`](incident-response.md) instead.

As of this phase, there is no dedicated `packages/alerting` package in
this repository (`ls packages/alerting` resolves to nothing) — alerting
integration is a future phase. This document covers what exists today:
`packages/reliability`'s SLO/error-budget model and
`packages/observability`'s health-check conventions.

## Reading the platform's reliability posture

### Circuit breakers

[`packages/reliability.CircuitBreaker`](../../packages/reliability)
generalizes `packages/router.CircuitBreaker`'s (Phase 012)
provider-specific Closed/Open/HalfOpen state machine to any
caller-named dependency: a Postgres pool, the Neo4j graph store, or an
external integration client (Phase 087). If a dependency's breaker is
`Open`, that dependency is currently being protected from further load
because it has been failing — check `CircuitBreakerRegistry` for which
named dependency tripped before assuming the fault is elsewhere.

### Degraded-mode responses

[`packages/reliability.Degrader[T]`](../../packages/reliability) wraps
a primary+fallback function pair: on primary failure, a
`DegradationMode` (serve-stale, skip-enrichment, reduced-scope,
static-default) is recorded in the `Result`. If users report reduced
functionality rather than an outright error, check for a recent
`Degraded` result and its recorded `Mode` before escalating — this may
be the system correctly protecting itself rather than a new fault.

### SLOs and error budgets

[`packages/reliability.SLO`](../../packages/reliability)/`EvaluateSLO`
evaluates a named service-level objective — a minimum success rate
(`SLOKindSuccessRate`) or a maximum P95 latency (`SLOKindLatency`) —
over a rolling window of recorded `Observation`s. For a
`SLOKindSuccessRate` SLO, `ComputeErrorBudget` derives how much of the
allowed failure margin has actually been consumed
(e.g. a 99.5% success-rate target permits 0.5% failure; that 0.5% is
the budget). `ErrorBudgetPolicy.Evaluate` surfaces a plain
`BlockRiskyDeploys` boolean once consumption crosses a configured
exhaustion threshold — **this package computes the signal only; it
does not itself block a deploy pipeline.** An operator or CI job
consulting `PolicyResult.BlockRiskyDeploys` before promoting a risky
change is a deployment-specific integration point, not something this
package enforces automatically.

This is deliberately parallel to, not merged with,
[`packages/perf.Budget`](../../packages/perf)/`Evaluate` (Phase 091):
`perf` compares a benchmark run's observed figures against a target in
CI/dev; `reliability.SLO` compares live production traffic against a
target in a rolling window. Same target-vs-observed shape, two
different data sources — see `packages/reliability/doc/reliability.md`'s
"perf vs reliability" section for the full rationale.

### Health-based traffic shifting

[`packages/reliability.TrafficShifter`](../../packages/reliability)
selects which of a set of named backends should currently receive
traffic, based on each backend's last-reported `HealthStatus`:
round-robin across all-healthy backends, degrading to the remaining
healthy set as backends fail, and failing closed
(`ErrNoHealthyBackends`) once none report healthy. If traffic seems
concentrated on fewer backends than expected, check each backend's
reported `HealthStatus` first.

## Post-deploy verification

Every deploy — any tier, see [`docs/deployment/`](../deployment/) —
should run [`packages/iac`](../../packages/iac)'s `Checklist` against
the following `CheckKind`s before considering the new version healthy:

| CheckKind | What it verifies |
|---|---|
| `CheckKindHealthEndpoint` | The deployment's `/healthz`/`/readyz` endpoints (`packages/observability`, Phase 003) respond healthy. |
| `CheckKindMigrationVersion` | The deployed instance's applied migration version matches what `packages/persistence`'s migration runner expects — catches a deploy that shipped code ahead of (or behind) its schema. |
| `CheckKindGuardrailSmokeTest` | The non-binding-analysis guardrail (`packages/guardrail`, Phase 057) is actually enforcing on the newly deployed instance — a smoke test, not a unit test, run against the live deployment. |
| `CheckKindCustom` | Any additional check a caller registers beyond the three above. |

A `DeploymentVerificationReport` recording these results is what
`packages/iac.PromotionPipeline.Promote` requires before advancing a
stage — see the "Promotion" section of your tier's deployment guide
under [`docs/deployment/`](../deployment/).

## Promotion and rollout

`packages/iac.PromotionPipeline` gates a deployment through
Dev → Staging → Prod, refusing to advance without a passing
`DeploymentVerificationReport` for the current stage — there is no
bypass. Once promoted, `packages/iac.RolloutStrategy` supports
blue-green, canary, or direct rollout; a canary's traffic percentage at
each step is computed by `packages/iac`'s own rollout calculator (see
that package's design doc).

`packages/cicdgate` (Phase 095) additionally documents (as CI
placeholders, since this repository has no deployed environment or
release-tag trigger yet) how real `StageHealth` samples — sourced from
`packages/observability` — are expected to feed
`EvaluatePromotion`/`EvaluateRollback` to gate and automatically roll
back a staged rollout, and how `GenerateReleaseNotes` is expected to
produce a release's changelog from its commit list. See
`.github/workflows/ci.yml`'s `release-automation` job and
`packages/cicdgate/doc/cicd.md` for the current state of that
integration.

## Resilience testing

`packages/reliability.FailureInjector` wraps a function and injects a
deterministic, cycled sequence of failure modes (latency, error,
panic-with-recovery) for use as a reusable test fixture — reach for
this before hand-rolling a one-off "fail every Nth call" harness when
validating a new dependency's retry/circuit-breaker/degradation
behavior.

## Escalation

If a reliability signal above indicates a real, ongoing incident rather
than routine degraded-but-serving behavior, move to
[`docs/operations/incident-response.md`](incident-response.md).
