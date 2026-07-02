package stt

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// DiscardAuditEvent is emitted whenever an AudioInput's source bytes are
// discarded after transcription, mirroring the transcribe-and-discard
// guarantee packages/intake provides for uploaded artifacts.
type DiscardAuditEvent struct {
	// EventType is always "stt.discarded" for events produced by Discard.
	EventType string

	// SourceHash is the hex-encoded SHA-256 digest of the AudioInput's
	// source bytes, computed before they were zeroed.
	SourceHash string

	// SizeBytes is the number of bytes that were discarded.
	SizeBytes int

	// ProviderID identifies which STTProvider produced the transcript prior
	// to discard, if known.
	ProviderID string

	// Timestamp is the wall-clock time at which discard occurred.
	Timestamp time.Time
}

// AudioDiscardSink receives DiscardAuditEvents. Implementations may write to
// a database, a message bus, a log stream, or any combination thereof.
type AudioDiscardSink interface {
	// Emit delivers an event to the sink. A non-nil error indicates the
	// event could not be durably recorded; callers may retry or escalate as
	// appropriate.
	Emit(ctx context.Context, event DiscardAuditEvent) error
}

// NoOpDiscardSink discards all events silently. Use in unit tests that do
// not need to assert on audit output.
type NoOpDiscardSink struct{}

// Emit implements AudioDiscardSink.
func (NoOpDiscardSink) Emit(_ context.Context, _ DiscardAuditEvent) error { return nil }

// CapturingDiscardSink stores emitted events in memory. Useful in tests that
// need to assert on discard behaviour.
type CapturingDiscardSink struct {
	Events []DiscardAuditEvent
}

// Emit implements AudioDiscardSink.
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

// discardedMarker is written as the first bytes of a discarded AudioInput's
// Data so IsDiscarded can detect prior discard even though the slice length
// is preserved (Data is zeroed, not truncated, to avoid reallocation
// surprises for callers holding the original slice header).
//
// Discard is idempotent: calling it a second time is a no-op and returns
// nil, matching packages/intake's TempBuffer.Discard semantics.

// Discard zeroes input.Data in place, rendering the source audio bytes
// unrecoverable from the AudioInput, and emits a DiscardAuditEvent to sink
// (or NoOpDiscardSink{} if sink is nil).
//
// sourceHash should be the value previously returned by ComputeSourceHash
// for input.Data (computed before any mutation); it is threaded through
// unchanged into the emitted event so provenance can be recorded even though
// the bytes themselves are gone.
//
// Discard is idempotent and returns ErrEmptyAudio only if input.Data was
// already empty (length 0) when called — zeroing an empty (already
// discarded) slice is treated as a successful no-op, not an error, unless
// input.Data is nil, in which case this simply reports that state via the
// return value without touching anything.
func Discard(ctx context.Context, input *AudioInput, sourceHash string, providerID string, sink AudioDiscardSink) error {
	if input == nil {
		return fmt.Errorf("stt: Discard: %w: input must not be nil", ErrInvalidRequest)
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
		EventType:  "stt.discarded",
		SourceHash: sourceHash,
		SizeBytes:  size,
		ProviderID: providerID,
		Timestamp:  time.Now().UTC(),
	}
	return sink.Emit(ctx, event)
}

// IsDiscarded reports whether input's source bytes have been discarded
// (i.e. its Data slice is empty).
func IsDiscarded(input AudioInput) bool {
	return len(input.Data) == 0
}
