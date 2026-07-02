package ocr

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// DiscardAuditEvent is emitted whenever an ImageInput's source bytes are
// discarded after extraction, mirroring the transcribe-and-discard
// guarantee packages/stt and packages/intake provide for other binary
// ingestion artifacts.
type DiscardAuditEvent struct {
	// EventType is always "ocr.discarded" for events produced by Discard.
	EventType string

	// SourceHash is the hex-encoded SHA-256 digest of the ImageInput's
	// source bytes, computed before they were zeroed.
	SourceHash string

	// SizeBytes is the number of bytes that were discarded.
	SizeBytes int

	// ProviderID identifies which OCRProvider produced the extraction result
	// prior to discard, if known.
	ProviderID string

	// Timestamp is the wall-clock time at which discard occurred.
	Timestamp time.Time
}

// ImageDiscardSink receives DiscardAuditEvents. Implementations may write to
// a database, a message bus, a log stream, or any combination thereof.
type ImageDiscardSink interface {
	// Emit delivers an event to the sink. A non-nil error indicates the
	// event could not be durably recorded; callers may retry or escalate as
	// appropriate.
	Emit(ctx context.Context, event DiscardAuditEvent) error
}

// NoOpDiscardSink discards all events silently. Use in unit tests that do
// not need to assert on audit output.
type NoOpDiscardSink struct{}

// Emit implements ImageDiscardSink.
func (NoOpDiscardSink) Emit(_ context.Context, _ DiscardAuditEvent) error { return nil }

// CapturingDiscardSink stores emitted events in memory. Useful in tests that
// need to assert on discard behaviour.
type CapturingDiscardSink struct {
	Events []DiscardAuditEvent
}

// Emit implements ImageDiscardSink.
func (c *CapturingDiscardSink) Emit(_ context.Context, event DiscardAuditEvent) error {
	c.Events = append(c.Events, event)
	return nil
}

// ComputeSourceHash returns the hex-encoded SHA-256 digest of data. Callers
// MUST call this (or otherwise capture the hash) before invoking Discard,
// since Discard zeroes the underlying bytes.
func ComputeSourceHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Discard zeroes input.Data in place, rendering the source image bytes
// unrecoverable from the ImageInput, and emits a DiscardAuditEvent to sink
// (or NoOpDiscardSink{} if sink is nil).
//
// sourceHash should be the value previously returned by ComputeSourceHash
// for input.Data (computed before any mutation); it is threaded through
// unchanged into the emitted event so provenance can be recorded even though
// the bytes themselves are gone.
//
// Discard is idempotent: calling it a second time on an already-discarded
// ImageInput is a no-op and returns nil, matching packages/stt's Discard
// semantics.
func Discard(ctx context.Context, input *ImageInput, sourceHash string, providerID string, sink ImageDiscardSink) error {
	if input == nil {
		return fmt.Errorf("ocr: Discard: %w: input must not be nil", ErrInvalidRequest)
	}
	if sink == nil {
		sink = NoOpDiscardSink{}
	}

	size := len(input.Data)
	for i := range input.Data {
		input.Data[i] = 0
	}
	// Truncate to zero length (retaining underlying capacity is irrelevant;
	// this signals IsDiscarded/emptiness to any caller that inspects len()).
	input.Data = input.Data[:0]

	event := DiscardAuditEvent{
		EventType:  "ocr.discarded",
		SourceHash: sourceHash,
		SizeBytes:  size,
		ProviderID: providerID,
		Timestamp:  time.Now().UTC(),
	}
	return sink.Emit(ctx, event)
}

// IsDiscarded reports whether input's source bytes have been discarded
// (i.e. its Data slice is empty).
func IsDiscarded(input ImageInput) bool {
	return len(input.Data) == 0
}
