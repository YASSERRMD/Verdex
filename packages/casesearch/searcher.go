package casesearch

import "context"

// Hit is one content match found within a single case by a CaseSearcher.
type Hit struct {
	// NodeID is the underlying irac.Node's ID this hit was found on, when
	// known. Empty for a pure case-metadata match (e.g. a filter-only
	// search with no Text).
	NodeID string

	// NodeType is the underlying node's irac.NodeType, when known.
	NodeType string

	// Text is the raw node/case text this hit was matched against, used
	// to build Snippet.
	Text string

	// Score is a relevance score in an implementation-defined range;
	// higher is more relevant. Search re-ranks purely by Score within
	// each case's hits and does not attempt to normalize scores across
	// different modes (see rank.go).
	Score float64

	// Explanation is a short human-readable trace of why this hit
	// matched (e.g. "vector similarity (rank 2)" or "issue path via
	// governing_rule"), typically forwarded from the underlying
	// hybridretrieval.Item.Explanation or treeindex.Path.
	Explanation string
}

// CaseSearcher searches the content of a single, already-known case. An
// Engine never constructs a CaseSearcher itself — see
// CaseSearcherResolver and doc.go, "Why a resolver, not a single shared
// KnowledgeAPI".
type CaseSearcher interface {
	// SearchKeyword returns every Hit in this searcher's case whose text
	// contains text as a case-insensitive substring, capped at topK.
	SearchKeyword(ctx context.Context, text string, topK int) ([]Hit, error)

	// SearchSemantic returns the topK highest-scoring Hits for text via
	// this searcher's underlying vector/hybrid retrieval.
	SearchSemantic(ctx context.Context, text string, topK int) ([]Hit, error)

	// SearchIssueOrRule returns every Hit reachable from rootNodeID via
	// this searcher's underlying treeindex paths, capped at topK. An
	// empty result (not an error) is expected and normal when
	// rootNodeID does not exist in this case's tree — that simply means
	// this case did not apply the named issue/rule/statute.
	SearchIssueOrRule(ctx context.Context, rootNodeID string, topK int) ([]Hit, error)
}

// CaseSearcherResolver resolves a CaseSearcher for a given case ID,
// typically backed by a per-case knowledgeapi.KnowledgeAPI (see
// knowledgeapi.go for the reference adapter). Implementations may
// construct a new CaseSearcher per call or return one from a cache; an
// Engine treats the returned value as valid only for the duration of the
// call it was resolved for.
//
// Returns an error (e.g. wrapping knowledgeisolation's not-found error)
// if the case cannot be resolved to a searchable knowledge store; Search
// treats a resolver error for one candidate case as "skip this case" and
// continues with the remaining candidates rather than failing the whole
// request, since a single case's tree not yet being assembled/indexed
// should not block search across every other case.
type CaseSearcherResolver func(ctx context.Context, caseID string) (CaseSearcher, error)
