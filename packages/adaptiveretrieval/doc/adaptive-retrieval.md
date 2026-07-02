# Adaptive retrieval structures (`packages/adaptiveretrieval`)

Phase 045 gives Verdex a cost-control layer that sits above the three
retrieval mechanisms built in prior phases: `packages/treeindex`'s
precomputed path index (Phase 042), `packages/traversal`'s live
graph-traversal DSL (Phase 043), and `packages/hybridretrieval`'s fused
semantic+structural retriever (Phase 044). This document explains what
problem this layer solves, when a caller should reach for it instead of
using one of those three packages directly, and the cost/staleness
tradeoffs it makes.

## The problem: full-graph pre-builds are not always worth it

`treeindex.Indexer.RebuildCase` materializes every rule-grouped-issue path
and every reasoning-chain path for an entire case, up front, regardless of
whether most of that structure is ever read. That cost is a good trade
when a case's tree is queried repeatedly from many different anchors — the
upfront cost is amortized over many cheap `LookupPaths` reads. It is a bad
trade for a case queried once, or queried from an anchor whose neighborhood
is small relative to the whole tree: a caller pays for structure it never
uses.

The opposite failure mode is naive unbounded `traversal.Walker.Execute`
calls with no cost ceiling: a densely-connected tree (many issues sharing
a governing rule, many precedents citing each other) can make a single
query's live traversal arbitrarily expensive, with no upper bound on nodes
visited or wall-clock time spent.

`adaptiveretrieval.Builder` answers a narrower question than either
extreme: *for this one query, build only the subgraph it actually needs,
right now, under a hard cost ceiling, and remember it for next time.*

## Composes with, does not duplicate

| Package | Owns | This package's relationship |
|---|---|---|
| `packages/traversal` | `Query`/`Walker`, bounded multi-hop graph traversal, `Path`/`Result`. | Read-only consumer via `Walker.Execute`. Every adaptive build is a `traversal.Query` translated from an `AdaptiveQuery`'s anchor and hops (see `translate.go`). |
| `packages/treeindex` | `Indexer`, full-case `RebuildCase`/`LookupPaths`. | Read-only consumer via `Indexer.LookupPathsWithDepth`, used only as the fallback when an adaptive build exceeds its budget (see `Builder.WithFallback`). `adaptiveretrieval` never calls `RebuildCase` itself — it is not in the business of maintaining a full-case index. |
| `packages/hybridretrieval` | `HybridQuery`, `ExpansionHop`, fused vector+graph retrieval. | `AdaptiveQuery` reuses `hybridretrieval.ExpansionHop`'s exact vocabulary (`ExpansionGoverningRule`/`ExpansionControllingPrecedent`/`ExpansionDistinguishingFacts`) rather than inventing a parallel one, and `FromHybridQuery` lets a caller that already ran a `hybridretrieval.Retriever.Retrieve` derive an `AdaptiveQuery` from that same query and its vector-hit count without re-deriving anything by hand. |
| `packages/graph` | `GraphStore` (used only to construct a `traversal.Walker`). | Not queried directly. |
| `packages/irac` | `TreeRevision`. | Read-only, via `Builder.SetRevision`/`ReindexOnRevision` for staleness detection. |

This package holds no graph state of its own beyond a `*traversal.Walker`,
a `Cache`, and an optional `*treeindex.Indexer` fallback reference.

## When to use this layer vs. the layers below it

- **Use `treeindex` directly** when a case's tree is queried repeatedly
  from many different anchors and the full-case rebuild cost is already
  amortized — especially if the case already has an event-driven
  `ReindexOnRevision` hook wired up, so the index is never stale.
- **Use `traversal` directly** when a caller has one specific,
  already-known multi-hop shape to walk and does not need cross-query
  reuse (e.g. a one-off diagnostic query).
- **Use `hybridretrieval` directly** when a caller has a semantic query
  vector and wants fused vector+graph ranking, and hybridretrieval's own
  per-query latency `Budget` is a sufficient cost control on its own.
