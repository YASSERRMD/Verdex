package hybridretrieval

import (
	"time"

	"github.com/YASSERRMD/verdex/packages/embedding"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/traversal"
	"github.com/YASSERRMD/verdex/packages/vectorindex"
)

// RetrievalPath names which retrieval signal(s) surfaced a given Item. A
// caller inspecting Result.Items can use this to explain "why was this
// returned" without re-deriving it from VectorRank/GraphRank.
type RetrievalPath string

const (
	// RetrievalPathVector labels an Item found only by vector recall: it
	// was a top-K semantic match but was never reached by graph
	// expansion from any anchor.
	RetrievalPathVector RetrievalPath = "vector"

	// RetrievalPathGraph labels an Item found only by graph traversal
	// expansion: it was reached by walking from a vector hit or a
	// caller-supplied anchor, but was not itself a top-K semantic match.
	RetrievalPathGraph RetrievalPath = "graph"

	// RetrievalPathBoth labels an Item found by both signals: it was a
	// top-K semantic match AND reachable via graph expansion from some
	// anchor (possibly itself, possibly a fellow vector hit). Both
	// signals is the strongest possible corroboration a fused result can
	// carry.
	RetrievalPathBoth RetrievalPath = "both"
)

// allRetrievalPaths is the exhaustive set of recognized RetrievalPath
// values.
var allRetrievalPaths = map[RetrievalPath]struct{}{
	RetrievalPathVector: {},
	RetrievalPathGraph:  {},
	RetrievalPathBoth:   {},
}

// IsValid reports whether p is one of the recognized RetrievalPath
// constants.
func (p RetrievalPath) IsValid() bool {
	_, ok := allRetrievalPaths[p]
	return ok
}

// Filter narrows a HybridQuery to nodes matching every non-empty field,
// applied identically across both the vector-recall and graph-expansion
// retrieval paths. This mirrors vectorindex.MetadataFilter's shape exactly
// (same field set, same "empty means unrestricted" semantics) rather than
// inventing a parallel filter type — see doc/hybrid-retrieval.md.
type Filter struct {
	JurisdictionCode vectorindex.JurisdictionCode
	CategoryCode     vectorindex.CategoryCode
	PartyID          vectorindex.PartyID
}

// toMetadataFilter converts f into the vectorindex.MetadataFilter shape
// vectorindex.QueryRequest.Filter expects.
func (f Filter) toMetadataFilter() vectorindex.MetadataFilter {
	return vectorindex.MetadataFilter{
		JurisdictionCode: f.JurisdictionCode,
		CategoryCode:     f.CategoryCode,
		PartyID:          f.PartyID,
	}
}

// matchesNode reports whether node's metadata satisfies every non-empty
// field of f. Used to filter graph-traversal expansion candidates, which
// (unlike VectorRecords) carry no metadata of their own on the bare
// irac.Node/traversal.PathNode shape — callers supply a NodeMetadataLookup
// (see HybridQuery) to answer this for expansion candidates.
func (f Filter) matches(meta NodeMetadata) bool {
	if f.JurisdictionCode != "" && f.JurisdictionCode != meta.JurisdictionCode {
		return false
	}
	if f.CategoryCode != "" && f.CategoryCode != meta.CategoryCode {
		return false
	}
	if f.PartyID != "" && f.PartyID != meta.PartyID {
		return false
	}
	return true
}

// NodeMetadata is the subset of vectorindex.IndexableLeaf/VectorRecord
// metadata needed to apply a Filter to a node discovered via graph
// traversal (which does not itself carry jurisdiction/category/party
// metadata — see packages/traversal's doc, "Path is derived purely from
// irac.Node/irac.Edge values").
type NodeMetadata struct {
	JurisdictionCode vectorindex.JurisdictionCode
	CategoryCode     vectorindex.CategoryCode
	PartyID          vectorindex.PartyID
}

// NodeMetadataLookup resolves the Filter-relevant metadata for a node
// discovered via graph traversal, keyed by node ID. A HybridQuery with a
// nil lookup treats every graph-discovered node as having no metadata
// (Filter.matches degrades gracefully: an empty Filter still matches
// everything, but a non-empty Filter will exclude every unresolvable
// graph-only node rather than guessing). Typical callers back this with
// the same source that populated vectorindex's IndexableLeaf.
// CategoryCode/PartyID for the case (e.g. a small in-memory map, or a
// wrapper around whatever service supplied ProjectionOptions to
// vectorindex).
type NodeMetadataLookup func(nodeID string) (NodeMetadata, bool)

