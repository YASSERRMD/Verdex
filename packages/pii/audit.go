package pii

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// Audit event type labels, mirroring packages/intake's AuditSink event-type
// convention.
const (
	// EventDetect is emitted after a Detect call completes.
	EventDetect = "pii.detected"

	// EventRedact is emitted after a Redact/pseudonymize call completes.
	EventRedact = "pii.redacted"

	// EventReveal is emitted after a PseudonymMap.Reveal call, whether it
	// succeeded or was denied.
	EventReveal = "pii.revealed"
)

// AuditEvent records a single detect/redact/reveal action against PII, with
// enough context to reconstruct who did what, when, and to how much data,
// without ever including the raw PII value itself.
type AuditEvent struct {
	// ID uniquely identifies this audit event.
	ID uuid.UUID

	// EventType is one of the Event* constants above (or a caller-defined
	// label for extension).
	EventType string

	// Actor identifies who/what performed the action (a user ID, role, or
	// service name). Mirrors packages/intake's UploaderID-style actor
	// field, generalized to a string since PII actions may be triggered by
	// services as well as end users.
	Actor string

	// JurisdictionCode is the jurisdiction code the action was evaluated
	// under, when applicable (see jurisdiction_rules.go). Empty when not
	// applicable.
	JurisdictionCode string

	// MatchCount is the number of PIIMatches involved in this event (e.g.
	// number detected, number redacted). Zero for reveal events, which
	// concern a single token.
	MatchCount int

	// Token is the pseudonym token involved, populated only for
	// EventReveal.
	Token string

	// Allowed records the outcome for EventReveal (true if access was
	// granted). Always true for EventDetect/EventRedact, which are not
	// access-gated.
	Allowed bool

	// Timestamp is the wall-clock time at which this event was generated.
	Timestamp time.Time
}

// AuditSink receives PII AuditEvents. Implementations may write to a
// database, a message bus, a log stream, or any combination thereof —
// mirroring packages/intake's AuditSink interface exactly.
type AuditSink interface {
	// Emit delivers an event to the sink. A non-nil error indicates the
	// event could not be durably recorded; callers may retry or escalate as
	// appropriate.
	Emit(ctx context.Context, event AuditEvent) error
}

// NoOpAuditSink discards all events silently. Use in unit tests or call
// sites that do not need PII audit output.
type NoOpAuditSink struct{}

// Emit implements AuditSink.
func (NoOpAuditSink) Emit(_ context.Context, _ AuditEvent) error { return nil }

// LoggingAuditSink writes every AuditEvent to the structured logger as an
// INFO-level message. The raw PII value is never logged — only match
// counts, tokens, and actor/jurisdiction metadata.
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
func (l *LoggingAuditSink) Emit(ctx context.Context, e AuditEvent) error {
	l.logger.InfoContext(ctx, fmt.Sprintf("pii audit: %s", e.EventType),
		"event_type", e.EventType,
		"actor", e.Actor,
		"jurisdiction_code", e.JurisdictionCode,
		"match_count", e.MatchCount,
		"token", e.Token,
		"allowed", e.Allowed,
		"timestamp", e.Timestamp.Format(time.RFC3339Nano),
	)
	return nil
}

// CapturingAuditSink stores emitted events in memory. Useful in tests that
// need to assert on the sequence of audit events.
type CapturingAuditSink struct {
	Events []AuditEvent
}

// Emit implements AuditSink.
func (c *CapturingAuditSink) Emit(_ context.Context, event AuditEvent) error {
	c.Events = append(c.Events, event)
	return nil
}

// newAuditEvent is a small helper constructing an AuditEvent with a fresh ID
// and current UTC timestamp, reducing boilerplate at call sites.
func newAuditEvent(eventType, actor, jurisdictionCode string, matchCount int, token string, allowed bool) AuditEvent {
	return AuditEvent{
		ID:               uuid.New(),
		EventType:        eventType,
		Actor:            actor,
		JurisdictionCode: jurisdictionCode,
		MatchCount:       matchCount,
		Token:            token,
		Allowed:          allowed,
		Timestamp:        time.Now().UTC(),
	}
}
