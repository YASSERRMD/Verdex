package fact

import (
	"strings"
	"time"

	"github.com/YASSERRMD/verdex/packages/evidence"
	"github.com/YASSERRMD/verdex/packages/irac"
)

// defaultGeneratedBy is the irac.Provenance.GeneratedBy label attached to
// every irac.FactNode this package builds, identifying this package's
// construction pipeline as the generating process, mirroring
// packages/issue/persist.go's defaultGeneratedBy convention (e.g.
// "irac-issue-extractor-v1" there, "verdex-fact-builder-v1" here).
const defaultGeneratedBy = "verdex-fact-builder-v1"

// SourceSpan mirrors packages/segmentation's SourceSpan shape (the subset
// packages/irac.SourceSpan also carries: rune offsets plus optional OCR
// page / STT millisecond fields). Defined locally, rather than imported,
// so BuildFactNode's caller can hand this package a span without this
// package taking a hard dependency on packages/segmentation — the same
// "opaque, locally-defined span" convention packages/irac itself
// documents in span.go.
type SourceSpan struct {
	// Start is the inclusive rune offset into the source text at which
	// the span begins.
	Start int

	// End is the exclusive rune offset into the source text at which the
	// span ends. End must be >= Start.
	End int

	// Page is the 1-based page number this span originated from, when
	// the source was an OCR extraction. Zero means unknown/not
	// applicable.
	Page int

	// StartMS is the offset in milliseconds from the start of the audio
	// at which this span begins, when the source was an STT transcript.
	StartMS int64

	// EndMS is the offset in milliseconds from the start of the audio at
	// which this span ends, when the source was an STT transcript.
	EndMS int64
}

// toIRACSpan converts s to its irac.SourceSpan equivalent.
func (s SourceSpan) toIRACSpan() irac.SourceSpan {
	return irac.SourceSpan{
		Start:   s.Start,
		End:     s.End,
		Page:    s.Page,
		StartMS: s.StartMS,
		EndMS:   s.EndMS,
	}
}

// BuildFactNode converts a single classified segment into a candidate
// irac.FactNode: the fact's Text is the segment's text, its Spans trace
// back to the segment's source location, and its Confidence starts from
// the classification's own Confidence (later stages, e.g. reliability.go,
// derive a distinct reliability signal rather than mutating this raw
// value).
//
// evidence.Classification carries only a SegmentID, not the segment's
// text or source span (see packages/evidence/classifier.go), so callers
// must supply both explicitly — mirroring packages/issue's
// "segmentText map[string]string" convention (see
// packages/issue/claim_map.go) for bridging a Classification back to its
// originating segment content.
//
// id and caseID stamp the resulting node's ID/CaseID; createdAt stamps
// CreatedAt and Provenance.GeneratedAt. Returns ErrClassificationInvalid
// if classification.SegmentID is empty or segmentText is empty/
// whitespace-only.
func BuildFactNode(classification evidence.Classification, segmentText string, span SourceSpan, id, caseID string, createdAt time.Time) (irac.FactNode, error) {
	if strings.TrimSpace(classification.SegmentID) == "" {
		return irac.FactNode{}, ErrClassificationInvalid
	}
	if strings.TrimSpace(segmentText) == "" {
		return irac.FactNode{}, ErrClassificationInvalid
	}

	provenance := irac.Provenance{
		GeneratedBy: defaultGeneratedBy,
		GeneratedAt: createdAt,
	}

	return irac.NewFactNode(
		id,
		caseID,
		strings.TrimSpace(segmentText),
		createdAt,
		classification.Confidence,
		provenance,
		span.toIRACSpan(),
	), nil
}
