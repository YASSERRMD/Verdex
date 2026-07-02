package citation

import (
	"github.com/YASSERRMD/verdex/packages/hybridretrieval"
	"github.com/YASSERRMD/verdex/packages/irac"
)

// Origin classifies which body of law a CitedUnit's underlying rule was
// drawn from: enacted statute or decided precedent. This mirrors
// packages/application's Origin/OriginatedRule convention exactly, and is
// redeclared locally (rather than imported) for the same reason
// packages/application does not import packages/statute or
// packages/precedent: both packages already represent their output as
// irac.RuleNode, so "which package produced this rule" is metadata a
// caller supplies, not something derivable from the node itself.
type Origin string

const (
	// OriginUnknown marks a CitedUnit whose Origin was never set (e.g. a
	// fact or issue node, which has no statute/precedent origin).
	OriginUnknown Origin = ""

	// OriginStatute marks a rule drawn from enacted statutory text (see
	// packages/statute).
	OriginStatute Origin = "statute"

	// OriginPrecedent marks a rule drawn from a decided case, binding or
	// persuasive (see packages/precedent).
	OriginPrecedent Origin = "precedent"
)

// IsValid reports whether o is one of the recognized Origin constants,
// including OriginUnknown.
func (o Origin) IsValid() bool {
	switch o {
	case OriginUnknown, OriginStatute, OriginPrecedent:
		return true
	default:
		return false
	}
}

// CitedUnit wraps a single retrieved unit of reasoning (a node, typically
// surfaced as a hybridretrieval.Item) with the exact source span(s) it was
// drawn from and its resolved, formatted citation text. This is the
// central artifact this package produces: every CitedUnit is a promise
// that its Citation can be traced back to real source text, and that the
// underlying node can be independently confirmed to exist (see verify.go).
type CitedUnit struct {
	// NodeID is the underlying irac.Node's ID this citation concerns.
	NodeID string

	// CaseID scopes this citation to a single case's reasoning tree.
	CaseID string

	// NodeType is the underlying node's irac.NodeType, when known.
	NodeType irac.NodeType

	// Text is the node's text content at the time this CitedUnit was
	// built, used by DetectBroken (broken.go) to detect staleness against
	// the node's current text in the GraphStore.
	Text string

	// Spans traces this unit's text back to the ingested source
	// document(s) it was drawn from. May be empty if the underlying node
	// carried no spans.
	Spans irac.Spans

	// Origin identifies whether the underlying rule was drawn from a
	// statute or a precedent. OriginUnknown for non-rule nodes.
	Origin Origin

	// Citation is the resolved, formatted citation text (e.g. "Act 12,
	// s.5(a)" or "Smith v Jones [2020] UKSC 1"), produced by a Resolver
	// (resolver.go) and/or a Formatter (formatter.go).
	Citation string

	// AnchorNodeID is the seed node graph expansion walked from to reach
	// this unit, carried through from hybridretrieval.Item.AnchorNodeID
	// when this CitedUnit was built from one.
	AnchorNodeID string

	// CombinedScore is the retrieval score carried through from
	// hybridretrieval.Item.CombinedScore, when this CitedUnit was built
	// from one. Zero if not built from a hybridretrieval.Item.
	CombinedScore float64
}

// FromItem constructs a CitedUnit from a hybridretrieval.Item, attaching
// caseID and leaving Spans/Citation/Origin unset (zero values) for a
// subsequent Resolver call to populate. This is the standard entry point
// for attaching a source-span-backed citation to every retrieved unit, per
// this package's core guarantee (see doc.go).
func FromItem(caseID string, item hybridretrieval.Item) CitedUnit {
	return CitedUnit{
		NodeID:        item.NodeID,
		CaseID:        caseID,
		NodeType:      item.NodeType,
		Text:          item.Text,
		AnchorNodeID:  item.AnchorNodeID,
		CombinedScore: item.CombinedScore,
	}
}

// FromItems constructs one CitedUnit per hybridretrieval.Item in items, in
// the same order, via FromItem.
func FromItems(caseID string, items []hybridretrieval.Item) []CitedUnit {
	out := make([]CitedUnit, 0, len(items))
	for _, item := range items {
		out = append(out, FromItem(caseID, item))
	}
	return out
}

// HasSpans reports whether u carries at least one SourceSpan.
func (u CitedUnit) HasSpans() bool {
	return len(u.Spans) > 0
}

// HasCitation reports whether u carries non-empty, resolved Citation text.
func (u CitedUnit) HasCitation() bool {
	return u.Citation != ""
}
