package intake

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// IntakeAuditEvent is emitted for every significant transition in the intake
// pipeline.  All fields are populated even for failure events so that
// downstream forensic tools can reconstruct the full operation timeline.
type IntakeAuditEvent struct {
	// EventType is a short label such as "intake.started", "intake.ready",
	// "intake.discarded", or "intake.failed".
	EventType string

	// IntakeID uniquely identifies the intake operation.
	IntakeID uuid.UUID

	// TenantID is the tenant that initiated the upload.
	TenantID uuid.UUID

	// CaseID optionally links the upload to a specific case.
	CaseID *uuid.UUID

	// UploaderID is the authenticated user that initiated the upload.
	UploaderID uuid.UUID

	// Filename is the original filename supplied by the client.
	Filename string

	// Hash is the hex-encoded SHA-256 provenance hash of the payload.  May be
	// empty for events that occur before hashing completes.
	Hash string

	// Status is the intake pipeline status at the time of this event.
	Status IntakeStatus

	// Timestamp is the wall-clock time at which this event was generated.
	Timestamp time.Time
}

// AuditSink receives IntakeAuditEvents.  Implementations may write to a
// database, a message bus, a log stream, or any combination thereof.
type AuditSink interface {
	// Emit delivers an event to the sink.  A non-nil error indicates the event
	// could not be durably recorded; callers may retry or escalate as
	// appropriate.
	Emit(ctx context.Context, event IntakeAuditEvent) error
}

// NoOpAuditSink discards all events silently.  Use in unit tests that do not
// need to assert on audit output.
type NoOpAuditSink struct{}

// Emit implements AuditSink.
func (NoOpAuditSink) Emit(_ context.Context, _ IntakeAuditEvent) error {
	return nil
}

// LoggingAuditSink writes every IntakeAuditEvent to the structured logger as
// an INFO-level message.
type LoggingAuditSink struct {
	logger *slog.Logger
}

// NewLoggingAuditSink creates a LoggingAuditSink using logger.  If logger is
// nil, slog.Default() is used.
func NewLoggingAuditSink(logger *slog.Logger) *LoggingAuditSink {
	if logger == nil {
		logger = slog.Default()
	}
	return &LoggingAuditSink{logger: logger}
}

// Emit implements AuditSink.
func (l *LoggingAuditSink) Emit(ctx context.Context, e IntakeAuditEvent) error {
	l.logger.InfoContext(ctx, fmt.Sprintf("intake audit: %s", e.EventType),
		"event_type", e.EventType,
		"intake_id", e.IntakeID,
		"tenant_id", e.TenantID,
		"uploader_id", e.UploaderID,
		"filename", e.Filename,
		"hash", e.Hash,
		"status", string(e.Status),
		"timestamp", e.Timestamp.Format(time.RFC3339Nano),
	)
	return nil
}

// CapturingAuditSink stores emitted events in memory.  Useful in tests that
// need to assert on the sequence of audit events.
type CapturingAuditSink struct {
	Events []IntakeAuditEvent
}

// Emit implements AuditSink.
func (c *CapturingAuditSink) Emit(_ context.Context, event IntakeAuditEvent) error {
	c.Events = append(c.Events, event)
	return nil
}
