package adaptiveretrieval

import (
	"strconv"
	"strings"

	"github.com/YASSERRMD/verdex/packages/hybridretrieval"
	"github.com/YASSERRMD/verdex/packages/irac"
)

// AdaptiveQuery describes a single query-driven subgraph build: an anchor
// node to walk outward from, the named legal-reasoning hops to follow
// (mirroring hybridretrieval.ExpansionHop's fixed, legally-meaningful
// shapes), and how many prior vector-recall hits (if any) already
// corroborated the anchor. VectorHitCount feeds AdaptiveDepth (depth.go)
// so a query backed by a strong semantic recall step can request a
// shallower, cheaper build than a purely structural one.
//
// Construct with NewAdaptiveQuery or FromHybridQuery, then chain the
// With* methods.
type AdaptiveQuery struct {
	// CaseID scopes the build to a single case's reasoning tree. Required.
	CaseID string

	// AnchorNodeID is the node the subgraph build starts from. Required.
	AnchorNodeID string

	// Hops is the ordered sequence of named hops to walk outward from
	// AnchorNodeID. Empty means "use AdaptiveDepth's default hop
	// sequence" (see depth.go).
	Hops []hybridretrieval.ExpansionHop

	// VectorHitCount is how many vector-recall hits already corroborated
	// this query (typically hybridretrieval.Result.VectorHitCount from an
	// earlier semantic recall step). Zero means "no semantic signal
	// available" — AdaptiveDepth treats this as the case calling for the
	// deepest configured walk, since there is nothing else to compensate
	// with. See AdaptiveDepth.
	VectorHitCount int

	// EdgeType, if non-empty, restricts the treeindex fallback lookup to
	// paths whose first hop matches this irac.EdgeType. Empty means "any
	// edge type", mirroring treeindex.Indexer.LookupPaths's own
	// convention. Ignored by the adaptive build itself (which always
	// walks the named Hops), consulted only when falling back to
	// treeindex.
	EdgeType irac.EdgeType
}

// NewAdaptiveQuery constructs an AdaptiveQuery scoped to caseID, starting
// from anchorNodeID, with no hops configured yet (AdaptiveDepth will
// choose a default sequence).
func NewAdaptiveQuery(caseID, anchorNodeID string) AdaptiveQuery {
	return AdaptiveQuery{CaseID: caseID, AnchorNodeID: anchorNodeID}
}

// FromHybridQuery derives an AdaptiveQuery from a
// hybridretrieval.HybridQuery and the vector-hit count an earlier
// vectorindex/hybridretrieval recall step already found for it. anchorID
// should be the seed node the caller wants the adaptive subgraph rooted
// at — typically the top vector-recall hit's node ID, or hq.AnchorNodeID
// when the hybrid query already carried one.
//
// This lets a caller that already ran a hybridretrieval.Retriever.Retrieve
// pass its query and result straight through to adaptiveretrieval without
// re-deriving CaseID, ExpansionHops, or MaxExpansionDepth by hand.
func FromHybridQuery(hq hybridretrieval.HybridQuery, anchorID string, vectorHitCount int) AdaptiveQuery {
	hops := make([]hybridretrieval.ExpansionHop, len(hq.ExpansionHops))
	copy(hops, hq.ExpansionHops)
	return AdaptiveQuery{
		CaseID:         hq.CaseID,
		AnchorNodeID:   anchorID,
		Hops:           hops,
		VectorHitCount: vectorHitCount,
	}
}

// clone returns a copy of q with its own independent Hops backing array.
func (q AdaptiveQuery) clone() AdaptiveQuery {
	out := q
	out.Hops = make([]hybridretrieval.ExpansionHop, len(q.Hops))
	copy(out.Hops, q.Hops)
	return out
}

// WithHop returns a copy of q with hop appended to Hops.
func (q AdaptiveQuery) WithHop(hop hybridretrieval.ExpansionHop) AdaptiveQuery {
	out := q.clone()
	out.Hops = append(out.Hops, hop)
	return out
}

// WithVectorHitCount returns a copy of q with VectorHitCount set to n.
func (q AdaptiveQuery) WithVectorHitCount(n int) AdaptiveQuery {
	out := q.clone()
	out.VectorHitCount = n
	return out
}

// WithEdgeType returns a copy of q with EdgeType set to edgeType.
func (q AdaptiveQuery) WithEdgeType(edgeType irac.EdgeType) AdaptiveQuery {
	out := q.clone()
	out.EdgeType = edgeType
	return out
}

// validate checks q for the structural errors Builder.Build rejects
// before ever touching a graph.GraphStore.
func (q AdaptiveQuery) validate() error {
	if q.CaseID == "" {
		return ErrEmptyCaseID
	}
	if q.AnchorNodeID == "" {
		return ErrEmptyAnchorNodeID
	}
	return nil
}

// shapeKey returns a stable string uniquely identifying q's cacheable
// shape: everything that influences what subgraph gets built, excluding
// VectorHitCount (which only influences how deep AdaptiveDepth walks, not
// what the cache should key on — two queries differing only in
// VectorHitCount that resolve to the same effective depth should share a
// cache entry). depth is passed in explicitly (the already-resolved
// AdaptiveDepth output) so the key reflects the actual walked depth rather
// than the raw hit count.
func (q AdaptiveQuery) shapeKey(depth int) string {
	var b strings.Builder
	b.WriteString(q.CaseID)
	b.WriteByte('|')
	b.WriteString(q.AnchorNodeID)
	b.WriteByte('|')
	b.WriteString(strconv.Itoa(depth))
	for _, h := range q.Hops {
		b.WriteByte('|')
		b.WriteString(string(h))
	}
	b.WriteByte('|')
	b.WriteString(string(q.EdgeType))
	return b.String()
}
