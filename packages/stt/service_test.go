package stt_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/stt"
)

func newTestSTTService(t *testing.T) (*stt.STTService, *stt.NoOpSTTProvider) {
	t.Helper()
	registry := stt.NewRegistry()
	p := stt.DefaultNoOpSTTProvider()
	if err := registry.Register(p.ID(), p); err != nil {
		t.Fatalf("Register: %v", err)
	}
	return stt.NewSTTService(registry, nil, nil), p
}

// TestSTTService_Transcribe_SegmentsOrdered verifies that returned segments
// are ordered by StartMS ascending.
func TestSTTService_Transcribe_SegmentsOrdered(t *testing.T) {
	svc, p := newTestSTTService(t)
	_ = p

	input := &stt.AudioInput{
		Data:       []byte("some raw audio bytes representing speech"),
		DurationMS: 9000,
	}
	svc.MaxChunkMS = 3000

	transcript, err := svc.Transcribe(context.Background(), "noop", input)
	if err != nil {
		t.Fatalf("Transcribe() unexpected error: %v", err)
	}

	if len(transcript.Segments) == 0 {
		t.Fatal("expected at least one segment")
	}

	for i := 1; i < len(transcript.Segments); i++ {
		if transcript.Segments[i].StartMS < transcript.Segments[i-1].StartMS {
			t.Fatalf("segments not ordered: segment[%d].StartMS=%d < segment[%d].StartMS=%d",
				i, transcript.Segments[i].StartMS, i-1, transcript.Segments[i-1].StartMS)
		}
	}
}

// TestSTTService_Transcribe_TimestampsMonotonic verifies that each segment's
// EndMS >= StartMS and that successive segments do not go backwards in time.
func TestSTTService_Transcribe_TimestampsMonotonic(t *testing.T) {
	svc, _ := newTestSTTService(t)

	input := &stt.AudioInput{
		Data:       []byte("more raw audio bytes representing longer speech content"),
		DurationMS: 12000,
	}
	svc.MaxChunkMS = 4000

	transcript, err := svc.Transcribe(context.Background(), "noop", input)
	if err != nil {
		t.Fatalf("Transcribe() unexpected error: %v", err)
	}

	var lastEnd int64
	for i, seg := range transcript.Segments {
		if seg.EndMS < seg.StartMS {
			t.Errorf("segment[%d]: EndMS %d < StartMS %d", i, seg.EndMS, seg.StartMS)
		}
		if seg.StartMS < lastEnd {
			t.Errorf("segment[%d]: StartMS %d precedes previous EndMS %d", i, seg.StartMS, lastEnd)
		}
		lastEnd = seg.EndMS
	}
}

// TestSTTService_Transcribe_ConfidenceInRange verifies every segment's
// Confidence lies in [0, 1].
func TestSTTService_Transcribe_ConfidenceInRange(t *testing.T) {
	svc, _ := newTestSTTService(t)

	input := &stt.AudioInput{
		Data:       []byte("audio bytes"),
		DurationMS: 1500,
	}

	transcript, err := svc.Transcribe(context.Background(), "noop", input)
	if err != nil {
		t.Fatalf("Transcribe() unexpected error: %v", err)
	}

	for i, seg := range transcript.Segments {
		if seg.Confidence < 0 || seg.Confidence > 1 {
			t.Errorf("segment[%d].Confidence = %v, want value in [0, 1]", i, seg.Confidence)
		}
	}
}

// TestSTTService_Transcribe_UnknownProvider_ReturnsErrProviderNotFound
// verifies error wrapping for a missing provider ID.
func TestSTTService_Transcribe_UnknownProvider_ReturnsErrProviderNotFound(t *testing.T) {
	svc, _ := newTestSTTService(t)

	input := &stt.AudioInput{Data: []byte("data"), DurationMS: 1000}
	_, err := svc.Transcribe(context.Background(), "does-not-exist", input)
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
	if !errors.Is(err, stt.ErrProviderNotFound) {
		t.Errorf("error %v does not wrap ErrProviderNotFound", err)
	}
}

// TestSTTService_Transcribe_EmptyAudio_ReturnsError verifies that an empty
// AudioInput is rejected.
func TestSTTService_Transcribe_EmptyAudio_ReturnsError(t *testing.T) {
	svc, _ := newTestSTTService(t)

	input := &stt.AudioInput{Data: nil}
	_, err := svc.Transcribe(context.Background(), "noop", input)
	if err == nil {
		t.Fatal("expected error for empty audio, got nil")
	}
	if !errors.Is(err, stt.ErrEmptyAudio) {
		t.Errorf("error %v does not wrap ErrEmptyAudio", err)
	}
}

// TestSTTService_Transcribe_NilInput_ReturnsError verifies that a nil
// *AudioInput is rejected without panicking.
func TestSTTService_Transcribe_NilInput_ReturnsError(t *testing.T) {
	svc, _ := newTestSTTService(t)

	_, err := svc.Transcribe(context.Background(), "noop", nil)
	if err == nil {
		t.Fatal("expected error for nil input, got nil")
	}
	if !errors.Is(err, stt.ErrInvalidRequest) {
		t.Errorf("error %v does not wrap ErrInvalidRequest", err)
	}
}

// TestSTTService_Transcribe_AppliesDiarizer verifies that a configured
// Diarizer's labels appear on the returned segments.
func TestSTTService_Transcribe_AppliesDiarizer(t *testing.T) {
	registry := stt.NewRegistry()
	p := stt.DefaultNoOpSTTProvider()
	if err := registry.Register(p.ID(), p); err != nil {
		t.Fatalf("Register: %v", err)
	}

	svc := stt.NewSTTService(registry, stt.NoOpDiarizer{AssignDefault: true}, nil)

	input := &stt.AudioInput{Data: []byte("audio bytes"), DurationMS: 1000}
	transcript, err := svc.Transcribe(context.Background(), "noop", input)
	if err != nil {
		t.Fatalf("Transcribe() unexpected error: %v", err)
	}

	for i, seg := range transcript.Segments {
		if seg.Speaker != stt.DefaultSpeakerLabel {
			t.Errorf("segment[%d].Speaker = %q, want %q", i, seg.Speaker, stt.DefaultSpeakerLabel)
		}
	}
}
