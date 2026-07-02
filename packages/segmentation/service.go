package segmentation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
)

// SegmentationService orchestrates the full segmentation pipeline:
//
//	split (sentence/clause) -> detect headings -> detect exhibits/citations
//	  -> attribute speakers (when transcript input is supplied)
//	  -> assign spans/order/confidence -> return []Segment
//
// This mirrors packages/multilingual's NormalizationService orchestration
// pattern: a single entry point wires together the package's otherwise
// independent, individually-testable building blocks.
type SegmentationService struct {
	// IDGenerator produces a unique ID for each segment. If nil,
	// NewRandomID is used.
	IDGenerator func() string
}

// NewSegmentationService constructs a SegmentationService with sensible
// defaults for every pluggable dependency left nil.
func NewSegmentationService() *SegmentationService {
	return &SegmentationService{IDGenerator: NewRandomID}
}

// SegmentRequest carries the input to SegmentationService.Segment. Exactly
// one of Text or Turns should be meaningfully populated: when Turns is
// non-empty, speaker-attributed segmentation is used; otherwise Text is
// split into sentence-level paragraph segments.
type SegmentRequest struct {
	// DocumentID optionally correlates the request with a source document.
	DocumentID string

	// Text is plain (or OCR/multilingual-normalized) source text to
	// segment. Ignored when Turns is non-empty.
	Text string

	// Language is the ISO 639-1 language code of Text/Turns, when known.
	// Propagated onto every produced Segment.
	Language string

	// Turns carries STT-shaped diarized transcript turns. When non-empty,
	// speaker-attributed segmentation (see speaker.go) is used instead of
	// plain sentence splitting.
	Turns []TranscriptTurn
}

// Segment runs the full segmentation pipeline over req and returns the
// resulting segments in document order, each with Span, Sequence,
// PrevID/NextID, and Confidence populated.
//
// Segment returns ErrEmptyInput if req has neither non-empty Text nor any
// Turn with non-empty Text.
func (s *SegmentationService) Segment(_ context.Context, req SegmentRequest) ([]Segment, error) {
	idGen := s.IDGenerator
	if idGen == nil {
		idGen = NewRandomID
	}

	var segs []Segment
	var sourceText string

	if len(req.Turns) > 0 {
		if !hasNonEmptyTurnText(req.Turns) {
			return nil, ErrEmptyInput
		}
		segs = AttributeSpeakers(req.Turns)
		// Build a synthetic concatenated source text so Span.Start/End can
		// still be populated with rune offsets, in addition to the
		// StartMS/EndMS already set by AttributeSpeakers.
		texts := make([]string, len(segs))
		for i, seg := range segs {
			texts[i] = seg.Text
		}
		sourceText = strings.Join(texts, "\n")
		segs = assignRuneSpansBySequentialJoin(segs, "\n")
	} else {
		if strings.TrimSpace(req.Text) == "" {
			return nil, ErrEmptyInput
		}
		sourceText = req.Text
		spans := SplitSentences(req.Text)
		segs = make([]Segment, 0, len(spans))
		for _, sp := range spans {
			if strings.TrimSpace(sp.Text) == "" {
				continue
			}
			segs = append(segs, Segment{
				Type: SegmentParagraph,
				Text: sp.Text,
				Span: SourceSpan{Start: sp.Start, End: sp.End},
			})
		}
	}

	if len(segs) == 0 {
		return nil, ErrEmptyInput
	}

	// Detect headings, then exhibits/citations (exhibits/citations must not
	// override headings — see TagExhibitsAndCitations).
	segs = TagHeadings(segs)
	segs = TagExhibitsAndCitations(segs)

	// Propagate language, assign IDs, default confidence, and order.
	for i := range segs {
		if segs[i].Language == "" {
			segs[i].Language = req.Language
		}
		if segs[i].ID == "" {
			segs[i].ID = idGen()
		}
	}
	segs = AssignDefaultConfidence(segs)
	segs = AssignOrder(segs)

	_ = sourceText // retained for callers that extend this pipeline; offsets already assigned above
	return segs, nil
}

// hasNonEmptyTurnText reports whether at least one turn has non-empty text.
func hasNonEmptyTurnText(turns []TranscriptTurn) bool {
	for _, t := range turns {
		if strings.TrimSpace(t.Text) != "" {
			return true
		}
	}
	return false
}

// assignRuneSpansBySequentialJoin sets Span.Start/Span.End on each segment
// as if segs' Text values were joined with sep, preserving each segment's
// existing StartMS/EndMS/Page/BoundingBox fields. Returns a new slice.
func assignRuneSpansBySequentialJoin(segs []Segment, sep string) []Segment {
	out := make([]Segment, len(segs))
	offset := 0
	sepLen := len([]rune(sep))
	for i, s := range segs {
		length := len([]rune(s.Text))
		s.Span.Start = offset
		s.Span.End = offset + length
		offset += length + sepLen
		out[i] = s
	}
	return out
}

// NewRandomID generates a random 16-byte hex-encoded identifier, used as
// the default Segment.ID generator.
func NewRandomID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand.Read failing is exceptionally rare (would indicate a
		// broken system entropy source); fall back to a fixed sentinel
		// rather than panicking, so segmentation can still proceed.
		return "id-unavailable"
	}
	return hex.EncodeToString(b)
}
