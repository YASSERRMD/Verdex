package segmentation

// SegmentType classifies the kind of logical unit a Segment represents.
type SegmentType string

const (
	// SegmentParagraph is a block of ordinary body text (the default unit
	// produced by sentence/clause splitting when no more specific structure
	// is detected).
	SegmentParagraph SegmentType = "paragraph"

	// SegmentStatement is a speaker-attributed utterance, typically produced
	// from STT-shaped transcript input (see speaker.go). Statement segments
	// carry a non-empty SpeakerLabel when diarization hints were available.
	SegmentStatement SegmentType = "statement"

	// SegmentExhibit marks a segment that introduces or refers to an
	// exhibit (e.g. "Exhibit A", "Ex. 3") — see exhibit.go.
	SegmentExhibit SegmentType = "exhibit"

	// SegmentHeading marks a segment identified as a heading or section
	// title via structural heuristics — see heading.go.
	SegmentHeading SegmentType = "heading"

	// SegmentCitation marks a segment identified as a statute or case-law
	// citation — see exhibit.go.
	SegmentCitation SegmentType = "citation"
)

// SpeakerLabel identifies the speaker attributed to a SegmentStatement
// segment. The zero value ("") means "no speaker attribution available",
// mirroring packages/stt's SpeakerLabel convention.
type SpeakerLabel string

// Segment is a single logical unit of a segmented document: a paragraph,
// a speaker statement, an exhibit marker, a heading, or a citation.
//
// Segments are the unit that downstream phases (evidence classification,
// party/timeline modeling, IRAC extraction) operate on. Every Segment
// produced by SegmentationService carries enough metadata (Span, Sequence,
// PrevID/NextID, Confidence) to be traced back to its exact position in the
// original source text and to be reassembled in order.
type Segment struct {
	// ID uniquely identifies this segment within its document.
	ID string

	// Type classifies the segment (see SegmentType).
	Type SegmentType

	// Text is the segment's normalized text content.
	Text string

	// Language is the ISO 639-1 language code of Text, when known. Empty
	// when the source language was not determined.
	Language string

	// SpeakerLabel identifies the attributed speaker for SegmentStatement
	// segments produced from transcript input. Empty for non-statement
	// segments or when no diarization hint was available.
	SpeakerLabel SpeakerLabel

	// Span carries the source-span offsets (and optional page/bbox/time
	// origin metadata) locating this segment in the original source — see
	// span.go.
	Span SourceSpan

	// Sequence is this segment's stable, zero-based position in document
	// order — see order.go.
	Sequence int

	// PrevID is the ID of the segment immediately preceding this one in
	// document order. Empty for the first segment.
	PrevID string

	// NextID is the ID of the segment immediately following this one in
	// document order. Empty for the last segment.
	NextID string

	// Confidence is this segment's confidence score, in the closed interval
	// [0, 1]. Propagated/aggregated from upstream OCR/STT confidence when
	// available; defaults to 1.0 for plain text with no upstream signal.
	Confidence float64
}
