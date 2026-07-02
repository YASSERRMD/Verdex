package uncertainty

import "strings"

// excerptRadius is the number of characters kept on each side of a
// matched phrase when building OverconfidencePhrasing.Excerpt.
const excerptRadius = 40

// overconfidentPhrases holds absolutist/over-confident terms and
// phrases that, per this phase's design, must never appear (case-
// insensitively) in a TentativeConclusion's Text without being flagged.
//
// This wordlist is deliberately disjoint from irac.verdictLanguageWordlist
// (the unexported list backing irac.ContainsVerdictLanguage): that list
// flags binding-outcome language ("guilty", "is hereby ordered", ...);
// this list flags epistemic over-claiming — phrasing that asserts
// certainty a draft, non-binding analysis should never claim, without
// necessarily asserting a binding outcome at all. See doc.go and
// doc/uncertainty-surfacing.md for the full distinction, and
// irac.ContainsVerdictLanguage, which Phase 057 turns into a hard gate —
// a check this package does not perform.
var overconfidentPhrases = []string{
	"definitely",
	"certainly",
	"undeniably",
	"beyond doubt",
	"beyond any doubt",
	"clearly proves",
	"without question",
	"unquestionably",
	"indisputably",
	"there is no doubt",
	"conclusively proves",
}

// scanOverconfidence finds every occurrence of every overconfidentPhrases
// entry in text (case-insensitive) and returns one OverconfidencePhrasing
// per match, tagged with issueNodeID.
//
// This is a flag-only scan: it never rewrites or mutates text. Rewriting
// over-confident conclusion text is out of scope for this package — see
// doc.go's "What this package deliberately does not do" pointer.
func scanOverconfidence(issueNodeID, text string) []OverconfidencePhrasing {
	if text == "" {
		return nil
	}
	lower := strings.ToLower(text)
	var out []OverconfidencePhrasing
	for _, phrase := range overconfidentPhrases {
		start := 0
		for {
			idx := strings.Index(lower[start:], phrase)
			if idx < 0 {
				break
			}
			absolute := start + idx
			out = append(out, OverconfidencePhrasing{
				IssueNodeID: issueNodeID,
				Phrase:      phrase,
				Excerpt:     excerpt(text, absolute, len(phrase)),
			})
			start = absolute + len(phrase)
		}
	}
	return out
}

// excerpt returns a short snippet of text centered on the match starting
// at byte offset matchStart with length matchLen, padded with
// excerptRadius characters on each side (clamped to text's bounds).
func excerpt(text string, matchStart, matchLen int) string {
	begin := matchStart - excerptRadius
	if begin < 0 {
		begin = 0
	}
	end := matchStart + matchLen + excerptRadius
	if end > len(text) {
		end = len(text)
	}
	return strings.TrimSpace(text[begin:end])
}

// identifyOverconfidence scans every TentativeConclusion in
// req.Opinion.Conclusions and returns every OverconfidencePhrasing found.
func identifyOverconfidence(req Request) []OverconfidencePhrasing {
	var out []OverconfidencePhrasing
	for _, tc := range req.Opinion.Conclusions {
		out = append(out, scanOverconfidence(tc.IssueNodeID, tc.Text)...)
	}
	return out
}
