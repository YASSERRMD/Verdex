package timeline

import "sort"

// Timeline is the assembled, ordered chronology of Events for a case.
type Timeline struct {
	// CaseID identifies the case this Timeline was assembled for.
	CaseID string

	// Events is the ordered event list: dated events first in ascending
	// OccurredAt order, followed by undated events, per AssembleTimeline's
	// documented ordering contract.
	Events []Event
}

// AssembleTimeline orders events into a Timeline for caseID: events with a
// known OccurredAt are sorted ascending by date, followed by all undated
// events (OccurredAt == nil) grouped at the end.
//
// Ordering is stable in both groups: events that compare equal (the same
// OccurredAt date, or all undated events) retain their relative input
// order, so repeated assembly of the same input is fully deterministic and
// callers can rely on ties being broken by extraction/insertion order
// rather than an arbitrary sort decision.
func AssembleTimeline(caseID string, events []Event) Timeline {
	dated := make([]Event, 0, len(events))
	undated := make([]Event, 0, len(events))
	for _, ev := range events {
		if ev.OccurredAt == nil {
			undated = append(undated, ev)
		} else {
			dated = append(dated, ev)
		}
	}

	sort.SliceStable(dated, func(i, j int) bool {
		return dated[i].OccurredAt.Before(*dated[j].OccurredAt)
	})

	ordered := make([]Event, 0, len(events))
	ordered = append(ordered, dated...)
	ordered = append(ordered, undated...)

	return Timeline{CaseID: caseID, Events: ordered}
}

// DatedEvents returns the subset of t.Events with a non-nil OccurredAt, in
// their existing (already-ascending) order.
func (t Timeline) DatedEvents() []Event {
	out := make([]Event, 0, len(t.Events))
	for _, ev := range t.Events {
		if ev.OccurredAt != nil {
			out = append(out, ev)
		}
	}
	return out
}

// UndatedEvents returns the subset of t.Events with a nil OccurredAt.
func (t Timeline) UndatedEvents() []Event {
	out := make([]Event, 0, len(t.Events))
	for _, ev := range t.Events {
		if ev.OccurredAt == nil {
			out = append(out, ev)
		}
	}
	return out
}
