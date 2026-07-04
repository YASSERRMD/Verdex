// Package scalability is Phase 092: a toolkit and documented
// architectural contract for scaling this platform's services and
// stores under load, rather than a rearchitecture of any existing
// service. It draws on the layered, file-based configuration pattern
// established in Phase 002 (packages/config), the per-provider
// circuit breaker and routing telemetry added in Phase 012
// (packages/router), the async job queue added in Phase 029
// (packages/ingestion), and the benchmarking/load-testing/
// resource-limiting shapes established in Phase 091 (packages/perf),
// composing them into a horizontal-scaling toolkit rather than
// duplicating any of them.
//
// # What is new here
//
//   - ChecklistAnswers / Contract.Verify / Report (contract.go): a
//     structured StatelessnessContract -- what a service must NOT do
//     to remain safely horizontally scalable (no in-process session
//     affinity, no local-disk-only state, no in-memory singleton
//     state, idempotent retries, externalized configuration, exposed
//     health checks, graceful shutdown) -- with real pass/fail
//     evaluation logic, not prose a reader has to self-certify
//     against. Covers tasks 1 ("make services horizontally scalable")
//     and 3 ("stateless service guarantees") as one contract, since
//     they are the same underlying requirement described from two
//     angles (task 1/3).
//   - Partitioner / NewConsistentHashPartitioner /
//     NewModuloPartitioner (partitioner.go): maps a work-item key to
//     one of N logical partitions/workers, via a real consistent-
//     hashing ring implementation or plain modulo hashing, with a
//     tested distribution-evenness guarantee (task 2).
//   - ShardStrategy / NewConsistentHashShardStrategy / Rebalance /
//     RebalancePlan (shard.go): maps a durable store key (tenant/case
//     ID) to a shard via the same consistent-hashing technique, plus
//     a real Rebalance plan calculator proving key movement stays
//     bounded when shard count changes, unlike naive modulo
//     resharding (task 4).
//   - ScalingPolicy / Decide / Decision (policy.go): a real
//     scale-up/scale-down/hold decision function over min/max
//     replicas, a target metric, and a hysteresis tolerance band that
//     provably prevents flapping when a metric oscillates near target
//     (task 5).
//   - BackpressureConfig / BackpressureController (backpressure.go):
//     a concurrency-safe, mutex-guarded in-flight-request counter
//     that sheds load past a configurable threshold and recovers
//     automatically, tested under real concurrent goroutine
//     contention with -race (task 6).
//   - CapacityInput / EstimateReplicas / CapacityEstimate
//     (capacity.go): a real arithmetic capacity-planning model --
//     ceil(target throughput / per-replica throughput), scaled by a
//     headroom fraction -- estimating required replica count from
//     historical throughput and a target SLA (task 7).
//   - ScaleOperation / ScaleTestConfig / ScaleTest / ScaleTestResult
//     (scaletest.go): a concurrency-ramp load-test harness recording
//     how throughput and error rate change as concurrency increases
//     across a sequence of configured levels (task 8).
//   - PolicyConfig / DefaultPolicyConfig / LoadPolicyConfig
//     (config.go): a YAML-loadable ScalingPolicy +
//     BackpressureConfig, following packages/config's config-as-data
//     convention (Default() plus an optional YAML file overlay)
//     rather than inventing a new configuration mechanism.
//   - scaling_behavior_test.go plus the tests alongside every file
//     above: real behavioral tests, including several that compose
//     multiple types together the way a real deployment would (tasks
//     9 and, via ScaleTest/BackpressureController's tests, 8).
//
// # What this phase does NOT modify
//
//   - No existing service in this codebase is rewritten to actually
//     run distributed, horizontally-scaled replicas. This phase ships
//     the toolkit and the documented contract (Contract/
//     ChecklistAnswers, ScalingPolicy, ShardStrategy, Partitioner,
//     BackpressureController, EstimateReplicas, ScaleTest); adopting
//     any of them into a specific service (caselifecycle, casesearch,
//     ingestion's orchestrator, etc.) is a decision and a change left
//     to that service's own future phase.
//   - packages/ingestion's JobQueue (queue.go) is not modified,
//     wrapped, or replaced. Partitioner composes with it (a caller
//     could use Partitioner to decide which of N separate JobQueue
//     instances to Enqueue onto) but this phase does not touch
//     packages/ingestion at all -- see "what is explicitly reused, not
//     duplicated" below.
//   - packages/router's CircuitBreaker/CircuitBreakerRegistry
//     (circuit_breaker.go) is not modified. That mechanism protects
//     against one failing LLM provider; this phase's
//     BackpressureController protects against overall system
//     overload, a broader and orthogonal concern (see doc/
//     scalability.md's composition table).
//   - packages/perf's LoadTest, Limiter, Budget/Measurement/Verdict,
//     and BenchmarkRun/Store are not modified. ScaleTest is a
//     deliberately separate, complementary implementation (see
//     scaletest.go's doc comment for why packages/perf was not
//     imported), and CapacityInput mirrors Budget/Measurement's shape
//     by name/tag only, without importing packages/perf.
//   - packages/config's Loader/Config/env/profile/secrets machinery is
//     not modified. PolicyConfig (config.go) follows the same
//     Default()-plus-YAML-file convention but does not add its own
//     environment-variable or named-profile layering; a deployment
//     wanting those embeds PolicyConfig in its own
//     packages/config.Config superset instead.
//
// # What is explicitly reused, not duplicated
//
//   - packages/config's Default()/loadYAMLFile precedent (Phase 002)
//     is the structural template PolicyConfig/LoadPolicyConfig follow
//     exactly (only fields a YAML document mentions overwrite the
//     Default() baseline) -- this package does not import
//     packages/config; PolicyConfig is deliberately smaller in scope
//     (no env vars, no named profiles) than packages/config.Config,
//     since those cross-cutting concerns remain packages/config's
//     job.
//   - packages/router's CircuitBreaker/CircuitBreakerRegistry (Phase
//     012) is the reference "one resilience primitive scoped to one
//     concern" precedent BackpressureController follows -- this
//     package does not import packages/router. CircuitBreaker
//     protects one provider from repeated failures; BackpressureController
//     protects one service instance from being overwhelmed by
//     request volume. Different failure mode, same "small,
//     concurrency-safe, mutex-guarded state machine" shape.
//   - packages/ingestion's JobQueue (Phase 029) remains the only
//     single-pipeline async handoff queue in this codebase.
//     Partitioner is explicitly designed to compose with it (deciding
//     which of N JobQueue instances receives a given key's work), not
//     to replace it -- see partitioner.go's doc comment.
//   - packages/perf's WorkFunc/LoadTest and ResourceLimits/Limiter
//     (Phase 091) are the reference shapes ScaleOperation/ScaleTest
//     and BackpressureController follow by signature/spirit
//     respectively, without importing packages/perf (see
//     scaletest.go's doc comment for the specific dependency-weight
//     reasoning). CapacityInput mirrors perf.Budget/perf.Measurement's
//     target-versus-observed shape by name, applied to
//     throughput/headroom instead of latency percentiles.
//   - This phase deliberately does not add any packages/identity.
//     Permission constants, following packages/perf's own precedent
//     exactly: packages/scalability has no per-tenant durable
//     secrets, no tenant-scoped repository, and no Engine-style
//     façade gating reads/writes on an authenticated actor. Its
//     types are operated by engineers, CI, and infrastructure
//     automation (an autoscaler evaluating Decide, a deployment
//     pipeline evaluating Contract.Verify), not by tenant-facing
//     request handlers. See doc/scalability.md for the fuller
//     rationale.
//   - No packages/persistence migration was added, for the same
//     reason packages/perf (Phase 091) added none: every type in this
//     package is either a pure function (Decide, EstimateReplicas,
//     Rebalance), an in-process runtime primitive
//     (BackpressureController, Partitioner, ShardStrategy), or a
//     file-loaded policy (PolicyConfig) -- none of them hold durable,
//     per-tenant records a real deployment cannot afford to lose on
//     restart. See doc/scalability.md's "Why no migration" section.
//
// See doc/scalability.md for the full write-up, the composition
// table, and worked examples.
package scalability
