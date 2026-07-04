# Performance benchmarking and budgeting (Phase 091)

This phase draws together the graph-traversal walker added in Phase 043
(`packages/traversal`), the hybrid retrieval pipeline added in Phase 044
(`packages/hybridretrieval`), the ingestion orchestrator added in Phase
029 (`packages/ingestion`), the indexed graph store added in Phase 032
(`packages/graph`), and the regression-detection shape established by
Phase 062 (`packages/reasoningeval`) -- into a benchmarking and
performance-governance layer: `packages/perf`.

## Goal

Give this platform a concrete, testable definition of "fast enough" for
its three heaviest real-time operations (hybrid retrieval, graph
traversal, ingestion), real Go benchmarks exercising each operation's
actual code path, a set of reusable performance utilities (cache,
batcher, load-test runner, concurrency limiter), and a way to track
historical benchmark runs and flag regressions over time -- without
rewriting or forking any of the packages it measures.

## What this package composes from, versus what is new

| Existing piece | What it already provides | What this phase adds |
|---|---|---|
| `packages/hybridretrieval` (Phase 044) | `Retriever`/`HybridQuery`/`Result`: fused vector + graph retrieval | `benchmark_retrieval_test.go` benchmarks `Retriever.Retrieve` over a real in-memory fixture; no file in that package is touched |
| `packages/traversal` (Phase 043) | `Walker`/`Query`/`Result`: bounded breadth-first graph walk | `benchmark_traversal_test.go` benchmarks `Walker.Execute`; that package's own `cache.go` is untouched and unrelated to this phase's `Cache[K, V]` |
| `packages/ingestion` (Phase 029) | `IngestionOrchestrator`/`Job`/`WorkflowState`: real end-to-end STT/OCR/normalize/segment/classify pipeline | `benchmark_ingestion_test.go` benchmarks `Process` end-to-end, wiring an isolated no-op `stt.Registry` the same way `packages/ingestion`'s own tests do |
| `packages/graph` (Phase 032) | `GraphStore`/`InMemoryGraphStore`, and `index.go`'s `indexMigrations`/`inMemoryIndex` secondary indexes | `recommendations.go` documents concrete, actionable indexing recommendations against `index.go`'s real, current behavior; no edit to `packages/graph` |
| `packages/reasoningeval` (Phase 062, by reference only) | `RegressionDetector`(`Threshold`)/`Store`/`InMemoryStore` shape for comparing runs over time | `BenchmarkRun`/`Store`/`InMemoryStore`/`DetectRegression` follow the identical structural shape, applied to latency/throughput instead of quality scores, without importing `packages/reasoningeval` |
| `packages/vectorindex` / `packages/embedding` / `packages/irac` | `VectorStore`/`EmbeddingVector`/`Node`/`Edge` -- the only vector-store, embedding, and reasoning-tree shapes in this codebase | `benchmark_helpers_test.go` builds fixtures directly against their existing exported constructors; no parallel fixture types invented |

## What this phase does NOT modify

- **`packages/graph`**: `index.go`, `inmemory.go`, and every other file
  are read and referenced by `recommendations.go` (which names
  `indexMigrations()`, `inMemoryIndex`, and `byType` by their real,
  current shape), but never edited. The `Recommendation` list documents
  proposed changes; it does not implement them. See
  `doc/graph-optimization-checklist.md`.
- **`packages/hybridretrieval`**: exercised read-only via its existing
  public constructors (`NewRetriever`, `NewHybridQuery`, the `With*`
  chain). `Cache[K, V]` (cache.go) is documented as a candidate future
  adopter for its per-query result caching, but is not wired into it.
- **`packages/traversal`**: exercised read-only via `NewWalker`/
  `NewQuery`/`Execute`. That package already has its own query-result
  `Cache` (`packages/traversal/cache.go`); this phase's `Cache[K, V]` is
  a separate, general-purpose utility, not a replacement or wrapper for
  it.
- **`packages/ingestion`**: `IngestionOrchestrator.Process` is exercised
  end-to-end, with every `OrchestratorConfig` field left at its
  documented in-memory default except `STT`, which is wired with an
  isolated `stt.Registry` carrying `stt.DefaultNoOpSTTProvider`
  registered under `"noop"` -- mirroring `packages/ingestion`'s own
  `newTestOrchestrator` test helper, since `stt.DefaultRegistry` (what
  `OrchestratorConfig.STT` falls back to when left nil) starts empty. No
  stage, retry, or queue logic is changed.