// ExpansionHop describes one traversal expansion a HybridQuery should run
// from every seed node (a vector hit, or a caller-supplied anchor): the
// named hop plus how deep to walk. Mirrors the three named legal-reasoning
// hops traversal.Query exposes as builder methods
// (ViaGoverningRule/ViaControllingPrecedent/ViaDistinguishingFacts) rather
// than exposing traversal's full Via(EdgeType, Direction, NodeType) escape
// hatch — a hybrid retriever's expansion policy is expected to be one of
// these fixed, legally-meaningful shapes, or a caller-supplied
// PrecedentResolver/DistinguishingFactResolver plugged into the
// HybridQuery's own resolver fields.
type ExpansionHop string

const (
	// ExpansionGoverningRule expands via traversal.Query.ViaGoverningRule.
	ExpansionGoverningRule ExpansionHop = "governing_rule"

	// ExpansionControllingPrecedent expands via
	// traversal.Query.ViaControllingPrecedent.
	ExpansionControllingPrecedent ExpansionHop = "controlling_precedent"

	// ExpansionDistinguishingFacts expands via
	// traversal.Query.ViaDistinguishingFacts.
	ExpansionDistinguishingFacts ExpansionHop = "distinguishing_facts"
)

// allExpansionHops is the exhaustive set of recognized ExpansionHop
// values.
var allExpansionHops = map[ExpansionHop]struct{}{
	ExpansionGoverningRule:        {},
	ExpansionControllingPrecedent: {},
	ExpansionDistinguishingFacts:  {},
}

// IsValid reports whether h is one of the recognized ExpansionHop
// constants.
func (h ExpansionHop) IsValid() bool {
	_, ok := allExpansionHops[h]
	return ok
}

// HybridQuery describes a fused semantic-plus-structural retrieval
// request. Construct with NewHybridQuery and chain the With* methods.
type HybridQuery struct {
	// CaseID scopes both the vector recall and the graph expansion to a
	// single case. Required.
	CaseID string

	// Vector is the query embedding used for vector recall. Required
	// unless AnchorNodeID is set and VectorTopK is zero (a pure
	// structural query with no semantic component).
	Vector embedding.EmbeddingVector

	// AnchorNodeID, if non-empty, seeds graph expansion from this node in
	// addition to (or instead of, if Vector is empty) whatever nodes
	// vector recall returns. Useful when a caller already knows a
	// starting point (e.g. "expand from this specific issue") and wants
	// vector recall layered on top for corroboration.
	AnchorNodeID string

	// VectorTopK caps how many vector-recall candidates seed graph
	// expansion. Zero means DefaultVectorTopK.
	VectorTopK int

	// ExpansionHops is the ordered sequence of named hops walked from
	// every seed node during graph expansion. Empty means "no graph
	// expansion" (the result is vector recall only, with GraphRank left
	// at zero for every item).
	ExpansionHops []ExpansionHop

	// MaxExpansionDepth bounds how many of ExpansionHops are actually
	// walked per seed, mirroring traversal.Query.MaxDepth. Zero means
	// "walk the full ExpansionHops sequence".
	MaxExpansionDepth int

	// Filter restricts both retrieval paths to nodes matching every
	// non-empty field.
	Filter Filter

	// MetadataLookup resolves Filter-relevant metadata for graph-expanded
	// nodes (see NodeMetadataLookup). Nil means graph-expanded nodes are
	// treated as having no metadata.
	MetadataLookup NodeMetadataLookup

	// MaxPerAnchor caps how many expansion results are kept per seed
	// anchor node, a dedup/diversity control preventing one
	// densely-connected seed from crowding out every other candidate's
	// expansion results. Zero means DefaultMaxPerAnchor.
	MaxPerAnchor int

	// TopK caps the number of items in the final fused Result. Zero means
	// DefaultTopK.
	TopK int

	// RRFConstant is the "k" constant in the reciprocal-rank-fusion
	// formula 1/(k+rank). Zero means DefaultRRFConstant. See fusion.go.
	RRFConstant float64

	// PrecedentResolver is forwarded to every traversal.Query built for
	// ExpansionControllingPrecedent hops. Nil means traversal.NoPrecedents.
	PrecedentResolver traversal.PrecedentResolver

	// DistinguishingFactResolver is forwarded to every traversal.Query
	// built for ExpansionDistinguishingFacts hops. Nil means
	// traversal.NoDistinguishingFacts.
	DistinguishingFactResolver traversal.DistinguishingFactResolver

	// Budget bounds the wall-clock time this query's graph-expansion
	// phase may spend. Zero (the default) means "no budget": expansion
	// always runs to completion. See budget.go.
	Budget time.Duration
}

