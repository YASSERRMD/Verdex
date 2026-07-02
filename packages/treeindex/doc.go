// Package treeindex materializes and caches hierarchy-shaped lookup paths
// over an assembled IRAC reasoning tree (packages/irac), stored in a
// packages/graph GraphStore, for fast structured retrieval.
//
// # Scope
//
// This package sits downstream of packages/graph (tree storage) and
// alongside packages/vectorindex (semantic recall over leaf nodes). Where
// packages/vectorindex answers "which nodes are semantically similar to
// this query text", treeindex answers "what is the materialized path
// structure rooted at this node" — a purely structural question requiring
// no embeddings and no LLM provider. treeindex never calls an embedding
// service or a model provider; it only reads irac.Node/irac.Edge values
// through a graph.GraphStore.
//
// # Path kinds
//
// treeindex indexes two kinds of materialized path, both derived strictly
// from the legal edge triples packages/irac actually defines (see
// irac.LegalEdgeTriples): there is no literal "issue governs sub-issue"
// or "application concludes-to conclusion" edge in the schema, so this
// package documents precisely what it substitutes for the plan's
// "issue -> sub-issue" and "rule -> application -> conclusion" language:
//
//   - Rule-grouped issue paths: every IssueNode governed by the same
//     RuleNode (via EdgeGoverns, Rule --governs--> Issue) is grouped into
//     one Path rooted at that RuleNode. This is treeindex's stand-in for
//     "issue -> sub-issue" relatedness: issues sharing a governing rule
//     are the closest analog available in the current schema to a
//     parent/sub-issue grouping, since irac.IssueNode itself carries no
//     ParentIssueID (that field lives on packages/issue's pre-tree
//     CandidateIssue, deliberately not imported here).
//   - Reasoning-chain paths: for each IssueNode, its governing RuleNode(s),
//     the ApplicationNode(s) that apply that rule, the FactNode(s)
//     supporting those applications, and the ConclusionNode(s) concluded
//     from those applications, assembled into one multi-hop Path. This is
//     the "rule -> application -> conclusion" structure the plan calls
//     for. Two of the four edge types must be walked in reverse of their
//     declared direction to assemble a human-meaningful chain (EdgeSupports
//     is Fact --supports--> Application, and EdgeConcludesFrom is
//     Conclusion --concludes_from--> Application), since the legal edge
//     triples point from the derived node back to what it derives from,
//     not from cause to effect.
//
// See doc/tree-indexing.md for the full schema write-up, the caching and
// maintenance tradeoffs, and how this package complements packages/
// vectorindex and the future Phase 043 graph-traversal package.
package treeindex
