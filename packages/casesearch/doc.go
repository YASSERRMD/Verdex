// Package casesearch searches across cases using Verdex's tree-structured
// knowledge index, rather than reimplementing retrieval.
//
// # Composition, not reimplementation
//
// casesearch is a thin cross-case orchestration layer over three existing
// packages:
//
//   - packages/caselifecycle supplies the tenant-scoped Case metadata
//     (title, reference, category, jurisdiction, state, dates) and the
//     access boundary every search result must respect.
//   - packages/knowledgeapi (backed by packages/hybridretrieval,
//     packages/treeindex, and packages/vectorindex) supplies per-case
//     content search: semantic recall, structural issue/rule-path lookup,
//     and node text for snippet extraction.
//   - packages/identity gates every call on an authenticated user holding
//     identity.PermViewCase, mirroring packages/knowledgeapi's own access
//     model.
//
// A search request is answered in two phases: first, caselifecycle.
// Repository.List narrows the tenant's cases by the structured filters
// (category, jurisdiction, state, date range) and, for a party filter, an
// injected PartyLookup; second, for each candidate case, a caller-supplied
// CaseSearcher (typically backed by a per-case knowledgeapi.KnowledgeAPI)
// is asked to search that case's content in keyword, semantic, or
// issue/rule mode. Results from every matched case are ranked and merged
// into one flat, paginated Results value with a highlighted snippet per
// hit.
//
// # Why a resolver, not a single shared KnowledgeAPI
//
// knowledgeapi.KnowledgeAPI is deliberately scoped to exactly one case
// (see its doc comment) — a hybridretrieval.Retriever and treeindex.
// Indexer are built once per case's knowledgeisolation-scoped store. A
// cross-case search therefore cannot hold a single KnowledgeAPI; it needs
// one per matched case. CaseSearcherResolver (searcher.go) is the seam:
// callers provide a function from case ID to a CaseSearcher (typically a
// thin adapter wrapping a per-case knowledgeapi.KnowledgeAPI, cached or
// constructed on demand), and this package never constructs a
// KnowledgeAPI itself.
//
// # Search modes
//
// Query.Mode selects how content matching works within each candidate
// case: ModeKeyword does plain substring/term matching against node text
// (no embeddings, no LLM), ModeSemantic delegates to the resolved
// CaseSearcher's vector/hybrid retrieval, and ModeIssueRule targets
// treeindex paths rooted at a named issue/rule/statute node ID. ModeAuto
// (the default) picks ModeSemantic when Query.Text is set and
// Query.IssueOrRuleID is empty, and ModeIssueRule when Query.IssueOrRuleID
// is set. See query.go.
//
// # Saved searches
//
// SavedSearchRepository persists a named Query per user (savedsearch.go),
// scoped to the same tenant boundary as everything else in this package.
//
// See doc/case-search.md for the full search algorithm, ranking formula,
// and access-control write-up.
package casesearch
