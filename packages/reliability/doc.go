// Package reliability is Phase 093: graceful degradation and fault
// tolerance for this platform's dependencies (databases, the graph
// store, external integrations added in Phase 087, and downstream
// providers). It draws on the provider-specific circuit breaker added
// in Phase 012 (packages/router), the per-stage idempotency ledger
// added in Phase 029 (packages/ingestion), the benchmark
// budget/regression-tracking shape established in Phase 091
// (packages/perf), and the liveness/readiness health-check endpoints
// added in Phase 003 (packages/observability), composing with each by
// reference/generalization rather than duplicating any of them.
//
// # What is new here
//
//   - RetryPolicy / Retry (retry.go): a generic, configurable
//     retry-with-exponential-backoff-and-jitter wrapper --
//     Retry(ctx, policy, fn) -- that stops promptly on ctx
//     cancellation/deadline, whether that happens before an attempt or
//     mid-backoff-sleep, and wraps the last underlying failure in
//     ErrRetriesExhausted once attempts run out (task 1).
//   - WithTimeout (timeout.go): a one-line per-attempt deadline helper
//     over context.WithTimeout, for composing a timeout into a single
//     Retry callback (task 1).
//   - State / CircuitBreaker / CircuitBreakerConfig /
//     CircuitBreakerRegistry (circuit_breaker.go): a generic
//     Closed/Open/HalfOpen circuit breaker keyed by a caller-chosen
//     dependency name (not a provider ID), generalizing
//     packages/router.CircuitBreaker's provider-specific state machine
//     (Phase 012) to arbitrary dependencies -- a Postgres pool, the
//     Neo4j graph store, an external integration client from Phase 087
//     (task 2).
//   - DegradationMode / Degrader[T] (degradation.go): a named
//     reduced-service fallback concept (serve-stale,
//     skip-enrichment, reduced-scope, static-default) plus a generic
//     wrapper around a primary+fallback function pair: on primary
//     failure, the fallback runs and the Result is marked Degraded
//     with its Mode recorded, rather than propagating the primary's
//     failure (task 3).
//   - IdempotencyGuard[T] (idempotency.go): a generic,
//     IdempotencyKey-based dedup guard --
//     Execute(ctx, key, fn) (T, error) -- that invokes fn at most once
//     per key within a TTL window, replaying the cached value/error to
//     every other caller using that key, including a caller racing a
//     still-in-flight call for the same key (task 4).
//   - FailureMode / FailureInjector / FailureInjectorConfig (chaos.go):
//     wraps a function and injects one of a fixed, deterministically
//     cycled sequence of failure modes (none, latency, error,
//     panic-with-recovery) per call, for use as a reusable test
//     fixture -- in this package's own tests and any other package's
//     -- rather than every package hand-rolling its own flaky "fail
//     every Nth call" scaffolding (task 5).
//   - HealthStatus / Backend / TrafficShifter (traffic_shift.go): given
//     a set of named backends each carrying a last-reported
//     HealthStatus, selects which should currently receive traffic:
//     round-robin across all-healthy, degrading to the remaining
//     healthy set as backends fail, failing closed
//     (ErrNoHealthyBackends) once none are healthy (task 6).
//   - SLOKind / SLO / Observation / SLOStatus / EvaluateSLO (slo.go): a
//     named service-level objective (a minimum success rate or a
//     maximum P95 latency) evaluated over a rolling time window against
//     recorded Observations, mirroring the target/observed-and-compare
//     shape packages/perf.Budget/Evaluate already established for
//     benchmark targets (task 7).
//   - ErrorBudget / ComputeErrorBudget / ErrorBudgetPolicy / PolicyResult
//     (error_budget.go): derives how much of an SLO's allowed failure
//     margin has been consumed by observed failures, and evaluates that
//     consumption against a configurable exhaustion threshold, surfacing
//     a plain BlockRiskyDeploys boolean signal a deploy pipeline could
//     consult -- this package computes the signal only; it does not
//     integrate with any actual deploy gate (task 8).
//   - resilience_test.go (task 9): integration-style tests combining
//     Retry, CircuitBreaker, Degrader, IdempotencyGuard, TrafficShifter,
//     SLO/ErrorBudget, and FailureInjector against a small, in-package
//     fakeDatabase and simulated traffic -- never against real external
//     services.
//   - doc/reliability.md and this doc.go (task 10): the full write-up,
//     including the composition table below.
//
// # perf vs reliability: two deliberately parallel, not merged, concepts
//
// packages/perf's Budget/Evaluate (Phase 091) and this package's
// SLO/EvaluateSLO look structurally similar -- both compare an observed
// value against a named target and report pass/fail. They are kept
// separate rather than merged into one type, because they answer
// different questions for different audiences at different times:
//
//   - packages/perf.Budget targets a *benchmark run*: numbers gathered
//     by `go test -bench` in CI or a developer's own machine, checked
//     against a fixed target before a change merges. Its audience is
//     engineers and CI; it never observes live production traffic.
//   - This package's SLO targets *live production traffic*: an ongoing
//     rolling window of real Observations from real requests, checked
//     continuously (or on-demand) to decide whether an operational
//     policy (ErrorBudgetPolicy) should currently discourage risky
//     deploys. Its audience is an operator or an automated reliability
//     gate, not a CI benchmark step.
//
// A regression in packages/perf's sense ("this build got slower than
// the last few builds") and an SLO violation in this package's sense
// ("production is currently failing more than its error budget allows")
// are both real and both worth tracking, but conflating "did this build
// regress" with "is production currently healthy" into one type would
// blur exactly the CI-vs-production distinction that makes each useful
// on its own. This package does not import packages/perf, and
// packages/perf is untouched by this phase.
//
// # Persistence: why this package has no SQL migration
//
// Every recent phase adding tenant-scoped operational state (081
// privacy, 082 compliance, 083 threatmodel) added -- or explicitly
// justified skipping -- a Postgres migration. This phase skips one too,
// for the same reason packages/perf's BenchmarkRun history did (Phase
// 091, see packages/perf/doc.go): every type in this package --
// CircuitBreaker state, IdempotencyGuard's cached entries,
// TrafficShifter's backend health, SLO Observations -- is live,
// in-process operational state whose value is entirely about *what is
// happening right now* in a running instance. A CircuitBreaker's Open
// state or a TrafficShifter's current backend selection is not a
// historical record to audit later; it is only ever meaningful as of
// "right now, in this process." Unlike packages/compliance's Control
// catalogue or packages/privacy's SubjectAccessRequest, none of this
// package's types are tenant-facing records a customer or auditor would
// ever query after the fact -- they are operated by the platform itself
// (an ingestion worker's retry loop, a gateway's traffic-shifting
// decision), the same "operated by CI and engineers, not by
// tenant-facing request handlers" rationale packages/perf's doc.go gives
// for adding no packages/identity.Permission constants either, which
// this phase follows for the same reason: no tenant-scoped durable
// secrets, no tenant-scoped repository, and no Engine-style façade
// gating reads/writes on an authenticated actor exist in this package.
//
// A deployment that wants durable, queryable SLO-violation history for
// dashboards or postmortems can persist SLOStatus/ErrorBudget snapshots
// itself (this package's types are plain, JSON-friendly structs with no
// external-package dependency); should a later phase want that built in
// as a first-class, tenant-facing feature, it should add a Store
// interface plus a migration then, following packages/perf.Store's
// exact Save/List shape (see packages/perf/store.go) rather than this
// phase speculatively building persistence nothing yet needs.
//
// # What is explicitly reused, not duplicated
//
//   - packages/router.CircuitBreaker and
//     packages/router.CircuitBreakerRegistry (Phase 012) are untouched
//     and remain the only circuit breaker gating LLM provider selection
//     inside packages/router's routing logic. This package's
//     CircuitBreaker is a separate, generic type for arbitrary
//     dependencies; packages/router is not imported, edited, or
//     wrapped by this phase.
//   - packages/ingestion.IdempotencyStore/RunWithRetry (Phase 029)
//     remain the only (key, stage) completion/attempt-count bookkeeping
//     ledger this codebase's ingestion pipeline consults before
//     deciding whether to re-run a stage. That ledger does not itself
//     cache or replay a stage's return *value* -- it is bookkeeping, not
//     a result cache. IdempotencyGuard[T] is a separate, general-purpose,
//     value-caching primitive; packages/ingestion is not imported or
//     modified by this phase.
//   - packages/perf.Budget/Evaluate/BenchmarkRun (Phase 091) remain the
//     only CI/benchmark-run performance-governance types in this
//     codebase. This package's SLO/ErrorBudget target live production
//     traffic instead (see "perf vs reliability" above);
//     packages/perf is not imported or modified by this phase.
//   - packages/observability.Checker/NamedChecker/ReadinessHandler
//     (Phase 003) remain the only HTTP liveness/readiness probe
//     machinery in this codebase. TrafficShifter.HealthCheckFunc uses
//     the identical "func(ctx) error, nil means healthy" convention by
//     reference -- named independently in this package rather than
//     imported, so a caller can share one underlying check function
//     between its /readyz handler and its TrafficShifter without this
//     package depending on packages/observability at all.
//
// See doc/reliability.md for the full write-up, including a
// composition table and worked examples.
package reliability
