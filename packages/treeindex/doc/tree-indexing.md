# Tree-structured indexing (`packages/treeindex`)

Phase 042 gives Verdex's assembled IRAC reasoning trees a structural
navigation index: given a case's tree, materialize and cache the
hierarchy-shaped lookup paths a caller actually needs ("what governs this
issue, and what was concluded from it?") instead of re-walking
`packages/graph`'s `GraphStore` on every request. This document explains
the schema, what "path" concretely means given the real `irac.Edge`
schema, how this complements `packages/vectorindex` and the future
Phase 043 traversal DSL, and the maintenance/caching tradeoffs chosen.

## Composes with, does not duplicate

| Package | Owns | This package's relationship |
|---|---|---|
| `packages/irac` | The `Node`/`Edge`/`NodeType`/`EdgeType` schema, `TreeRevision`. | Read-only consumer. `treeindex` never constructs or mutates an `irac.Node`/`irac.Edge`. |
| `packages/graph` | Tree storage (`GraphStore`, `InMemoryGraphStore`). | `Indexer.RebuildCase` reads a case's nodes and edges via `GraphStore.Traverse`/`GetNode`, and opportunistically via the `EdgesForCase` capability (see "Reading edges" below). |
| `packages/vectorindex` | Semantic/embedding-based recall over leaf nodes (`FactNode`, `RuleNode`, `ConclusionNode`). | A sibling index, not a dependency in either direction. `vectorindex` answers "which nodes mean something like this query text"; `treeindex` answers "what is the structural path from this node". Neither package imports the other. |
| `packages/treevalidation` | The `CanFinalize` hard gate over a `Report`. | Not enforced here. Indexing ideally happens after a tree passes `CanFinalize` — an unvalidated tree may contain orphaned or unsupported claims that are misleading to surface as a "reasoning chain" — but, like `vectorindex`, this package is infrastructure: it indexes whatever tree content a caller gives it, and the finalize-before-index policy belongs to whatever service wires `treevalidation` and `treeindex` together. |

### Why not one package?

`vectorindex` and `treeindex` solve genuinely different problems over the
same underlying `GraphStore` — one needs an embedding model and a
similarity metric, the other needs none of that and only cares about edge
topology — so splitting them keeps `treeindex` free of any embedding or
LLM-provider dependency (see "No hardcoded provider" below) and keeps
`vectorindex` free of any graph-topology-specific caching logic.

### Relationship to the future Phase 043 traversal package

A later phase is expected to add a dynamic, multi-hop, DSL-driven graph
traversal capability (querying the tree with arbitrary, caller-specified
hop patterns at request time). `treeindex` is deliberately *not* that:
it precomputes and caches a small, fixed set of path *shapes* (the two
`PathKind`s below) that are common enough to be worth materializing once
per tree revision rather than re-derived per request. The two are
complementary:

- `treeindex` — fixed path shapes, precomputed at build time, served from
  an in-memory cache. Fast, but only answers the questions its two
  `PathKind`s encode.
- Phase 043's traversal package — arbitrary path shapes, computed at query
  time via a DSL. Flexible, but pays a traversal cost per query.

A caller wanting "give me the reasoning chain for this issue, fast" uses
`treeindex`. A caller wanting "find every Fact within 4 hops of this
Application that also satisfies some ad hoc predicate" is squarely
Phase 043's job. Neither package is expected to depend on the other.

## What "path" concretely means here

The IRAC edge schema (`packages/irac/edge.go`) defines exactly four legal
edge triples, and none of them is a literal "issue governs sub-issue" or
"application concludes-to conclusion" edge:

```
Rule         --governs--------> Issue
Application  --applies_to-----> Fact
Application  --applies_to-----> Rule
Fact         --supports--------> Application
Conclusion   --concludes_from--> Application
```

Two things fall directly out of reading this table literally, both
documented up front rather than glossed over:

1. **There is no `ParentIssueID` on `irac.IssueNode`.** That field exists
   only on `packages/issue`'s pre-tree `CandidateIssue` (used during issue
   *decomposition*, before nodes are persisted into a `GraphStore`). By
   the time a tree is assembled and stored, that parent/sub-issue
   relationship is not represented as an edge at all. `treeindex`
   therefore does **not** import `packages/issue` and does not attempt to
   reconstruct a literal issue/sub-issue hierarchy — there is nothing in
   the persisted schema to reconstruct it from.
