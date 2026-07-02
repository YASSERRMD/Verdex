package segmentation

// SourceSpan locates a Segment's text within the original source, carrying
// both the byte/rune offset range into the normalized source string and any
// optional origin metadata (page/bounding box from OCR, or timestamps from
// STT) that should be carried through so downstream phases can trace a
// segment back to its exact place in the original document, image, or
// audio.
//
// This mirrors the span-like fields already present in upstream packages:
// packages/stt's TranscriptSegment (StartMS/EndMS), packages/ocr's TextBlock
// (Page/BoundingBox). SourceSpan generalizes both into a single shape that
// segmentation can attach to any Segment regardless of its origin.
type SourceSpan struct {
	// Start is the inclusive rune offset into the source text at which this
	// segment begins.
	Start int

	// End is the exclusive rune offset into the source text at which this
	// segment ends. End must be >= Start.
	End int

	// Page is the 1-based page number this segment originated from, when
	// the source was an OCR extraction. Zero means unknown/unspecified/not
	// applicable.
	Page int

	// BoundingBox is the pixel-coordinate bounding box locating this
	// segment on its Page, when the source was an OCR extraction. The zero
	// value means unknown/not applicable.
	BoundingBox BoundingBox

	// StartMS is the offset in milliseconds from the start of the audio at
	// which this segment begins, when the source was an STT transcript.
	// Zero (with EndMS also zero) means unknown/not applicable.
	StartMS int64

	// EndMS is the offset in milliseconds from the start of the audio at
	// which this segment ends, when the source was an STT transcript.
	EndMS int64
}

// BoundingBox is an axis-aligned pixel-coordinate box locating a segment's
// origin region on an OCR page, with the origin (0,0) at the top-left
// corner. Mirrors packages/ocr's BoundingBox shape so OCR-origin metadata
// carries through without lossy conversion.
type BoundingBox struct {
	// X is the horizontal offset of the box's top-left corner, in pixels.
	X int
	// Y is the vertical offset of the box's top-left corner, in pixels.
	Y int
	// Width is the box width, in pixels.
	Width int
	// Height is the box height, in pixels.
	Height int
}

// Len returns the rune length of the span (End - Start). Returns 0 if End
// <= Start.
func (s SourceSpan) Len() int {
	if s.End <= s.Start {
		return 0
	}
	return s.End - s.Start
}

// Overlaps reports whether s and other describe overlapping rune ranges.
// Adjacent, non-overlapping spans (s.End == other.Start) do not overlap.
func (s SourceSpan) Overlaps(other SourceSpan) bool {
	return s.Start < other.End && other.Start < s.End
}

// ValidateSpanCoverage checks that spans, taken in the given order, cover
// the source text of the given rune length with no gaps and no overlaps:
// the first span must start at 0, each span's End must equal the next
// span's Start, and the last span's End must equal totalRunes.
//
// It returns ErrSpanOverlap if any two consecutive spans overlap or leave a
// gap, or if coverage does not start at 0 or end at totalRunes. An empty
// spans slice is valid only when totalRunes is 0.
func ValidateSpanCoverage(spans []SourceSpan, totalRunes int) error {
	if len(spans) == 0 {
		if totalRunes == 0 {
			return nil
		}
		return ErrSpanOverlap
	}

	if spans[0].Start != 0 {
		return ErrSpanOverlap
	}
	for i, s := range spans {
		if s.End < s.Start {
			return ErrSpanOverlap
		}
		if i > 0 && s.Start != spans[i-1].End {
			return ErrSpanOverlap
		}
	}
	if spans[len(spans)-1].End != totalRunes {
		return ErrSpanOverlap
	}
	return nil
}
