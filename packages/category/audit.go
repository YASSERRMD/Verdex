package category

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Audit event type labels. A CategoryAuditEvent's EventType is always one
// of these constants.
const (
	// AuditEventSuggested is emitted when a Suggester produces candidate
	// Suggestions for a case.
	AuditEventSuggested = "category.suggested"

	// AuditEventOverridden is emitted when a ManualOverride is applied.
	AuditEventOverridden = "category.overridden"

	// AuditEventChanged is emitted when a case's final assigned Category
	// changes from a previously recorded assignment.
	AuditEventChanged = "category.changed"

	// AuditEventValidated is emitted when a Category is validated against a
	// jurisdiction's Taxonomy (see validate.go).
	AuditEventValidated = "category.validated"
)

// CategoryAuditEvent is emitted for every significant transition in the
// categorization pipeline: a suggestion being produced, an override being
// applied, a final category changing, or a validation occurring — mirroring
// packages/intake's IntakeAuditEvent/AuditSink pattern.
type CategoryAuditEvent struct {
	// EventType is one of the AuditEvent* constants above.
	EventType string

	// CaseID identifies the case this event describes.
	CaseID string

	// JurisdictionCode identifies the jurisdiction the case's category was
	// evaluated against, when applicable. May be empty for events that are
	// not jurisdiction-scoped.
	JurisdictionCode string

	// CategoryCode is the category code involved in this event (e.g. the
	// suggested, overridden, or newly assigned code).
	CategoryCode CategoryCode

	// Actor identifies who or what triggered this event: a user ID for a
	// human-initiated override, or a fixed system identifier (e.g.
	// "system:keyword-suggester") for an automated step.
	Actor string

	// Confidence is the confidence score associated with this event, when
	// applicable (e.g. a suggestion's or the final assignment's
	// confidence). Zero for events with no associated confidence.
	Confidence float64

	// Timestamp is the wall-clock time at which this event was generated.
	Timestamp time.Time
}

// AuditSink receives CategoryAuditEvents. Implementations may write to a
// database, a message bus, a log stream, or any combination thereof.
type AuditSink interface {
	// Emit delivers an event to the sink. A non-nil error indicates the
	// event could not be durably recorded; callers may retry or escalate as
	// appropriate.
	Emit(ctx context.Context, event CategoryAuditEvent) error
}

// NoOpAuditSink discards all events silently. Use in unit tests that do not
// need to assert on audit output.
type NoOpAuditSink struct{}

// Emit implements AuditSink.
func (NoOpAuditSink) Emit(_ context.Context, _ CategoryAuditEvent) error {
	return nil
}

// LoggingAuditSink writes every CategoryAuditEvent to the structured logger
// as an INFO-level message.
type LoggingAuditSink struct {
	logger *slog.Logger
}

// NewLoggingAuditSink creates a LoggingAuditSink using logger. If logger is
// nil, slog.Default() is used.
func NewLoggingAuditSink(logger *slog.Logger) *LoggingAuditSink {
	if logger == nil {
		logger = slog.Default()
	}
	return &LoggingAuditSink{logger: logger}
}

// Emit implements AuditSink.
func (l *LoggingAuditSink) Emit(ctx context.Context, e CategoryAuditEvent) error {
	l.logger.InfoContext(ctx, fmt.Sprintf("category audit: %s", e.EventType),
		"event_type", e.EventType,
		"case_id", e.CaseID,
		"jurisdiction_code", e.JurisdictionCode,
		"category_code", string(e.CategoryCode),
		"actor", e.Actor,
		"confidence", e.Confidence,
		"timestamp", e.Timestamp.Format(time.RFC3339Nano),
	)
	return nil
}

// CapturingAuditSink stores emitted events in memory. Useful in tests that
// need to assert on the sequence of audit events.
type CapturingAuditSink struct {
	Events []CategoryAuditEvent
}

// Emit implements AuditSink.
func (c *CapturingAuditSink) Emit(_ context.Context, event CategoryAuditEvent) error {
	c.Events = append(c.Events, event)
	return nil
}
