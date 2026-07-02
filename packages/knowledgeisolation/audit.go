package knowledgeisolation

import (
	"sync"
	"time"
)

// ViolationKind classifies the operation that triggered a scope
// violation, so an audit reviewer or alert consumer can tell a rejected
// read apart from a rejected write without parsing free text.
type ViolationKind string

// ViolationKind constants for AccessAttempt.Kind.
const (
	// ViolationGetNode marks a rejected CaseScopedStore.GetNode call.
	ViolationGetNode ViolationKind = "get_node"

	// ViolationCreateEdge marks a rejected CaseScopedStore.CreateEdge
	// call (an edge whose endpoints span two different cases).
	ViolationCreateEdge ViolationKind = "create_edge"

	// ViolationTraverse marks a Traverse call that would otherwise have
	// returned nodes outside the authorized case (the guard filters
	// them, but still records the attempt for audit).
	ViolationTraverse ViolationKind = "traverse"

	// ViolationVectorQuery marks a rejected or filtered VectorStore
	// Query call.
	ViolationVectorQuery ViolationKind = "vector_query"

	// ViolationVectorUpsert marks a rejected VectorStore Upsert call.
	ViolationVectorUpsert ViolationKind = "vector_upsert"

	// ViolationCrossCaseAnalysis marks a rejected attempt to use the
	// cross-case analysis escape hatch (see CrossCaseAuthorization).
	ViolationCrossCaseAnalysis ViolationKind = "cross_case_analysis"

	// ViolationDeleteTree marks a rejected CaseScopedStore.DeleteTree
	// call for a case other than the one the guard is scoped to.
	ViolationDeleteTree ViolationKind = "delete_tree"
)

// AccessAttempt records a single rejected (or, for Traverse, filtered)
// cross-case access attempt with enough detail to support a security
// review: which case(s) the caller was authorized for, which case(s) it
// attempted to reach, which node/query was involved, and when.
type AccessAttempt struct {
	// Kind identifies which operation was attempted.
	Kind ViolationKind

	// AuthorizedCases lists the case(s) the guard performing the check
	// was scoped to at the time of the attempt.
	AuthorizedCases []CaseID

	// AttemptedCase is the case the caller attempted to access, when
	// known (e.g. the owning case of a rejected node, or the case a
	// cross-edge's other endpoint belongs to). Empty if not applicable.
	AttemptedCase CaseID

	// NodeID identifies the node involved in the attempt, when
	// applicable (GetNode, CreateEdge, Traverse, VectorUpsert). Empty
	// for query-shaped attempts with no single node identity
	// (VectorQuery).
	NodeID string

	// Detail is a short human-readable description of the attempt,
	// e.g. "edge endpoints span cases case-a and case-b".
	Detail string

	// OccurredAt is the time the attempt was recorded.
	OccurredAt time.Time
}

// AlertSink receives AccessAttempt events for delivery to an external
// system (e.g. a SIEM, a metrics platform, or a paging system). Mirrors
// packages/accounting's AlertSink pattern: a minimal interface with a
// no-op default so alerting is opt-in.
type AlertSink interface {
	// Notify delivers attempt. Implementations should be fast and
	// non-blocking; heavy I/O should be offloaded to a goroutine.
	Notify(attempt AccessAttempt)
}

// NoOpAlertSink is an AlertSink that silently discards every event. It is
// the default used when a guard is constructed without an explicit sink.
type NoOpAlertSink struct{}

// Notify implements AlertSink by doing nothing.
func (NoOpAlertSink) Notify(AccessAttempt) {}

// FuncAlertSink adapts a plain function to the AlertSink interface,
// mirroring packages/traversal's function-type indirection convention
// (e.g. PrecedentResolver) for simple, stateless sinks.
type FuncAlertSink func(AccessAttempt)

// Notify implements AlertSink by calling f.
func (f FuncAlertSink) Notify(attempt AccessAttempt) {
	if f != nil {
		f(attempt)
	}
}

// MultiAlertSink fans out a single AccessAttempt to multiple AlertSink
// implementations, in order.
type MultiAlertSink struct {
	Sinks []AlertSink
}

// Notify implements AlertSink by calling Notify on each child sink.
func (m MultiAlertSink) Notify(attempt AccessAttempt) {
	for _, s := range m.Sinks {
		if s != nil {
			s.Notify(attempt)
		}
	}
}

// auditRecorder is a package-local, mutex-protected audit log of
// AccessAttempts, mirroring the treeindex/traversal/adaptiveretrieval
// convention of an unexported "xxxRecorder" (mutex + struct) backing a
// Stats()/snapshot() accessor rather than depending on
// packages/observability.
type auditRecorder struct {
	mu       sync.Mutex
	attempts []AccessAttempt
	sink     AlertSink
	now      func() time.Time
}

// newAuditRecorder constructs an auditRecorder that forwards every
// recorded attempt to sink. A nil sink is replaced with NoOpAlertSink. A
// nil now defaults to time.Now.
func newAuditRecorder(sink AlertSink, now func() time.Time) *auditRecorder {
	if sink == nil {
		sink = NoOpAlertSink{}
	}
	if now == nil {
		now = time.Now
	}
	return &auditRecorder{sink: sink, now: now}
}

// record appends attempt (with OccurredAt filled in) to the recorder's
// log and forwards it to the configured AlertSink.
func (r *auditRecorder) record(attempt AccessAttempt) {
	attempt.OccurredAt = r.now()

	r.mu.Lock()
	r.attempts = append(r.attempts, attempt)
	r.mu.Unlock()

	r.sink.Notify(attempt)
}

// snapshot returns a copy of every AccessAttempt recorded so far, in
// recording order. Safe for concurrent use.
func (r *auditRecorder) snapshot() []AccessAttempt {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]AccessAttempt, len(r.attempts))
	copy(out, r.attempts)
	return out
}
