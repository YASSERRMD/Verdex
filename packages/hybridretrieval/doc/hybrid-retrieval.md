# Hybrid retrieval engine (`packages/hybridretrieval`)

Phase 044 gives Verdex a single fused retrieval entry point over the two
retrieval mechanisms built in prior phases: `packages/vectorindex`'s
semantic recall (Phase 041) and `packages/traversal`'s dynamic graph
expansion (Phase 043). This document explains the fusion algorithm, the
tunables a caller can adjust, and exactly where this package's
responsibility starts and stops relative to the packages it composes.

## Composes with, does not duplicate

| Package | Owns | This package's relationship |
|---|---|---|
| `packages/vectorindex` | `VectorStore`/`InMemoryVectorStore`, embedding-based top-K semantic search, `ScoredResult`. | Read-only consumer via `VectorStore.Query`. `hybridretrieval` never embeds text, builds a `VectorRecord`, or reimplements cosine similarity — a caller is expected to have already indexed a case (via `vectorindex.IndexingService`) before running a `HybridQuery` against it. |
| `packages/traversal` | `Query`/`Walker`, bounded multi-hop graph traversal, `Path`/`Result`, `Path.Explain()`. | Read-only consumer via `Walker.Execute`. `hybridretrieval` builds `traversal.Query` values from its own `ExpansionHop` vocabulary and reads back `Path`/`Result`, but never touches a `graph.GraphStore` directly — all graph reads go through `traversal.Walker`. |
| `packages/graph` | `GraphStore` (used only to construct a `traversal.Walker`). | Not queried directly; see above. |
| `packages/irac` | `Node`/`NodeType`. | Read-only, via `vectorindex.VectorRecord.NodeType` and `traversal.PathNode.Type`. |

This package's own state is limited to a `VectorStore` reference and a
`*traversal.Walker`: it holds no index, no graph, and no cache of its own.
Every piece of retrieval logic that already exists in `vectorindex` or
`traversal` is called through, not reimplemented — this package's actual
contribution is exclusively the *fusion* of their two ranked outputs into
one.

## The `HybridQuery` shape

A `HybridQuery` names one semantic query vector, an optional structural
anchor node, and an ordered sequence of named expansion hops:

```go
query := hybridretrieval.NewHybridQuery(caseID, queryVector).
    WithAnchor(issueID).
    WithExpansion(hybridretrieval.ExpansionGoverningRule).
    WithFilter(hybridretrieval.Filter{JurisdictionCode: "us-ny"}).
    WithBudget(200 * time.Millisecond)

retriever, _ := hybridretrieval.NewRetriever(vectorStore, graphStore)
result, err := retriever.Retrieve(ctx, query)
```

`ExpansionHop` deliberately exposes only the three named legal-reasoning
hops `traversal.Query` already provides as builder methods
(`ViaGoverningRule`/`ViaControllingPrecedent`/`ViaDistinguishingFacts`),
not `traversal`'s general-purpose `Via(EdgeType, Direction, NodeType)`
escape hatch. A hybrid retriever's expansion policy is expected to be one
of these three fixed, legally-meaningful shapes; a caller needing a fully
custom hop sequence should call `traversal` directly and feed the result
into this package's fusion functions (`fuse` is exported implicitly
through `Retrieve`'s architecture, but `Retriever` itself is the intended
integration surface for the common case).

## The retrieval pipeline

`Retriever.Retrieve` runs four phases in order:

### 1. Vector recall

`runVectorRecall` (`vectorrecall.go`) calls `VectorStore.Query` once with
`HybridQuery.Vector`, `VectorTopK` (default `DefaultVectorTopK` = 10), and
`Filter` translated to `vectorindex.MetadataFilter`. If `Vector` is empty
(a pure structural query seeded only by `AnchorNodeID`), this phase is
skipped entirely — not an error, since `AnchorNodeID` already gives graph
expansion somewhere to start.

### 2. Graph expansion

Every distinct vector-recall hit's node ID, plus `AnchorNodeID` if set,
becomes a seed for graph expansion (`expansionSeeds` in `retriever.go`).
For each seed, `expandSeed` (`expansion.go`) builds a `traversal.Query`
from `HybridQuery.ExpansionHops`/`MaxExpansionDepth`/the two resolver
fields, executes it via `Walker.Execute`, and keeps the best-scoring
`traversal.Path` per newly-reached node (a node can appear in more than
one `Path` from the same seed; only the highest-`Path.Score` one is kept).
Results are capped per seed at `MaxPerAnchor` (default
`DefaultMaxPerAnchor` = 5) before being handed to fusion — see "Diversity
and dedup" below for why this cap exists at the per-seed stage as well as
the final result stage.