## Budget: concrete performance targets (task 2)

`Budget`/`Measurement`/`Verdict`/`Evaluate` (budget.go) give this
platform a named-operation-to-target mapping and a real comparison
function, not aspirational prose. Note: the brief for this phase calls
this type `PerfBudget`; it is named `Budget` here instead because
`golangci-lint`'s `revive` `exported` check (enabled in this
repository's `.golangci.yml`) flags `perf.PerfBudget` as a name stutter
against the `perf` package name, and no existing package in this
repository suppresses a lint finding with `//nolint` -- `Budget` is the
idiomatic-Go, lint-clean equivalent with identical behavior.
`DefaultBudgets()` returns:

| Operation | p50 | p95 | p99 | Min. throughput |
|---|---|---|---|---|
| `hybrid_retrieval` | < 150ms | < 500ms | < 900ms | >= 20 ops/sec |
| `graph_traversal` | < 80ms | < 300ms | < 600ms | >= 40 ops/sec |
| `ingestion_pipeline` | < 2s | < 5s | < 8s | >= 5 ops/sec |

`graph_traversal` is budgeted fastest: it is a bounded, in-memory
breadth-first walk over a single case's reasoning tree.
`hybrid_retrieval` is budgeted slower: it fuses a vector-recall floor
with a budget-sensitive graph-expansion enrichment on top (see
`packages/hybridretrieval/budget.go`). `ingestion_pipeline` is budgeted
slowest and lowest-throughput: it chains intake, transcription/OCR,
normalization, segmentation, and classification in sequence per job,
each a real (if no-op-backed in this benchmark's fixture) sibling
service call.

`Evaluate(operationName, observed)` returns a `Verdict` reporting
pass/fail per dimension (`P50`/`P95`/`P99`/`Throughput`) plus an overall
`Passed` that is true only when every dimension passes. Returns
`ErrUnknownOperation` for a name with no registered budget.

## Real Go benchmarks (task 3)

Three benchmark files exercise real code paths over in-memory fixtures
built by `benchmark_helpers_test.go`:

- **`benchmark_traversal_test.go`**: `traversal.Walker.Execute` over an
  in-memory `graph.GraphStore` populated with issue/rule node pairs
  connected by real `irac.Edge{Type: irac.EdgeGoverns}` edges, walked via
  both the named `ViaGoverningRule()` hop and the general-purpose
  `Via(EdgeType, Direction, NodeType)` builder, plus a fixture-size sweep
  (50/500/2000 issues).
- **`benchmark_retrieval_test.go`**: `hybridretrieval.Retriever.Retrieve`
  over an in-memory `vectorindex.VectorStore` (1000 synthetic vectors) and
  `graph.GraphStore` fixture, both with `ExpansionHops` unset
  (vector-recall-only) and with `ExpansionGoverningRule` configured
  (vector recall plus graph expansion), plus a corpus-size sweep
  (100/1000/5000 records).
- **`benchmark_ingestion_test.go`**: `ingestion.IngestionOrchestrator.
  Process` end-to-end over a synthetic audio `Job`, both sequentially and
  under `b.RunParallel`.

### How to run

```sh
go test -bench=. -benchmem -run=^$ ./packages/perf/...
```

`-run=^$` skips every non-benchmark test function so only benchmarks
execute; `-benchmem` reports allocations per operation alongside ns/op.

## Cache[K, V] (task 4)

`Cache[K, V]` (cache.go) is a generic, mutex-guarded, TTL-based cache
with `Get`/`Set`/`Invalidate`, safe for concurrent use, and built with an
injectable clock (`now func() time.Time`) so expiry tests run
deterministically without real sleeps. It is a standalone utility this
phase does not wire into any existing package -- see "what this phase
does NOT modify" above for why `packages/hybridretrieval` and
`packages/traversal` are documented candidates rather than modified
adopters.

## Graph-optimization recommendations (task 5)

`Recommendation` (recommendations.go) is a structured, actionable
record: `ID`, `Title`, `Rationale`, `TargetPackage`, `TargetFile`,
`Impact` (`Priority`), `Status` (`RecommendationStatus`).
`GraphIndexRecommendations()` returns four concrete recommendations
against `packages/graph/index.go`'s real, current behavior (two separate
single-column Neo4j indexes instead of one composite; an in-memory
`byType` index with no corresponding `byCase`+`byType` composite,
forcing an intersection scan for the platform's real `(case_id, type)`
query shape; and two lower-priority documentation/observability
recommendations). See `doc/graph-optimization-checklist.md` for the full
write-up.

