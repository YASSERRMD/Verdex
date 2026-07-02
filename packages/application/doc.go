// Package application connects packages/irac IssueNodes to the rules
// that govern them and builds irac.ApplicationNodes — the IRAC reasoning
// step that applies a rule to a set of facts — with precedent linkage,
// distinguishing facts, multi-hop rule chains, and legal-family
// weighting.
//
// Core concepts:
//
//   - Origin / OriginatedRule: a local enum (OriginStatute,
//     OriginPrecedent) wrapping an irac.RuleNode, so this package never
//     imports packages/statute or packages/precedent directly. A future
//     orchestration phase constructs OriginatedRules from either
//     package's output — both already represent their rules as plain
//     irac.RuleNode (origin.go).
//   - MatchIssueToRules / RuleMatch: scores each candidate OriginatedRule
//     against an irac.IssueNode via a deterministic keyword/token-overlap
//     heuristic, mirroring packages/fact and packages/issue's existing
//     overlap conventions (match.go).
//   - BuildApplicationNode: converts a matched (issue, rule, facts)
//     triple into an irac.ApplicationNode via irac.NewApplicationNode
//     (build.go).
//   - PrecedentIssueLink: explicitly records that a precedent-origin
//     rule was linked to an issue, with a rationale, distinct from a
//     plain statute match (precedent_link.go).
//   - DistinguishingFact: records a current-case fact that diverges from
//     the typical fact pattern a precedent was decided on. Only
//     meaningful when Origin == OriginPrecedent (distinguish.go).
//   - RuleChain: an ordered sequence of rules that must be applied
//     together (e.g. cross-referencing statute sections), with Validate
//     rejecting cycles (chain.go).
//   - WeightByLegalFamily: weights a rule's Origin against the case's
//     dominant legal family — precedent favored under common_law,
//     statute favored under civil_law (weight.go).
//   - ComputeConfidence / ApplyConfidence: combines a RuleMatch's Score
//     with WeightByLegalFamily's output into a single [0, 1] confidence
//     value set on the built ApplicationNode (confidence.go).
//   - PersistApplicationSubgraph: persists an ApplicationNode via
//     graph.GraphStore.CreateNode, then creates every legal edge this
//     package owns per packages/irac/edge.go's constraint table:
//     Application--applies_to-->Rule, Application--applies_to-->Fact,
//     and Rule--governs-->Issue (persist.go).
//   - ApplicationService: orchestrates the full pipeline — match issue
//     to rules -> build application nodes -> link
//     precedents/distinguishing facts -> resolve rule chains -> weight
//     by legal family -> score confidence -> persist subgraph -> return
//     []irac.ApplicationNode (service.go), mirroring
//     packages/issue/service.go and packages/fact/service.go's
//     orchestration pattern.
//
// Design principles:
//
//   - No separate node types. Like packages/statute and
//     packages/precedent, this package never adds to the fixed IRAC
//     schema (Issue/Rule/Fact/Application/Conclusion, fixed in Phase
//     031). Every origin-specific concept (PrecedentIssueLink,
//     DistinguishingFact, RuleChain) is a local wrapper struct that
//     references irac.RuleNode/irac.FactNode/irac.IssueNode/
//     irac.ApplicationNode, never a new node type.
//   - No hard dependency on packages/statute or packages/precedent. The
//     Origin/OriginatedRule abstraction in origin.go lets a future
//     orchestration phase hand this package rules sourced from either
//     package without creating an import cycle or coupling this
//     package's tests to either package's fixtures.
//   - Every edge this package creates is checked against
//     irac.IsLegalEdgeTriple before being sent to graph.GraphStore.
//     Fact--supports-->Application edges are deliberately left to
//     packages/fact's own PersistFacts, which already owns that triple.
//   - No ML models. Matching, weighting, and confidence scoring are all
//     deterministic, lexical/table-driven heuristics, mirroring this
//     project's shared "no ML models, rule based" design principle for
//     every extraction/linkage package built so far.
//
// See doc/application-model.md for a detailed prose write-up.
package application
