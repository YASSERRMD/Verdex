# Vector index over leaf nodes (`packages/vectorindex`)

Phase 041 gives Verdex's assembled IRAC reasoning trees a semantic-recall
index: given a case's tree, find the facts, rules, and conclusions whose
meaning is closest to a query, not just the ones matching by keyword or
graph position. This document explains the design, the leaf-projection
rule, and the extension point a real ANN backend would plug into.

## Composes with, does not duplicate

This package is infrastructure over three existing packages, not a
replacement for any of them:

| Package | Owns | This package's relationship |
|---|---|---|
| `packages/irac` | The `Node`/`NodeType` schema, typed wrappers (`FactNode`, `RuleNode`, `ConclusionNode`, ...), `SourceSpan`, `TreeRevision`. | Read-only consumer. `vectorindex` never constructs or mutates an `irac.Node`. |
| `packages/graph` | Tree storage (`GraphStore`, `InMemoryGraphStore`). | `ProjectLeaves` reads a case's nodes via `GraphStore.Traverse` rather than this package keeping its own copy of node content. |
| `packages/embedding` | Embedding generation, caching, and versioning (`EmbeddingService`). | `EmbedLeaves` calls `EmbeddingService.Embed` for every leaf's text. This package never calls an LLM provider directly — see `packages/embedding`'s own "no hardcoded provider" convention, which this package inherits by construction (it doesn't even import `packages/provider`). |

`packages/treevalidation`'s `CanFinalize` gate is a **caller** concern, not
enforced here: a tree should generally pass validation before being
indexed for retrieval, since an unvalidated tree may contain orphaned or
unsupported claims that would be misleading to surface in a semantic-
recall result. This package is infrastructure — it indexes whatever tree
content a caller gives it — so the finalize-before-index policy belongs in
whatever service wires `treevalidation` and `vectorindex` together (a
later phase), not in this one.

## The leaf-node projection rule

Not every `irac.Node` is worth indexing for semantic recall. The IRAC tree
has five `NodeType`s:

- `NodeIssue`, `NodeApplication` — **structural** nodes. An issue is a
  question the tree exists to answer; an application is the reasoning
  step connecting a rule to a fact. Neither asserts new content on its
  own — their `Text` is typically a framing or a paraphrase of the nodes
  they connect. A semantic-recall query ("find facts like this one",
  "find rules like this one") is never looking for a bare issue framing
  or an intermediate reasoning step.
- `NodeFact`, `NodeRule`, `NodeConclusion` — **content-bearing leaves**.
  These are the tree's actual assertions: what happened (fact), what the
  law says (rule/statute/precedent text), and what was concluded
  (conclusion, always carrying the non-binding `draft_analysis`
  guardrail label per `packages/irac/guardrail.go`).

`IsLeafNodeType` (`leaf.go`) implements this rule with an exhaustive
switch (enforced by this repo's `exhaustive` lint rule), and every
projection function in this package filters through it. This is
deliberately a fixed, small rule rather than a configurable one: the
IRAC schema has exactly five node types, and the structural/content-
bearing split is a property of the schema, not a per-case policy choice.

### Two projection paths, one output shape

`IndexableLeaf` (`leaf.go`) is the projected shape both paths produce:
`ID`, `NodeType`, `CaseID`, `JurisdictionCode`, `CategoryCode`, `PartyID`,
`Text`, `SourceSpans`.

- **`ProjectLeaves(ctx, graph.GraphStore, caseID, ProjectionOptions)`** —
  reads a case's nodes through `GraphStore.Traverse`. This is the
  "steady-state" path: most callers only have a persisted tree, not the
  original typed construction values.

- **`ProjectLeavesFromNodes([]irac.NodeLike, ProjectionOptions)`** —
  projects directly from typed nodes (e.g. `treeassembly.Tree.Nodes`,
  before persistence). Prefer this path when you have it.

These two paths exist, and produce different fidelity, because of a real
gap in `packages/graph`: `GraphStore.CreateNode`/`GetNode`/`Traverse` all
operate on the **base** `irac.Node` shape, which has no `Spans`,
`JurisdictionCode`, or `LegalFamily` fields — those live only on the
typed wrapper structs (`FactNode`, `RuleNode`, `ConclusionNode`). This is
not a bug introduced here; it's the same lossy flattening
`packages/graph/backup.go`'s `toNode`/`toNodeLike` round-trip already
accepts (see its `toNodeLike`, which reconstructs a `RuleNode` with
`JurisdictionCode`/`LegalFamily` hard-coded to `""`). Consequently:

- `ProjectLeaves` (via `GraphStore`) can only ever populate
  `JurisdictionCode`/`CategoryCode`/`PartyID` from `ProjectionOptions` —
  caller-supplied case-level metadata — and never populates
  `SourceSpans`.
- `ProjectLeavesFromNodes` (via typed nodes) retains `SourceSpans` and
  prefers a `RuleNode`'s own non-empty `JurisdictionCode` over the
  `ProjectionOptions` fallback.

A future phase that wants full fidelity through `GraphStore` would need
to extend `packages/graph`'s persisted shape (or add a side-channel
metadata store) — out of scope here; this package works within
`GraphStore`'s current contract rather than reaching around it.

`packages/jurisdiction` and `packages/category` are **not** imported:
`JurisdictionCode`, `CategoryCode`, and `PartyID` (`leaf.go`) are opaque
local string types, mirroring `packages/irac/jurisdiction.go`'s own
`JurisdictionCode`/`LegalFamily` convention — this package is
infrastructure over `irac` and `graph` and should not gain a hard
dependency on either metadata-schema package just to carry a filter
value.

## Embedding generation

`EmbedLeaves` (`embed.go`) calls `embedding.EmbeddingService.Embed` once
with every leaf's `Text`, in order, and zips the results back into
`VectorRecord`s (`recordFromLeaf`). Batching, caching, and provider
selection are entirely `packages/embedding`'s concern — this package
supplies text in, and consumes `EmbeddedText` (vector + model/provider
stamp) out.

## Storage: `VectorStore` and `InMemoryVectorStore`

`VectorStore` (`store.go`) is the storage-agnostic contract: `Upsert`,
`Query`, `Delete`, `DeleteCase`, `Health`. This mirrors
`packages/graph`'s `GraphStore`/`InMemoryGraphStore` split exactly:

- `Upsert` is an idempotent overwrite-by-ID, like `GraphStore.CreateNode`.
- `Delete`/`DeleteCase` are not errors when nothing matches, like
  `GraphStore.DeleteTree`.
- `Health` mirrors `graph.HealthCheck`'s "in-memory backend is always
  healthy; a real backend answers its own probe" pattern; `health.go`'s
  free-function `HealthCheck(ctx, store)` dispatcher exists purely for
  call-site consistency with `packages/graph`'s equivalent.

`InMemoryVectorStore` (`inmemory.go`) is the only implementation in this
phase: a `sync.RWMutex`-guarded map performing **exact, brute-force**
cosine-similarity search — every `Query` call scores every candidate
record and sorts. This is a deliberate v1 choice, the same one
`InMemoryGraphStore` made for graph storage: correct and simple now,
with the interface shaped so a real backend can slot in later (see
below).

`InMemoryVectorStore` enforces a single vector dimensionality per store
instance (established by the first `Upsert`), returning
`ErrDimensionMismatch` for anything else — brute-force cosine similarity
is undefined across mismatched dimensions, and a real ANN index would
reject this even more strictly (most ANN backends fix dimensionality at
index-creation time).

### Metadata filters

`MetadataFilter` (`types.go`) implements the category/jurisdiction/party
filters requested by the phase plan: `JurisdictionCode`, `CategoryCode`,
`PartyID`, each optional (empty = "no restriction on this dimension").
`QueryRequest.Filter` is applied *before* ranking (candidates that don't
match are excluded from scoring entirely, not just re-ranked down), and
`QueryRequest.CaseID` provides a fourth, always-available restriction
dimension on top of the three metadata fields, since "search within this
one case" is the most common retrieval scope.

### ANN extension point: `IndexConfig`

`IndexConfig` (`types.go`) models the tunable knobs a real Approximate
Nearest Neighbor backend would consume:

- `Metric` — `MetricCosine` (the only one `InMemoryVectorStore` actually
  implements), plus `MetricDotProduct`/`MetricEuclidean` declared as
  named constants for a future backend to implement.
- `DefaultTopK` — used when a caller's `QueryRequest.TopK` is zero.
- `EfSearch` — models HNSW's search-time candidate-list size.
- `Candidates` — models an IVF-style index's probed-partition count.

`EfSearch` and `Candidates` are **explicitly documented no-ops** for
`InMemoryVectorStore`: since it always does an exhaustive scan, there is
no approximation knob to tune. They exist so a caller can configure
`IndexConfig` once and pass the identical value to either
`InMemoryVectorStore` (ignored) or a future ANN-backed `VectorStore`
(consumed) without changing call sites when the backend is swapped —
this is the same reason `packages/graph`'s `TraversalQuery` carries a
`MaxDepth` field that only matters once `FromNodeID` is set.

The natural real-backend candidate is **pgvector**, already an indirect
dependency of this workspace (via `packages/graph`'s Postgres/pgvector
plumbing — see the `github.com/pgvector/pgvector-go` entry in
`packages/graph/go.mod`). A `PGVectorStore` implementing `VectorStore`
would translate `IndexConfig.Metric` into pgvector's distance-operator
choice (`<=>` for cosine, `<#>` for inner product, `<->` for L2) and
`EfSearch`/`Candidates` into `SET hnsw.ef_search` / `SET
ivfflat.probes` session variables, with `MetadataFilter` and `CaseID`
becoming ordinary `WHERE` clauses — no change required to this package's
`VectorStore` interface, `IndexingService`, or `ReindexOnRevision`.

## Hybrid score fields

`ScoredResult` (`types.go`) carries three score fields, only one of which
this package ever populates:

- `VectorScore` — the cosine similarity this package computed. Always
  set by `InMemoryVectorStore.Query`.
- `GraphScore` — a placeholder for a graph-traversal-based relevance
  score (e.g. proximity to a seed node, edge-weighted reachability).
  **Always `0`** in every result this package returns.
- `CombinedScore` — a placeholder for whatever fusion of `VectorScore`
  and `GraphScore` a later phase computes (weighted sum, reciprocal-rank
  fusion, etc). **Always `0`** in every result this package returns.

This phase deliberately does not implement graph traversal or score
fusion — that is Phase 044's "hybrid retrieval" — but the field shape is
fixed now so Phase 044 can consume `ScoredResult` without this package
needing a breaking change later.

## Re-index on tree revision

`ReindexOnRevision(ctx, IndexingService, irac.TreeRevision)`
(`reindex.go`) responds to a case's tree changing: it re-runs
`IndexingService.IndexCase` for `revision.CaseID`.

**This is a full re-embed, not a true delta, for v1.**
`irac.TreeRevision` (`packages/irac/version.go`) deliberately carries no
node list — it's a lightweight pointer ("case X has a new snapshot"), not
the snapshot's content. Diffing revision N against N-1 would require
either this package taking on a dependency on `packages/treeassembly`'s
`Tree`/`SnapshotStore` to fetch both revisions' content, or
`GraphStore` exposing a revision-scoped read (it does not —
`GraphStore.Traverse` always reads current state). Rather than reach
around either package's current contract, v1 accepts the simpler
tradeoff: re-project and re-embed every current leaf on every revision.

This is cheaper than it sounds: `embedding.EmbeddingService`'s own
`Cache` (keyed by content hash — see `embedding.CacheKey`) means
re-embedding a leaf whose `Text` hasn't changed since the last revision
is a cache hit, not a new provider call. `VectorStore.Upsert` is
similarly idempotent-by-ID, so re-indexing an unchanged leaf simply
overwrites its record with identical content. The actual new work done
on each revision is proportional to what changed, even though the code
path re-visits everything — a pragmatic full-recompute-over-a-cache
strategy rather than a fully delta-aware one, matching the tradeoff
already accepted by `packages/embedding` for its own cache and by
`InMemoryVectorStore` for its own brute-force query.

A future optimization narrowing this to a true delta (diffing node sets
directly, once a revision-scoped read is available) would only need to
change what set of `IndexableLeaf`s gets passed into
`IndexingService.IndexCase`'s embed/upsert steps — the `VectorStore`,
`EmbeddingService`, and `IndexingService` contracts would not need to
change.

## Health checks

`HealthCheck(ctx, VectorStore)` (`health.go`) mirrors
`packages/graph.HealthCheck`'s free-function dispatcher shape: nil store
is an error, `InMemoryVectorStore` is always healthy (nothing external to
fail against), and any other implementation answers its own `Health`
method. A production deployment wiring a real ANN backend's
`VectorStore` into `packages/observability`'s readiness handler should
register this the same way `packages/graph.Neo4jHealthChecker` is
registered today.
