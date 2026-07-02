// Package traversal implements dynamic, multi-hop graph traversal
// queries over an assembled IRAC reasoning tree (packages/irac), stored
// in a packages/graph GraphStore.
//
// # Scope: dynamic queries, not precomputed paths
//
// packages/treeindex (Phase 042) precomputes and caches a small, fixed
// set of path shapes over a case's tree — issues grouped by shared
// governing rule, and Issue -> Rule -> Application -> Facts/Conclusions
// reasoning chains — because those shapes are common enough to be worth
// materializing once per tree revision. This package is the
// deliberately-not-that counterpart its doc/tree-indexing.md predicted:
// arbitrary, caller-specified hop sequences evaluated at query time via
// a fluent Query builder (query.go), for questions treeindex's two fixed
// PathKinds don't answer — most notably the legal-reasoning chain the
// phase plan calls for, issue -> governing rule -> controlling precedent
// -> distinguishing facts, which requires hops beyond what a literal
// irac.Edge encodes (see "Resolver-backed hops" below).
//
// Neither package depends on the other. A caller wanting "the reasoning
// chain for this issue, fast" uses treeindex; a caller wanting "find
// every controlling precedent within N hops of this rule, ranked by
// authority" uses this package's Walker.
//
// # Query DSL
//
// A Query is built fluently: NewQuery(caseID,
// startNodeID).ViaGoverningRule().ViaControllingPrecedent().
// ViaDistinguishingFacts().WithMaxDepth(3). Each Via* call appends one
// HopSpec (hop.go); Walker.Execute (walk.go) walks them in order,
// producing a TraversalResult of ranked Paths (result.go). See
// doc/graph-traversal.md for the full DSL reference and per-hop-type
// semantics.
//
// # Resolver-backed hops
//
// Two of the three named legal-reasoning hops have no literal irac.Edge
// to walk: "rule -> controlling precedent" and "precedent ->
// distinguishing facts" are legal concepts the IRAC schema does not
// encode as edges. Rather than invent synthetic edges or import
// packages/application / packages/statute / packages/precedent directly
// (which would tie this general-purpose traversal layer to one
// vocabulary and violate the project's no-hardcoded-provider-style
// decoupling), those two hops are resolved via caller-supplied function
// values: PrecedentResolver (precedent.go) and
// DistinguishingFactResolver (distinguish.go). This mirrors how
// packages/application itself avoids importing packages/statute or
// packages/precedent by defining a local Origin abstraction instead (see
// packages/application/origin.go).
//
// # Weighted-path ranking
//
// Discovered Paths are ranked by a caller-supplied ScoreFunc (score.go).
// DefaultScoreFunc favors shorter paths; ConfidenceWeightedScoreFunc
// layers in caller-supplied per-node weights (e.g. derived from a
// precedent's AuthorityScore or a statute's specificity) without this
// package importing either package directly, for the same decoupling
// reason as the resolver-backed hops above.
//
// # No hardcoded provider
//
// traversal never imports packages/provider, packages/embedding, or any
// LLM/embedding client. Every Path it produces is derived purely from
// irac.Node/irac.Edge values read through a graph.GraphStore, plus
// whatever a caller's own PrecedentResolver/DistinguishingFactResolver/
// ScoreFunc chooses to do — this package itself makes no provider calls.
package traversal
