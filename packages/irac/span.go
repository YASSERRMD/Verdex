package irac

// SourceSpan locates a claim made by an IRAC node within the original
// ingested source text, carrying both the rune offset range into the
// normalized source string and any optional origin metadata (page from
// OCR, or timestamps from STT) needed to trace the claim back to its
// exact place in the original document, image, or audio.
//
// This mirrors packages/segmentation's SourceSpan shape, but is defined
// locally here (rather than imported) to avoid a hard module dependency
// from packages/irac on packages/segmentation — this phase is a pure
// schema/domain-model phase with no storage or cross-module wiring yet.
type SourceSpan struct {
	// Start is the inclusive rune offset into the source text at which
	// the span begins.
	Start int `json:"start"`

	// End is the exclusive rune offset into the source text at which the
	// span ends. End must be >= Start.
	End int `json:"end"`

	// Page is the 1-based page number this span originated from, when the
	// source was an OCR extraction. Zero means unknown/unspecified/not
	// applicable.
	Page int `json:"page,omitempty"`

	// StartMS is the offset in milliseconds from the start of the audio
	// at which this span begins, when the source was an STT transcript.
	// Zero (with EndMS also zero) means unknown/not applicable.
	StartMS int64 `json:"start_ms,omitempty"`

	// EndMS is the offset in milliseconds from the start of the audio at
	// which this span ends, when the source was an STT transcript.
	EndMS int64 `json:"end_ms,omitempty"`
}

// Len returns the rune length of the span (End - Start). Returns 0 if End
// <= Start.
func (s SourceSpan) Len() int {
	if s.End <= s.Start {
		return 0
	}
	return s.End - s.Start
}

// IsValid reports whether the span's End is at least its Start and both
// offsets are non-negative.
func (s SourceSpan) IsValid() bool {
	return s.Start >= 0 && s.End >= s.Start
}

// Spans is a slice of SourceSpan, embedded by every concrete node type so
// every claim in the reasoning tree traces back to ingested text. A node
// may cite zero, one, or many spans (e.g. an ApplicationNode may draw on
// spans from multiple source locations).
type Spans []SourceSpan
