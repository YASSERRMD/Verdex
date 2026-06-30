package observability

import (
	"context"
	"io"
	"log/slog"
	"time"
)

// AuditEvent is the minimal contract for an audit record: who did
// what, to what, and with what outcome, at what time.
//
// This is intentionally small. Phase 077 owns the full audit trail
// (richer event taxonomy, retention/storage guarantees, tamper
// evidence, query interfaces, etc.); this type only establishes that
// audit events flow through a channel separate from application logs,
// with the minimum fields any later extension will need to carry
// forward. Treat additions to this struct as a later-phase concern
// unless a current phase has a concrete, immediate need.
type AuditEvent struct {
	// Time is when the audited action occurred. Callers should set
	// this explicitly (rather than relying on AuditLogger to stamp it)
	// when replaying or batching events, but AuditLogger.Log fills in
	// time.Now() if Time is the zero value.
	Time time.Time `json:"time"`

	// Actor identifies who (or what) performed the action - a user
	// ID, service account, or system process name.
	Actor string `json:"actor"`

	// Action is a short, stable verb-phrase identifying what happened,
	// e.g. "case.viewed" or "ruling.exported".
	Action string `json:"action"`

	// Target identifies what the action was performed on, e.g. a case
	// ID or document ID. May be empty for actions with no single
	// target.
	Target string `json:"target,omitempty"`

	// Outcome records the result of the action, e.g. "success" or
	// "denied". Kept as a free-form string in this phase rather than a
	// closed enum, since the full taxonomy belongs to Phase 077.
	Outcome string `json:"outcome"`
}

// AuditLogger writes AuditEvents to a sink that is kept separate from
// the application's regular Logger output, so audit records are never
// commingled with debug/info/warn/error application log lines and can
// be routed, retained, and access-controlled independently.
type AuditLogger struct {
	slog *slog.Logger
}

// NewAuditLogger returns an AuditLogger that writes one JSON object
// per event to w. w is typically a distinct file, file descriptor, or
// network sink from the application Logger's output - that separation
// is the entire point of this type. JSON is used unconditionally
// (rather than following the application Logger's configured format)
// since audit records are meant for machine processing and downstream
// audit pipelines, not console readability.
func NewAuditLogger(w io.Writer) *AuditLogger {
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo})
	return &AuditLogger{slog: slog.New(handler)}
}

// Log records event. If event.Time is the zero value, it is set to
// time.Now() before writing.
func (a *AuditLogger) Log(ctx context.Context, event AuditEvent) {
	if event.Time.IsZero() {
		event.Time = time.Now().UTC()
	}
	a.slog.LogAttrs(ctx, slog.LevelInfo, "audit_event",
		slog.Time("time", event.Time),
		slog.String("actor", event.Actor),
		slog.String("action", event.Action),
		slog.String("target", event.Target),
		slog.String("outcome", event.Outcome),
	)
}
