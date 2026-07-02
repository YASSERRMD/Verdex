package ocr_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/ocr"
)

func TestExtractionResult_FullText(t *testing.T) {
	r := &ocr.ExtractionResult{
		Blocks: []ocr.TextBlock{
			{Text: "hello"},
			{Text: "world"},
		},
	}
	if got, want := r.FullText(), "hello world"; got != want {
		t.Errorf("FullText() = %q, want %q", got, want)
	}
}

func TestExtractionResult_FullText_Empty(t *testing.T) {
	var r *ocr.ExtractionResult
	if got := r.FullText(); got != "" {
		t.Errorf("FullText() on nil = %q, want empty", got)
	}

	r2 := &ocr.ExtractionResult{}
	if got := r2.FullText(); got != "" {
		t.Errorf("FullText() on no blocks = %q, want empty", got)
	}
}

func TestExtractionResult_SortBlocks(t *testing.T) {
	r := &ocr.ExtractionResult{
		Blocks: []ocr.TextBlock{
			{Text: "page2", Page: 2, BoundingBox: ocr.BoundingBox{Y: 0, X: 0}},
			{Text: "page1-bottom", Page: 1, BoundingBox: ocr.BoundingBox{Y: 100, X: 0}},
			{Text: "page1-top-right", Page: 1, BoundingBox: ocr.BoundingBox{Y: 0, X: 50}},
			{Text: "page1-top-left", Page: 1, BoundingBox: ocr.BoundingBox{Y: 0, X: 0}},
		},
	}

	r.SortBlocks()

	want := []string{"page1-top-left", "page1-top-right", "page1-bottom", "page2"}
	if len(r.Blocks) != len(want) {
		t.Fatalf("SortBlocks() produced %d blocks, want %d", len(r.Blocks), len(want))
	}
	for i, w := range want {
		if r.Blocks[i].Text != w {
			t.Errorf("Blocks[%d].Text = %q, want %q", i, r.Blocks[i].Text, w)
		}
	}
}

func TestExtractionResult_SortBlocks_Nil(t *testing.T) {
	var r *ocr.ExtractionResult
	r.SortBlocks() // must not panic
}

func TestClampConfidence(t *testing.T) {
	tests := []struct {
		in   float64
		want float64
	}{
		{-1, 0},
		{0, 0},
		{0.5, 0.5},
		{1, 1},
		{2, 1},
	}
	for _, tt := range tests {
		if got := ocr.ClampConfidence(tt.in); got != tt.want {
			t.Errorf("ClampConfidence(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestAverageConfidence(t *testing.T) {
	if got := ocr.AverageConfidence(nil); got != 0 {
		t.Errorf("AverageConfidence(nil) = %v, want 0", got)
	}

	blocks := []ocr.TextBlock{{Confidence: 0.2}, {Confidence: 0.6}, {Confidence: 1.0}}
	if got, want := ocr.AverageConfidence(blocks), 0.6; got < want-1e-9 || got > want+1e-9 {
		t.Errorf("AverageConfidence() = %v, want %v", got, want)
	}
}

func TestLowConfidenceBlocks(t *testing.T) {
	blocks := []ocr.TextBlock{
		{Text: "low", Confidence: 0.2},
		{Text: "high", Confidence: 0.9},
		{Text: "borderline", Confidence: 0.5},
	}

	got := ocr.LowConfidenceBlocks(blocks, 0.5)
	if len(got) != 1 {
		t.Fatalf("LowConfidenceBlocks() returned %d blocks, want 1", len(got))
	}
	if got[0].Text != "low" {
		t.Errorf("LowConfidenceBlocks()[0].Text = %q, want %q", got[0].Text, "low")
	}
}
