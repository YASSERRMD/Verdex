# Reliability and resilience (Phase 093)

This phase adds `packages/reliability`: graceful degradation and fault
tolerance for this platform's dependencies -- databases, the graph
store, external integrations added in Phase 087, and downstream
providers. It draws on four earlier threads and generalizes or
composes with each rather than duplicating them.

## Goal

Give every package in this codebase a shared, tested toolbox for
surviving a misbehaving dependency: bounded retries with real backoff,
a circuit breaker that stops hammering a dependency that is clearly
down, a graceful-degradation wrapper that serves a reduced result
instead of failing outright, a dedup guard so retried/duplicated
requests don't double-apply a side effect, a deterministic
failure-injection fixture for testing all of the above, health-based
traffic shifting across redundant backends, and SLO/error-budget
tracking to quantify how much unreliability a deployment has actually
experienced.

## What this package composes from, versus what is new

| Existing piece | What it already provides | What this phase adds |
|---|---|---|
| `packages/router.CircuitBreaker`/`CircuitBreakerRegistry` (Phase 012) | A Closed/Open/HalfOpen breaker hardcoded to gate a single LLM provider inside that package's routing logic | `CircuitBreaker`/`CircuitBreakerRegistry`: the identical state machine, generalized to any caller-named dependency (DB, graph store, integration client) -- **`packages/router`'s breaker is untouched and remains in place** |
| `packages/ingestion.IdempotencyStore`/`RunWithRetry` (Phase 029) | A `(key, stage)` completion/attempt-count bookkeeping ledger consulted before re-running a pipeline stage -- no backoff/sleep, no cached return value | `RetryPolicy`/`Retry`: a real, timed exponential-backoff-with-jitter loop (additive to ingestion's own retry shape); `IdempotencyGuard[T]`: a value-*caching* dedup primitive generalizing the same "don't redo completed work for this key" idea to any pipeline, not just ingestion's stages |
| `packages/perf.Budget`/`Evaluate`/`BenchmarkRun` (Phase 091) | Named-operation target-vs-observed comparison for CI/dev benchmark runs | `SLO`/`EvaluateSLO`: the same target-vs-observed comparison shape, applied to live production traffic in a rolling window instead of a benchmark run -- see "perf vs reliability" below for why these stay separate |
| `packages/observability.Checker`/`NamedChecker`/`ReadinessHandler` (Phase 003) | HTTP liveness (`/healthz`) and readiness (`/readyz`) probes, `func(ctx) error` convention | `TrafficShifter`: routes traffic across named backends by health, using the identical `func(ctx) error, nil means healthy` convention (named independently as `HealthCheckFunc`, not imported) so a caller can share one check function between its `/readyz` handler and its `TrafficShifter` |

## Timeouts and retries (task 1)

`RetryPolicy`/`Retry` (retry.go) is a generic wrapper:

```go
err := reliability.Retry(ctx, reliability.RetryPolicy{
    MaxAttempts: 5,
    BaseDelay:   100 * time.Millisecond,
    MaxDelay:    2 * time.Second,
    Jitter:      0.2,
}, func(ctx context.Context) error {
    return db.Query(ctx)
})
```

- Attempts are bounded by `MaxAttempts` (falling back to
  `DefaultMaxAttempts` = 3 if unset).
- Backoff is exponential (`BaseDelay * 2^(attempt-1)`, capped at
  `MaxDelay`), with a configurable `Jitter` fraction applied uniformly
  within `[(1-jitter)*d, (1+jitter)*d]`.
- `ctx` cancellation is checked both before each attempt and while
  sleeping between attempts -- `Retry` never sleeps past a canceled
  context, and returns `ctx.Err()` immediately in either case.
- Exhausting every attempt without success returns the last underlying
  error wrapped in `ErrRetriesExhausted` (both are reachable via
  `errors.Is`/`errors.As`).

`WithTimeout` (timeout.go) is a one-line `context.WithTimeout` wrapper
so a per-attempt deadline composes into a single `Retry` callback.

## Circuit breakers on dependencies (task 2)

`CircuitBreaker` (circuit_breaker.go) implements the standard
Closed → Open → HalfOpen state machine:

- **Closed**: calls proceed; `RecordFailure` increments a counter that
  trips to Open at `FailureThreshold` (default 5).
- **Open**: `Allow()` rejects immediately until `Cooldown` (default 30s)
  elapses, at which point the *next* `Allow()` call transitions to
  HalfOpen and permits exactly one probe.
- **HalfOpen**: exactly one call is admitted; `RecordSuccess` closes the
  breaker, `RecordFailure` reopens it (restarting the cooldown).

`CircuitBreakerRegistry` lazily constructs one `CircuitBreaker` per
caller-chosen dependency name (`registry.Get("postgres")`,
`registry.Get("graph-store")`), the same lazy-per-key registry shape
`packages/router.CircuitBreakerRegistry` already uses for providers,
generalized to any dependency name.

**`packages/router`'s provider-specific `CircuitBreaker` is untouched.**
It remains the only breaker gating LLM provider selection inside that
package's routing logic; this phase does not edit, import, or wrap it.
A future phase could migrate `packages/router` onto this package's
generic breaker if desired, but that migration is out of scope here --
today, two independent breaker implementations coexist deliberately.

## Graceful degradation modes (task 3)

`Degrader[T]` (degradation.go) wraps a primary+fallback function pair:

```go
d := reliability.NewDegrader(reliability.ModeServeStale,
    func(ctx context.Context) (CaseSummary, error) { return liveLookup(ctx) },
    func(ctx context.Context, primaryErr error) (CaseSummary, error) { return cache.Get(ctx) },
)
result, err := d.Run(ctx)
// result.Degraded == true, result.Mode == ModeServeStale, if the primary failed
```

Four named `DegradationMode`s are provided as a starter vocabulary
(`ModeServeStale`, `ModeSkipEnrichment`, `ModeReducedScope`,
`ModeStaticDefault`) -- an open string type, so a consuming package can
name its own mode without a change to this package.

## Idempotency across pipelines (task 4)

`IdempotencyGuard[T]` (idempotency.go) deduplicates by key within a TTL
window:

```go
guard := reliability.NewIdempotencyGuard[PaymentResult](10 * time.Minute)
result, err := guard.Execute(ctx, requestID, func(ctx context.Context) (PaymentResult, error) {
    return chargeCard(ctx)
})
```

A second `Execute` call with the same key -- whether it arrives after
the first completed, or races in while the first is still in flight --
returns the first call's cached value/error without invoking `fn`
again. This generalizes
`packages/ingestion.IdempotencyStore`'s `(key, stage)` bookkeeping
ledger (Phase 029): that ledger tracks *whether* a stage completed and
*how many attempts* it took, for `RunWithRetry` to consult, but it does
not itself cache or replay the stage's return value. `IdempotencyGuard`
is a reusable, value-caching primitive suited to any pipeline needing
"the second call with this key returns the first call's result"
semantics -- a webhook handler replaying a provider callback, an API
endpoint retried by a client after a timeout -- not just
`packages/ingestion`'s stage-keyed one, which is untouched.

## Chaos / failure injection (task 5)

`FailureInjector` (chaos.go) wraps a function and injects one of a
fixed, deterministically-cycled `Pattern` of `FailureMode`s per call:

```go
fi := reliability.NewFailureInjector(reliability.FailureInjectorConfig{
    Pattern: []reliability.FailureMode{
        reliability.FailureModeError,
        reliability.FailureModeNone,
        reliability.FailureModeLatency,
    },
})
err := fi.Execute(ctx, func(ctx context.Context) error { return realCall(ctx) })
```

- `FailureModeNone`: `fn` runs unmodified.
- `FailureModeError`: `fn` is not invoked; a configured error returns
  immediately.
- `FailureModeLatency`: sleeps a configured duration (honoring `ctx`)
  before invoking `fn`.
- `FailureModePanic`: triggers and recovers a panic within the same
  call stack, surfacing `ErrInjectedPanic` -- the calling goroutine
  never crashes.

The `Pattern` is a fixed slice, not a random rate: this makes
`FailureInjector`'s own tests, and any other package's tests using it
as a fixture, deterministic and reproducible rather than flaky.

## Health-based traffic shifting (task 6)

`TrafficShifter` (traffic_shift.go) tracks a `HealthStatus` per named
`Backend` and selects who should receive the next unit of traffic:

```go
ts := reliability.NewTrafficShifter("primary", "replica")
ts.RefreshFromCheckers(ctx, map[string]reliability.HealthCheckFunc{
    "primary": pingPrimary,
    "replica": pingReplica,
})
backend, err := ts.Select() // round-robin among healthy backends, or ErrNoHealthyBackends
```

- **All healthy**: round-robin across every backend.
- **Some unhealthy**: degrades to round-robin over the remaining
  healthy set.
- **None healthy**: fails closed -- `ErrNoHealthyBackends` -- rather
  than routing to a backend known to be down.

`HealthCheckFunc` is the exact `func(ctx) error, nil means healthy`
shape `packages/observability.Checker` already uses for its
`/readyz` handler (Phase 003), named independently here (not imported)
so a caller can reuse the same underlying check function for both.

## SLO definitions and tracking, and error-budget policy (tasks 7-8)

`SLO` (slo.go) names a target -- a minimum success rate or a maximum
P95 latency -- over a rolling window:

```go
slo := reliability.SLO{
    Name: "ingestion-availability", Kind: reliability.SLOKindSuccessRate,
    Target: 0.99, Window: 24 * time.Hour,
}
status, _ := reliability.EvaluateSLO(slo, observations, time.Now())
// status.Met, status.Observed, status.SampleCount
```

`ComputeErrorBudget` (error_budget.go) derives how much of the SLO's
implied failure allowance (`1 - Target`) has been consumed by observed
failures in the same window:

```go
budget, _ := reliability.ComputeErrorBudget(status)
// budget.ConsumedFraction, budget.RemainingFraction (clamped to >= 0)

result, _ := reliability.ErrorBudgetPolicy{}.Evaluate(budget)
// result.Exhausted, result.BlockRiskyDeploys
```

`ErrorBudgetPolicy.Evaluate` reports `Exhausted`/`BlockRiskyDeploys`
once `ConsumedFraction` crosses a configurable threshold (default
1.0 -- fully consumed). This is a **signal only**: this package computes
the boolean; it does not integrate with, gate, or call out to any
actual deploy pipeline.

### perf vs reliability: two deliberately parallel, not merged, concepts

`packages/perf.Budget`/`Evaluate` and this package's `SLO`/`EvaluateSLO`
share a shape (compare observed against target, report pass/fail), but
answer different questions for different audiences:

- **`packages/perf.Budget`** targets a *benchmark run*: numbers from
  `go test -bench` in CI or on a developer's machine, checked once per
  build. Audience: engineers and CI. Never sees live traffic.
- **This package's `SLO`** targets *live production traffic*: a rolling
  window of real request `Observation`s, checked continuously to drive
  an operational policy. Audience: an operator or automated reliability
  gate.

A CI regression ("this build got slower") and a production SLO
violation ("we're currently failing more than budget allows") are both
real, but merging them into one type would blur the CI-vs-production
distinction that makes each useful. `packages/reliability` does not
import `packages/perf`, and `packages/perf` is untouched.

## Persistence: why this package has no SQL migration

Every type in this package -- `CircuitBreaker` state,
`IdempotencyGuard`'s cached entries, `TrafficShifter`'s backend health,
recorded `Observation`s -- is live, in-process operational state whose
entire meaning is "what is happening right now in a running instance."
A breaker's Open state or a shifter's current backend selection is not
a historical record to audit later. This mirrors exactly why
`packages/perf.BenchmarkRun` history (Phase 091) stayed
`InMemoryStore`-only rather than gaining a migration: no
tenant-scoped durable secret, no tenant-facing repository, no
`Engine`-style façade gating reads/writes on an authenticated actor --
this package's primitives are operated by the platform itself (a
worker's retry loop, a gateway's traffic-shifting decision), not
queried after the fact by a tenant or auditor. For the identical
reason `packages/perf`'s doc.go gives, this phase adds no
`packages/identity.Permission` constants either.

