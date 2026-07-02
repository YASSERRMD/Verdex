package lawapplication

import "strings"

// Origin classifies which body of law a controlling rule was drawn from:
// enacted statute or decided precedent. Mirrors
// packages/application.Origin and packages/citation.Origin exactly (same
// three string values), but is this package's own type — this package
// does not import packages/application (a tree-assembly-time package;
// see doc/law-application.md) and does not require every caller to have
// gone through packages/citation to supply an Origin.
type Origin string

const (
	// OriginUnknown marks a rule whose Origin could not be determined
	// from any available signal.
	OriginUnknown Origin = ""

	// OriginStatute marks a rule drawn from enacted statutory text.
	OriginStatute Origin = "statute"

	// OriginPrecedent marks a rule drawn from a decided case (binding or
	// persuasive authority).
	OriginPrecedent Origin = "precedent"
)

// IsValid reports whether o is one of the recognized Origin constants
// (including OriginUnknown).
func (o Origin) IsValid() bool {
	switch o {
	case OriginUnknown, OriginStatute, OriginPrecedent:
		return true
	default:
		return false
	}
}

// statuteKeywords are lowercase substrings whose presence in a RuleRef's
// Text suggests it was drawn from enacted statutory text.
var statuteKeywords = []string{
	"§",
	"section ",
	"statute",
	"code ann",
	"u.s.c.",
	"c.f.r.",
	"act of",
	"pub. l.",
	"public law",
	"enacted",
	"codified at",
}

// precedentKeywords are lowercase substrings whose presence in a
// RuleRef's Text suggests it was drawn from a decided case.
var precedentKeywords = []string{
	" v. ",
	" v ",
	"holding that",
	"the court held",
	"court found",
	"precedent",
	"f.2d",
	"f.3d",
	"f. supp",
	"u.s. ",
	"cir. ",
}

// InferOrigin determines rule's Origin, in priority order:
//  1. rule.OriginHint, if non-empty and valid — an explicit caller
//     override always wins.
//  2. A lexical keyword heuristic over rule.Text: statute-shaped
//     citation fragments (section symbols, U.S.C./C.F.R. references,
//     "enacted", etc.) versus case-shaped citation fragments (" v. ",
//     reporter abbreviations, "the court held", etc.). If both fire,
//     statute wins, mirroring evidenceweighing.ClassifyEvidenceKind's
//     "more conservative/verifiable classification wins on overlap"
//     convention.
//  3. OriginUnknown, if neither signal is present.
//
// This is a known limitation, not a ground-truth classification: the
// authoritative signal (packages/citation's own Origin, surfaced via
// knowledgeapi.ResolveCitation's CitationDTO.Origin) is a strictly
// better source when available, since citation resolution may already
// know a rule's origin from its underlying CitedUnit. A caller that has
// already resolved a rule's citation should populate RuleRef.OriginHint
// from that CitationDTO.Origin value rather than relying on this
// heuristic — see doc/law-application.md's "Origin-inference limitation"
// section for the full tradeoff.
func InferOrigin(rule RuleRef) Origin {
	if rule.OriginHint.IsValid() && rule.OriginHint != OriginUnknown {
		return rule.OriginHint
	}

	if rule.Text == "" {
		return OriginUnknown
	}
	lower := strings.ToLower(rule.Text)

	if containsAny(lower, statuteKeywords) {
		return OriginStatute
	}
	if containsAny(lower, precedentKeywords) {
		return OriginPrecedent
	}
	return OriginUnknown
}

// containsAny reports whether s contains any of substrs.
func containsAny(s string, substrs []string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
