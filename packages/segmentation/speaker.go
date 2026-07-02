package segmentation

import "strings"

// TranscriptTurn is the STT-shaped input AttributeSpeakers consumes: one
// diarized turn of a transcript, mirroring packages/stt's
// TranscriptSegment shape (StartMS/EndMS/Text/Speaker/Confidence) without
// importing packages/stt directly, so this package has no dependency on
// the STT provider stack. Callers adapt stt.TranscriptSegment to
// TranscriptTurn at the call site.
type TranscriptTurn struct {
	// Speaker is the diarized speaker label for this turn. Empty means no
	// diarization hint was available for this turn.
	Speaker SpeakerLabel

	// Text is the transcribed text for this turn.
	Text string

	// StartMS is the offset in milliseconds from the start of the audio at
	// which this turn begins.
	StartMS int64

	// EndMS is the offset in milliseconds from the start of the audio at
	// which this turn ends.
	EndMS int64

	// Confidence is the upstream STT confidence score for this turn, in the
	// closed interval [0, 1].
	Confidence float64
}

// AttributeSpeakers converts STT-shaped diarized turns into SegmentStatement
// segments, one per non-empty turn, each carrying the turn's SpeakerLabel,
// text, confidence, and a SourceSpan populated with StartMS/EndMS (Start/End
// rune offsets are left zero here; the caller/service is responsible for
// filling them in once the turn's text is placed in a larger normalized
// source string — see service.go).
//
// Turns with empty (whitespace-only) Text are skipped. AttributeSpeakers
// never merges or splits turns: each input turn maps to exactly one output
// Segment (or zero, if skipped), preserving order.
func AttributeSpeakers(turns []TranscriptTurn) []Segment {
	segs := make([]Segment, 0, len(turns))
	for _, t := range turns {
		if strings.TrimSpace(t.Text) == "" {
			continue
		}
		segs = append(segs, Segment{
			Type:         SegmentStatement,
			Text:         strings.TrimSpace(t.Text),
			SpeakerLabel: t.Speaker,
			Confidence:   t.Confidence,
			Span: SourceSpan{
				StartMS: t.StartMS,
				EndMS:   t.EndMS,
			},
		})
	}
	return segs
}

// HasSpeakerHints reports whether at least one turn carries a non-empty
// Speaker label, indicating diarization was performed upstream and
// speaker-attributed segmentation should be preferred over plain
// paragraph/sentence splitting.
func HasSpeakerHints(turns []TranscriptTurn) bool {
	for _, t := range turns {
		if t.Speaker != "" {
			return true
		}
	}
	return false
}
