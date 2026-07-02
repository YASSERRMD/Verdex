package stt_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/stt"
)

func TestTranscript_FullText(t *testing.T) {
	tr := &stt.Transcript{
		Segments: []stt.TranscriptSegment{
			{StartMS: 0, EndMS: 1000, Text: "hello"},
			{StartMS: 1000, EndMS: 2000, Text: "world"},
		},
	}
	if got, want := tr.FullText(), "hello world"; got != want {
		t.Errorf("FullText() = %q, want %q", got, want)
	}
}

func TestTranscript_FullText_Empty(t *testing.T) {
	var tr *stt.Transcript
	if got := tr.FullText(); got != "" {
		t.Errorf("FullText() on nil transcript = %q, want empty string", got)
	}

	tr2 := &stt.Transcript{}
	if got := tr2.FullText(); got != "" {
		t.Errorf("FullText() on empty transcript = %q, want empty string", got)
	}
}

func TestTranscript_SortSegments(t *testing.T) {
	tr := &stt.Transcript{
		Segments: []stt.TranscriptSegment{
			{StartMS: 2000, Text: "second"},
			{StartMS: 0, Text: "first"},
			{StartMS: 1000, Text: "middle"},
		},
	}
	tr.SortSegments()

	want := []string{"first", "middle", "second"}
	for i, seg := range tr.Segments {
		if seg.Text != want[i] {
			t.Errorf("segment[%d].Text = %q, want %q", i, seg.Text, want[i])
		}
	}
}

func TestAssembleTranscript_OffsetsChunks(t *testing.T) {
	chunkSegments := [][]stt.TranscriptSegment{
		{{StartMS: 0, EndMS: 500, Text: "chunk0-a"}},
		{{StartMS: 0, EndMS: 300, Text: "chunk1-a"}, {StartMS: 300, EndMS: 700, Text: "chunk1-b"}},
	}
	offsets := []int64{0, 1000}

	tr := stt.AssembleTranscript("noop", "en", chunkSegments, offsets)

	if len(tr.Segments) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(tr.Segments))
	}
	if tr.Segments[0].StartMS != 0 || tr.Segments[0].EndMS != 500 {
		t.Errorf("segment[0] = %+v, want StartMS=0 EndMS=500", tr.Segments[0])
	}
	if tr.Segments[1].StartMS != 1000 || tr.Segments[1].EndMS != 1300 {
		t.Errorf("segment[1] = %+v, want StartMS=1000 EndMS=1300", tr.Segments[1])
	}
	if tr.Segments[2].StartMS != 1300 || tr.Segments[2].EndMS != 1700 {
		t.Errorf("segment[2] = %+v, want StartMS=1300 EndMS=1700", tr.Segments[2])
	}

	// Ordering invariant.
	for i := 1; i < len(tr.Segments); i++ {
		if tr.Segments[i].StartMS < tr.Segments[i-1].StartMS {
			t.Fatalf("segments not ordered at index %d", i)
		}
	}

	if tr.ProviderID != "noop" {
		t.Errorf("ProviderID = %q, want %q", tr.ProviderID, "noop")
	}
	if tr.Language != "en" {
		t.Errorf("Language = %q, want %q", tr.Language, "en")
	}
}
