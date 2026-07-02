# Dynamic graph traversal queries (`packages/traversal`)

Phase 043 gives Verdex a dynamic, multi-hop query layer over an assembled
IRAC reasoning tree: given a case's tree in a `graph.GraphStore`, walk an
arbitrary, caller-specified sequence of hops at query time (`issue ->
governing rule -> controlling precedent -> distinguishing facts`, or any
other hop sequence a caller composes) and get back ranked, explainable
`Path`s. This document covers the query DSL, the semantics of each named
hop type, how this complements `packages/treeindex`, and the caching and
ranking model.

## Composes with, does not duplicate

| Package | Owns | This package's relationship |
|---|---|---|
| `packages/irac` | The `Node`/`Edge`/`NodeType`/`EdgeType` schema. | Read-only consumer. `traversal` never constructs or mutates an `irac.Node`/`irac.Edge`. |
| `packages/graph` | Tree storage (`GraphStore`, `InMemoryGraphStore`). | `Walker.Execute` reads a case's nodes and edges via `GraphStore.GetNode` and (opportunistically, via the same `edgeLister` capability-detection pattern `packages/treeindex` uses) `EdgesForCase`. |
| `packages/treeindex` | Precomputed, cached `PathKindRuleGroupedIssues` / `PathKindReasoningChain` paths, rebuilt per tree revision. | A sibling, not a dependency in either direction. `treeindex` answers a small fixed set of structural questions fast, from a cache; `traversal` answers arbitrary hop sequences, computed fresh (or served from its own `Cache`) per query. Neither package imports the other. |
| `packages/application` | `Origin`/`OriginatedRule`, `DistinguishingFact`. | Not imported. `traversal` defines its own `PrecedentResolver`/`DistinguishingFactResolver` function types instead, so a caller who already has `application.DistinguishingFact`-shaped logic can wrap it without this package taking a hard dependency — see "Resolver-backed hops" below. |

### Why not extend `treeindex` instead of adding a new package

`treeindex`'s own doc/tree-indexing.md predicted this package by name
("the future Phase 043 traversal package") and explicitly scoped itself
to *not* cover this: it precomputes and caches two fixed path shapes at
tree-build time, while this package walks arbitrary hop sequences at
query time. Folding dynamic queries into `treeindex` would mean every
`RebuildCase` call either explodes combinatorially (materializing every
possible hop sequence up front) or the package quietly grows a second,
query-time code path anyway — at which point it is two packages wearing
one name. Keeping them separate keeps `treeindex` a pure "materialize
and cache fixed shapes" package and this one a pure "walk whatever hops
the caller asks for, right now" package.

## The query DSL

A `Query` is built fluently:

```go
query := traversal.NewQuery(caseID, issueID).
    ViaGoverningRule().
    ViaControllingPrecedent().
    ViaDistinguishingFacts().
    WithMaxDepth(3).
    RankBy(myScoreFunc).
    WithPrecedentResolver(myPrecedentResolver).
    WithDistinguishingFactResolver(myFactResolver)

walker, _ := traversal.NewWalker(store)
result, err := walker.Execute(ctx, query)
```

Every builder method (`Via`, `ViaGoverningRule`, `ViaControllingPrecedent`,
`ViaDistinguishingFacts`, `WithMaxDepth`, `RankBy`,
`WithPrecedentResolver`, `WithDistinguishingFactResolver`) returns a new
`Query` value rather than mutating the receiver, so a partially-built
`Query` can safely be reused as a template for several variants (e.g. the
same starting hops evaluated at different `MaxDepth` bounds) without
manual slice cloning.

### Why a fluent builder over a bare struct literal

A struct-based query spec (`Query{CaseID: ..., Hops: []HopSpec{...}}`)
was considered and rejected as the *primary* API. The three named
legal-reasoning hops read far more clearly as chained method calls than
as a hand-assembled `HopSpec` slice, and the fluent form composes
naturally with per-hop options like `MaxDepth` and `RankBy`. `Query`'s
fields (`CaseID`, `StartNodeID`, `Hops`, `MaxDepth`, `ScoreFunc`, the two
resolvers) are still exported, so a caller that prefers assembling a
`[]HopSpec` programmatically can do so and assign it directly to
`Query.Hops`.

## Per-hop-type semantics

### `ViaGoverningRule` — issue to governing rule

