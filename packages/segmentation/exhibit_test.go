package segmentation_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/segmentation"
)

func TestIsExhibitMarker(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{"exhibit-letter", "Please refer to Exhibit A for the signed agreement.", true},
		{"ex-abbreviation", "See Ex. 3 attached hereto.", true},
		{"ex-no-period", "See Ex 3 attached hereto.", true},
		{"annexure", "As shown in Annexure B.", true},
		{"schedule", "Refer to Schedule 1 of the contract.", true},
		{"no-marker", "The petitioner filed the suit on the given date.", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := segmentation.IsExhibitMarker(tt.text); got != tt.want {
				t.Errorf("IsExhibitMarker(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestIsCitationMarker(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{"usc-citation", "This claim arises under 12 U.S.C. § 1983.", true},
		{"section-ipc", "The accused was charged under Section 302 IPC.", true},
		{"article-citation", "This violates Article 21 of the Constitution.", true},
		{"scc-citation", "As held in (2020) 3 SCC 45, the court found for the appellant.", true},
		{"air-citation", "See AIR 1978 SC 597 for the leading precedent.", true},
		{"case-name-v", "The precedent in Smith v. Jones controls this matter.", true},
		{"no-citation", "The petitioner filed the suit on the given date.", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := segmentation.IsCitationMarker(tt.text); got != tt.want {
				t.Errorf("IsCitationMarker(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

func TestTagExhibitsAndCitations(t *testing.T) {
	segs := []segmentation.Segment{
		{Type: segmentation.SegmentParagraph, Text: "Please refer to Exhibit A for the signed agreement."},
		{Type: segmentation.SegmentParagraph, Text: "The accused was charged under Section 302 IPC."},
		{Type: segmentation.SegmentParagraph, Text: "The petitioner filed the suit on the given date."},
		{Type: segmentation.SegmentHeading, Text: "Exhibit A"}, // already a heading, must not be reclassified
	}

	got := segmentation.TagExhibitsAndCitations(segs)

	if got[0].Type != segmentation.SegmentExhibit {
		t.Errorf("segment[0].Type = %v, want SegmentExhibit", got[0].Type)
	}
	if got[1].Type != segmentation.SegmentCitation {
		t.Errorf("segment[1].Type = %v, want SegmentCitation", got[1].Type)
	}
	if got[2].Type != segmentation.SegmentParagraph {
		t.Errorf("segment[2].Type = %v, want SegmentParagraph (unchanged)", got[2].Type)
	}
	if got[3].Type != segmentation.SegmentHeading {
		t.Errorf("segment[3].Type = %v, want SegmentHeading (unchanged, headings take precedence)", got[3].Type)
	}
}

func TestSplitOnExhibitBoundaries(t *testing.T) {
	text := "The witness testified at length. Exhibit A shows the signed contract. Exhibit B shows the receipt."

	spans := segmentation.SplitOnExhibitBoundaries(text)

	if len(spans) != 3 {
		t.Fatalf("SplitOnExhibitBoundaries() = %d spans, want 3; spans=%+v", len(spans), spans)
	}

	wantPrefixes := []string{"The witness testified at length.", "Exhibit A shows", "Exhibit B shows"}
	for i, want := range wantPrefixes {
		if len(spans[i].Text) < len(want) || spans[i].Text[:len(want)] != want {
			t.Errorf("span[%d].Text = %q, want prefix %q", i, spans[i].Text, want)
		}
	}

	assertFullCoverage(t, text, spans)
}

func TestSplitOnExhibitBoundaries_NoMarker(t *testing.T) {
	text := "The petitioner filed the suit on the given date."
	spans := segmentation.SplitOnExhibitBoundaries(text)
	if len(spans) != 1 {
		t.Fatalf("SplitOnExhibitBoundaries() = %d spans, want 1", len(spans))
	}
	assertFullCoverage(t, text, spans)
}

func TestSplitOnExhibitBoundaries_Empty(t *testing.T) {
	spans := segmentation.SplitOnExhibitBoundaries("")
	if len(spans) != 0 {
		t.Errorf("SplitOnExhibitBoundaries(\"\") = %d spans, want 0", len(spans))
	}
}
