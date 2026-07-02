package segmentation_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/segmentation"
)

func TestAttributeSpeakers(t *testing.T) {
	turns := []segmentation.TranscriptTurn{
		{Speaker: "speaker-0", Text: "Please state your name for the record.", StartMS: 0, EndMS: 2500, Confidence: 0.95},
		{Speaker: "speaker-1", Text: "My name is Jane Doe.", StartMS: 2600, EndMS: 4000, Confidence: 0.88},
		{Speaker: "speaker-0", Text: "  ", StartMS: 4000, EndMS: 4200, Confidence: 0.9}, // whitespace-only, must be skipped
		{Speaker: "", Text: "Objection, your honor.", StartMS: 4300, EndMS: 5000, Confidence: 0.7},
	}

	segs := segmentation.AttributeSpeakers(turns)

	if len(segs) != 3 {
		t.Fatalf("AttributeSpeakers() returned %d segments, want 3 (whitespace-only turn skipped)", len(segs))
	}

	// Round-trip: speaker labels, text, confidence, and timing must survive
	// attribution unchanged for every non-empty turn.
	wantSpeakers := []segmentation.SpeakerLabel{"speaker-0", "speaker-1", ""}
	wantTexts := []string{"Please state your name for the record.", "My name is Jane Doe.", "Objection, your honor."}
	wantConfidence := []float64{0.95, 0.88, 0.7}
	wantStartMS := []int64{0, 2600, 4300}
	wantEndMS := []int64{2500, 4000, 5000}

	for i, seg := range segs {
		if seg.Type != segmentation.SegmentStatement {
			t.Errorf("segment[%d].Type = %v, want SegmentStatement", i, seg.Type)
		}
		if seg.SpeakerLabel != wantSpeakers[i] {
			t.Errorf("segment[%d].SpeakerLabel = %q, want %q", i, seg.SpeakerLabel, wantSpeakers[i])
		}
		if seg.Text != wantTexts[i] {
			t.Errorf("segment[%d].Text = %q, want %q", i, seg.Text, wantTexts[i])
		}
		if seg.Confidence != wantConfidence[i] {
			t.Errorf("segment[%d].Confidence = %v, want %v", i, seg.Confidence, wantConfidence[i])
		}
		if seg.Span.StartMS != wantStartMS[i] {
			t.Errorf("segment[%d].Span.StartMS = %v, want %v", i, seg.Span.StartMS, wantStartMS[i])
		}
		if seg.Span.EndMS != wantEndMS[i] {
			t.Errorf("segment[%d].Span.EndMS = %v, want %v", i, seg.Span.EndMS, wantEndMS[i])
		}
	}
}

func TestAttributeSpeakers_Empty(t *testing.T) {
	segs := segmentation.AttributeSpeakers(nil)
	if len(segs) != 0 {
		t.Errorf("AttributeSpeakers(nil) = %d segments, want 0", len(segs))
	}
}

func TestHasSpeakerHints(t *testing.T) {
	tests := []struct {
		name  string
		turns []segmentation.TranscriptTurn
		want  bool
	}{
		{"has-hints", []segmentation.TranscriptTurn{{Speaker: "speaker-0", Text: "hello"}}, true},
		{"no-hints", []segmentation.TranscriptTurn{{Text: "hello"}, {Text: "world"}}, false},
		{"empty", nil, false},
		{"mixed", []segmentation.TranscriptTurn{{Text: "hello"}, {Speaker: "speaker-1", Text: "world"}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := segmentation.HasSpeakerHints(tt.turns); got != tt.want {
				t.Errorf("HasSpeakerHints() = %v, want %v", got, tt.want)
			}
		})
	}
}
