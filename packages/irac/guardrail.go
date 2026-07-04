package irac

import (
	"regexp"
	"strings"
	"time"
)

// DraftAnalysisLabel is the mandatory non-binding-guardrail label required
// by CONTRIBUTING.md: "Every module that produces reasoning output must
// attach the draft_analysis label. Verdict or directive language is
// rejected." A ConclusionNode is reasoning output, so this label is always
// attached, unconditionally, by NewConclusionNode — there is no
// constructor path that omits it.
const DraftAnalysisLabel = "draft_analysis"

// verdictLanguageWordlist holds verdict/directive-sounding words and
// phrases that must never appear (case-insensitively) in the guardrail
// label itself. This package does not scan node Text for these — that is
// a downstream concern — but it guarantees the guardrail label constant
// stays purely descriptive/non-binding and never drifts into directive
// language.
var verdictLanguageWordlist = []string{
	"guilty",
	"liable",
	"shall pay",
	"is ordered",
	"is hereby ordered",
	"judgment for",
	"convicted",
	"acquitted",
	"sentenced",
}

// NewConclusionNode constructs a ConclusionNode with Type fixed to
// NodeConclusion and Label unconditionally set to DraftAnalysisLabel. This
// is the only exported way to build a ConclusionNode, so it is not
// possible to construct one without the mandatory guardrail label
// attached.
func NewConclusionNode(id, caseID, text string, createdAt time.Time, confidence float64, provenance Provenance, spans ...SourceSpan) ConclusionNode {
	return ConclusionNode{
		Node: Node{
			ID:         id,
			Type:       NodeConclusion,
			CaseID:     caseID,
			Text:       text,
			CreatedAt:  createdAt,
			Confidence: confidence,
			Provenance: provenance,
		},
		Label: DraftAnalysisLabel,
		Spans: spans,
	}
}

// HasGuardrailLabel reports whether c carries the mandatory
// DraftAnalysisLabel. Always true for any ConclusionNode built via
// NewConclusionNode; used defensively by validate.go and serialize.go to
// catch a ConclusionNode that reached this package's boundary some other
// way (e.g. decoded from untrusted JSON with the field stripped).
func (c ConclusionNode) HasGuardrailLabel() bool {
	return c.Label == DraftAnalysisLabel
}

// whitespaceRun matches one or more consecutive whitespace characters, used
// to normalize runs of spaces/tabs/newlines to a single space before
// wordlist matching. This closes an obfuscation gap where padding a
// wordlist phrase with extra internal whitespace (e.g. "shall   pay")
// defeated the exact-substring match despite carrying the same meaning.
var whitespaceRun = regexp.MustCompile(`\s+`)

// ContainsVerdictLanguage reports whether s contains any verdict- or
// directive-sounding word or phrase from verdictLanguageWordlist,
// case-insensitively and independent of internal whitespace variation
// (runs of spaces/tabs/newlines within s are normalized to a single space
// before matching). Exposed so callers and tests can confirm the
// guardrail label (and, optionally, other reasoning-output text) never
// carries binding verdict/directive language.
func ContainsVerdictLanguage(s string) bool {
	lower := whitespaceRun.ReplaceAllString(strings.ToLower(s), " ")
	for _, word := range verdictLanguageWordlist {
		if strings.Contains(lower, word) {
			return true
		}
	}
	return false
}