- **Use `adaptiveretrieval`** when none of the above precomputation is
  known to be worthwhile yet — a first-touch query against a case that may
  never be queried again, or a query pattern that does not justify a full
  `RebuildCase`, but where an unbounded live traversal risks runaway cost
  on a large or densely-connected tree. This is the layer to reach for
  when the answer to "should I precompute this case's whole tree
  structure?" is "I don't know yet."

## The `AdaptiveQuery` shape

An `AdaptiveQuery` names an anchor node, an ordered sequence of named
hops (or lets `AdaptiveDepth` choose a default sequence), and how many
vector-recall hits already corroborated the query:

```go
q := adaptiveretrieval.NewAdaptiveQuery(caseID, issueID).
    WithHop(hybridretrieval.ExpansionGoverningRule).
    WithHop(hybridretrieval.ExpansionControllingPrecedent).
    WithVectorHitCount(2)

builder, _ := adaptiveretrieval.NewBuilder(graphStore, adaptiveretrieval.BuilderOptions{
    Budget: adaptiveretrieval.DefaultBuildBudget(),
})
builder = builder.WithFallback(treeIndexer) // optional

subgraph, err := builder.Build(ctx, q)
```

Or, deriving directly from an already-run `hybridretrieval.HybridQuery`
and its `Result.VectorHitCount`:

```go
q := adaptiveretrieval.FromHybridQuery(hybridQuery, topHit.NodeID, result.VectorHitCount)
subgraph, err := builder.Build(ctx, q)
```

## Cost control: `BuildBudget`

Every adaptive build is bounded by three independent limits, whichever is
reached first:

- `MaxNodes` — how many distinct nodes the build may visit (default 200).
- `MaxHops` — how many hops deep the build may walk (default 4).
- `MaxWallClock` — how long the build may run (default 250ms).

`MaxWallClock` is enforced two ways: via a `context.WithDeadline` passed
into `traversal.Walker.Execute` (so a `graph.GraphStore` implementation
that observes context cancellation mid-walk — e.g. a network-backed
store — aborts early), and via a direct wall-clock check immediately after
`Execute` returns (so a build that completes without ever checking `ctx`,
like `InMemoryGraphStore`'s synchronous map lookups, is still caught if it
ran past its deadline). This double enforcement is the "update-latency
safeguard": one caller's expensive query can never hold a build open past
`MaxWallClock`, regardless of what the underlying store does with context
cancellation, and never blocks any other concurrent `Build` call (`Builder`
holds no build-wide lock).

When a budget is exceeded, `Build` does not return a partial, silently
truncated subgraph — it degrades to the configured `treeindex` fallback
(see below), or returns `ErrBudgetExceeded`/`ErrNoFallbackAvailable` if no
fallback is configured.

## Adaptive depth: `AdaptiveDepth`

`AdaptiveDepth` resolves how many of a query's hops to actually walk based
on `AdaptiveQuery.VectorHitCount`:

- Few or no vector-recall hits (`VectorHitCount <= FewVectorHits`, i.e.
  0 or 1): walk the full configured hop sequence. There is no other signal
  to lean on, so the structural walk should do the most work.
- A moderate number of hits (`FewVectorHits < VectorHitCount <
  ManyVectorHits`): walk one fewer hop.
- Many hits (`VectorHitCount >= ManyVectorHits`, default 5): walk two
  fewer hops (minimum 1). Strong semantic corroboration already exists;
  the structural walk only needs to confirm, not compensate.

This mirrors the intuition behind `hybridretrieval`'s own reciprocal-rank
fusion — a result found by both signals is stronger evidence than one
found by only one — applied one layer up: instead of fusing two
fixed-depth signals after the fact, `adaptiveretrieval` varies the
structural signal's depth based on how much the semantic signal already
found, so the two together do a roughly constant amount of combined work
regardless of which one carries the load.

