package reasoningprofile

import (
	"sync"
	"time"
)

// Event records a single per-case Family override, with enough detail to
// support an audit/compliance review: which case, what the auto-resolved
// family would have been, what it was overridden to, why, and when.
// Mirrors packages/guardrail.Event's shape exactly.
type Event struct {
	// CaseID identifies the case the override applies to.
	CaseID string

	// PreviousFamily is the family that was in effect before this
	// override — either a prior override or the auto-resolved family, as
	// known by the caller supplying it to SetOverride.
	PreviousFamily Family

	// OverrideFamily is the family the case is now pinned to.
	OverrideFamily Family

	// Reason is a short human-readable justification for the override.
	Reason string

	// AppliedAt is the time the override was recorded.
	AppliedAt time.Time
}

// AlertSink receives Event values for delivery to an external system (a
// SIEM, a metrics platform, a paging system). Mirrors
// packages/guardrail.AlertSink precisely: a minimal interface with a
// no-op default so alerting is opt-in.
type AlertSink interface {
	// Notify delivers event. Implementations should be fast and
	// non-blocking; heavy I/O should be offloaded to a goroutine.
	Notify(event Event)
}

// NoOpAlertSink is an AlertSink that silently discards every event. It is
// the default used by any OverrideRegistry constructed without an
// explicit sink.
type NoOpAlertSink struct{}

// Notify implements AlertSink by doing nothing.
func (NoOpAlertSink) Notify(Event) {}

// FuncAlertSink adapts a plain function to the AlertSink interface, for
// simple stateless sinks — mirroring packages/guardrail.FuncAlertSink.
type FuncAlertSink func(Event)

// Notify implements AlertSink by calling f.
func (f FuncAlertSink) Notify(event Event) {
	if f != nil {
		f(event)
	}
}

// MultiAlertSink fans out a single Event to multiple AlertSink
// implementations, in order.
type MultiAlertSink struct {
	Sinks []AlertSink
}

// Notify implements AlertSink by calling Notify on each child sink.
func (m MultiAlertSink) Notify(event Event) {
	for _, s := range m.Sinks {
		if s != nil {
			s.Notify(event)
		}
	}
}

// OverrideRegistry lets a deployment pin a specific case to a Family
// other than the one ResolveFamily would auto-resolve from the case's
// jurisdiction — e.g. a case that is nominally filed in a mixed-family
// jurisdiction but that the parties have contractually agreed to argue
// under common-law evidentiary rules. Every override is recorded and
// forwarded to the configured AlertSink; there is no way to set an
// override without it being audited.
type OverrideRegistry struct {
	mu        sync.Mutex
	overrides map[string]Family
	events    []Event
	sink      AlertSink
	now       func() time.Time
}

// NewOverrideRegistry constructs an OverrideRegistry that forwards every
// recorded override to sink. A nil sink is replaced with NoOpAlertSink. A
// nil now defaults to time.Now.
func NewOverrideRegistry(sink AlertSink, now func() time.Time) *OverrideRegistry {
	if sink == nil {
		sink = NoOpAlertSink{}
	}
	if now == nil {
		now = time.Now
	}
	return &OverrideRegistry{
		overrides: make(map[string]Family),
		sink:      sink,
		now:       now,
	}
}

// SetOverride pins caseID to family, recording an audit Event with reason
// and the previous family (the case's existing override, if any, or
// FamilyCommonLaw's zero-value absence represented as the empty Family).
// Returns ErrEmptyCaseID if caseID is empty, or ErrUnknownFamily if
// family is not one of the four canonical constants.
func (r *OverrideRegistry) SetOverride(caseID string, family Family, reason string) error {
	if caseID == "" {
		return ErrEmptyCaseID
	}
	if !family.IsValid() {
		return ErrUnknownFamily
	}

	r.mu.Lock()
	previous := r.overrides[caseID]
	r.overrides[caseID] = family
	event := Event{
		CaseID:         caseID,
		PreviousFamily: previous,
		OverrideFamily: family,
		Reason:         reason,
		AppliedAt:      r.now(),
	}
	r.events = append(r.events, event)
	r.mu.Unlock()

	r.sink.Notify(event)
	return nil
}

// OverrideFor returns the Family override recorded for caseID, if any.
// The second return value reports whether an override exists.
func (r *OverrideRegistry) OverrideFor(caseID string) (Family, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	family, ok := r.overrides[caseID]
	return family, ok
}

// Events returns a defensive copy of every override Event recorded so
// far, in recording order. Safe for concurrent use.
func (r *OverrideRegistry) Events() []Event {
	r.mu.Lock()
	defer r.mu.Unlock()

	out := make([]Event, len(r.events))
	copy(out, r.events)
	return out
}

// ResolveWithOverride resolves the effective Family for caseID: the
// registry's override if one has been set via SetOverride, otherwise
// fallback (typically the result of ResolveFamily applied to the case's
// jurisdiction). A nil registry is treated as having no overrides.
func ResolveWithOverride(r *OverrideRegistry, caseID string, fallback Family) Family {
	if r == nil {
		return fallback
	}
	if family, ok := r.OverrideFor(caseID); ok {
		return family
	}
	return fallback
}
