package application

import "github.com/YASSERRMD/verdex/packages/irac"

// Origin classifies which body of law an OriginatedRule was drawn from:
// enacted statute or decided precedent. This package deliberately does
// not import packages/statute or packages/precedent (see doc.go's design
// principles) — Origin is a local, opaque enum a future orchestration
// phase can set when handing this package rules sourced from either
// package.
type Origin string

const (
	// OriginStatute marks a rule drawn from enacted statutory text.
	OriginStatute Origin = "statute"

	// OriginPrecedent marks a rule drawn from a decided case (binding or
	// persuasive authority).
	OriginPrecedent Origin = "precedent"
)

// IsValid reports whether o is one of the recognized Origin constants.
func (o Origin) IsValid() bool {
	switch o {
	case OriginStatute, OriginPrecedent:
		return true
	default:
		return false
	}
}

// OriginatedRule wraps an irac.RuleNode with the Origin it was drawn
// from. Callers (a future orchestration phase) construct OriginatedRules
// from either packages/statute or packages/precedent output without this
// package needing to import either — both packages already represent
// their output as irac.RuleNode (see packages/irac/node.go's RuleNode
// doc comment: "a legal rule, statute, or precedent invoked to resolve
// an Issue" — there is no separate node type per origin).
type OriginatedRule struct {
	// Rule is the underlying IRAC rule node.
	Rule irac.RuleNode

	// Origin identifies whether Rule was drawn from a statute or a
	// precedent.
	Origin Origin
}
