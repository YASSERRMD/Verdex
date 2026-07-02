package stt_test

import (
	"context"
	"testing"

	"github.com/YASSERRMD/verdex/packages/stt"
)

// TestDiscard_ZeroesSourceBytes verifies that Discard clears the source
// bytes of an AudioInput in place.
func TestDiscard_ZeroesSourceBytes(t *testing.T) {
	input := &stt.AudioInput{
		Data:     []byte("synthetic audio payload bytes"),
		MIMEType: "audio/wav",
	}
	original := append([]byte(nil), input.Data...)
	hash := stt.ComputeSourceHash(input.Data)

	sink := &stt.CapturingDiscardSink{}
	if err := stt.Discard(context.Background(), input, hash, "noop", sink); err != nil {
		t.Fatalf("Discard() unexpected error: %v", err)
	}

	if len(input.Data) != 0 {
		t.Fatalf("Discard() left %d bytes in input.Data, want 0", len(input.Data))
	}
	if !stt.IsDiscarded(*input) {
		t.Error("IsDiscarded() = false after Discard(), want true")
	}
	if len(original) == 0 {
		t.Fatal("test setup error: original should not be empty")
	}
}

// TestDiscard_EmitsAuditEvent verifies that Discard emits exactly one
// DiscardAuditEvent carrying the pre-computed hash and byte count.
func TestDiscard_EmitsAuditEvent(t *testing.T) {
	input := &stt.AudioInput{Data: []byte("some audio bytes here")}
	size := len(input.Data)
	hash := stt.ComputeSourceHash(input.Data)

	sink := &stt.CapturingDiscardSink{}
	if err := stt.Discard(context.Background(), input, hash, "noop", sink); err != nil {
		t.Fatalf("Discard() unexpected error: %v", err)
	}

	if len(sink.Events) != 1 {
		t.Fatalf("expected 1 discard event, got %d", len(sink.Events))
	}
	ev := sink.Events[0]
	if ev.EventType != "stt.discarded" {
		t.Errorf("EventType = %q, want %q", ev.EventType, "stt.discarded")
	}
	if ev.SourceHash != hash {
		t.Errorf("SourceHash = %q, want %q", ev.SourceHash, hash)
	}
	if ev.SizeBytes != size {
		t.Errorf("SizeBytes = %d, want %d", ev.SizeBytes, size)
	}
	if ev.ProviderID != "noop" {
		t.Errorf("ProviderID = %q, want %q", ev.ProviderID, "noop")
	}
	if ev.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
}

// TestDiscard_NilInput_ReturnsError verifies that Discard rejects a nil
// AudioInput pointer.
func TestDiscard_NilInput_ReturnsError(t *testing.T) {
	if err := stt.Discard(context.Background(), nil, "somehash", "noop", nil); err == nil {
		t.Fatal("Discard(nil) expected error, got nil")
	}
}

// TestDiscard_Idempotent verifies that calling Discard a second time on an
// already-discarded AudioInput is safe and does not error.
func TestDiscard_Idempotent(t *testing.T) {
	input := &stt.AudioInput{Data: []byte("payload")}
	hash := stt.ComputeSourceHash(input.Data)

	if err := stt.Discard(context.Background(), input, hash, "noop", nil); err != nil {
		t.Fatalf("first Discard() unexpected error: %v", err)
	}
	if err := stt.Discard(context.Background(), input, hash, "noop", nil); err != nil {
		t.Fatalf("second Discard() should be a no-op, got error: %v", err)
	}
	if !stt.IsDiscarded(*input) {
		t.Error("IsDiscarded() = false after double Discard(), want true")
	}
}

// TestComputeSourceHash_Deterministic verifies that ComputeSourceHash returns
// the same digest for the same input and a different digest for different
// input.
func TestComputeSourceHash_Deterministic(t *testing.T) {
	a := []byte("audio payload A")
	b := []byte("audio payload B")

	h1 := stt.ComputeSourceHash(a)
	h2 := stt.ComputeSourceHash(append([]byte(nil), a...))
	h3 := stt.ComputeSourceHash(b)

	if h1 != h2 {
		t.Errorf("ComputeSourceHash not deterministic: %q != %q", h1, h2)
	}
	if h1 == h3 {
		t.Errorf("ComputeSourceHash collision for different inputs: %q == %q", h1, h3)
	}
	if len(h1) != 64 {
		t.Errorf("ComputeSourceHash length = %d, want 64 (hex-encoded SHA-256)", len(h1))
	}
}

// TestSTTService_Transcribe_DiscardsSourceAudio verifies that running the
// full service pipeline discards the source AudioInput bytes.
func TestSTTService_Transcribe_DiscardsSourceAudio(t *testing.T) {
	registry := stt.NewRegistry()
	provider := stt.DefaultNoOpSTTProvider()
	if err := registry.Register(provider.ID(), provider); err != nil {
		t.Fatalf("Register: %v", err)
	}

	sink := &stt.CapturingDiscardSink{}
	svc := stt.NewSTTService(registry, nil, sink)

	input := stt.AudioInput{
		Data:       []byte("raw audio bytes for transcription"),
		DurationMS: 2000,
	}

	transcript, err := svc.Transcribe(context.Background(), provider.ID(), &input)
	if err != nil {
		t.Fatalf("Transcribe() unexpected error: %v", err)
	}
	if transcript.SourceHash == "" {
		t.Error("Transcript.SourceHash should be populated")
	}
	if !stt.IsDiscarded(input) {
		t.Error("source AudioInput.Data should be discarded (zeroed) after Transcribe")
	}
	if len(sink.Events) != 1 {
		t.Fatalf("expected 1 discard event, got %d", len(sink.Events))
	}
}
