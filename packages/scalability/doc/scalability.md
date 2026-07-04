# Scalability & horizontal scaling (Phase 092)

This phase draws together the layered, file-based configuration
pattern established in Phase 002 (`packages/config`), the
per-provider circuit breaker and routing telemetry added in Phase 012
(`packages/router`), the async job queue added in Phase 029
(`packages/ingestion`), and the benchmarking/load-testing/
resource-limiting shapes established in Phase 091 (`packages/perf`)
-- into a horizontal-scaling toolkit: `packages/scalability`.

## Goal

Give this platform a concrete, testable toolkit and documented
contract for scaling services and stores under load: a stateless-
service checklist with real pass/fail evaluation, queue-based
workload distribution, store partitioning/sharding with a real
rebalance calculator, autoscaling decision logic with flapping
prevention, concurrency-safe backpressure/load-shedding, a capacity-
planning arithmetic model, and a concurrency-ramp scale-test harness
-- without rewriting any existing service to actually run distributed.

## What this package composes from, versus what is new

| Existing piece | What it already provides | What this phase adds |
|---|---|---|
| `packages/config` (Phase 002) | `Default()` + optional YAML file, layered precedence | `PolicyConfig`/`DefaultPolicyConfig`/`LoadPolicyConfig` (config.go) follow the identical Default()-plus-YAML-overlay convention for `ScalingPolicy` + `BackpressureConfig`, without importing packages/config or duplicating its env/profile/secrets machinery |
| `packages/router` (Phase 012) | `CircuitBreaker`/`CircuitBreakerRegistry`: per-LLM-provider failure isolation | `BackpressureController` (backpressure.go) is a structurally similar (small, mutex-guarded state machine) but conceptually distinct primitive: it protects one service instance from being overwhelmed by request *volume*, not one provider from repeated *failures*. Neither replaces the other; see "What this phase does NOT modify" below |
| `packages/ingestion` (Phase 029) | `JobQueue`/`InMemoryJobQueue`: single-pipeline async handoff queue | `Partitioner` (partitioner.go) decides *which* of N workers/queue-instances a key's work should land on; it does not enqueue, dequeue, or buffer anything itself, and composes with (does not replace) `JobQueue` |
| `packages/perf` (Phase 091, by reference only) | `WorkFunc`/`LoadTest`: single-concurrency-level load test; `ResourceLimits`/`Limiter`: semaphore-based concurrency ceiling; `Budget`/`Measurement`/`Verdict`: target-vs-observed evaluation | `ScaleOperation`/`ScaleTest` (scaletest.go) is signature-compatible with `perf.WorkFunc` but ramps through a *sequence* of concurrency levels to show how throughput/error-rate change as load increases; `CapacityInput`/`EstimateReplicas` (capacity.go) mirrors `Budget`/`Measurement`'s target-vs-observed shape by name, applied to replica-count capacity planning instead of latency percentiles. Neither imports `packages/perf` -- see scaletest.go's doc comment for the specific dependency-weight reasoning |

## What this phase does NOT modify

