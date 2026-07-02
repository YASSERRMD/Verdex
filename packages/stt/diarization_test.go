package stt_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/stt"
)

func TestNoOpDiarizer_LeavesSegmentsUnchangedByDefault(t *testing.T) {
	d := stt.NoOpDiarizer{}
	segments := []stt.TranscriptSegment{
		{StartMS: 0, EndMS: 1000, Text: "a"},
		{StartMS: 1000, EndMS: 2000, Text: "b"},
	}

	out, err := d.Diarize(t.Context(), segments)
	if err != nil {
		t.Fatalf("Diarize() unexpected error: %v", err)
	}
	if len(out) != len(segments) {
		t.Fatalf("Diarize() returned %d segments, want %d", len(out), len(segments))
	}
	for i, seg := range out {
		if seg.Speaker != "" {
			t.Errorf("segment[%d].Speaker = %q, want empty", i, seg.Speaker)
		}
		if seg.Text != segments[i].Text {
			t.Errorf("segment[%d].Text = %q, want %q", i, seg.Text, segments[i].Text)
		}
	}
}

func TestNoOpDiarizer_AssignsDefaultLabel(t *testing.T) {
	d := stt.NoOpDiarizer{AssignDefault: true}
	segments := []stt.TranscriptSegment{
		{StartMS: 0, EndMS: 1000, Text: "a"},
		{StartMS: 1000, EndMS: 2000, Text: "b"},
	}

	out, err := d.Diarize(t.Context(), segments)
	if err != nil {
		t.Fatalf("Diarize() unexpected error: %v", err)
	}
	for i, seg := range out {
		if seg.Speaker != stt.DefaultSpeakerLabel {
			t.Errorf("segment[%d].Speaker = %q, want %q", i, seg.Speaker, stt.DefaultSpeakerLabel)
		}
	}
}

func TestNoOpDiarizer_PreservesOrderAndCount(t *testing.T) {
	d := stt.NoOpDiarizer{AssignDefault: true}
	segments := make([]stt.TranscriptSegment, 5)
	for i := range segments {
		segments[i] = stt.TranscriptSegment{StartMS: int64(i * 1000), EndMS: int64((i + 1) * 1000)}
	}

	out, err := d.Diarize(t.Context(), segments)
	if err != nil {
		t.Fatalf("Diarize() unexpected error: %v", err)
	}
	if len(out) != len(segments) {
		t.Fatalf("Diarize() changed segment count: got %d, want %d", len(out), len(segments))
	}
	for i, seg := range out {
		if seg.StartMS != segments[i].StartMS || seg.EndMS != segments[i].EndMS {
			t.Errorf("segment[%d] timing changed: got {%d,%d}, want {%d,%d}",
				i, seg.StartMS, seg.EndMS, segments[i].StartMS, segments[i].EndMS)
		}
	}
}
