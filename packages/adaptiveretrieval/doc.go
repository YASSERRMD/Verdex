// Package adaptiveretrieval builds only the subgraph a specific query
// actually needs, instead of relying on an eagerly precomputed structure.
//
// # Scope and layering
//
// This package sits above packages/treeindex, packages/traversal, and
// packages/hybridretrieval — it is a cost-control and on-demand-
// construction layer, not a replacement for any of them:
//
//   - packages/treeindex precomputes a full PathIndex for a case up front
//     (RebuildCase) and serves LookupPaths reads from that materialized
//     structure. This is cheapest when a case's tree is queried
//     repeatedly from many different starting nodes, but the upfront
//     RebuildCase cost is paid whether or not most of that structure is
//     ever read.
//   - packages/traversal walks a graph.GraphStore live, on every call,
//     with no memory of previous walks beyond its own optional Cache.
//   - packages/hybridretrieval fuses vectorindex semantic recall with a
//     traversal.Walker graph expansion, gated by a per-query latency
//     Budget.
//
// adaptiveretrieval answers a narrower question than any of the three:
// "for this one query, what is the minimal subgraph worth building right
// now, and have I already built it recently enough to reuse?" It walks
// outward from a query's anchor node via a traversal.Walker under a tight
// BuildBudget, caches the result keyed by (case ID, query shape), detects
// when a cached entry has gone stale relative to the case's current
// irac.TreeRevision, and falls back to treeindex.Indexer.LookupPaths when
// an adaptive build is not worth attempting or exceeds its budget.
//
// # When to use this package vs. the layers below it
//
//   - Use treeindex directly when a case's tree is queried repeatedly from
//     many different anchors and the full-case rebuild cost is already
//     amortized (e.g. a case actively being worked, with an event-driven
//     ReindexOnRevision hook already wired up).
//   - Use traversal directly when a caller has one specific, already-known
//     multi-hop shape to walk and does not need cross-query reuse.
//   - Use hybridretrieval directly when a caller has a semantic query
//     vector and wants fused vector+graph ranking, and is content with its
//     own latency Budget as the only cost control.
//   - Use adaptiveretrieval when none of the above precomputation is known
//     to be worthwhile yet: a first-touch query against a case whose tree
//     may never be queried again, where paying treeindex's full
//     RebuildCase cost would be wasted work, but where naive unbounded
//     traversal risks runaway cost on a large or densely-connected tree.
//
// See doc/adaptive-retrieval.md for the full cost/staleness tradeoff
// discussion.
package adaptiveretrieval
