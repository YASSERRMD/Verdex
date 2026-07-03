package casesearch_test

import (
	"strings"
	"testing"

	"github.com/YASSERRMD/verdex/packages/casesearch"
)

func TestExtractSnippet_HighlightsMatch(t *testing.T) {
	text := "The tenant breached the lease by failing to pay rent for three consecutive months."
	got := casesearch.ExtractSnippet(text, "breached the lease")

	if !strings.Contains(got, casesearch.SnippetHighlightOpen+"breached the lease"+casesearch.SnippetHighlightClose) {
		t.Fatalf("expected highlighted match in snippet, got %q", got)
	}
}

func TestExtractSnippet_CaseInsensitive(t *testing.T) {
	text := "This Contract was BREACHED by the defendant."
	got := casesearch.ExtractSnippet(text, "breached")

	if !strings.Contains(strings.ToLower(got), "breached") {
		t.Fatalf("expected match found case-insensitively, got %q", got)
	}
	if !strings.Contains(got, casesearch.SnippetHighlightOpen) {
		t.Fatalf("expected a highlight marker present, got %q", got)
	}
}

func TestExtractSnippet_NoMatch_ReturnsPlainExcerpt(t *testing.T) {
	text := "Some unrelated node text with no matching terms at all."
	got := casesearch.ExtractSnippet(text, "nonexistent")

	if strings.Contains(got, casesearch.SnippetHighlightOpen) {
		t.Fatalf("expected no highlight marker when query not found, got %q", got)
	}
	if got == "" {
		t.Fatal("expected a non-empty excerpt")
	}
}

func TestExtractSnippet_EmptyText_ReturnsEmpty(t *testing.T) {
	if got := casesearch.ExtractSnippet("", "query"); got != "" {
		t.Fatalf("expected empty snippet for empty text, got %q", got)
	}
}

func TestExtractSnippet_TruncatesLongText(t *testing.T) {
	longText := strings.Repeat("a", 500) + " needle " + strings.Repeat("b", 500)
	got := casesearch.ExtractSnippet(longText, "needle")

	if len(got) >= len(longText) {
		t.Fatalf("expected snippet shorter than source text, got len=%d source_len=%d", len(got), len(longText))
	}
	if !strings.Contains(got, casesearch.SnippetHighlightOpen+"needle"+casesearch.SnippetHighlightClose) {
		t.Fatalf("expected highlighted needle, got %q", got)
	}
}