A deployment wanting durable, queryable SLO-violation history for
dashboards or postmortems can persist `SLOStatus`/`ErrorBudget`
snapshots itself -- every type here is a plain, JSON-friendly struct
with no dependency on any other Verdex package. Should a later phase
want that built in as a first-class, tenant-facing feature, it should
add a `Store` interface plus a migration then, following
`packages/perf.Store`'s exact `SaveRun`/`ListRuns` shape, rather than
this phase speculatively building persistence nothing yet needs.

## Tests for resilience (task 9)

`resilience_test.go` builds a small, in-package `fakeDatabase` (a
mutex-guarded struct whose failure behavior a test scripts explicitly
-- fail the next N calls, or fail permanently) and drives combinations
of this package's primitives against it and against simulated traffic:

- `Retry` recovering from a transient (self-healing) failure run.
- `CircuitBreaker` protecting against a permanently-down dependency:
  trips after the failure threshold, rejects without reaching the fake
  dependency while open, allows exactly one probe after cooldown.
- `Degrader` serving a stale cached value when the primary is down.
- `Retry` + `CircuitBreaker` + `Degrader` composed into one layered
  call chain.
- `IdempotencyGuard` + `Retry` ensuring a side-effecting call runs
  exactly once across concurrent duplicate requests sharing an
  idempotency key.
- `TrafficShifter` + `SLO`/`ErrorBudget` together: traffic shifts away
  from a failing backend while the SLO/error-budget machinery
  independently confirms the failure burst blew through the allowed
  budget.
- `FailureInjector` driving a `CircuitBreaker` deterministically through
  every state transition, rather than hand-scripted inline failures.

None of these tests touch a real database, HTTP server, or other
external service -- every dependency is an in-package fake.

## Running the test suite

From `packages/reliability/`:

```sh
go build ./...
go vet ./...
go test ./... -race
golangci-lint run ./...
```

All concurrency-sensitive types (`CircuitBreaker`, `IdempotencyGuard`,
`TrafficShifter`, `FailureInjector`) have a dedicated concurrent-access
test exercised under `-race`.
