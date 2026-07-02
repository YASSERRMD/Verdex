package traversal

import (
	"fmt"
	"strings"

	"github.com/YASSERRMD/verdex/packages/irac"
)

// Query is a declarative, multi-hop traversal request over a case's
// reasoning tree: start at one node, then follow a sequence of HopSpecs
// (each either a real irac.Edge walk or a resolver-backed logical hop),
// bounded by MaxDepth, and rank the discovered Paths with ScoreFunc.
//
// Query is built via the fluent NewQuery(...).Via(...).MaxDepth(...)
// pattern rather than being constructed as a bare struct literal in
// normal usage — the builder methods return a modified copy, so a
// partially-built Query can be safely reused as a template for several
// variants (e.g. the same starting hops with different MaxDepth bounds)
// without the caller needing to clone slices by hand.
//
// # Why a builder over a bare struct
//
// A struct-based query spec (`Query{CaseID: ..., Hops: []HopSpec{...}}`)
// was considered and rejected as the primary API: the three named legal-
// reasoning hops the phase plan calls for (issue -> governing rule,
// rule -> controlling precedent, precedent -> distinguishing facts) read
// far more clearly as chained method calls
// (`.ViaGoverningRule().ViaControllingPrecedent().ViaDistinguishingFacts()`)
// than as a slice of HopSpec literals a caller has to assemble by hand,
// and the fluent form composes naturally with per-hop options like
// MaxDepth and RankBy. Query's fields are still exported, so a caller
// that does prefer building a HopSpec slice directly (e.g. programmatic
// query generation) can do so and assign it to Hops.
type Query struct {
	// CaseID scopes the traversal to a single case's reasoning tree,
	// mirroring graph.TraversalQuery.CaseID.
	CaseID string

	// StartNodeID is the node the traversal begins from.
	StartNodeID string

	// Hops is the ordered sequence of steps to walk from StartNodeID.
	// Must contain at least one HopSpec.
	Hops []HopSpec

	// MaxDepth bounds how many hops the traversal will follow. Zero (the
	// default) means "walk the full Hops sequence, unbounded by depth" —
	// note this differs from "unbounded" in the sense of graph size: a
	// Query's own Hops slice already bounds the number of distinct hop
	// steps, so MaxDepth further restricts within that when a caller
	// wants a shorter prefix of a longer configured Query (e.g. a
	// template Query with 4 Hops, executed with MaxDepth 2 to get only
	// the first two).
	MaxDepth int

	// ScoreFunc ranks discovered Paths. Nil means DefaultScoreFunc.
	ScoreFunc ScoreFunc

	// PrecedentResolver resolves HopKindControllingPrecedent hops. Nil
	// means NoPrecedents (the hop always yields zero results).
	PrecedentResolver PrecedentResolver

	// DistinguishingFactResolver resolves HopKindDistinguishingFacts
	// hops. Nil means NoDistinguishingFacts (the hop always yields zero
	// results).
	DistinguishingFactResolver DistinguishingFactResolver
}

// NewQuery constructs a Query starting traversal from startNodeID within
// caseID, with no hops yet configured. Chain Via/ViaGoverningRule/
// ViaControllingPrecedent/ViaDistinguishingFacts to add hops, then pass
// the result to Walker.Execute.
func NewQuery(caseID, startNodeID string) Query {
	return Query{CaseID: caseID, StartNodeID: startNodeID}
}

// clone returns a copy of q with its own independent Hops backing array,
// so builder methods never mutate a Query some other variable still
// references.
func (q Query) clone() Query {
	out := q
	out.Hops = make([]HopSpec, len(q.Hops))
	copy(out.Hops, q.Hops)
	return out
}

// Via appends a general-purpose hop walking edgeType in the given
// direction, optionally restricting the resulting nodes to nodeTypeFilter
// (pass "" for no filter). Returns a new Query; the receiver is
// unmodified.
func (q Query) Via(edgeType irac.EdgeType, direction Direction, nodeTypeFilter irac.NodeType) Query {
	out := q.clone()
	out.Hops = append(out.Hops, HopSpec{
		Kind:           HopKindCustom,
		EdgeType:       edgeType,
		Direction:      direction,
		NodeTypeFilter: nodeTypeFilter,
	})
	return out
}

