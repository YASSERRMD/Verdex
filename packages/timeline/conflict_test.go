package timeline

import (
	"testing"
	"time"
)

func TestDetectConflicts(t *testing.T) {
	tests := []struct {
		name          string
		facts         []PartyFact
		wantConflicts int
	}{
		{
			name: "contradictory same subject different parties",
			facts: []PartyFact{
				{ID: "f1", PartyID: "p1", SegmentID: "s1", Text: "The tenant did not pay rent in March.", Subject: "rent-payment"},
				{ID: "f2", PartyID: "p2", SegmentID: "s2", Text: "The tenant paid rent in March.", Subject: "rent-payment"},
			},
			wantConflicts: 1,
		},
		{
			name: "same party, no conflict even if contradictory",
			facts: []PartyFact{
				{ID: "f1", PartyID: "p1", SegmentID: "s1", Text: "The tenant did not pay rent.", Subject: "rent-payment"},
				{ID: "f2", PartyID: "p1", SegmentID: "s2", Text: "The tenant paid rent.", Subject: "rent-payment"},
			},
			wantConflicts: 0,
		},
		{
			name: "different subject, no conflict",
			facts: []PartyFact{
				{ID: "f1", PartyID: "p1", SegmentID: "s1", Text: "The tenant did not pay rent.", Subject: "rent-payment"},
				{ID: "f2", PartyID: "p2", SegmentID: "s2", Text: "The landlord gave notice.", Subject: "notice"},
			},
			wantConflicts: 0,
		},
		{
			name: "no contradictory keywords, no conflict",
			facts: []PartyFact{
				{ID: "f1", PartyID: "p1", SegmentID: "s1", Text: "The tenant lived in unit 4B.", Subject: "residence"},
				{ID: "f2", PartyID: "p2", SegmentID: "s2", Text: "The tenant lived in unit 4B.", Subject: "residence"},
			},
			wantConflicts: 0,
		},
		{
			name: "empty subject excluded",
			facts: []PartyFact{
				{ID: "f1", PartyID: "p1", SegmentID: "s1", Text: "The tenant did not pay rent.", Subject: ""},
				{ID: "f2", PartyID: "p2", SegmentID: "s2", Text: "The tenant paid rent.", Subject: ""},
			},
			wantConflicts: 0,
		},
		{
			name: "denied vs admitted",
			facts: []PartyFact{
				{ID: "f1", PartyID: "p1", SegmentID: "s1", Text: "The defendant denied entering the property.", Subject: "trespass"},
				{ID: "f2", PartyID: "p2", SegmentID: "s2", Text: "The defendant admitted entering the property.", Subject: "trespass"},
			},
			wantConflicts: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conflicts := DetectConflicts(tt.facts, nil)
			if len(conflicts) != tt.wantConflicts {
				t.Errorf("DetectConflicts() = %d conflicts, want %d: %+v", len(conflicts), tt.wantConflicts, conflicts)
			}
		})
	}
}

func TestDetectConflicts_DateGate(t *testing.T) {
	facts := []PartyFact{
		{ID: "f1", PartyID: "p1", SegmentID: "s1", Text: "The tenant did not pay rent.", Subject: "rent-payment"},
		{ID: "f2", PartyID: "p2", SegmentID: "s2", Text: "The tenant paid rent.", Subject: "rent-payment"},
	}

	d1 := time.Date(2024, time.March, 1, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2024, time.April, 1, 0, 0, 0, 0, time.UTC)

	// Different dates: gated out.
	gate := EventsSameOrOverlappingDate(map[string]*time.Time{"s1": &d1, "s2": &d2})
	if got := DetectConflicts(facts, gate); len(got) != 0 {
		t.Errorf("DetectConflicts() with differing dates = %d, want 0", len(got))
	}

	// Same date: allowed through.
	gate = EventsSameOrOverlappingDate(map[string]*time.Time{"s1": &d1, "s2": &d1})
	if got := DetectConflicts(facts, gate); len(got) != 1 {
		t.Errorf("DetectConflicts() with same date = %d, want 1", len(got))
	}
}

func TestDetectConflicts_Deterministic(t *testing.T) {
	facts := []PartyFact{
		{ID: "f1", PartyID: "p1", SegmentID: "s1", Text: "The tenant did not pay rent.", Subject: "rent-payment"},
		{ID: "f2", PartyID: "p2", SegmentID: "s2", Text: "The tenant paid rent.", Subject: "rent-payment"},
	}

	c1 := DetectConflicts(facts, nil)
	c2 := DetectConflicts(facts, nil)

	if len(c1) != 1 || len(c2) != 1 {
		t.Fatalf("expected 1 conflict each run, got %d and %d", len(c1), len(c2))
	}
	if c1[0].FactAID != "f1" || c1[0].FactBID != "f2" {
		t.Errorf("FactAID/FactBID = %s/%s, want f1/f2 (input order)", c1[0].FactAID, c1[0].FactBID)
	}
	if c1[0] != c2[0] {
		t.Errorf("non-deterministic output: %+v vs %+v", c1[0], c2[0])
	}
}