Fixed thresholds (rather than a continuous formula) are a deliberate
choice: a formula that changes depth on every off-by-one hit-count
difference would defeat cache sharing (`AdaptiveQuery.shapeKey` folds the
resolved depth into the cache key so that two queries landing on the same
effective depth can share a cached `Subgraph`), for no real accuracy
benefit — vector-hit counts are a noisy signal, not a precise measurement.

## Caching and staleness

`Cache` stores built `Subgraph`s keyed by `(caseID, query shape)`, where
"query shape" includes the anchor, the resolved hop sequence and depth,
and the fallback `EdgeType` filter — everything that determines what gets
built, but *not* the raw `VectorHitCount` (two queries that resolve to the
same effective depth share a cache entry even if their hit counts
differed).

Unlike `packages/traversal`'s `Cache` (which folds the tree revision
directly into the cache key, so a stale entry simply becomes unreachable),
`adaptiveretrieval.Cache` keeps a stale entry reachable and classifies a
`Get` as a **stale hit** rather than a miss. This distinction exists so
`BuildTelemetry.StaleRefreshes` can be reported separately from
`BuildTelemetry.CacheMisses` — an operator watching this package's
telemetry can tell "this case's tree keeps changing out from under the
cache" (high `StaleRefreshes`) apart from "this cache is cold" (high
`CacheMisses`, low `StaleRefreshes`).

A caller reacts to a new `irac.TreeRevision` the same way it would for
`treeindex`/`traversal`:

```go
err := adaptiveretrieval.ReindexOnRevision(builder.Cache(), revision)
// or, if only a *Builder is in scope:
err := builder.SetRevision(revision)
```

This does not eagerly rebuild anything — it only marks the case's cached
subgraphs stale so the *next* `Build` call for that case rebuilds instead
of serving a cached result that no longer reflects the tree.

## Fallback to `treeindex`

`Builder.WithFallback(idx)` attaches a `*treeindex.Indexer`. When an
adaptive build cannot complete within its `BuildBudget`, or the underlying
`traversal.Walker.Execute` call errors, `Build` calls
`idx.LookupPathsWithDepth` instead and converts the result into a
`Subgraph` with `Source == SourceFallback`, incrementing
`BuildTelemetry.FallbacksTriggered`. This assumes the case has already
been indexed by a call to `idx.RebuildCase` (or `treeindex.ReindexOnRevision`)
at some point — `adaptiveretrieval` never triggers that rebuild itself,
since doing so unconditionally on every budget-exceeded query would
reintroduce exactly the eager full-graph cost this package exists to
avoid. A caller wanting an always-available fallback should wire
`treeindex.ReindexOnRevision` into the same "tree changed" event that
feeds `adaptiveretrieval.ReindexOnRevision`.

If no fallback is configured, a budget-exceeded build surfaces
`ErrBudgetExceeded` (wrapped) or `ErrNoFallbackAvailable` directly, so a
caller in "adaptive-only" mode can decide for itself how to degrade
(retry with a smaller `AdaptiveQuery`, return a partial answer, etc.)
rather than silently falling back to a structure this package has no way
to verify is available or fresh.

## Telemetry

`Builder.Telemetry()` returns a `BuildTelemetry` snapshot: `Builds`,
`CacheHits`, `CacheMisses`, `NodesVisited`, `TotalBuildTime`,
`FallbacksTriggered`, and `StaleRefreshes`. These are the six numbers this
phase's cost/staleness tradeoffs are meant to be observed through:

- A high `CacheHits`-to-`Builds` ratio means the cost-control layer is
  earning its keep — most queries are being served from prior work.
- A high `FallbacksTriggered` count relative to `Builds` suggests the
  configured `BuildBudget` is too tight for the case's actual tree shape,
  or that this case would be better served by an eager `treeindex`
  rebuild instead of on-demand construction.
- A high `StaleRefreshes` count relative to `CacheMisses` means the case's
  tree is being revised faster than queries can reuse cached structure —
  another signal that the adaptive-build tradeoff may not be paying off
  for that particular case.