// NewHybridQuery constructs a HybridQuery scoped to caseID with query
// vector vec. Chain the With* methods to configure expansion, filters,
// and budget before passing the result to Retriever.Retrieve.
func NewHybridQuery(caseID string, vec embedding.EmbeddingVector) HybridQuery {
	return HybridQuery{CaseID: caseID, Vector: vec}
}

// clone returns a copy of q with its own independent ExpansionHops backing
// array, mirroring traversal.Query.clone's rationale: a partially-built
// HybridQuery can be safely reused as a template without the builder
// methods mutating a shared slice.
func (q HybridQuery) clone() HybridQuery {
	out := q
	out.ExpansionHops = make([]ExpansionHop, len(q.ExpansionHops))
	copy(out.ExpansionHops, q.ExpansionHops)
	return out
}

// WithAnchor returns a copy of q with AnchorNodeID set to nodeID.
func (q HybridQuery) WithAnchor(nodeID string) HybridQuery {
	out := q.clone()
	out.AnchorNodeID = nodeID
	return out
}

// WithExpansion returns a copy of q with hop appended to ExpansionHops.
func (q HybridQuery) WithExpansion(hop ExpansionHop) HybridQuery {
	out := q.clone()
	out.ExpansionHops = append(out.ExpansionHops, hop)
	return out
}

// WithMaxExpansionDepth returns a copy of q with MaxExpansionDepth set to
// depth.
func (q HybridQuery) WithMaxExpansionDepth(depth int) HybridQuery {
	out := q.clone()
	out.MaxExpansionDepth = depth
	return out
}

// WithFilter returns a copy of q with Filter set to f.
func (q HybridQuery) WithFilter(f Filter) HybridQuery {
	out := q.clone()
	out.Filter = f
	return out
}

// WithMetadataLookup returns a copy of q with MetadataLookup set to
// lookup.
func (q HybridQuery) WithMetadataLookup(lookup NodeMetadataLookup) HybridQuery {
	out := q.clone()
	out.MetadataLookup = lookup
	return out
}

// WithTopK returns a copy of q with TopK set to topK.
func (q HybridQuery) WithTopK(topK int) HybridQuery {
	out := q.clone()
	out.TopK = topK
	return out
}

// WithVectorTopK returns a copy of q with VectorTopK set to topK.
func (q HybridQuery) WithVectorTopK(topK int) HybridQuery {
	out := q.clone()
	out.VectorTopK = topK
	return out
}

// WithMaxPerAnchor returns a copy of q with MaxPerAnchor set to max.
func (q HybridQuery) WithMaxPerAnchor(max int) HybridQuery {
	out := q.clone()
	out.MaxPerAnchor = max
	return out
}

// WithBudget returns a copy of q with Budget set to d.
func (q HybridQuery) WithBudget(d time.Duration) HybridQuery {
	out := q.clone()
	out.Budget = d
	return out
}

// WithPrecedentResolver returns a copy of q with PrecedentResolver set to
// resolver.
func (q HybridQuery) WithPrecedentResolver(resolver traversal.PrecedentResolver) HybridQuery {
	out := q.clone()
	out.PrecedentResolver = resolver
	return out
}

// WithDistinguishingFactResolver returns a copy of q with
// DistinguishingFactResolver set to resolver.
func (q HybridQuery) WithDistinguishingFactResolver(resolver traversal.DistinguishingFactResolver) HybridQuery {
	out := q.clone()
	out.DistinguishingFactResolver = resolver
	return out
}

// validate checks q for the structural errors Retriever.Retrieve rejects
// before ever touching a VectorStore or GraphStore.
func (q HybridQuery) validate() error {
	if q.CaseID == "" {
		return ErrEmptyCaseID
	}
	if len(q.Vector) == 0 && q.AnchorNodeID == "" {
		return ErrEmptyVector
	}
	if q.TopK < 0 {
		return ErrInvalidTopK
	}
	if q.MaxPerAnchor < 0 {
		return ErrInvalidMaxPerAnchor
	}
	return nil
}

