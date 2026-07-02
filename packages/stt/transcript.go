package stt

import "sort"

// TranscriptSegment represents one contiguous span of transcribed speech.
type TranscriptSegment struct {
	// StartMS is the offset in milliseconds from the start of the audio at
	// which this segment begins.
	StartMS int64

	// EndMS is the offset in milliseconds from the start of the audio at
	// which this segment ends. EndMS must be >= StartMS.
	EndMS int64

	// Text is the transcribed text for this segment.
	Text string

	// Speaker is the diarized speaker label for this segment, if speaker
	// diarization was performed. Empty when no diarization was applied.
	Speaker SpeakerLabel

	// Confidence is the provider's confidence score for this segment, in the
	// closed interval [0, 1]. A value of 0 with no other signal typically
	// means "unknown confidence" for adapters that don't report scores.
	Confidence float64
}

// Transcript is the timestamped, provider-agnostic result of transcribing an
// AudioInput.
type Transcript struct {
	// ProviderID identifies which STTProvider produced this transcript.
	ProviderID string

	// Language is the ISO 639-1 language code the transcript was produced
	// in, when known.
	Language string

	// Segments holds the ordered transcript segments. Segments MUST be
	// ordered by StartMS ascending and MUST NOT overlap in a way that
	// violates monotonicity (each segment's StartMS >= previous EndMS is the
	// expected invariant, though adapters may relax this for overlapping
	// speech).
	Segments []TranscriptSegment

	// SourceHash is the SHA-256 hex digest of the original AudioInput.Data,
	// computed before the source bytes were discarded.
	SourceHash string
}

// FullText concatenates the text of every segment, in order, separated by a
// single space.
func (t *Transcript) FullText() string {
	if t == nil || len(t.Segments) == 0 {
		return ""
	}
	out := t.Segments[0].Text
	for _, seg := range t.Segments[1:] {
		out += " " + seg.Text
	}
	return out
}

// SortSegments orders Segments by StartMS ascending (stable, so segments with
// equal StartMS retain their relative order). Assembly logic should call this
// after concatenating segments produced from multiple chunks.
func (t *Transcript) SortSegments() {
	if t == nil {
		return
	}
	sort.SliceStable(t.Segments, func(i, j int) bool {
		return t.Segments[i].StartMS < t.Segments[j].StartMS
	})
}

// AssembleTranscript builds a single Transcript from the segments produced
// for each audio chunk, offsetting each chunk's segment timestamps by the
// chunk's start offset so the final timeline is relative to the original,
// unsegmented audio. Chunks are expected to be in the same order as returned
// by Segment (see normalize.go).
func AssembleTranscript(providerID, language string, chunkSegments [][]TranscriptSegment, chunkOffsetsMS []int64) *Transcript {
	t := &Transcript{
		ProviderID: providerID,
		Language:   language,
	}

	for i, segs := range chunkSegments {
		var offset int64
		if i < len(chunkOffsetsMS) {
			offset = chunkOffsetsMS[i]
		}
		for _, s := range segs {
			t.Segments = append(t.Segments, TranscriptSegment{
				StartMS:    s.StartMS + offset,
				EndMS:      s.EndMS + offset,
				Text:       s.Text,
				Speaker:    s.Speaker,
				Confidence: s.Confidence,
			})
		}
	}

	t.SortSegments()
	return t
}
