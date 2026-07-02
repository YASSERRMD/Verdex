package citation

import (
	"context"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
)

// ResolvedCitation is what a Resolver produces for a single node: the
// formatted citation text, the Origin it was drawn from, and how
// certain the resolution is. This package never trusts a Resolver's
// output blindly — Verify (verify.go) independently confirms the node
// exists, and Score (confidence.go) folds Certainty into the final
// confidence score.
type ResolvedCitation struct {
	// Text is the formatted citation string (e.g. "Act 12, s.5(a)" or
	// "Smith v Jones [2020] UKSC 1").
	Text string

	// Origin identifies whether the resolved rule was drawn from a
	// statute or a precedent. OriginUnknown for non-rule nodes.
	Origin Origin

	// Certainty classifies how the Resolver produced Text: an exact match
	// against a known citation record, or a fallback heuristic (e.g.
	// synthesizing a citation from bare node text when no structured
	// citation record was available). See Certainty.
	Certainty Certainty
}

// Certainty classifies how confident a Resolver is in a ResolvedCitation.
type Certainty string

const (
	// CertaintyExact marks a ResolvedCitation backed by a structured
	// citation record (e.g. packages/statute's Citation or
	// packages/precedent's PrecedentRule.Citation) looked up by node ID.
	CertaintyExact Certainty = "exact"

	// CertaintyHeuristic marks a ResolvedCitation synthesized from
	// whatever text was available (e.g. the bare node text) because no
	// structured citation record could be found for the node.
	CertaintyHeuristic Certainty = "heuristic"

	// CertaintyNone marks a Resolver's explicit "no citation available"
	// answer distinguishing "I looked and found nothing" from
	// ErrUnresolvedCitation being an actual failure.
	CertaintyNone Certainty = "none"
)

// allCertainties is the exhaustive set of recognized Certainty values.
var allCertainties = map[Certainty]struct{}{
	CertaintyExact:     {},
	CertaintyHeuristic: {},
	CertaintyNone:      {},
}

// IsValid reports whether c is one of the recognized Certainty constants.
func (c Certainty) IsValid() bool {
	_, ok := allCertainties[c]
	return ok
}

// Resolver resolves a node to a formatted citation, given the node's own
// data. Implementations typically close over a lookup keyed by node ID
// into whatever structured citation records packages/statute or
// packages/precedent produced (e.g. a map from node ID to
// statute.Citation.String() or precedent.PrecedentRule.Citation) — this
// package never imports either directly, mirroring
// packages/traversal.PrecedentResolver's decoupling pattern: the caller
// that already has both a reasoning tree and the statute/precedent output
// used to build it is best positioned to supply this glue.
//
// A Resolver that has nothing to say about node should return
// ResolvedCitation{Certainty: CertaintyNone}, nil rather than an error —
// errors are reserved for resolution failures (e.g. a backing store being
// unreachable), not for "this node has no citation".
type Resolver func(ctx context.Context, node irac.Node) (ResolvedCitation, error)

// NoResolver is a Resolver that always resolves to CertaintyNone,
// producing no citation text for any node. Useful as a default when a
// caller wants CitedUnit.Spans/Text populated from the GraphStore without
// attempting citation-text resolution.
func NoResolver(_ context.Context, _ irac.Node) (ResolvedCitation, error) {
	return ResolvedCitation{Certainty: CertaintyNone}, nil
}

// Resolve populates unit's Text, Spans, Origin, and Citation fields by
// fetching unit's underlying node from store (scoped to unit.CaseID) and
// running resolver against it. It returns ErrNilGraphStore or
// ErrNilResolver if either dependency is nil, and propagates
// graph.ErrNodeNotFound (or any other store/resolver error) unchanged so
// callers can distinguish "node does not exist" from "resolver failed".
//
// Resolve does not itself verify the node belongs to unit.CaseID beyond
// what store.GetNode already scopes internally; call Verify (verify.go)
// after Resolve for the full anti-hallucination check.
func Resolve(ctx context.Context, store graph.GraphStore, resolver Resolver, unit CitedUnit) (CitedUnit, error) {
	if store == nil {
		return unit, ErrNilGraphStore
	}
	if resolver == nil {
		return unit, ErrNilResolver
	}
	if unit.NodeID == "" {
		return unit, ErrEmptyNodeID
	}

	node, err := store.GetNode(ctx, unit.NodeID)
	if err != nil {
		return unit, err
	}

	resolved, err := resolver(ctx, node)
	if err != nil {
		return unit, err
	}

	out := unit
	out.NodeType = node.Type
	out.Text = node.Text
	out.Origin = resolved.Origin
	out.Citation = resolved.Text
	return out, nil
}

// ResolveAll runs Resolve over every unit in units, stopping at the first
// error and returning it alongside the partially-resolved slice completed
// so far.
func ResolveAll(ctx context.Context, store graph.GraphStore, resolver Resolver, units []CitedUnit) ([]CitedUnit, error) {
	out := make([]CitedUnit, 0, len(units))
	for _, u := range units {
		resolved, err := Resolve(ctx, store, resolver, u)
		if err != nil {
			return out, err
		}
		out = append(out, resolved)
	}
	return out, nil
}

// RuleTextResolver returns a Resolver that synthesizes a CertaintyHeuristic
// ResolvedCitation directly from a NodeRule node's own Text, tagged with
// origin. This is the fallback resolver used when no structured citation
// record is available for a rule node — it still gives a caller something
// traceable, but Certainty flags it as weaker than an exact structured
// lookup. Non-rule nodes resolve to CertaintyNone.
func RuleTextResolver(origin Origin) Resolver {
	return func(_ context.Context, node irac.Node) (ResolvedCitation, error) {
		if node.Type != irac.NodeRule {
			return ResolvedCitation{Certainty: CertaintyNone}, nil
		}
		if node.Text == "" {
			return ResolvedCitation{Certainty: CertaintyNone}, nil
		}
		return ResolvedCitation{
			Text:      node.Text,
			Origin:    origin,
			Certainty: CertaintyHeuristic,
		}, nil
	}
}

// LookupResolver returns a Resolver backed by a caller-supplied map from
// node ID to a pre-formatted citation string and Origin, produced exactly
// once (e.g. from packages/statute.BuiltRule.Citation.String() or
// packages/precedent.PrecedentRule.Citation) by whatever orchestration
// phase built the reasoning tree from statute/precedent output. Every
// entry resolves at CertaintyExact; node IDs absent from records fall
// back to fallback (which may be nil, meaning CertaintyNone).
func LookupResolver(records map[string]ResolvedCitation, fallback Resolver) Resolver {
	return func(ctx context.Context, node irac.Node) (ResolvedCitation, error) {
		if rc, ok := records[node.ID]; ok {
			rc.Certainty = CertaintyExact
			return rc, nil
		}
		if fallback == nil {
			return ResolvedCitation{Certainty: CertaintyNone}, nil
		}
		return fallback(ctx, node)
	}
}