Walks `irac.EdgeGoverns` in **Reverse**. The legal edge triple is `Rule
--governs--> Issue` (declared `FromID` is the rule, `ToID` is the issue),
so reaching the rule *from* the issue means walking against the edge's
declared direction — exactly the same "two of the four legal edge triples
point from the derived node back to what it derives from" observation
`packages/treeindex`'s doc/tree-indexing.md documents for its own
reasoning-chain assembly. This package makes the same choice, for the
same reason, and calls it out explicitly per-hop via `HopSpec.Direction`
rather than leaving direction implicit.

### `ViaControllingPrecedent` — rule to controlling precedent

Has **no literal `irac.Edge`** to walk: the IRAC schema does not encode
"this rule is controlled by that precedent" as an edge triple at all.
This hop is resolved by calling the `Query`'s `PrecedentResolver`
function once per live `RuleNode` in the current frontier. A `nil`
resolver (the default) is `NoPrecedents`, which always yields zero
results — a `Query` that never calls `WithPrecedentResolver` simply
finds nothing at this hop, rather than erroring.

`PrecedentResolver`'s signature (`func(ctx, irac.RuleNode)
([]irac.RuleNode, error)`) deliberately says nothing about *what* makes
one rule "control" another — that is a legal-domain judgment call left
to the caller, backed by whatever authority/citation metadata
`packages/precedent`/`packages/statute`/`packages/application` carry.
This package only defines where the question sits in a traversal.

### `ViaDistinguishingFacts` — precedent to distinguishing facts

Also has no literal edge, for the same reason: "this fact distinguishes
that precedent" is exactly `packages/application`'s `DistinguishingFact`
concept, but expressing it as a graph edge would require encoding a
one-off relationship type the core IRAC schema doesn't have room for.
Resolved via the `Query`'s `DistinguishingFactResolver`
(`func(ctx, caseID, irac.RuleNode) ([]irac.FactNode, error)`), called
once per live precedent-origin `RuleNode` in the frontier. A `nil`
resolver defaults to `NoDistinguishingFacts` (always zero results).

### `Via` — the general-purpose escape hatch

Any other hop — including ones treeindex already precomputes, like
`Application --applies_to--> Fact/Rule`, `Fact --supports--> Application`,
`Conclusion --concludes_from--> Application`, or their reverses — is
expressed via the general `Via(edgeType, direction, nodeTypeFilter)`
method. `HopKindGoverningRule` is really just `Via(irac.EdgeGoverns,
Reverse, irac.NodeRule)` with a documented label attached; the two are
interchangeable, and `HopKindCustom` and `HopKindGoverningRule` hops are
resolved through the exact same edge-walking code path
(`caseEdgeIndex.neighbors` in `edges.go`).

### Resolver-backed hops: why not import `packages/application` directly

`packages/traversal` never imports `packages/application`,
`packages/statute`, or `packages/precedent`. `PrecedentResolver` and
`DistinguishingFactResolver` are the seams a caller plugs real
domain logic into, the same "accept a local abstraction instead of a
hard import" pattern `packages/application` itself uses for `Origin`
(see `packages/application/origin.go`'s doc comment) to avoid importing
`packages/statute`/`packages/precedent`. A caller wiring this package
into a real retrieval pipeline is expected to supply resolvers backed by
`application.OriginatedRule`/`application.DistinguishingFact` logic,
translated to the bare `irac.RuleNode`/`irac.FactNode` values these
function types traffic in.

## Bounded depth and pruning

`Query.MaxDepth` bounds how many of `Query.Hops` are actually walked
(zero means "walk the full configured sequence"). Independently of
`MaxDepth`, `Walker.Execute` maintains one visited-node set for the whole
query execution: a single `Path` can never revisit a node it has already
passed through, though the same node can still appear as the destination
of more than one distinct `Path` (e.g. two issues sharing a governing
rule each produce their own one-hop `Path` to that rule). A `HopSpec`'s
`NodeTypeFilter`, when set, prunes candidates failing the filter before
they are ever added to the frontier — this is branch-level early
termination, not a whole-query abort.

`Result.Truncated` reports whether the walk stopped specifically because
it exhausted `MaxDepth` before consuming the full `Hops` sequence; it
does **not** fire when the walk simply runs out of reachable nodes
before `MaxDepth` (that is a normal, complete result, not a truncated
one) — see `TestWalker_ZeroHopFrontier_StopsCleanly` and
`TestWalker_MaxDepth_Truncates` for the two cases distinguished.

## Weighted-path ranking

Every `Path` in a `Result` is scored by the `Query`'s `ScoreFunc`
(`func(Path) float64`) and `Result.Paths` is sorted descending by that
score. `DefaultScoreFunc` (used when `ScoreFunc` is `nil`) favors shorter
paths — a reasonable, dependency-free default. `ConfidenceWeightedScoreFunc`
is offered as a ready-to-use alternative that layers in caller-supplied
per-node weights (e.g. derived from a precedent's `AuthorityScore` or a
statute's specificity) without this package importing either package —
same decoupling rationale as the resolver-backed hops.

## Traversal result shaping

`Path` (`result.go`) is the per-route result type: an ordered
`[]PathNode` (a minimal, immutable node projection, mirroring
`treeindex.NodeRef`'s "snapshot, not a live node" convention but defined
independently since the two packages don't depend on each other), a
`[]TraversedHop` describing how each subsequent node was reached
(`Hops[i]` produced `Nodes[i+1]`, with `FromIndex` naming the originating
node for fan-out shapes), and a cumulative `Score`. `Path.Explain()`
renders a short human-readable trace of the hop sequence, e.g.:

```
issue-1 --governing_rule(reverse:governs)--> rule-1 --controlling_precedent--> rule-9 --distinguishing_facts--> fact-3
```

`Result` wraps every discovered `Path` (already sorted by score) plus
`Truncated` and `VisitedCount` bookkeeping, mirroring the observability
role `treeindex.Stats` plays for that package.

## Caching

`Cache` (`cache.go`) is a small, dependency-free LRU built on
`container/list`, mirroring `packages/treeindex`'s own `lruCache`. It is
opt-in: `Walker.WithCache(cache)` returns a `Walker` that consults the
cache in `ExecuteCached`; a `Walker` without a configured cache behaves
identically to calling `Execute` directly.

### Invalidation model: revision-tagged keys, not eager purge

Unlike `treeindex`, which purges every cached entry for a case on each
`RebuildCase` call, `Cache` folds a per-case **current revision number**
into every entry's key (see `Cache.SetRevision` /
`traversal.ReindexOnRevision`). Bumping a case's revision does not walk
and evict that case's existing entries — they simply become permanently
unreachable (no future lookup can match their old revision number) and
are reclaimed by the ordinary LRU eviction policy over time. This trades
a small amount of transient memory for O(1) invalidation instead of
`treeindex`'s O(entries for that case) purge, a reasonable tradeoff since
a `Query`'s cached result set is typically much smaller than a full
`treeindex.PathIndex`.

`traversal.ReindexOnRevision(cache, revision)` mirrors
`treeindex.ReindexOnRevision`'s and `vectorindex`'s identically-shaped
hooks, so a caller wiring a "tree changed" event into every downstream
index/cache can treat all three uniformly.

### What's excluded from the cache key

`Query.cacheKey()` intentionally excludes `ScoreFunc`,
`PrecedentResolver`, and `DistinguishingFactResolver` — function values
can't be compared meaningfully, and keying on them would mean two
`Query`s with identical hop shapes but different (perhaps
freshly-closed-over) function values would never share a cache entry.
The entire scored, sorted `Result` is what gets cached and replayed
verbatim on a hit; this package does not re-run scoring or re-sort a
cached `Result` against a different `ScoreFunc` at read time. A caller
that changes `ScoreFunc` between calls with an otherwise-identical
`Query` will get back the previous call's ranking, not a re-ranked one,
until the cache entry is evicted or the case's revision changes — a
documented tradeoff of the current cache-key design.

## Relationship to the future Phase 044 hybrid retrieval consumer

The phase plan describes Phase 044 ("Hybrid retrieval engine") as fusing
vector recall (`packages/vectorindex`) with graph traversal — this
package. `Path.Explain()` exists specifically to support that consumer's
"explanation of retrieval path" requirement: a hybrid retriever expanding
a vector-recalled leaf node via `Walker.Execute` can attach
`Path.Explain()`'s rendering directly to a retrieval result's citation
trail without inventing its own explanation format. `Result.VisitedCount`
and `Result.Truncated` likewise give a hybrid retriever's latency-budget
logic (the plan's "latency budget controls" item) the bookkeeping it
needs to decide whether a traversal expansion completed fully or was cut
short.

## No hardcoded provider

`traversal` never imports `packages/provider`, `packages/embedding`, or
any LLM/embedding client. Every `Path` it produces is derived purely from
`irac.Node`/`irac.Edge` values read through a `graph.GraphStore`, plus
whatever a caller's own `PrecedentResolver`/`DistinguishingFactResolver`/
`ScoreFunc` chooses to do — this package itself makes no provider calls,
consistent with the overall project rule that no package outside
`packages/provider` and its adapters may hardcode a specific model
provider.