A seed whose ID does not resolve to a graph node (`traversal.
ErrStartNodeNotFound`) yields no expansion hits rather than failing the
whole query: graph expansion is best-effort enrichment layered on top of
vector recall, never a hard requirement for a hybrid query to succeed.

### 3. Reciprocal rank fusion

`fuse` (`fusion.go`) merges the ranked vector-hit list and the ranked
(deduplicated-by-node, best-score-wins) graph-hit list using **reciprocal
rank fusion (RRF)**:

```
CombinedScore(item) = 1/(k + vectorRank) + 1/(k + graphRank)
```

where a rank of 0 (the item did not appear in that list at all)
contributes 0, and `k` is `RRFConstant` (default `DefaultRRFConstant` =
60, the constant recommended by the original RRF paper — Cormack, Clarke
& Buettcher, 2009).

**Why RRF instead of a weighted sum of raw scores:** `VectorScore` (a
cosine similarity in `[-1, 1]`, typically `[0, 1]` for the embeddings this
system uses) and `traversal.Path.Score` (`DefaultScoreFunc`'s inverse-
depth score, or whatever a caller's custom `ScoreFunc`/`ConfidenceWeightedScoreFunc`
produces) live on **incomparable scales that this package has no principled
way to normalize** — `traversal.Query.ScoreFunc` is a fully pluggable
function type (see `packages/traversal`'s doc), so `hybridretrieval`
cannot assume anything about the numeric range or distribution of graph
scores a given deployment's `ScoreFunc` will produce. RRF sidesteps this
entirely: it only cares about *rank position* within each list, which is
scale-invariant by construction. This is the same reason RRF is the
standard fusion technique for combining a BM25 lexical-search ranking
with a dense-vector ranking in general information-retrieval systems — the
two signals here (embedding-space similarity vs. graph-traversal
proximity) are exactly analogous in kind.

Each fused `Item` carries `VectorScore`/`VectorRank` and
`GraphScore`/`GraphRank` independently (not just the combined result) so a
caller that *does* want to build its own normalization/weighting on top
can do so from the raw per-signal numbers `Retrieve` already computed,
without this package forcing one fusion policy as the only option.

### 4. Dedup, diversity, and TopK

`dedupAndDiversify` (`dedup.go`) trims the RRF-sorted item list down to
`TopK` (default `DefaultTopK` = 10), applying two independent controls
along the way:

- **Per-anchor cap** (`MaxPerAnchor`, same default as the per-seed
  expansion cap): at most `MaxPerAnchor` items sharing one non-empty
  `AnchorNodeID` survive, in score order. This prevents one
  densely-connected seed node (e.g. a rule governing a dozen issues) from
  crowding out every other seed's contribution to the final result purely
  by having a larger fan-out. Applying the same cap both at the raw
  per-seed expansion stage (`expandSeed`'s `maxPerAnchor` parameter) and
  again here is deliberate: the first cap bounds how much work/memory one
  seed's expansion can consume, and the second bounds the final result
  shape after RRF has re-ranked everything (a node could be a low-scoring
  expansion result from its own seed but still worth keeping if its
  `CombinedScore` after fusion is high).
- **Near-duplicate text collapse**: items whose `Text` normalizes to the
  same value (lowercased, whitespace-collapsed) as an already-kept item
  are dropped, keeping the first (highest-`CombinedScore`) occurrence.

**Why exact-normalized-text dedup instead of full MMR:** Maximal Marginal
Relevance (MMR) is the standard technique for diversity-aware re-ranking,
but a real MMR pass needs a pairwise similarity function between
*arbitrary* candidate pairs — which here would mean either embedding every
graph-only candidate (this package has no `EmbeddingService` dependency
and should not gain one just for diversity scoring, per the "no hardcoded
provider" rule) or defining a second, ad hoc text-similarity metric
whose behavior would be difficult to reason about alongside RRF's already
rank-based fusion. Exact-normalized-text collapse handles the concrete
case this package needs to guard against — the same underlying fact or
rule surfacing twice under two IDs, or a vector hit and its own graph-
reached copy colliding on text — without introducing a second scoring
system. A future phase wanting true embedding-distance MMR can layer it
on top of `Retrieve`'s output (every `Item` already carries `NodeID`
sufficient to re-fetch a vector for true MMR) without this package's
fusion internals needing to change.

## Filters applied consistently across both paths

`Filter` (`types.go`) mirrors `vectorindex.MetadataFilter`'s exact shape
(`JurisdictionCode`, `CategoryCode`, `PartyID`, each optional) rather than
inventing a parallel filter type, per the phase plan's "reuse
vectorindex's metadata filter shape where possible" guidance.

- **Vector-recall path**: `Filter` is translated to a
  `vectorindex.MetadataFilter` and passed straight into
  `vectorindex.QueryRequest.Filter` — filtering happens *before* ranking,
  exactly as `vectorindex.InMemoryVectorStore.Query` already implements
  it (see `packages/vectorindex`'s own doc).
- **Graph-expansion path**: a `traversal.PathNode` carries no
  jurisdiction/category/party metadata of its own (`packages/traversal`
  is deliberately schema-thin — see its doc's "no hardcoded provider"
  section). To apply the identical `Filter` to graph-discovered nodes,
  `HybridQuery.MetadataLookup` (a caller-supplied `func(nodeID string)
  (NodeMetadata, bool)`) resolves the same three fields per node ID.
  `filterGraphHits` (`filter.go`) is **conservative**: a non-empty
  `Filter` with no `MetadataLookup` configured, or a lookup that can't
  resolve a given node, excludes that node rather than letting it through
  unfiltered. A zero-value `Filter` never consults `MetadataLookup` at
  all — an unrestricted query should never require metadata to be
  resolvable.

Vector-recall hits are never re-filtered against `Filter` a second time
inside this package: `VectorStore.Query` already guarantees every
returned record matches, so re-checking would be redundant.

## Explanation of retrieval path

Every `Item` carries:

- `Path` — one of `RetrievalPathVector`, `RetrievalPathGraph`, or
  `RetrievalPathBoth`, naming which signal(s) surfaced it.
- `Explanation` — a human-readable string. For a vector-only hit, this is
  `"vector similarity (rank N, score S)"`. For a graph-reached hit, this
  appends `"graph expansion from <anchor> (rank N): <path.Explain()>"`,
  reusing `traversal.Path.Explain()`'s rendering verbatim rather than
  building a second explanation format — `packages/traversal`'s own doc
  calls out that `Path.Explain()` exists specifically to support this
  phase's explanation requirement.
- `AnchorNodeID` — which seed node the graph-expansion path (if any)
  started from, so a caller can trace a `Both`/`Graph` item back to its
  originating vector hit or the query's own `AnchorNodeID`.

## Latency budget controls

`HybridQuery.Budget` (a `time.Duration`, zero meaning "unbounded") is
tracked by `budgetTracker` (`budget.go`), started the moment `Retrieve` is
called:

- **Vector recall is never budget-gated.** It is treated as the fast,
  always-available floor of a hybrid query's latency — skipping or
  truncating it would defeat the purpose of a "hybrid" query (there would
  be nothing to expand from, and no results at all under extreme time
  pressure). Every latency-budget decision applies only to the graph-
  expansion phase.
- **If the budget is already exhausted before expansion starts** (e.g.
  vector recall alone consumed it, or `Budget` was set unreasonably
  small), expansion is skipped outright. `Result.ExpansionSkipped` is set,
  and the result is vector-recall-only — every `Item.Path` is
  `RetrievalPathVector`.
- **If the budget runs out partway through expanding seeds,** remaining
  seeds are skipped (not started) and `Result.ExpansionTruncated` is set.
  Seeds already fully expanded before the deadline keep their results;
  this is a coarse per-seed truncation rather than a mid-traversal
  cutoff, because `traversal.Walker.Execute` does not itself expose a
  mid-execution cancellation checkpoint finer than the `context.Context`
  it already accepts — the derived, deadline-bound `context.Context`
  passed to `Walker.Execute` (`budgetTracker.withDeadline`) gives
  `Walker` the opportunity to fail fast on its own internal `ctx` checks
  for the *current* seed, while the retriever's own seed-loop check stops
  it from *starting* the next one.
- **`MaxExpansionDepth`** (mirroring `traversal.Query.MaxDepth`) is a
  separate, non-time-based control: it bounds how many
  `ExpansionHops` are walked per seed regardless of budget. A caller
  under a known tight latency requirement can combine a short `Budget`
  with a shallow `MaxExpansionDepth` to bound both wall-clock time and
  worst-case traversal breadth.

## What this package deliberately does not do

- **No embedding generation.** A `HybridQuery.Vector` must already be an
  embedded query vector; this package never calls an `embedding.
  EmbeddingService` (consistent with the "no hardcoded provider" rule —
  see `packages/vectorindex`'s identical stance).
- **No index building or tree mutation.** `Retriever` only reads from an
  already-indexed `VectorStore` and an already-populated `GraphStore`
  (via `Walker`). Indexing is `vectorindex.IndexingService`'s job; tree
  assembly is `packages/treeassembly`'s.
- **No new graph-edge semantics.** `ExpansionHop`'s three values map
  directly onto `traversal.Query`'s three named hop builders; this
  package adds no new hop kinds and does not reach around `traversal`'s
  `PrecedentResolver`/`DistinguishingFactResolver` seams — `HybridQuery`
  simply forwards its own resolver fields to every `traversal.Query` it
  builds.
