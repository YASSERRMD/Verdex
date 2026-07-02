package segmentation_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/segmentation"
)

func TestIsHeadingLine(t *testing.T) {
	tests := []struct {
		name string
		line string
		want bool
	}{
		{"numbered-section", "1. Introduction", true},
		{"nested-numbering", "1.1 Background of the Case", true},
		{"roman-numeral", "IV. Findings of Fact", true},
		{"lettered-subsection", "(a) The petitioner's claim", true},
		{"section-keyword", "Section 3 Jurisdiction", true},
		{"article-keyword", "Article 12 Fundamental Rights", true},
		{"chapter-keyword", "Chapter IV Remedies", true},
		{"all-caps-short", "STATEMENT OF FACTS", true},
		{"title-case-short", "Statement Of Facts", true},
		{"title-case-with-connectors", "Order Of The Court", true},
		{"plain-sentence-not-heading", "The court finds for the petitioner.", false},
		{"long-all-caps-not-heading", "THE COURT HEREBY FINDS THAT THE RESPONDENT IS LIABLE FOR ALL DAMAGES CLAIMED", false},
		{"empty", "", false},
		{"whitespace-only", "   ", false},
		{"lowercase-sentence-fragment", "he appeared before the magistrate", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := segmentation.IsHeadingLine(tt.line); got != tt.want {
				t.Errorf("IsHeadingLine(%q) = %v, want %v", tt.line, got, tt.want)
			}
		})
	}
}

func TestTagHeadings(t *testing.T) {
	segs := []segmentation.Segment{
		{Type: segmentation.SegmentParagraph, Text: "STATEMENT OF FACTS"},
		{Type: segmentation.SegmentParagraph, Text: "The petitioner filed suit on the given date."},
		{Type: segmentation.SegmentExhibit, Text: "ANNEXURE B"}, // already tagged, should not change
	}

	got := segmentation.TagHeadings(segs)

	if got[0].Type != segmentation.SegmentHeading {
		t.Errorf("segment[0].Type = %v, want SegmentHeading", got[0].Type)
	}
	if got[1].Type != segmentation.SegmentParagraph {
		t.Errorf("segment[1].Type = %v, want SegmentParagraph", got[1].Type)
	}
	if got[2].Type != segmentation.SegmentExhibit {
		t.Errorf("segment[2].Type = %v, want SegmentExhibit (unchanged)", got[2].Type)
	}

	// TagHeadings must not mutate the input slice's underlying data.
	if segs[0].Type != segmentation.SegmentParagraph {
		t.Errorf("input segment[0].Type mutated to %v", segs[0].Type)
	}
}
