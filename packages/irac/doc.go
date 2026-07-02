// Package irac defines the formal schema for the Issue-Rule-Application-
// Conclusion (IRAC) reasoning tree: the structural backbone every later
// reasoning phase (033-062) builds on. This phase defines TYPES and
// VALIDATION only — there is no storage backend here; that is
// Phase 032 (graph store integration), which builds on this schema.
//
// Core concepts:
//
//   - NodeType / Node / NodeLike: the common node shape (ID, Type, CaseID,
//     Text, CreatedAt, Confidence, Provenance) shared by every position in
//     the tree, plus five concrete typed wrappers — IssueNode, RuleNode,
//     FactNode, ApplicationNode, ConclusionNode — each embedding Node and
//     adding type-specific fields (node.go).
//   - EdgeType / Edge / legal-triple constraint table: the four
//     relationship kinds that connect nodes (governs, applies_to,
//     supports, concludes_from) and the exhaustive table of which
//     (FromNodeType, EdgeType, ToNodeType) combinations are legal
//     (edge.go).
//   - SourceSpan: a local mirror of packages/segmentation's SourceSpan
//     shape (rune offsets plus optional OCR page / STT millisecond
//     fields), embedded as a []SourceSpan on every concrete node type so
//     every claim traces back to ingested text (span.go). Defined locally
//     to avoid a hard module dependency, since this phase must not
//     depend on packages/segmentation.
//   - Provenance / ValidConfidence: GeneratedBy, GeneratedAt, and
//     UpstreamNodeIDs recording how a node came to exist, alongside a
//     Confidence score in [0, 1] on every node (provenance.go).
//   - RuleNode jurisdiction tagging: JurisdictionCode and LegalFamily are
//     opaque strings carried on every RuleNode (no hard dependency on
//     packages/jurisdiction), with helper predicates for checking tagging
//     completeness (jurisdiction.go).
//   - TreeRevision: a case's reasoning tree is a sequence of immutable
//     revisions, never mutated in place — RevisionNumber, CaseID,
//     CreatedAt, and an optional ParentRevision link (version.go).
//   - ValidateTree / ValidationIssue: checks a candidate set of nodes and
//     edges for tree integrity — dangling edge references, illegal edge
//     triples, self-loops, unknown node/edge types, and missing
//     draft_analysis guardrail labels — collecting every issue found
//     instead of failing fast (validate.go).
//   - MarshalTree / UnmarshalTree / TreeEnvelope: a stable, lossless JSON
//     envelope for a tree's nodes (as a discriminated union keyed by
//     NodeType), edges, and revision metadata (serialize.go).
//   - DraftAnalysisLabel / NewConclusionNode / ContainsVerdictLanguage:
//     the mandatory non-binding-guardrail enforcement (guardrail.go) — see
//     "Non-binding guardrail" below.
//
// Design principles:
//
//   - Types and validation only. This phase produces no persistence
//     layer, no HTTP handlers, no LLM calls. It is the shared vocabulary
//     later reasoning phases (033-062) and the Phase 032 graph store
//     integration build on.
//   - No hard cross-module dependencies. SourceSpan, JurisdictionCode,
//     and LegalFamily are all defined or typed locally, opaque to this
//     package, rather than importing packages/segmentation or
//     packages/jurisdiction — this package must be buildable and
//     testable in complete isolation.
//   - Trees are immutable revision sequences, not mutable graphs. Every
//     change produces a new TreeRevision; nothing is edited in place
//     (version.go).
//   - Validation collects, never fails fast. ValidateTree returns every
//     ValidationIssue found in one pass so a caller can see everything
//     wrong with a candidate tree at once (validate.go).
//   - Non-binding guardrail, enforced by construction. Per
//     CONTRIBUTING.md: "Every module that produces reasoning output must
//     attach the draft_analysis label. Verdict or directive language is
//     rejected." NewConclusionNode is the only exported way to build a
//     ConclusionNode, and it unconditionally sets Label to
//     DraftAnalysisLabel — there is no constructor path that omits it.
//     ValidateTree, MarshalTree, and UnmarshalTree all additionally check
//     for the label defensively, in case a ConclusionNode reaches this
//     package's boundary some other way (e.g. decoded from untrusted
//     JSON with the field stripped).
//
// See doc/irac-schema.md for a detailed model write-up, including the
// full node/edge diagram and constraint table.
package irac
