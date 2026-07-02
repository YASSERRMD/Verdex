package segmentation_test

import (
	"strings"
	"testing"

	"github.com/YASSERRMD/verdex/packages/segmentation"
)

func TestSplitSentences(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "simple-sentences",
			text: "The court finds for the petitioner. The respondent shall pay costs.",
			want: []string{"The court finds for the petitioner.", "The respondent shall pay costs."},
		},
		{
			name: "abbreviation-not-boundary",
			text: "Mr. Smith appeared before the court. He was represented by counsel.",
			want: []string{"Mr. Smith appeared before the court.", "He was represented by counsel."},
		},
		{
			name: "question-and-exclamation",
			text: "Did the accused appear? He did not! The case was adjourned.",
			want: []string{"Did the accused appear?", "He did not!", "The case was adjourned."},
		},
		{
			name: "no-terminal-punctuation-trailing-fragment",
			text: "The court finds for the petitioner. Costs awarded",
			want: []string{"The court finds for the petitioner.", "Costs awarded"},
		},
		{
			name: "empty",
			text: "",
			want: []string{},
		},
		{
			name: "whitespace-only",
			text: "   \n\t  ",
			want: []string{},
		},
		{
			name: "closing-quote-after-terminator",
			text: `He said "guilty." The room fell silent.`,
			want: []string{`He said "guilty."`, "The room fell silent."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spans := segmentation.SplitSentences(tt.text)
			got := make([]string, len(spans))
			for i, s := range spans {
				got[i] = s.Text
			}
			if len(got) != len(tt.want) {
				t.Fatalf("SplitSentences(%q) = %d spans %v, want %d spans %v", tt.text, len(got), got, len(tt.want), tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("span[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}

			// Fidelity invariant: spans must cover the full rune range of
			// text with no gaps and no overlaps.
			assertFullCoverage(t, tt.text, spans)
		})
	}
}

func TestSplitClauses(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "comma-separated-clauses",
			text: "The petitioner, having filed the suit, sought relief.",
			want: []string{"The petitioner,", "having filed the suit,", "sought relief."},
		},
		{
			name: "semicolon-clause",
			text: "The first claim was dismissed; the second was upheld.",
			want: []string{"The first claim was dismissed;", "the second was upheld."},
		},
		{
			name: "clauses-are-finer-than-sentences",
			text: "First, the facts. Second, the law.",
			want: []string{"First,", "the facts.", "Second,", "the law."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spans := segmentation.SplitClauses(tt.text)
			got := make([]string, len(spans))
			for i, s := range spans {
				got[i] = s.Text
			}
			if len(got) != len(tt.want) {
				t.Fatalf("SplitClauses(%q) = %d spans %v, want %d spans %v", tt.text, len(got), got, len(tt.want), tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("span[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
			assertFullCoverage(t, tt.text, spans)
		})
	}
}

// assertFullCoverage verifies spans, taken in order, cover the full rune
// range of text with no gaps and no overlaps (span.Start/End refer to the
// original text, prior to per-span trimming).
func assertFullCoverage(t *testing.T, text string, spans []segmentation.Span) {
	t.Helper()
	totalRunes := len([]rune(text))
	if len(spans) == 0 {
		// SplitSentences/SplitClauses intentionally return an empty slice
		// for empty OR whitespace-only text, not just totalRunes == 0.
		if strings.TrimSpace(text) != "" {
			t.Errorf("no spans produced for non-empty text %q", text)
		}
		return
	}
	if spans[0].Start != 0 {
		t.Errorf("first span does not start at 0: %+v", spans[0])
	}
	for i := 1; i < len(spans); i++ {
		if spans[i].Start != spans[i-1].End {
			t.Errorf("gap/overlap between span[%d]=%+v and span[%d]=%+v", i-1, spans[i-1], i, spans[i])
		}
	}
	if spans[len(spans)-1].End != totalRunes {
		t.Errorf("last span does not end at %d: %+v", totalRunes, spans[len(spans)-1])
	}
}
