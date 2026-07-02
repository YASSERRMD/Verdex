package multilingual

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// AuditStep identifies a single normalization stage that may be applied to
// a piece of text.
type AuditStep string

const (
	// StepUnicodeNormalized records that NormalizeUnicode ran.
	StepUnicodeNormalized AuditStep = "unicode-normalized"

	// StepScriptDetected records that DetectScript/DetectLanguage ran.
	StepScriptDetected AuditStep = "script-detected"

	// StepRTLFlagged records that DetectRTLRuns ran and RTL metadata was
	// attached.
	StepRTLFlagged AuditStep = "rtl-flagged"

	// StepLegalTermNormalized records that LegalTermNormalizer ran.
	StepLegalTermNormalized AuditStep = "legal-term-normalized"

	// StepTransliterated records that a Transliterator ran.
	StepTransliterated AuditStep = "transliterated"

	// StepTranslated records that a Translator pass ran (Translate was
	// invoked, whether or not it changed the text).
	StepTranslated AuditStep = "translated"

	// StepTokenized records that a Tokenizer ran.
	StepTokenized AuditStep = "tokenized"
)

// AuditEvent records that a single normalization Step was applied to a
// piece of text, mirroring packages/intake's IntakeAuditEvent pattern:
// every meaningful pipeline transition emits an event carrying enough
// context for downstream forensic tools to reconstruct the full
// normalization timeline for a document.
type AuditEvent struct {
	// Step identifies which normalization stage this event describes.
	Step AuditStep

	// DocumentID optionally correlates this event with the source
	// document/transcript being normalized. May be empty if the caller
	// has no identifier to attach.
	DocumentID string

	// Language is the (candidate) language the text was processed as at
	// this step, if applicable.
	Language Language

	// Detail is a short, human-readable note about the step's outcome
	// (e.g. "form=NFC", "script=arabic language=ur", "tokens=12").
	Detail string

	// Timestamp is the wall-clock time this step completed.
	Timestamp time.Time
}

// AuditSink receives AuditEvents. Implementations may write to a database,
// a message bus, a log stream, or any combination thereof.
type AuditSink interface {
	// Emit delivers an event to the sink. A non-nil error indicates the
	// event could not be durably recorded; callers may retry or escalate
	// as appropriate.
	Emit(ctx context.Context, event AuditEvent) error
}

// NoOpAuditSink discards all events silently. Use in unit tests or
// call sites that do not need to record audit output.
type NoOpAuditSink struct{}

// Emit implements AuditSink.
func (NoOpAuditSink) Emit(_ context.Context, _ AuditEvent) error {
	return nil
}

// LoggingAuditSink writes every AuditEvent to the structured logger as an
// INFO-level message.
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
	l.logger.InfoContext(ctx, fmt.Sprintf("multilingual audit: %s", e.Step),
		"step", string(e.Step),
		"document_id", e.DocumentID,
		"language", string(e.Language),
		"detail", e.Detail,
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