// ViaGoverningRule appends the "issue -> governing rule" hop
// (HopKindGoverningRule): walks irac.EdgeGoverns in Reverse to go from an
// IssueNode to the RuleNode(s) governing it. See HopKindGoverningRule's
// doc comment for why this is a Reverse walk.
func (q Query) ViaGoverningRule() Query {
	out := q.clone()
	out.Hops = append(out.Hops, HopSpec{
		Kind:           HopKindGoverningRule,
		EdgeType:       irac.EdgeGoverns,
		Direction:      Reverse,
		NodeTypeFilter: irac.NodeRule,
	})
	return out
}

// ViaControllingPrecedent appends the "rule -> controlling precedent" hop
// (HopKindControllingPrecedent), resolved at Walker.Execute time via the
// Query's PrecedentResolver rather than a literal graph edge. See
// precedent.go.
func (q Query) ViaControllingPrecedent() Query {
	out := q.clone()
	out.Hops = append(out.Hops, HopSpec{Kind: HopKindControllingPrecedent, NodeTypeFilter: irac.NodeRule})
	return out
}

// ViaDistinguishingFacts appends the "precedent -> distinguishing facts"
// hop (HopKindDistinguishingFacts), resolved at Walker.Execute time via
// the Query's DistinguishingFactResolver rather than a literal graph
// edge. See distinguish.go.
func (q Query) ViaDistinguishingFacts() Query {
	out := q.clone()
	out.Hops = append(out.Hops, HopSpec{Kind: HopKindDistinguishingFacts, NodeTypeFilter: irac.NodeFact})
	return out
}

// WithMaxDepth returns a copy of q with MaxDepth set to maxDepth.
func (q Query) WithMaxDepth(maxDepth int) Query {
	out := q.clone()
	out.MaxDepth = maxDepth
	return out
}

// RankBy returns a copy of q with ScoreFunc set to scoreFunc.
func (q Query) RankBy(scoreFunc ScoreFunc) Query {
	out := q.clone()
	out.ScoreFunc = scoreFunc
	return out
}

// WithPrecedentResolver returns a copy of q with PrecedentResolver set to
// resolver.
func (q Query) WithPrecedentResolver(resolver PrecedentResolver) Query {
	out := q.clone()
	out.PrecedentResolver = resolver
	return out
}

// WithDistinguishingFactResolver returns a copy of q with
// DistinguishingFactResolver set to resolver.
func (q Query) WithDistinguishingFactResolver(resolver DistinguishingFactResolver) Query {
	out := q.clone()
	out.DistinguishingFactResolver = resolver
	return out
}

// validate checks q for the structural errors Walker.Execute rejects
// before ever touching a graph.GraphStore.
func (q Query) validate() error {
	if q.CaseID == "" {
		return ErrEmptyCaseID
	}
	if q.StartNodeID == "" {
		return ErrEmptyStartNodeID
	}
	if len(q.Hops) == 0 {
		return ErrNoHops
	}
	if q.MaxDepth < 0 {
		return ErrInvalidMaxDepth
	}
	for i, hop := range q.Hops {
		if hop.Kind == HopKindCustom && hop.EdgeType == "" {
			return fmt.Errorf("traversal: hop %d: %w", i, ErrNoHops)
		}
	}
	return nil
}

// effectiveDepth returns the number of hops Walker.Execute should
// actually walk: the full Hops sequence, or a shorter prefix when
// MaxDepth is set and smaller than len(Hops).
func (q Query) effectiveDepth() int {
	if q.MaxDepth <= 0 || q.MaxDepth >= len(q.Hops) {
		return len(q.Hops)
	}
	return q.MaxDepth
}

// cacheKey returns a stable string uniquely identifying q's cacheable
// shape (every field influencing the traversal's result set), for use as
// a Cache key. Function-valued fields (ScoreFunc, PrecedentResolver,
// DistinguishingFactResolver) are deliberately excluded: two Querys
// differing only in which Go function value they carry would otherwise
// never hit the cache, defeating its purpose, and re-ranking a cached
// path set by a different ScoreFunc is cheap enough to redo on every
// call (see Cache.Get in cache.go).
func (q Query) cacheKey() string {
	var b strings.Builder
	b.WriteString(q.CaseID)
	b.WriteByte('|')
	b.WriteString(q.StartNodeID)
	b.WriteByte('|')
	fmt.Fprintf(&b, "%d", q.effectiveDepth())
	for _, h := range q.Hops {
		b.WriteByte('|')
		b.WriteString(string(h.Kind))
		b.WriteByte(':')
		b.WriteString(string(h.EdgeType))
		b.WriteByte(':')
		b.WriteString(h.Direction.String())
		b.WriteByte(':')
		b.WriteString(string(h.NodeTypeFilter))
	}
	return b.String()
}