## Batcher[T] (task 6)

`Batcher[T]` (batcher.go) is a generic, concurrency-safe async batching
utility: `Add` buffers an item, triggering a flush via a caller-supplied
`FlushFunc[T]` when either `MaxSize` items have accumulated or `MaxWait`
has elapsed since the first buffered item, whichever comes first. `Stop`
flushes any remainder and blocks until every in-flight flush completes.

## LoadTest (task 7)

`LoadTest(ctx, LoadTestConfig, WorkFunc)` (loadtest.go) drives a
caller-supplied operation with configurable concurrency and a
duration-or-iteration-count bound, aggregating `P50`/`P95`/`P99` latency
and `ErrorRate` from real sampled latencies.

**Percentile convention**: nearest-rank -- `rank = ceil(p/100 * n)`,
`index = rank - 1` (clamped to `[0, n-1]`). For a 100-element sorted
sample, p95 selects `sorted[94]` (the 95th-smallest value, 0-indexed),
not an interpolation between two neighboring elements. This is
deterministic and matches the convention most load-testing tools (wrk,
autocannon) use for "p95 latency."

## ResourceLimits + Limiter (task 8)

`ResourceLimits` (limits.go) names per-operation-class concurrency
ceilings (`Ingestion`/`Retrieval`/`Traversal`, mirroring `OperationName`'s
three benchmarked operations). `Limiter` enforces them with a stdlib
buffered-channel counting semaphore per class -- no new dependency is
added; `golang.org/x/sync/semaphore` is not imported, since a
`chan struct{}` of capacity `limit` is the standard idiomatic Go
semaphore and this repository's own `.golangci.yml`/dependency policy
favors not adding dependencies where the standard library already
suffices.

## BenchmarkRun history + regression detection (task 9)

`BenchmarkRun` (regression.go) is a historical run record: `RunID`,
`Operation`, `Measurement`, optional `TenantID`/`DeploymentTag`,
`RecordedAt`. `Store`/`InMemoryStore` (store.go) mirror
`packages/reasoningeval.Store`/`InMemoryStore`'s shape exactly
(`sync.RWMutex`-guarded, in-process, most-recent-first `List*`) applied
to `BenchmarkRun` instead of `QualityScore`. `CompareRuns`/
`DetectRegression` compare a current run's P95 latency and throughput
against the average of historical runs for the same `Operation`,
flagging a regression when either dimension crosses
`RegressionThreshold` (20%). `DetectRegression(current, historical)
bool` is the minimum contract this phase's brief calls for, implemented
as a thin wrapper over `CompareRuns`'s richer `RegressionResult`.

This is **in-memory only** -- no Postgres migration was added. See
"Why no migration" below.

## Why no migration

Every other recent phase (`compliance`, `privacy`, `backupdr`,
`vulnmanagement`, `securitytesting`, `integration`) added a
tenant-scoped Postgres migration because those packages hold durable,
per-tenant records a real deployment cannot afford to lose on restart
(controls, evidence, backup policies, findings, connector
configurations). `packages/perf`'s `BenchmarkRun` history is different
in kind: it is an engineering/CI artifact describing this codebase's own
performance over time, not tenant data, and following
`packages/reasoningeval`'s own precedent (Phase 062: `InMemoryStore` is
that package's real production `Store` implementation too, no Postgres
migration was ever added there either) an in-memory `Store` is the
correct-weight solution here. This also keeps the phase's blast radius
to `packages/perf` alone and avoids the Docker/testcontainers dependency
a Postgres-backed implementation would pull in.

## Why no new identity.Permission constants

`packages/perf` has no per-tenant durable secrets, no tenant-scoped
repository, and no `Engine`-style façade gating reads/writes on an
authenticated actor -- its benchmarks and utilities are operated by CI
and engineers, not by tenant-facing request handlers. Unlike
`compliance`/`privacy`/`threatmodel`/`securitytesting`/`vulnmanagement`/
`backupdr`/`integration` (each of which added a `PermView*`/`PermManage*`
pair because it gates tenant-facing reads/writes), this phase
deliberately adds none. Should a later phase persist `BenchmarkRun`
durably and expose it through a tenant-facing API, that phase should add
the permission pair then, following the exact precedent this platform's
`packages/identity/permission.go` already establishes.
