package ocr

import "sort"

// TextBlock represents one contiguous span of extracted text, anchored to a
// location on the source page via a bounding box.
type TextBlock struct {
	// Text is the extracted text for this block.
	Text string

	// Confidence is the provider's confidence score for this block, in the
	// closed interval [0, 1]. A value of 0 with no other signal typically
	// means "unknown confidence" for adapters that don't report scores.
	Confidence float64

	// Page is the 1-based page number this block was extracted from. A zero
	// value means unknown/unspecified/single-page.
	Page int

	// BoundingBox is the source-span reference locating this block on the
	// page, in pixel coordinates.
	BoundingBox BoundingBox

	// RegionType classifies the kind of region this block belongs to (e.g.
	// paragraph, heading, table, figure), when layout detection has run.
	// Empty when unclassified.
	RegionType RegionType
}

// ExtractionResult is the provider-agnostic result of extracting text from
// an ImageInput.
type ExtractionResult struct {
	// ProviderID identifies which OCRProvider produced this result.
	ProviderID string

	// Language is the ISO 639-1 language code the text was extracted in,
	// when known.
	Language string

	// Blocks holds the ordered text blocks. Blocks MUST be ordered by page
	// ascending, then by reading order (top-to-bottom, left-to-right for
	// left-to-right scripts) within a page.
	Blocks []TextBlock

	// Regions holds the detected layout regions (paragraphs, tables,
	// headings, figures), when layout detection has run. May be empty if no
	// LayoutDetector was configured beyond the no-op default.
	Regions []Region

	// Tables holds structured table data extracted from any regions
	// classified as RegionTypeTable. May be empty if the document contains
	// no tables or table extraction was not performed.
	Tables []Table

	// SourceHash is the SHA-256 hex digest of the original ImageInput.Data,
	// computed before the source bytes were discarded.
	SourceHash string
}

// FullText concatenates the text of every block, in order, separated by a
// single space.
func (r *ExtractionResult) FullText() string {
	if r == nil || len(r.Blocks) == 0 {
		return ""
	}
	out := r.Blocks[0].Text
	for _, b := range r.Blocks[1:] {
		out += " " + b.Text
	}
	return out
}

// SortBlocks orders Blocks by Page ascending, then by BoundingBox.Y
// ascending, then by BoundingBox.X ascending (stable, so blocks with equal
// position retain their relative order). Assembly logic should call this
// after concatenating blocks produced from multiple pages or passes.
func (r *ExtractionResult) SortBlocks() {
	if r == nil {
		return
	}
	sort.SliceStable(r.Blocks, func(i, j int) bool {
		a, b := r.Blocks[i], r.Blocks[j]
		if a.Page != b.Page {
			return a.Page < b.Page
		}
		if a.BoundingBox.Y != b.BoundingBox.Y {
			return a.BoundingBox.Y < b.BoundingBox.Y
		}
		return a.BoundingBox.X < b.BoundingBox.X
	})
}

// ClampConfidence clamps c into the closed interval [0, 1].
func ClampConfidence(c float64) float64 {
	if c < 0 {
		return 0
	}
	if c > 1 {
		return 1
	}
	return c
}

// AverageConfidence returns the unweighted arithmetic mean of every block's
// Confidence score. It returns 0 for an empty slice.
func AverageConfidence(blocks []TextBlock) float64 {
	if len(blocks) == 0 {
		return 0
	}
	var sum float64
	for _, b := range blocks {
		sum += b.Confidence
	}
	return sum / float64(len(blocks))
}

// LowConfidenceBlocks returns the subset of blocks whose Confidence is
// strictly below threshold, preserving order. Useful for flagging portions
// of an extraction that may need human review.
func LowConfidenceBlocks(blocks []TextBlock, threshold float64) []TextBlock {
	var out []TextBlock
	for _, b := range blocks {
		if b.Confidence < threshold {
			out = append(out, b)
		}
	}
	return out
}
