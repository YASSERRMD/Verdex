package timeline

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/segmentation"
)

func TestPartyFact_Validate(t *testing.T) {
	tests := []struct {
		name    string
		fact    PartyFact
		wantErr error
	}{
		{
			name:    "valid",
			fact:    PartyFact{ID: "f1", PartyID: "p1", SegmentID: "s1", Text: "paid rent on time"},
			wantErr: nil,
		},
		{
			name:    "empty ID",
			fact:    PartyFact{ID: "", PartyID: "p1", SegmentID: "s1", Text: "text"},
			wantErr: ErrEmptyInput,
		},
		{
			name:    "empty party ID",
			fact:    PartyFact{ID: "f1", PartyID: "", SegmentID: "s1", Text: "text"},
			wantErr: ErrInvalidParty,
		},
		{
			name:    "empty segment ID",
			fact:    PartyFact{ID: "f1", PartyID: "p1", SegmentID: "", Text: "text"},
			wantErr: ErrEmptyInput,
		},
		{
			name:    "empty text",
			fact:    PartyFact{ID: "f1", PartyID: "p1", SegmentID: "s1", Text: "   "},
			wantErr: ErrEmptyInput,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fact.Validate()
			if err != tt.wantErr {
				t.Errorf("Validate() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewPartyFact(t *testing.T) {
	seg := segmentation.Segment{
		ID:   "seg-1",
		Text: "The tenant did not pay rent in March.",
		Span: segmentation.SourceSpan{Start: 10, End: 48},
	}

	fact := NewPartyFact("fact-1", "party-1", seg, "rent-payment")

	if fact.ID != "fact-1" {
		t.Errorf("ID = %q, want %q", fact.ID, "fact-1")
	}
	if fact.PartyID != "party-1" {
		t.Errorf("PartyID = %q, want %q", fact.PartyID, "party-1")
	}
	if fact.SegmentID != seg.ID {
		t.Errorf("SegmentID = %q, want %q", fact.SegmentID, seg.ID)
	}
	if fact.Text != seg.Text {
		t.Errorf("Text = %q, want %q", fact.Text, seg.Text)
	}
	if fact.Span != seg.Span {
		t.Errorf("Span = %+v, want %+v", fact.Span, seg.Span)
	}
	if fact.Subject != "rent-payment" {
		t.Errorf("Subject = %q, want %q", fact.Subject, "rent-payment")
	}

	if err := fact.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
}
