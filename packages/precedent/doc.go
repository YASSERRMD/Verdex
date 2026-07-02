// Package precedent loads prior cases/precedents for common-law
// jurisdictions as structured packages/irac RuleNodes (see
// packages/irac/node.go's RuleNode doc comment: "a legal rule, statute,
// or precedent invoked to resolve an Issue" — there is no separate
// PrecedentNode type in the fixed IRAC schema), enriching each rule with
// its extracted holding and ratio decidendi, a formatted case citation,
// category/issue tags, court-hierarchy weighting, and recency/authority
// scoring, before persisting it via graph.GraphStore.
//
// Core concepts:
//
//   - Loader / DefaultLoader: reads a precedent corpus (structured text
//     or JSON, no network fetch) and returns []RawPrecedent — one entry
//     per case, with unparsed FullText judgment text (loader.go).
//   - ExtractHoldingAndRatio: a deterministic heuristic that locates a
//     "HELD:"/"HOLDING:" marker section in FullText and extracts the
//     court's core determination (Holding) plus the reasoning behind it
//     (RatioDecidendi), via a "RATIO:"/"REASONING:" marker when present
//     or a sentence-boundary fallback otherwise. ExtractorFunc is a
//     pluggable extension point for callers that want a different
//     extraction strategy (holding.go).
//   - PrecedentRule / BuildPrecedentRule(s): converts a RawPrecedent into
//     an irac.RuleNode via irac.NewRuleNode, wrapped in a local
//     PrecedentRule struct carrying the extracted Holding,
//     RatioDecidendi, and a formatted Citation (rule.go).
//   - CategoryCode / TagPrecedents / ExtractIssueKeywords: attaches a
//     case-category taxonomy code (mirroring packages/category's own
//     convention as an opaque string) and candidate issue keywords drawn
//     from the holding+ratio text, for later issue-matching (tagging.go).
//   - CourtLevel / ClassifyCourtLevel / ApplyCourtHierarchy: classifies a
//     precedent's deciding court into a binding-authority tier
//     (CourtSupreme > CourtAppellate > CourtTrial > CourtUnknown), each
//     with a Weight() reflecting its strength as binding authority
//     (hierarchy.go).
//   - EmbedPrecedents: computes embeddings for each precedent's combined
//     holding+ratio text via the existing
//     embedding.EmbeddingService.EmbedChunked — this package never
//     reimplements embedding or references a specific provider
//     (embed.go).
//   - AuthorityScore / AuthorityScoreAsOf / ScorePrecedents: combines a
//     precedent's CourtLevel.Weight() and its RecencyScore (an
//     exponential-decay function of age since DecidedDate) into a single
//     authority score, with court hierarchy weighted as the dominant
//     signal and recency as a secondary adjustment (authority.go).
//   - PersistPrecedents / LoadPrecedentsForJurisdiction: persists rule
//     nodes via graph.GraphStore.CreateNode, scoped to a jurisdiction
//     code, and fetches them back via GraphStore.GetNode (persist.go).
//   - PrecedentIngestionService: orchestrates the full pipeline — load ->
//     extract holding/ratio -> build rule nodes with citations -> tag ->
//     weight by court hierarchy -> embed -> score authority -> persist ->
//     return []PrecedentRule (service.go), mirroring
//     packages/statute's StatuteIngestionService orchestration pattern.
//
// Design principles:
//
//   - No separate PrecedentNode type. The IRAC schema (Issue, Rule, Fact,
//     Application, Conclusion) was fixed in Phase 031 and is never
//     modified afterward; precedents are irac.RuleNodes, exactly like
//     packages/statute represents statutes. All precedent-specific
//     metadata (Holding, RatioDecidendi, Citation, CourtLevel,
//     AuthorityScore) lives in this package's local PrecedentRule wrapper
//     and its sibling structs, never on packages/irac itself.
//   - No network fetch, no ML models. Loader is a pure parser over
//     caller-supplied input; holding/ratio extraction, court
//     classification, and keyword extraction are deterministic
//     regex/text heuristics, mirroring packages/statute's shared
//     "no ML models, rule based" design principle for everything except
//     embedding, which is delegated entirely to
//     embedding.EmbeddingService.
//   - No hard dependency on packages/category. CategoryCode is an opaque
//     string type defined locally, the same way irac.RuleNode's
//     JurisdictionCode/LegalFamily fields are opaque strings.
//   - Partial extraction is a valid intermediate state. A precedent whose
//     FullText lacks a recognizable holding marker is not dropped from
//     the corpus — it is still built into a PrecedentRule (falling back
//     to its full text), with its ID recorded in
//     IngestResult.FailedHoldingIDs so callers can inspect what could not
//     be cleanly extracted.
//
// See doc/precedent-model.md for a detailed prose write-up.
package precedent
