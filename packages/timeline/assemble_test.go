package timeline

import (
	"testing"
	"time"
)

func date(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return &t
}

func TestAssembleTimeline_Ordering(t *testing.T) {
	events := []Event{
		{ID: "e3", OccurredAt: nil},
		{ID: "e1", OccurredAt: date(2024, time.March, 15)},
		{ID: "e4", OccurredAt: nil},
		{ID: "e2", OccurredAt: date(2024, time.January, 1)},
	}

	tl := AssembleTimeline("case-1", events)

	if tl.CaseID != "case-1" {
		t.Errorf("CaseID = %q, want case-1", tl.CaseID)
	}

	wantOrder := []string{"e2", "e1", "e3", "e4"}
	if len(tl.Events) != len(wantOrder) {
		t.Fatalf("len(Events) = %d, want %d", len(tl.Events), len(wantOrder))
	}
	for i, id := range wantOrder {
		if tl.Events[i].ID != id {
			t.Errorf("Events[%d].ID = %q, want %q", i, tl.Events[i].ID, id)
		}
	}
}

func TestAssembleTimeline_StableForSameDate(t *testing.T) {
	sameDate := date(2024, time.March, 15)
	events := []Event{
		{ID: "a", OccurredAt: sameDate},
		{ID: "b", OccurredAt: sameDate},
		{ID: "c", OccurredAt: sameDate},
	}

	tl := AssembleTimeline("case-1", events)

	wantOrder := []string{"a", "b", "c"}
	for i, id := range wantOrder {
		if tl.Events[i].ID != id {
			t.Errorf("Events[%d].ID = %q, want %q (stability broken)", i, tl.Events[i].ID, id)
		}
	}
}

func TestAssembleTimeline_StableForUndated(t *testing.T) {
	events := []Event{
		{ID: "u1", OccurredAt: nil},
		{ID: "u2", OccurredAt: nil},
		{ID: "u3", OccurredAt: nil},
	}

	tl := AssembleTimeline("case-1", events)

	wantOrder := []string{"u1", "u2", "u3"}
	for i, id := range wantOrder {
		if tl.Events[i].ID != id {
			t.Errorf("Events[%d].ID = %q, want %q (stability broken)", i, tl.Events[i].ID, id)
		}
	}
}

func TestAssembleTimeline_Empty(t *testing.T) {
	tl := AssembleTimeline("case-1", nil)
	if len(tl.Events) != 0 {
		t.Errorf("len(Events) = %d, want 0", len(tl.Events))
	}
}

func TestTimeline_DatedUndatedEvents(t *testing.T) {
	events := []Event{
		{ID: "e1", OccurredAt: date(2024, time.January, 1)},
		{ID: "e2", OccurredAt: nil},
		{ID: "e3", OccurredAt: date(2024, time.February, 1)},
		{ID: "e4", OccurredAt: nil},
	}
	tl := AssembleTimeline("case-1", events)

	dated := tl.DatedEvents()
	if len(dated) != 2 || dated[0].ID != "e1" || dated[1].ID != "e3" {
		t.Errorf("DatedEvents() = %+v, want [e1, e3]", dated)
	}

	undated := tl.UndatedEvents()
	if len(undated) != 2 || undated[0].ID != "e2" || undated[1].ID != "e4" {
		t.Errorf("UndatedEvents() = %+v, want [e2, e4]", undated)
	}
}
