package stt_test

import (
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/stt"
)

func TestNormalize_FillsDefaults(t *testing.T) {
	in := stt.AudioInput{Data: []byte("abc")}

	out, err := stt.Normalize(in)
	if err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}
	if out.SampleRateHz != stt.DefaultSampleRateHz {
		t.Errorf("SampleRateHz = %d, want %d", out.SampleRateHz, stt.DefaultSampleRateHz)
	}
	if out.Channels != stt.DefaultChannels {
		t.Errorf("Channels = %d, want %d", out.Channels, stt.DefaultChannels)
	}
}

func TestNormalize_PreservesExplicitValues(t *testing.T) {
	in := stt.AudioInput{Data: []byte("abc"), SampleRateHz: 44100, Channels: 2}

	out, err := stt.Normalize(in)
	if err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}
	if out.SampleRateHz != 44100 {
		t.Errorf("SampleRateHz = %d, want 44100", out.SampleRateHz)
	}
	if out.Channels != 2 {
		t.Errorf("Channels = %d, want 2", out.Channels)
	}
}

func TestNormalize_EmptyAudio_ReturnsError(t *testing.T) {
	_, err := stt.Normalize(stt.AudioInput{})
	if !errors.Is(err, stt.ErrEmptyAudio) {
		t.Errorf("Normalize() error = %v, want ErrEmptyAudio", err)
	}
}

func TestSegment_SingleChunkWhenUnderLimit(t *testing.T) {
	in := stt.AudioInput{Data: make([]byte, 100), DurationMS: 1000}

	chunks, err := stt.Segment(in, 5000)
	if err != nil {
		t.Fatalf("Segment() unexpected error: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("Segment() returned %d chunks, want 1", len(chunks))
	}
	if len(chunks[0].Data) != 100 {
		t.Errorf("chunk[0] data length = %d, want 100", len(chunks[0].Data))
	}
}

func TestSegment_SplitsIntoBoundedChunks(t *testing.T) {
	in := stt.AudioInput{Data: make([]byte, 1000), DurationMS: 10000}

	chunks, err := stt.Segment(in, 3000)
	if err != nil {
		t.Fatalf("Segment() unexpected error: %v", err)
	}
	// 10000ms / 3000ms => 4 chunks (3000, 3000, 3000, 1000)
	if len(chunks) != 4 {
		t.Fatalf("Segment() returned %d chunks, want 4", len(chunks))
	}

	var totalBytes int
	var lastEnd int64
	for i, c := range chunks {
		if c.DurationMS <= 0 {
			t.Errorf("chunk[%d].DurationMS = %d, want > 0", i, c.DurationMS)
		}
		if c.DurationMS > 3000 {
			t.Errorf("chunk[%d].DurationMS = %d exceeds maxChunkMS 3000", i, c.DurationMS)
		}
		if c.OffsetMS < lastEnd {
			t.Errorf("chunk[%d].OffsetMS = %d, want >= %d", i, c.OffsetMS, lastEnd)
		}
		lastEnd = c.OffsetMS + c.DurationMS
		totalBytes += len(c.Data)
	}
	if totalBytes != 1000 {
		t.Errorf("total chunk bytes = %d, want 1000 (no bytes dropped)", totalBytes)
	}
}

func TestSegment_Deterministic(t *testing.T) {
	in := stt.AudioInput{Data: make([]byte, 777), DurationMS: 8888}

	chunks1, err := stt.Segment(in, 2500)
	if err != nil {
		t.Fatalf("Segment() unexpected error: %v", err)
	}
	chunks2, err := stt.Segment(in, 2500)
	if err != nil {
		t.Fatalf("Segment() unexpected error: %v", err)
	}

	if len(chunks1) != len(chunks2) {
		t.Fatalf("non-deterministic chunk count: %d vs %d", len(chunks1), len(chunks2))
	}
	for i := range chunks1 {
		if chunks1[i].OffsetMS != chunks2[i].OffsetMS || chunks1[i].DurationMS != chunks2[i].DurationMS {
			t.Errorf("chunk[%d] differs between runs: %+v vs %+v", i, chunks1[i], chunks2[i])
		}
		if len(chunks1[i].Data) != len(chunks2[i].Data) {
			t.Errorf("chunk[%d] data length differs between runs: %d vs %d", i, len(chunks1[i].Data), len(chunks2[i].Data))
		}
	}
}

func TestSegment_UnknownDuration_ReturnsSingleChunk(t *testing.T) {
	in := stt.AudioInput{Data: make([]byte, 500)}

	chunks, err := stt.Segment(in, 1000)
	if err != nil {
		t.Fatalf("Segment() unexpected error: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("Segment() with unknown duration returned %d chunks, want 1", len(chunks))
	}
	if len(chunks[0].Data) != 500 {
		t.Errorf("chunk[0] data length = %d, want 500", len(chunks[0].Data))
	}
}

func TestSegment_EmptyAudio_ReturnsError(t *testing.T) {
	_, err := stt.Segment(stt.AudioInput{DurationMS: 1000}, 500)
	if !errors.Is(err, stt.ErrEmptyAudio) {
		t.Errorf("Segment() error = %v, want ErrEmptyAudio", err)
	}
}

func TestSegment_InvalidMaxChunk_ReturnsError(t *testing.T) {
	in := stt.AudioInput{Data: []byte("data"), DurationMS: 1000}

	_, err := stt.Segment(in, 0)
	if !errors.Is(err, stt.ErrInvalidRequest) {
		t.Errorf("Segment() error = %v, want ErrInvalidRequest", err)
	}

	_, err = stt.Segment(in, -1)
	if !errors.Is(err, stt.ErrInvalidRequest) {
		t.Errorf("Segment() error = %v, want ErrInvalidRequest", err)
	}
}