// DefaultVectorTopK is the VectorTopK used when a HybridQuery leaves it
// zero.
const DefaultVectorTopK = 10

// DefaultTopK is the final fused Result size used when a HybridQuery
// leaves TopK zero.
const DefaultTopK = 10

// DefaultMaxPerAnchor is the MaxPerAnchor used when a HybridQuery leaves
// it zero.
const DefaultMaxPerAnchor = 5

// DefaultRRFConstant is the RRFConstant used when a HybridQuery leaves it
// zero. 60 is the constant recommended by the original reciprocal rank
// fusion paper (Cormack, Clarke & Buettcher, 2009) and is a reasonable,
// dependency-free default that dampens the influence of any single
// extremely-high rank without needing per-deployment tuning.
const DefaultRRFConstant = 60.0

// Item is one fused result: a node surfaced by vector recall, graph
// expansion, or both, carrying every signal Retriever.Retrieve computed
// for it plus a human-readable explanation of how it was found.
type Item struct {
	// NodeID is the underlying irac.Node's ID.
	NodeID string

	// NodeType is the underlying node's irac.NodeType, when known. Always
	// known for vector hits (from VectorRecord.NodeType); may be known
	// for graph-only hits if the traversal.PathNode carried it (it
	// always does — see traversal.PathNode.Type).
	NodeType irac.NodeType

	// Text is the node's text content, when known.
	Text string

	// Path names which signal(s) surfaced this item (vector-only,
	// graph-only, or both). See RetrievalPath.
	Path RetrievalPath

	// VectorScore is the raw cosine-similarity score vectorindex computed
	// for this item, or 0 if this item was never a vector-recall hit.
	VectorScore float64

	// VectorRank is this item's 1-based rank within the vector-recall
	// result set, or 0 if it was never a vector-recall hit.
	VectorRank int

	// GraphScore is the raw traversal.Path.Score of the best (highest-
	// scoring) path that reached this item during graph expansion, or 0
	// if this item was never reached by graph expansion.
	GraphScore float64

	// GraphRank is this item's 1-based rank within the graph-expansion
	// result set (ranked by GraphScore, best path per node), or 0 if it
	// was never reached by graph expansion.
	GraphRank int

	// CombinedScore is the reciprocal-rank-fusion of VectorRank and
	// GraphRank (see fusion.go). This is the score Result.Items is
	// sorted by, descending.
	CombinedScore float64

	// AnchorNodeID is the seed node graph expansion walked from to reach
	// this item (itself, for a vector hit that was also used as an
	// expansion seed and matched trivially at depth 0; the originating
	// seed otherwise). Empty for a vector-only item that was never used
	// as an expansion seed.
	AnchorNodeID string

	// Explanation is a human-readable trace of why this item was
	// retrieved, e.g. "vector similarity (rank 2, score 0.83)" or
	// "graph expansion from fact-1: fact-1 --governing_rule--> rule-4"
	// (built from traversal.Path.Explain() for graph-sourced items). See
	// explain.go.
	Explanation string
}

// Result is the outcome of Retriever.Retrieve: every fused Item, sorted
// by descending CombinedScore, plus bookkeeping about how the retrieval
// was bounded.
type Result struct {
	// Items is every fused result, sorted by descending CombinedScore,
	// deduplicated and capped per HybridQuery.TopK/MaxPerAnchor. Empty
	// (not nil) when nothing was found.
	Items []Item

	// VectorHitCount is how many candidates vector recall returned before
	// fusion, dedup, and TopK truncation.
	VectorHitCount int

	// ExpansionSeedCount is how many distinct seed nodes graph expansion
	// was attempted from (vector hits, plus AnchorNodeID if set).
	ExpansionSeedCount int

	// ExpansionSkipped reports whether graph expansion was skipped
	// entirely because the query's Budget was already exhausted by
	// vector recall alone. See budget.go.
	ExpansionSkipped bool

	// ExpansionTruncated reports whether graph expansion was cut short
	// (fewer seeds walked, or a shallower MaxExpansionDepth applied, than
	// the query requested) because the Budget ran out partway through.
	ExpansionTruncated bool
}
