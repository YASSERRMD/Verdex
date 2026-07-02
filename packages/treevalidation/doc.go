// Package treevalidation is Verdex's capstone integrity-and-validation
// gate for an assembled IRAC reasoning tree (packages/treeassembly's
// Tree). It is the final phase in the Phases 021-040 batch: everything
// upstream (extraction, construction, application, assembly) produces a
// tree; this package is what decides whether that tree is trustworthy
// enough to be used downstream (e.g. by Phase 055's synthesis).
//
// # Composition, not duplication
//
// This package deliberately composes with, rather than reimplements,
// validation already done by earlier phases:
//
//   - packages/irac's ValidateTree (validate.go) already checks
//     structural integrity: dangling edge references, illegal
//     (FromNodeType, EdgeType, ToNodeType) triples, self-loops, unknown
//     node/edge types, and missing guardrail labels. This package calls
//     treeassembly.ValidateIntegrity — which itself wraps
//     irac.ValidateTree — rather than re-checking any of that here.
//
//   - packages/treeassembly's DetectGaps (gap.go) already does semantic
//     gap detection: an issue with no application addressing it, an
//     application with no conclusion resolving it. This package calls
//     DetectGaps directly rather than re-deriving those gaps.
//
//   - packages/application's RuleChain.Validate (chain.go) already
//     detects a repeated rule ID within one flat, local rule chain. This
//     package's DetectCycles is a distinct, broader check: a full-graph
//     DFS across every node and edge in the assembled tree, catching
//     cycles that span multiple rule chains, applications, or node
//     types — something a single RuleChain can never see.
//
// service.go's TreeValidationService.Validate runs both of the above
// composed checks and this package's own six new checks in one pipeline,
// aggregating every result into a single Report.
//
// # What this package adds
//
// Six new checks not covered by any earlier phase:
//
//  1. Conclusion traceability (traceability.go): every ConclusionNode
//     must trace, via its edges, to at least one FactNode and at least
//     one RuleNode — not just "has some edge" (that's orphan detection,
//     see below), but specifically reaches both a fact and a rule.
//
//  2. True orphan detection (orphan.go): a node — of any type — with
//     zero edges at all, incoming or outgoing.
//
//  3. Full-graph cycle detection (cycle.go): DFS-based cycle detection
//     across the entire assembled tree, not just within one rule chain.
//
//  4. Unsupported-claim flagging (unsupported.go): nodes with empty
//     source-span backing, or confidence below a configurable threshold.
//
//  5. Confidence-propagation checks (propagation.go): a conclusion's
//     confidence must not exceed what its supporting chain's confidences
//     justify.
//
//  6. Jurisdiction-consistency checks (jurisdiction_consistency.go):
//     every RuleNode's jurisdiction must match the case's declared
//     jurisdiction, with an explicit override allow-list for
//     intentionally cited foreign/persuasive authority.
//
// # The hard gate
//
// report.go defines Finding (with Severity: SeverityCritical,
// SeverityWarning, SeverityInfo) and Report, which aggregates every
// Finding from every check above. gate.go's CanFinalize is the hard
// gate: it returns false and ErrCriticalFindings whenever a Report
// contains at least one SeverityCritical Finding. Per CONTRIBUTING.md's
// non-binding guardrail spirit — a tree with critical integrity failures
// must not be usable for further reasoning — every future phase that
// consumes an assembled Tree (e.g. Phase 055's synthesis) must call
// CanFinalize (directly, or transitively via
// TreeValidationService.Validate's returned error) before treating that
// tree as usable.
//
// See doc/integrity-guarantees.md for the full narrative write-up of
// these guarantees.
package treevalidation
