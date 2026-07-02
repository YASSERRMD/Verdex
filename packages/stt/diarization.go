package stt

import "context"

// SpeakerLabel identifies a distinct speaker within a diarized transcript.
// The zero value ("") means "speaker not determined".
type SpeakerLabel string

// Diarizer assigns SpeakerLabel values to transcript segments.
//
// Diarization may be performed natively by an STTProvider (see
// Capability.SupportsDiarization) or as a separate post-processing step
// applied by STTService after Transcribe returns. Implementations MUST be
// deterministic for a given input so tests can assert exact output.
type Diarizer interface {
	// Diarize assigns a Speaker label to each segment in segments and
	// returns the updated slice. It must not change the number, order, or
	// timing of segments — only the Speaker field.
	Diarize(ctx context.Context, segments []TranscriptSegment) ([]TranscriptSegment, error)
}

// NoOpDiarizer is a Diarizer that leaves every segment's Speaker field
// unchanged (or sets a single default label when AssignDefault is true).
//
// Use this in air-gapped or single-speaker contexts, and as the default
// diarizer for STTService when no real diarization backend is configured.
type NoOpDiarizer struct {
	// AssignDefault, when true, sets every segment's Speaker to
	// DefaultSpeakerLabel instead of leaving it untouched.
	AssignDefault bool
}

// DefaultSpeakerLabel is the label NoOpDiarizer assigns when AssignDefault is
// true.
const DefaultSpeakerLabel SpeakerLabel = "speaker-0"

// Diarize implements Diarizer. It is a deterministic no-op: segment order,
// count, and timing are preserved exactly.
func (d NoOpDiarizer) Diarize(_ context.Context, segments []TranscriptSegment) ([]TranscriptSegment, error) {
	if !d.AssignDefault {
		return segments, nil
	}

	out := make([]TranscriptSegment, len(segments))
	for i, s := range segments {
		s.Speaker = DefaultSpeakerLabel
		out[i] = s
	}
	return out, nil
}