- **No existing service is rewritten to actually run distributed,
  horizontally-scaled replicas.** This phase ships the toolkit
  (`Contract`, `ScalingPolicy`, `ShardStrategy`, `Partitioner`,
  `BackpressureController`, `EstimateReplicas`, `ScaleTest`) and the
  documented contract; adopting any of them into a specific service
  (`packages/caselifecycle`, `packages/casesearch`,
  `packages/ingestion`'s orchestrator, etc.) is a decision left to
  that service's own future phase.
- **`packages/ingestion`**: `JobQueue`/`InMemoryJobQueue` (queue.go)
  are read and referenced by `partitioner.go`'s doc comment, but no
  file in that package is touched. `Partitioner` composes with
  `JobQueue` conceptually (deciding which of N `JobQueue` instances a
  key's work lands on); this phase does not wire that composition
  into `packages/ingestion` itself.
- **`packages/router`**: `CircuitBreaker`/`CircuitBreakerRegistry`
  (circuit_breaker.go) are read and referenced as the structural
  precedent `BackpressureController` follows, but not modified.
  `CircuitBreaker` remains the only per-provider failure-isolation
  mechanism in this codebase.
- **`packages/perf`**: `LoadTest`, `Limiter`, `Budget`/`Measurement`/
  `Verdict`, and `BenchmarkRun`/`Store` are read and referenced by
  name/shape but not imported or modified. `ScaleTest` is a
  deliberately separate implementation (see below).
- **`packages/config`**: `Loader`/`Config`/`env.go`/`profile.go`/
  `secrets.go` are read and referenced as `PolicyConfig`'s structural
  template but not modified or imported. `PolicyConfig` is
  deliberately narrower in scope than `config.Config` (no
  environment-variable layer, no named-profile overlay of its own).

## StatelessnessContract (tasks 1 and 3)

The phase brief separately lists "make services horizontally
scalable" (task 1) and "add stateless service guarantees" (task 3).
This package treats them as one documented contract rather than two
mechanisms, because a service is horizontally scalable *because* it
is stateless in the specific ways that matter for replica
interchangeability.

`ChecklistAnswers` (contract.go) is a service owner's self-
attestation across seven boolean checks:

1. `NoInProcessSessionAffinity` -- no sticky-session assumption.
2. `NoLocalDiskOnlyState` -- no unrecoverable local-disk-only state.
3. `NoInMemorySingletonState` -- no process-global mutable state a
   caller depends on surviving a restart.
4. `IdempotentRetries` -- every externally-triggered operation is
   safe to retry against a different replica.
5. `ExternalizedConfiguration` -- config comes from
   `packages/config` (or an equivalent externalized source), not a
   replica-local file.
6. `HealthCheckExposed` -- a liveness/readiness signal an
   orchestrator can poll.
7. `GracefulShutdownHandled` -- in-flight work drains on shutdown
   signal rather than being dropped.

`Contract.Verify(serviceName, answers)` returns a real `Report`:
every unattested (`false`) field is listed by name in `Failed`,
`Score` is the fraction of the 7 checks that passed, and `Passed` is
true only when every check passed -- a service that has never filled
out the checklist fails by default (the zero value of
`ChecklistAnswers` is "not yet attested," not "compliant").

## Partitioner (task 2)

`Partitioner` (partitioner.go) maps a work-item key to one of N
logical partitions/workers:

- `NewConsistentHashPartitioner(n)` -- a consistent-hashing ring with
  150 virtual nodes per partition, bounding how many keys move if N
  changes later (see ShardStrategy/Rebalance below for the same
  technique applied to durable data, where bounded movement is the
  primary reason to choose it).
- `NewModuloPartitioner(n)` -- plain key-hash-modulo-n, simpler and
  tighter distribution in expectation, but remaps nearly every key on
  any resize. Offered for callers who never resize their partition
  count.

Both are tested for distribution evenness across 32,000 synthetic
keys, asserting on *mean* deviation across all N partitions (a
statistically stable metric) rather than any single partition's worst
case -- see partitioner_test.go's `TestPartitionerDistributionEvenness`
doc comment for why consistent hashing has real, expected per-
partition variance even with many virtual nodes.

### Relationship to `packages/ingestion`'s `JobQueue`

`JobQueue` (Phase 029) is the single-pipeline handoff between one
producer and one worker loop -- `Enqueue`/`Dequeue`/`Close` over one
channel. `Partitioner` answers a different, broader question: given N
independent consumers or shards, which one should a given key's work
land on, so that repeated work for the same key (all jobs for one
case ID, say) is handled consistently. A caller wiring cross-worker
or cross-service distribution on top of `JobQueue` would use
`Partitioner` to decide *which* of N `JobQueue` instances to
`Enqueue` onto; `Partitioner` does not itself enqueue, dequeue, or
buffer anything, and this phase does not modify `packages/ingestion`.

## ShardStrategy and Rebalance (task 4)

`ShardStrategy` (shard.go) maps a durable store key (a tenant ID or
case ID) to a shard, via the same consistent-hashing ring technique
as `Partitioner`, chosen specifically because store data must
physically move when shard count changes and bounding that movement
is the point.

`Rebalance(oldStrategy, newStrategy, keys)` computes a real
`RebalancePlan`: for a concrete population of keys, which ones change
shard assignment (`Moves`), and what fraction of the whole population
that represents (`MovedFraction`). Tested with a real 8-shard-to-
10-shard growth over 5,000 keys, asserting `MovedFraction` stays
bounded (<=60%) -- and, as an explicit contrast, a naive
modulo-sharding baseline over the identical resize is asserted to
move >=70% of keys, demonstrating why consistent hashing is used for
durable data rather than plain modulo.

## ScalingPolicy and Decide (task 5)

`ScalingPolicy` (policy.go) names `MinReplicas`/`MaxReplicas`, a
`TargetMetric` (e.g. target CPU utilization or normalized queue
depth), an `UpperTolerance`/`LowerTolerance` band around that target,
and `ScaleUpStep`/`ScaleDownStep`.

`Decide(policy, currentReplicas, observedMetric)` is a pure function
returning a `Decision` (`ActionScaleUp`/`ActionScaleDown`/
`ActionHold`, a `TargetReplicas`, and a human-readable `Reason`).
The tolerance band is real hysteresis: `TestDecideFlappingPrevention`
drives a metric jittering between 0.60 and 0.80 around a 0.70 target
(all within the default 15%-each-side band) across nine consecutive
evaluations and asserts every single one holds -- proving the policy
does not flap the replica count on noise right at target.
`TestDecideRealScalingScenario` separately walks a full
sustained-high-load-then-sustained-low-load episode, confirming
`Decide` scales up repeatedly, then down repeatedly, then holds once
load stabilizes.

## BackpressureController (task 6)

`BackpressureController` (backpressure.go) is a mutex-guarded
in-flight-request counter: `Admit()` succeeds while fewer than
`MaxInFlight` admitted requests are outstanding, and returns
`ErrLoadShed` the instant that threshold would be exceeded; `Release()`
frees a slot. `TestBackpressureConcurrentLoadShedsAtThreshold` hammers
a 10-slot controller with 200 concurrent goroutines (run with
`-race`), confirming the observed in-flight count never exceeds 10,
at least one goroutine is shed, and a fresh batch is admitted cleanly
once the first batch releases -- a real concurrency test, not just
sequential Admit/Release calls.

## EstimateReplicas capacity model (task 7)

`CapacityInput` (`ObservedThroughputPerReplica`, `TargetThroughput`,
`HeadroomFraction`) mirrors `packages/perf`'s `Budget`/`Measurement`
shape by name/tag -- a target versus an observed/historical figure --
without importing `packages/perf`: `perf.Budget`/`perf.Measurement`
are latency-percentile-shaped (P50/P95/P99/Throughput for one named
benchmarked operation); a capacity-planning model needs
throughput-and-headroom shaped inputs for a whole deployment, a poor
fit for a direct type reuse even though the import itself would be
trivial.

`EstimateReplicas(in)` computes `BaseReplicas = ceil(TargetThroughput
/ ObservedThroughputPerReplica)`, then `RecommendedReplicas =
ceil(BaseReplicas * (1 + HeadroomFraction))` -- real arithmetic,
tested for exact division, fractional rounding, headroom scaling, and
a worked example against `packages/perf`'s own real
`OpIngestionPipeline` budget number (5 ops/sec minimum throughput; see
`TestEstimateReplicasRealWorldScenario`).

## ScaleTest harness (task 8)

`ScaleTest(ctx, cfg, op)` (scaletest.go) drives a caller-supplied
`ScaleOperation` (signature-identical to `perf.WorkFunc`) through a
sequence of increasing concurrency levels
(`ScaleTestConfig.ConcurrencyLevels`), recording per-stage
`TotalCalls`/`ErrorRate`/`ThroughputPerSecond`/`MeanLatency` so a
caller can see how they change as concurrency rises.

### Why not import `packages/perf.LoadTest`

`packages/perf.LoadTest` already does exactly this at one fixed
concurrency level. Importing `packages/perf` to reuse it directly was
considered and rejected: `packages/perf`'s go.mod pulls in
`packages/graph`, `packages/hybridretrieval`, `packages/ingestion`,
`packages/vectorindex`, and their own transitive dependencies (a
Neo4j driver, pgvector, testcontainers) -- because those are the real
operations `packages/perf` benchmarks. None of that is a clean fit
for a generic concurrency-ramp harness that should be importable by
any package in this codebase without dragging in a graph database
driver. `ScaleTest` is implemented locally instead, with
`ScaleOperation`'s signature kept identical to `perf.WorkFunc` so a
function written for one composes with the other without adapting
it -- demonstrated in `scaling_behavior_test.go`, which builds a
`ScaleOperation` that calls into `ShardStrategy` and another that
calls into `BackpressureController`.

## PolicyConfig (config-as-data)

`PolicyConfig` (config.go) is the YAML-loadable
`ScalingPolicy`+`BackpressureConfig` pair a deployment actually runs
with, following `packages/config`'s (Phase 002) `Default()`-plus-
optional-YAML-file convention exactly:

```go
cfg, err := scalability.LoadPolicyConfig("/etc/verdex/scaling-policy.yaml")
if err != nil {
    log.Fatalf("scalability: %v", err)
}
```

An empty path is a no-op that returns `DefaultPolicyConfig()`
unchanged. A file only needs to mention the fields it wants to
override -- see `doc/policy.example.yaml`, a real fixture loaded by
`TestLoadPolicyConfigExampleFile` (not just a decorative sample) that
documents every field:

```yaml
scaling:
  min_replicas: 2
  max_replicas: 20
  target_metric: 0.70
  upper_tolerance: 0.15
  lower_tolerance: 0.15
  scale_up_step: 1
  scale_down_step: 1

backpressure:
  max_in_flight: 100
```

`PolicyConfig` does not add its own environment-variable or
named-profile layer -- those remain `packages/config`'s
general-purpose, cross-cutting job. A deployment wanting env/profile
overrides for its scaling policy embeds `PolicyConfig` as a field of
its own service-specific config superset, exactly the pattern
`packages/config.Config`'s own doc comment already describes
("directly or embedded in a service-specific superset"), and gets
env/profile layering for free from `packages/config`'s existing
`Loader` rather than this package reimplementing that machinery.

## Tests for scaling behavior (task 9)

Every file above ships unit tests alongside it
(`contract_test.go`, `partitioner_test.go`, `shard_test.go`,
`policy_test.go`, `backpressure_test.go`, `capacity_test.go`,
`scaletest_test.go`, `config_test.go`). `scaling_behavior_test.go`
additionally composes multiple pieces together in plausible
end-to-end scenarios a per-type test cannot catch:

- An autoscaling `Decide` and an independent `EstimateReplicas`
  capacity estimate both reconciled against the same
  `PolicyConfig.Scaling` bounds loaded from the real
  `doc/policy.example.yaml` fixture.
- `Partitioner` routing composed with one `BackpressureController` per
  partition, confirming saturating one partition never affects
  another partition's admission decisions.
- `ShardStrategy.Rebalance` computing a real 4-to-6-shard growth plan,
  followed by a `ScaleTest` run proving the new topology is actually
  usable added capacity (higher concurrency yields higher measured
  throughput with zero errors).
- `Contract.Verify` used as an iterative CI-style gate: a partially
  answered checklist fails, a fully answered one passes.

Concurrency-sensitive tests (`TestBackpressureConcurrentLoadShedsAtThreshold`)
are run with `-race` as part of this phase's definition of done.

## Why no migration

Every type in this package is either a pure function (`Decide`,
`EstimateReplicas`, `Rebalance`), an in-process runtime primitive
(`BackpressureController`, `Partitioner`, `ShardStrategy`), or a
file-loaded policy (`PolicyConfig`) -- none of them hold durable,
per-tenant records a real deployment cannot afford to lose on
restart. This mirrors `packages/perf`'s (Phase 091) own precedent
exactly: that phase's `BenchmarkRun` history is in-memory only, with
no Postgres migration, because it is an engineering/CI artifact
rather than tenant data. `packages/scalability`'s toolkit is
operated the same way -- by an autoscaler process, a CI gate, or an
engineer running a capacity plan -- not by a tenant-facing request
handler persisting durable per-tenant state. Should a later phase
persist, say, a history of `Decision`s or `Report`s per tenant and
expose it through a tenant-facing API, that phase should add the
migration then.

## Why no new identity.Permission constants

Following `packages/perf`'s own precedent exactly:
`packages/scalability` has no per-tenant durable secrets, no
tenant-scoped repository, and no `Engine`-style façade gating
reads/writes on an authenticated actor -- its types are operated by
CI, infrastructure automation, and engineers (an autoscaler
evaluating `Decide` on a metrics-scrape interval, a deployment
pipeline evaluating `Contract.Verify` as a merge gate), not by
tenant-facing request handlers. Unlike
`compliance`/`privacy`/`threatmodel`/`securitytesting`/
`vulnmanagement`/`backupdr`/`integration` (each of which added a
`PermView*`/`PermManage*` pair because it gates tenant-facing
reads/writes), this phase deliberately adds none. Should a later
phase expose any of this package's decision logic through a
tenant-facing API, that phase should add the permission pair then,
following the exact precedent this platform's
`packages/identity/permission.go` already establishes.
