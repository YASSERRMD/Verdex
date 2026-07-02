package guardrail

import (
	"sync"
	"time"
)

// ViolationKind classifies which guardrail check failed, so an audit
// reviewer or alert consumer can tell a missing-label finding apart from
// a verdict-language finding or a blocked finalization without parsing
// free text. Mirrors packages/knowledgeisolation's ViolationKind
// convention exactly.
type ViolationKind string

// ViolationKind constants for GuardrailEvent.Kind.
const (
	// ViolationMissingLabel marks a RequireLabel/ValidateLabeled failure:
	// a reasoning output did not carry the mandatory draft_analysis
	// label.
	ViolationMissingLabel ViolationKind = "missing_label"

	// ViolationVerdictLanguage marks a CheckText failure: the checked
	// text contained verdict or directive phrasing.
	ViolationVerdictLanguage ViolationKind = "verdict_language"

	// ViolationFinalizeBlocked marks a CanFinalize failure: a case was
	// not approved for finalization (no sign-off recorded, sign-off
	// rejected, or the gate itself errored).
	ViolationFinalizeBlocked ViolationKind = "finalize_blocked"
)

// GuardrailEvent records a single guardrail-check failure with enough
// detail to support a security/compliance review: which check failed,
// for which case, why, and when. Mirrors
// packages/knowledgeisolation.AccessAttempt's shape.
type GuardrailEvent struct {
	// Kind identifies which guardrail check failed.
	Kind ViolationKind

	// CaseID identifies the case the checked output belongs to, when
	// known. Empty when the check was performed on text with no case
	// association (e.g. an ad hoc CheckText call).
	CaseID string

	// Detail is a short human-readable description of the failure, e.g.
	// the error returned by the failed check.
	Detail string

	// OccurredAt is the time the event was recorded.
	OccurredAt time.Time
}

// AlertSink receives GuardrailEvent values for delivery to an external
// system (a SIEM, a metrics platform, a paging system). Mirrors
// packages/knowledgeisolation.AlertSink precisely: a minimal interface
// with a no-op default so alerting is opt-in.
type AlertSink interface {
	// Notify delivers event. Implementations should be fast and
	// non-blocking; heavy I/O should be offloaded to a goroutine.
	Notify(event GuardrailEvent)
}

// NoOpAlertSink is an AlertSink that silently discards every event. It is
// the default used by any Recorder constructed without an explicit sink.
type NoOpAlertSink struct{}

// Notify implements AlertSink by doing nothing.
func (NoOpAlertSink) Notify(GuardrailEvent) {}

// FuncAlertSink adapts a plain function to the AlertSink interface, for
// simple stateless sinks — mirroring
// packages/knowledgeisolation.FuncAlertSink.
type FuncAlertSink func(GuardrailEvent)

// Notify implements AlertSink by calling f.
func (f FuncAlertSink) Notify(event GuardrailEvent) {
	if f != nil {
		f(event)
	}
}

// MultiAlertSink fans out a single GuardrailEvent to multiple AlertSink
// implementations, in order.
type MultiAlertSink struct {
	Sinks []AlertSink
}

// Notify implements AlertSink by calling Notify on each child sink.
func (m MultiAlertSink) Notify(event GuardrailEvent) {
	for _, s := range m.Sinks {
		if s != nil {
			s.Notify(event)
		}
	}
}

// Recorder is an optional audit log for guardrail enforcement: every
// CheckText/RequireLabel/CanFinalize failure a caller chooses to record
// is appended here and forwarded to the configured AlertSink. Recording
// is opt-in and additive — none of CheckText, RequireLabel,
// ValidateLabeled, or CanFinalize call a Recorder themselves, since they
// are plain functions with no recorder to hold; a caller that wants an
// audit trail constructs a Recorder and calls RecordXxx alongside its
// own error handling. This keeps the core checks dependency-free while
// still providing the standard audit shape for callers that want it.
type Recorder struct {
	mu     sync.Mutex
	events []GuardrailEvent
	sink   AlertSink
	now    func() time.Time
}

// NewRecorder constructs a Recorder that forwards every recorded event to
// sink. A nil sink is replaced with NoOpAlertSink. A nil now defaults to
// time.Now.
func NewRecorder(sink AlertSink, now func() time.Time) *Recorder {
	if sink == nil {
		sink = NoOpAlertSink{}
	}
	if now == nil {
		now = time.Now
	}
	return &Recorder{sink: sink, now: now}
}

// Record appends event (with OccurredAt filled in) to the recorder's log
// and forwards it to the configured AlertSink. Safe for concurrent use.
func (r *Recorder) Record(event GuardrailEvent) {
	event.OccurredAt = r.now()

	r.mu.Lock()
	r.events = append(r.events, event)
	r.mu.Unlock()

	r.sink.Notify(event)
}

// RecordCheckTextFailure records a CheckText failure for caseID, using
// err's message as Detail. A nil err is a no-op — nothing failed, so
// nothing is recorded.
func (r *Recorder) RecordCheckTextFailure(caseID string, err error) {
	if err == nil {
		return
	}
	r.Record(GuardrailEvent{Kind: ViolationVerdictLanguage, CaseID: caseID, Detail: err.Error()})
}

// RecordLabelFailure records a RequireLabel/ValidateLabeled failure for
// caseID, using err's message as Detail. A nil err is a no-op.
func (r *Recorder) RecordLabelFailure(caseID string, err error) {
	if err == nil {
		return
	}
	r.Record(GuardrailEvent{Kind: ViolationMissingLabel, CaseID: caseID, Detail: err.Error()})
}

// RecordFinalizeBlocked records a CanFinalize failure for caseID, using
// err's message as Detail. A nil err is a no-op.
func (r *Recorder) RecordFinalizeBlocked(caseID string, err error) {
	if err == nil {
		return
	}
	r.Record(GuardrailEvent{Kind: ViolationFinalizeBlocked, CaseID: caseID, Detail: err.Error()})
}

// Events returns a defensive copy of every GuardrailEvent recorded so
// far, in recording order. Safe for concurrent use.
func (r *Recorder) Events() []GuardrailEvent {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]GuardrailEvent, len(r.events))
	copy(out, r.events)
	return out
}
