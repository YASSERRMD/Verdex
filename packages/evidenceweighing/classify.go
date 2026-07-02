package evidenceweighing

import "strings"

// testimonyKeywords are lowercase substrings whose presence in a
// FactRef's Text suggests the fact derives from a witness statement,
// deposition, or party assertion rather than a document. This is a
// best-effort lexical heuristic, not a classifier — see
// doc/evidence-weighing.md's "Testimony vs documentary evidence" section
// for the known limitation this accepts: packages/evidence's
// EvidenceType classification (Phase 026) is not surfaced through
// knowledgeapi.NodeDTO today, so this package cannot inherit that
// upstream signal and instead falls back to a keyword heuristic over the
// fact's own text.
var testimonyKeywords = []string{
	"testified",
	"testimony",
	"stated that",
	"said that",
	"deposition",
	"witness",
	"according to",
	"claims that",
	"alleges that",
	"recalled",
}

// documentaryKeywords are lowercase substrings whose presence in a
// FactRef's Text suggests the fact derives from a document, record, or
// instrument.
var documentaryKeywords = []string{
	"contract",
	"agreement",
	"invoice",
	"receipt",
	"record shows",
	"document",
	"exhibit",
	"letter dated",
	"email dated",
	"report states",
	"registered",
	"filed on",
}

// ClassifyEvidenceKind heuristically classifies text as
// EvidenceKindTestimony, EvidenceKindDocumentary, or EvidenceKindUnknown
// by lexical keyword match. When both testimony and documentary keywords
// are present, documentary wins — a fact referencing a document, even
// when phrased as reported speech (e.g. "the report states that..."), is
// evidentially anchored in the underlying record described, which is the
// more conservative (per jurisdiction.go's EvidenceKindUnknown handling)
// and verifiable classification of the two.
func ClassifyEvidenceKind(text string) EvidenceKind {
	if text == "" {
		return EvidenceKindUnknown
	}
	lower := strings.ToLower(text)

	isDocumentary := containsAny(lower, documentaryKeywords)
	if isDocumentary {
		return EvidenceKindDocumentary
	}

	if containsAny(lower, testimonyKeywords) {
		return EvidenceKindTestimony
	}

	return EvidenceKindUnknown
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
