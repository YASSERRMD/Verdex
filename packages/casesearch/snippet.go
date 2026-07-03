package casesearch

import "strings"

// SnippetContextChars is how many characters of context are kept on each
// side of a matched term in ExtractSnippet.
const SnippetContextChars = 60

// SnippetHighlightOpen and SnippetHighlightClose bracket the matched
// portion of a snippet, mirroring a lightweight, markup-agnostic
// convention (rather than embedding HTML) so callers — a JSON API, a
// terminal, a future UI — can each apply their own rendering.
const (
	SnippetHighlightOpen  = "**"
	SnippetHighlightClose = "**"
)

// ExtractSnippet returns a short, contextual excerpt of text centered on
// the first case-insensitive occurrence of query, with the match itself
// wrapped in SnippetHighlightOpen/Close. If query is empty or not found
// in text, ExtractSnippet returns a plain leading excerpt of text (no
// highlight) so every Result still carries a readable preview.
func ExtractSnippet(text, query string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	query = strings.TrimSpace(query)
	if query == "" {
		return truncate(text, SnippetContextChars*2)
	}

	lowerText := strings.ToLower(text)
	lowerQuery := strings.ToLower(query)
	idx := strings.Index(lowerText, lowerQuery)
	if idx < 0 {
		return truncate(text, SnippetContextChars*2)
	}

	start := idx - SnippetContextChars
	prefix := ""
	if start < 0 {
		start = 0
	} else {
		prefix = "…"
	}

	end := idx + len(query) + SnippetContextChars
	suffix := ""
	if end > len(text) {
		end = len(text)
	} else {
		suffix = "…"
	}

	before := text[start:idx]
	match := text[idx : idx+len(query)]
	after := text[idx+len(query) : end]

	return prefix + before + SnippetHighlightOpen + match + SnippetHighlightClose + after + suffix
}

// truncate returns the first n bytes of s (rune-safe truncation is not
// required here since this is a display convenience, not a data
// boundary), appending an ellipsis if truncated.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
