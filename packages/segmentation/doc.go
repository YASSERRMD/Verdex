// Package segmentation splits normalized case text — from STT transcripts,
// OCR extractions, or plain documents — into logical units (Segments) that
// later Verdex phases (evidence classification, party/timeline modeling,
// IRAC extraction) operate on.
//
// Core concepts:
//
//   - Segment / SegmentType: the unit of segmentation — a paragraph,
//     speaker statement, exhibit marker, heading, or citation (types.go).
//   - SplitSentences / SplitClauses: deterministic, rule-based, punctuation-
//     and abbreviation-aware sentence/clause splitting (splitter.go).
//   - IsHeadingLine / TagHeadings: structural heading/section detection via
//     numbering patterns and short all-caps/title-case lines (heading.go).
//   - TranscriptTurn / AttributeSpeakers: speaker-attributed segmentation
//     for STT-shaped diarized transcript input (speaker.go).
//   - IsExhibitMarker / IsCitationMarker / TagExhibitsAndCitations /
//     SplitOnExhibitBoundaries: exhibit and citation boundary detection
//     (exhibit.go).
//   - SourceSpan / ValidateSpanCoverage: source-span offsets (rune range,
//     plus optional OCR page/bounding-box or STT timestamp metadata) that
//     locate every Segment in its original source, with a no-gaps/no-
//     overlaps coverage invariant (span.go).
//   - AssignOrder / ValidateOrder: stable, zero-based Sequence assignment
//     and PrevID/NextID linkage in document order (order.go).
//   - DefaultConfidence / AssignDefaultConfidence / AggregateConfidence:
//     segment-level confidence, propagated/aggregated from upstream OCR/STT
//     confidence when available, defaulting to 1.0 for plain text
//     (confidence.go).
//   - SegmentationService: orchestrates the full pipeline — split -> detect
//     headings -> detect exhibits/citations -> attribute speakers (when
//     transcript input is supplied) -> assign IDs/spans/order/confidence ->
//     return []Segment (service.go).
//
// Design principles:
//
//   - No ML models. Sentence/clause splitting, heading detection, and
//     exhibit/citation detection are all deterministic, rule-based
//     functions of punctuation, numbering patterns, and regular
//     expressions — mirroring packages/multilingual's design principle.
//   - Every Segment traces back to its exact source position. SourceSpan
//     generalizes packages/stt's TranscriptSegment (StartMS/EndMS) and
//     packages/ocr's TextBlock (Page/BoundingBox) into a single shape any
//     Segment can carry, regardless of origin.
//   - Full coverage, no gaps or overlaps. Every splitting function in this
//     package produces spans that, taken in order, cover the complete rune
//     range of their input — verified by ValidateSpanCoverage and exercised
//     by every splitter/service test.
//
// See doc/segmentation-model.md for a detailed design write-up.
package segmentation
