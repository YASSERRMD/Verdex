// Package perf is Phase 091: benchmarking, budgeting, and documenting this
// platform's performance characteristics rather than re-architecting any
// existing package. It draws on the hybrid retrieval pipeline added in
// Phase 044 (packages/hybridretrieval), the graph-traversal walker added in
// Phase 043 (packages/traversal), the ingestion orchestrator added in
// Phase 029 (packages/ingestion), the indexed graph store added in Phase 032
// (packages/graph), and the regression-detection shape established by
// Phase 062 (packages/reasoningeval), composing them into a benchmarking and
// performance-governance layer rather than duplicating any of them.
//
// # What is new here
//
//   - Budget / Measurement / Verdict / Evaluate (budget.go): a
//     named-operation -> target-latency(p50/p95/p99)+throughput mapping,
//     with real, concrete numbers for hybrid_retrieval, graph_traversal, and
//     ingestion_pipeline (the three benchmark targets below). Evaluate
//     compares an observed Measurement against the named operation's budget
//     and reports a per-dimension pass/fail Verdict, not a stub that always
//     reports passing (task 2).
//   - Real Go benchmarks (benchmark_ingestion_test.go,
//     benchmark_retrieval_test.go, benchmark_traversal_test.go): actual
//     `func BenchmarkX(b *testing.B)` functions exercising real code paths --
//     ingestion.IngestionOrchestrator.Process, hybridretrieval.Retriever.
//     Retrieve, and traversal.Walker.Execute -- over in-memory fixtures built
//     by benchmark_helpers_test.go, runnable via `go test -bench=. -benchmem
//     -run=^$ ./...` (task 3).
//   - Cache[K, V] (cache.go): a generic, mutex-guarded, TTL-based cache
//     wrapper with Get/Set/Invalidate and an injectable clock for
//     deterministic expiry testing. Documented (not wired) as a candidate
//     for packages/hybridretrieval's and packages/traversal's hot paths --
//     see "what this phase does NOT modify" below (task 4).
//   - Recommendation / graph-optimization checklist (recommendations.go,
//     doc/graph-optimization-checklist.md): a structured, actionable list of
//     indexing recommendations for packages/graph's existing index.go,
//     grounded in what that file actually does today (task 5).
//   - Batcher[T] (batcher.go): a generic, concurrency-safe async batching
//     utility flushing on a size threshold or a time threshold, whichever
//     comes first (task 6).
//   - LoadTest (loadtest.go): a configurable concurrency/duration-or-
//     iteration load-test runner aggregating p50/p95/p99 latency and error
//     rate from real sampled latencies, using a documented nearest-rank
//     percentile convention (task 7).
//   - ResourceLimits / Limiter (limits.go): a semaphore-based concurrency
//     limiter (stdlib buffered channel, no new dependency) plus a
//     ResourceLimits type naming per-operation-class concurrency ceilings
//     for ingestion/retrieval/traversal (task 8).
//   - BenchmarkRun / Store / InMemoryStore / DetectRegression (regression.go,
//     store.go): a historical benchmark-run record plus a thread-safe
//     in-memory store and a regression detector, mirroring
//     packages/reasoningeval's RegressionDetector+Store shape applied to
//     BenchmarkRun instead of QualityScore (task 9).
//   - doc/performance.md and this doc.go: the full write-up of the above,
//     the Budget target table, and how to run the benchmark suite
//     (task 10).
//
// # What this phase does NOT modify
//
//   - packages/graph: index.go, inmemory.go, and every other file are read
//     and referenced (recommendations.go cites index.go's real
//     inMemoryIndex/byType/indexMigrations shape by name), but never edited.
//     This phase's Recommendation list documents proposed changes; it does
//     not implement them.
//   - packages/hybridretrieval: Retriever/HybridQuery/Result are exercised
//     read-only by benchmark_retrieval_test.go via their existing public
//     constructors. No file in that package is touched, and Cache[K, V] is
//     documented as a candidate adopter for its future use, not wired into
//     it.
//   - packages/traversal: Walker/Query/Result are exercised read-only by
//     benchmark_traversal_test.go the same way. traversal already has its
//     own Cache (packages/traversal/cache.go) for query-result caching; this
//     phase's Cache[K, V] is a separate, general-purpose utility and is not
//     used to replace or wrap traversal's cache.
//   - packages/ingestion: IngestionOrchestrator.Process is exercised
//     end-to-end by benchmark_ingestion_test.go, with every
//     OrchestratorConfig field left at its documented in-memory default
//     except STT, which is wired with an isolated stt.Registry carrying
//     stt.DefaultNoOpSTTProvider registered under "noop" -- mirroring
//     packages/ingestion's own newTestOrchestrator test helper
//     (orchestrator_test.go), since stt.DefaultRegistry (what
//     OrchestratorConfig.STT falls back to when left nil) starts empty and
//     has nothing self-registered into it at init time. No stage, retry,
//     or queue logic is changed.
//
// # What is explicitly reused, not duplicated
//
//   - packages/reasoningeval's RegressionDetector (Threshold field,
//     Compare-style comparison) and Store/InMemoryStore
//     (sync.RWMutex-guarded, in-process) shape is the structural precedent
//     BenchmarkRun/Store/InMemoryStore/DetectRegression follow -- this
//     package does not import packages/reasoningeval; DetectRegression
//     operates over this package's own BenchmarkRun records, which carry
//     latency percentiles and throughput rather than reasoning-quality
//     scores.
//   - packages/graph's index.go (indexMigrations, inMemoryIndex.byType) is
//     the real, unmodified object every Recommendation in
//     recommendations.go and doc/graph-optimization-checklist.md is grounded
//     in -- this package reads and cites it, never edits or forks it.
//   - vectorindex.VectorStore, graph.GraphStore, embedding.EmbeddingVector,
//     and irac.Node/irac.Edge remain the only vector-store, graph-store,
//     embedding, and reasoning-tree-node shapes in this codebase;
//     benchmark_helpers_test.go builds fixtures directly against their
//     existing exported constructors (vectorindex.NewInMemoryVectorStore,
//     graph.NewInMemoryGraphStore) rather than inventing parallel fixture
//     types.
//   - This phase deliberately does not add any packages/identity.Permission
//     constants: unlike compliance/privacy/threatmodel/securitytesting/
//     vulnmanagement/backupdr/integration, packages/perf has no per-tenant
//     durable secrets, no tenant-scoped repository, and no Engine-style
//     façade gating reads/writes on an authenticated actor -- its
//     BenchmarkRun history is an in-memory, process-local artifact, and its
//     benchmarks/utilities are operated by CI and engineers, not by
//     tenant-facing request handlers. Should a later phase persist
//     BenchmarkRun durably and expose it through a tenant-facing API, that
//     phase should add the permission pair then, following the exact
//     PermView*/PermManage* precedent this package's doc.go documents by
//     reference.
//
// See doc/performance.md for the full write-up and
// doc/graph-optimization-checklist.md for the graph-indexing
// recommendations.
package perf