2. **Two of the four edges point from the derived node back to what it
   derives from, not from cause to effect.** `EdgeSupports` is
   `Fact --supports--> Application` (the fact is the edge's source), and
   `EdgeConcludesFrom` is `Conclusion --concludes_from--> Application`
   (the conclusion is the edge's source). A human-meaningful "what
   happened, in order" narrative — Issue, then the Rule governing it, then
   the Application, then the Facts that support it and the Conclusion
   drawn from it — requires walking `EdgeSupports` and `EdgeConcludesFrom`
   in **reverse** of their declared `FromID -> ToID` direction. `Hop.
   Reverse` (`path.go`) records exactly this for every hop treeindex
   assembles.

Given that, `treeindex` implements two concrete `PathKind`s, each mapping
cleanly onto edges that actually exist, rather than inventing a
"sub-issue" edge type the schema doesn't have:

### `PathKindRuleGroupedIssues` — the stand-in for "issue -> sub-issue"

Built by `buildRuleGroupedIssuePaths` (`grouping.go`) by walking every
`EdgeGoverns` edge in a case: one `Path` per `RuleNode`, whose `Nodes`
are `[rule, issue-1, issue-2, ...]` for every issue that rule governs.

This is **not** a literal parent/sub-issue relationship — it is issues
*related through a shared governing rule*, which is the closest concept
the actual schema supports. Two issues governed by the same rule are
treated as "related" the same way two sub-issues under one parent issue
would be in a system that had a literal parent-issue edge. This
substitution is called out explicitly (here, and in `doc.go`'s package
comment) rather than silently relabeled, since forcing a naming match to
"sub-issue" that isn't literally backed by an edge would be misleading.

### `PathKindReasoningChain` — the "rule -> application -> conclusion" path

Built by `buildReasoningChainPaths` (`chain.go`): for every `IssueNode`,
find its governing `RuleNode`(s) (`EdgeGoverns`, reversed), then the
`ApplicationNode`(s) that apply that rule (`EdgeAppliesTo`, reversed), then
every `FactNode` supporting that application (`EdgeSupports`, reversed)
and every `ConclusionNode` concluded from it (`EdgeConcludesFrom`,
reversed). This *is* the concrete "rule -> application -> conclusion"
structure the phase plan calls for — assembled head-first from the Issue
so the resulting `Path` reads as a coherent narrative, even though two of
its four hops are walked against their stored edge direction.

An `Application` can be supported by multiple `Fact`s and concluded-from
by multiple `Conclusion`s. Rather than materialize the combinatorial cross
product of every `(fact, conclusion)` pairing as separate `Path`s, one
chain includes every reachable `Fact` and `Conclusion` as sibling tail
nodes, each recording (via `Hop.FromIndex`) that it hopped directly from
the `Application`. See "Path is a small tree, not always a line" below.

## Path is a small tree, not always a line

`Path.Nodes` is a flat slice, but `Path.Hops` does not assume a strict
linear chain (`Hops[i]` connecting `Nodes[i]` to `Nodes[i+1]`) — a
reasoning chain fans out at the `Application` node, so each `Hop` instead
carries an explicit `FromIndex` naming which `Nodes` element it
originates from:

```go
type Hop struct {
    FromIndex int          // index into Nodes this hop originates from
    EdgeType  irac.EdgeType
    Reverse   bool
}
```

`Hops[i]` always produces `Nodes[i+1]` (so `len(Hops) == len(Nodes)-1`,
matching the "every non-root node was produced by exactly one hop"
invariant), but multiple hops can share the same `FromIndex` when a node
fans out to several children — as happens at the `Application` step of a
reasoning chain. `Path.Depth()` walks this shape to compute true
root-to-node hop distance (not just node count), and `Path.Truncate
(maxDepth)` prunes it correctly, rewriting `FromIndex` for the nodes that
survive.

## Reading edges: `EdgesForCase` capability detection

`loadEdges` (`edges.go`) prefers a `GraphStore`'s optional
`EdgesForCase(caseID) []irac.Edge` method, detected via a small
package-local interface (`edgeLister`) and a type assertion — the exact
same opportunistic pattern `packages/graph/backup.go`'s `Export` function
already uses to read raw edges beyond what the `GraphStore` interface
itself exposes. `InMemoryGraphStore` implements `EdgesForCase`; a future
Neo4j-backed store might prefer answering this via a direct Cypher
`MATCH` query instead of adding the method, which is exactly why this
capability is a documented extension point rather than part of the
`GraphStore` interface proper.

When a store does *not* implement `edgeLister`, `loadEdgesViaTraverse`
falls back to reconstructing one-hop adjacency via repeated
`GraphStore.Traverse(FromNodeID: n.ID, MaxDepth: 1)` calls. This fallback
is correct for connectivity but **cannot recover the exact `EdgeType` or
declared direction** of an edge — `Traverse` returns reachable nodes, not
the edges connecting them. This is a documented, known fidelity gap of
the fallback path: any `GraphStore` implementation whose call sites need
exact `EdgeType`-aware paths should implement `EdgesForCase`.

## Maintenance: full rebuild, not incremental patching

`Indexer.RebuildCase(ctx, caseID)` performs a full rebuild of a case's
`PathIndex` — it re-derives every `PathKindRuleGroupedIssues` and
`PathKindReasoningChain` path from scratch and replaces whatever the
`Indexer` previously held for that case, purging any cached lookups for
it in the same call.

An incremental alternative was considered — patching the `PathIndex` in
place as individual node/edge-creation events arrive — and rejected for
v1, for the same class of reason `packages/vectorindex`'s
`ReindexOnRevision` doc comment gives for its own full-re-embed choice:
a single new edge can invalidate an arbitrarily large slice of a case's
materialized paths (adding one more `EdgeGoverns` edge to a rule that
already governs ten issues means re-deriving all ten issues' grouped
path; adding a `Fact` to a heavily-reused `Application` means re-deriving
every reasoning chain running through it), and getting incremental patch
logic exactly right for all four edge types — including the two that are
walked in reverse — is exactly the kind of easy-to-get-subtly-wrong logic
worth avoiding until it's a proven bottleneck. A full rebuild reuses the
same code path as the very first build, so it is correct by construction
rather than by careful case analysis.

`ReindexOnRevision(ctx, *Indexer, irac.TreeRevision)` (`reindex.go`)
mirrors `vectorindex.ReindexOnRevision`'s shape exactly, so a caller
reacting to a new `TreeRevision` (e.g. from `packages/treeassembly`) can
drive both packages' maintenance identically. Like its `vectorindex`
counterpart, this is a full rebuild, not a delta, for the same reason:
`irac.TreeRevision` carries no node/edge list, only a "case X changed"
pointer.

## Caching: a dependency-free LRU in front of `LookupPaths`

`Indexer.LookupPaths(ctx, caseID, fromNodeID, edgeType)` and its
depth-bounded sibling `LookupPathsWithDepth` are served through a small
LRU cache (`cache.go`) keyed by `(caseID, fromNodeID, edgeType)`, built on
the standard library's `container/list` rather than a third-party
dependency — this package's only non-workspace, non-stdlib imports are
the indirect ones pulled in transitively through `packages/graph`.

- A cache **hit** returns the previously computed `[]Path` (after
  re-applying `Path.Truncate` for the caller's requested `maxDepth` — see
  "Index-level short-circuiting" below) without touching the underlying
  `PathIndex`.
- A cache **miss** filters `PathIndex.PathsFromRoot(fromNodeID)` by
  `edgeType`, caches the *untruncated* result under the lookup key, and
  then truncates for the caller. Caching the untruncated result means a
  later call with a *different* `maxDepth` for the same
  `(caseID, fromNodeID, edgeType)` is still a hit — the cache key
  deliberately does not include `maxDepth`.
- `Indexer.RebuildCase` calls `purgeCase`, which removes every cached
  entry for that case in one pass, so a rebuild can never leave a stale
  cached path behind. This is asserted directly in
  `TestIndexer_LookupPaths_CacheHitsAndMisses`: a lookup performed
  immediately after a `RebuildCase` call is a fresh cache miss, not a
  hit against pre-rebuild data.

`Indexer.Stats()` exposes cumulative `CacheHits`/`CacheMisses` counters
(see below) precisely so this behavior is externally observable, not just
internally correct.

## Index-level short-circuiting for depth-bounded lookups

`graph.TraversalQuery.MaxDepth` bounds a *live* breadth-first walk over a
`GraphStore` — it stops following edges once the walk has gone deep
enough. `treeindex` reuses the same "zero means unbounded" convention for
its own depth parameter, but applies it one layer up: `LookupPathsWithDepth`
does not re-invoke `GraphStore.Traverse` with a smaller `MaxDepth` at all.
Instead, it calls `Path.Truncate(maxDepth)` against the already-fully-
materialized `Path` values sitting in the `PathIndex` (or, on a cache hit,
already sitting in the LRU cache). The full-depth path was already
computed once, at `RebuildCase` time; a depth-bounded request is just a
slice of that in-memory structure, not a new graph walk. This is the
"index-level short-circuiting" the phase plan calls for: the short-circuit
happens against the index, not against the store.

## Index statistics

`Indexer.Stats()` returns a point-in-time `Stats` snapshot:

- `IndexedPaths` / `IndexedCases` — the total materialized path count and
  distinct case count the `Indexer` currently holds.
- `CacheHits` / `CacheMisses` — cumulative counters across every
  `LookupPaths`/`LookupPathsWithDepth` call, updated atomically since
  lookups may run concurrently.
- `LastBuildDuration` / `LastBuildAt` — how long the most recent
  `RebuildCase` (or `ReindexOnRevision`) call took, and when it completed,
  across all cases (not broken out per case in v1 — a per-case build-timing
  history would need its own bounded ring buffer, deferred until there is
  a concrete observability consumer asking for it).

## No hardcoded provider

`treeindex` never imports `packages/provider`, `packages/embedding`, or
any LLM/embedding client. Every `Path` it materializes is derived purely
from `irac.Node`/`irac.Edge` values already sitting in a `graph.GraphStore`
— this phase is pure structural graph indexing, consistent with the
overall project rule that no package outside `packages/provider` and its
adapters may hardcode a specific model provider.
