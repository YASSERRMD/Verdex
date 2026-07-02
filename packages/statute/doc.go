// Package statute loads jurisdiction statutes as structured
// packages/irac RuleNodes with citations, hierarchy, cross-references,
// and embeddings for retrieval.
//
// Core concepts:
//
//   - Loader / DefaultLoader: reads a statute corpus (structured text or
//     JSON, no network fetch) and returns []RawStatute — one entry per
//     top-level act, with unparsed Body text (loader.go).
//   - StatuteNode: a recursive Act -> Section -> Clause tree.
//     ParseHierarchy turns a RawStatute's Body into this tree by
//     scanning for "Section N" and "(x)" line markers (hierarchy.go).
//   - Citation / BuildRuleNodes: converts StatuteNode tree nodes (at a
//     configurable granularity — clause by default, or section) into
//     irac.RuleNodes via irac.NewRuleNode, each carrying a formatted
//     Citation ("Act 12, s.5(a)") back to its position in the act
//     (rule.go).
//   - CategoryCode / TagRules: attaches a case-category taxonomy code
//     (mirroring packages/category's CategoryCode convention as an
//     opaque string) and jurisdiction/legal-family tags to each rule
//     (tagging.go).
//   - Amendment / AmendmentRecord: tracks a rule's EffectiveDate,
//     chronological amendment History, and an optional SupersededBy
//     link to a replacement rule node — SupersessionChain walks that
//     link with cycle detection (amendment.go).
//   - CrossReference / DetectCrossReferences / ResolveCrossReferences:
//     detects citation-shaped references within rule text ("see Section
//     12") and resolves them to other rule node IDs within the same
//     loaded corpus where possible, leaving unresolved references with
//     an empty ResolvedRuleID otherwise (xref.go).
//   - EmbeddedRule / EmbedRules: computes embeddings for each rule's
//     text via the existing embedding.EmbeddingService.EmbedChunked —
//     this package never reimplements embedding or references a
//     specific provider (embed.go).
//   - PersistRules / LoadRulesForJurisdiction: persists rule nodes via
//     graph.GraphStore.CreateNode, scoped to a jurisdiction code, and
//     fetches them back via GraphStore.GetNode (persist.go).
//   - StatuteIngestionService: orchestrates the full pipeline — load ->
//     parse hierarchy -> build rule nodes with citations -> tag ->
//     track amendments -> resolve cross-refs -> embed -> persist ->
//     return []irac.RuleNode (service.go), mirroring packages/fact's
//     FactConstructionService orchestration pattern.
//
// Design principles:
//
//   - No network fetch, no ML models. Loader is a pure parser over
//     caller-supplied input; hierarchy parsing and cross-reference
//     detection are deterministic regex/text heuristics, mirroring
//     packages/fact, packages/evidence, and packages/timeline's shared
//     "no ML models, rule based" design principle for everything except
//     embedding, which is delegated entirely to
//     embedding.EmbeddingService.
//   - No hard dependency on packages/category. CategoryCode is an
//     opaque string type defined locally, the same way
//     irac.RuleNode.JurisdictionCode/LegalFamily are opaque strings
//     rather than direct packages/jurisdiction bindings. This package
//     does, however, import packages/jurisdiction directly for its
//     LegalFamily enum (see tagging.go's TagOptions.LegalFamily and
//     go.mod's require+replace) since a shared enum already exists
//     there and re-declaring it locally would fragment the type.
//   - Amendment history is not embedded on irac.RuleNode. This phase
//     must not modify packages/irac, so AmendmentRecord is a sibling
//     struct keyed by RuleID and carried alongside (not inside) the
//     irac.RuleNode it describes throughout the pipeline (see
//     AmendedRule, EmbeddedRule).
//   - Partial resolution is a valid intermediate state. Cross-reference
//     resolution and corpus parsing do not hard-fail the whole batch on
//     a single bad act or unresolved reference; callers that need
//     strict guarantees inspect IngestResult.UnresolvedXRefs or test
//     errors.Is against the sentinel errors in errors.go.
//
// See doc/statute-model.md for a detailed prose write-up.
package statute
