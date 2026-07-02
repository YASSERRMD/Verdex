package ocr_test

import (
	"context"
	"testing"

	"github.com/YASSERRMD/verdex/packages/ocr"
)

// TestDiscard_ZeroesSourceBytes verifies that Discard clears the source
// bytes of an ImageInput in place.
func TestDiscard_ZeroesSourceBytes(t *testing.T) {
	input := &ocr.ImageInput{
		Data:     []byte("synthetic scanned page bytes"),
		MIMEType: "image/png",
	}
	original := append([]byte(nil), input.Data...)
	hash := ocr.ComputeSourceHash(input.Data)

	sink := &ocr.CapturingDiscardSink{}
	if err := ocr.Discard(context.Background(), input, hash, "noop", sink); err != nil {
		t.Fatalf("Discard() unexpected error: %v", err)
	}

	if len(input.Data) != 0 {
		t.Fatalf("Discard() left %d bytes in input.Data, want 0", len(input.Data))
	}
	if !ocr.IsDiscarded(*input) {
		t.Error("IsDiscarded() = false after Discard(), want true")
	}
	if len(original) == 0 {
		t.Fatal("test setup error: original should not be empty")
	}
}

// TestDiscard_EmitsAuditEvent verifies that Discard emits exactly one
// DiscardAuditEvent carrying the pre-computed hash and byte count.
func TestDiscard_EmitsAuditEvent(t *testing.T) {
	input := &ocr.ImageInput{Data: []byte("some image bytes here")}
	size := len(input.Data)
	hash := ocr.ComputeSourceHash(input.Data)

	sink := &ocr.CapturingDiscardSink{}
	if err := ocr.Discard(context.Background(), input, hash, "noop", sink); err != nil {
		t.Fatalf("Discard() unexpected error: %v", err)
	}

	if len(sink.Events) != 1 {
		t.Fatalf("expected 1 discard event, got %d", len(sink.Events))
	}
	ev := sink.Events[0]
	if ev.EventType != "ocr.discarded" {
		t.Errorf("EventType = %q, want %q", ev.EventType, "ocr.discarded")
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
// ImageInput pointer.
func TestDiscard_NilInput_ReturnsError(t *testing.T) {
	if err := ocr.Discard(context.Background(), nil, "somehash", "noop", nil); err == nil {
		t.Fatal("Discard(nil) expected error, got nil")
	}
}

// TestDiscard_Idempotent verifies that calling Discard a second time on an
// already-discarded ImageInput is safe and does not error.
func TestDiscard_Idempotent(t *testing.T) {
	input := &ocr.ImageInput{Data: []byte("payload")}
	hash := ocr.ComputeSourceHash(input.Data)

	if err := ocr.Discard(context.Background(), input, hash, "noop", nil); err != nil {
		t.Fatalf("first Discard() unexpected error: %v", err)
	}
	if err := ocr.Discard(context.Background(), input, hash, "noop", nil); err != nil {
		t.Fatalf("second Discard() should be a no-op, got error: %v", err)
	}
	if !ocr.IsDiscarded(*input) {
		t.Error("IsDiscarded() = false after double Discard(), want true")
	}
}

// TestComputeSourceHash_Deterministic verifies that ComputeSourceHash
// returns the same digest for the same input and a different digest for
// different input.
func TestComputeSourceHash_Deterministic(t *testing.T) {
	a := []byte("image payload A")
	b := []byte("image payload B")

	h1 := ocr.ComputeSourceHash(a)
	h2 := ocr.ComputeSourceHash(append([]byte(nil), a...))
	h3 := ocr.ComputeSourceHash(b)

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

// TestOCRService_Extract_DiscardsSourceImage verifies that running the full
// service pipeline discards the source ImageInput bytes.
func TestOCRService_Extract_DiscardsSourceImage(t *testing.T) {
	registry := ocr.NewRegistry()
	provider := ocr.DefaultNoOpOCRProvider()
	if err := registry.Register(provider.ID(), provider); err != nil {
		t.Fatalf("Register: %v", err)
	}

	sink := &ocr.CapturingDiscardSink{}
	svc := ocr.NewOCRService(registry, nil, nil, sink)

	input := ocr.ImageInput{
		Data:     []byte("raw scanned page bytes for extraction"),
		WidthPx:  800,
		HeightPx: 1000,
	}

	result, err := svc.Extract(context.Background(), provider.ID(), &input, 0)
	if err != nil {
		t.Fatalf("Extract() unexpected error: %v", err)
	}
	if result.SourceHash == "" {
		t.Error("ExtractionResult.SourceHash should be populated")
	}
	if !ocr.IsDiscarded(input) {
		t.Error("source ImageInput.Data should be discarded (zeroed) after Extract")
	}
	if len(sink.Events) != 1 {
		t.Fatalf("expected 1 discard event, got %d", len(sink.Events))
	}
}
