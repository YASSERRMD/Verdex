package fact_test

import (
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/fact"
	"github.com/YASSERRMD/verdex/packages/timeline"
)

func TestAnchorToEvent_SharedSegmentID(t *testing.T) {
	occurred := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
	events := []timeline.Event{
		{ID: "event-1", Description: "The contract was signed.", SegmentID: "seg-1", OccurredAt: &occurred},
	}

	anchor := fact.AnchorToEvent("fact-1", "The contract was signed on March 15.", "seg-1", events)
	if anchor.EventID != "event-1" {
		t.Fatalf("expected EventID event-1, got %q", anchor.EventID)
	}
	if anchor.OccurredAt == nil || !anchor.OccurredAt.Equal(occurred) {
		t.Errorf("expected OccurredAt %v, got %v", occurred, anchor.OccurredAt)
	}
}

func TestAnchorToEvent_FallsBackToTextOverlap(t *testing.T) {
	occurred := time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC)
	events := []timeline.Event{
		{ID: "event-1", Description: "The supply agreement contract was executed by both parties", SegmentID: "seg-other", OccurredAt: &occurred},
	}

	anchor := fact.AnchorToEvent("fact-1", "The supply agreement contract was executed by both parties", "seg-1", events)
	if anchor.EventID != "event-1" {
		t.Fatalf("expected EventID event-1 via text overlap, got %q", anchor.EventID)
	}
}

func TestAnchorToEvent_NoMatch(t *testing.T) {
	events := []timeline.Event{
		{ID: "event-1", Description: "Completely unrelated subject matter here", SegmentID: "seg-other"},
	}

	anchor := fact.AnchorToEvent("fact-1", "The witness saw the car crash.", "seg-1", events)
	if anchor.EventID != "" {
		t.Errorf("expected no match, got EventID %q", anchor.EventID)
	}
	if anchor.OccurredAt != nil {
		t.Errorf("expected nil OccurredAt, got %v", anchor.OccurredAt)
	}
}

func TestAnchorToEvent_NoEvents(t *testing.T) {
	anchor := fact.AnchorToEvent("fact-1", "some text", "seg-1", nil)
	if anchor.EventID != "" || anchor.OccurredAt != nil {
		t.Errorf("expected empty anchor for no events, got %+v", anchor)
	}
}
