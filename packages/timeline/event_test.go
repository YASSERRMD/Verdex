package timeline

import (
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/segmentation"
)

func TestExtractDate(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		wantOK    bool
		wantYear  int
		wantMonth time.Month
		wantDay   int
		minConf   float64
	}{
		{
			name: "iso date", text: "The incident occurred on 2024-03-15 in the evening.",
			wantOK: true, wantYear: 2024, wantMonth: time.March, wantDay: 15, minConf: 0.9,
		},
		{
			name: "long form date", text: "The tenant vacated on March 15, 2024 without notice.",
			wantOK: true, wantYear: 2024, wantMonth: time.March, wantDay: 15, minConf: 0.85,
		},
		{
			name: "long form no comma", text: "Filed on July 4 2023 per the docket.",
			wantOK: true, wantYear: 2023, wantMonth: time.July, wantDay: 4, minConf: 0.85,
		},
		{
			name: "slash date", text: "Payment was due 03/15/2024 under the lease.",
			wantOK: true, wantYear: 2024, wantMonth: time.March, wantDay: 15, minConf: 0.5,
		},
		{
			name: "no date", text: "The tenant did not pay rent at all.",
			wantOK: false,
		},
		{
			name: "invalid iso date rejected, falls through", text: "Ref 2024-13-40 then March 15, 2024 happened.",
			wantOK: true, wantYear: 2024, wantMonth: time.March, wantDay: 15, minConf: 0.85,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, conf, ok := ExtractDate(tt.text)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if d.Year() != tt.wantYear || d.Month() != tt.wantMonth || d.Day() != tt.wantDay {
				t.Errorf("date = %v, want %d-%d-%d", d, tt.wantYear, tt.wantMonth, tt.wantDay)
			}
			if conf < tt.minConf {
				t.Errorf("confidence = %v, want >= %v", conf, tt.minConf)
			}
			if conf < 0 || conf > 1 {
				t.Errorf("confidence out of [0,1]: %v", conf)
			}
		})
	}
}

func TestExtractEvent(t *testing.T) {
	seg := segmentation.Segment{ID: "seg-1", Text: "On 2024-01-10 the landlord issued a notice to quit."}

	ev := ExtractEvent("evt-1", seg, "party-1")

	if ev.ID != "evt-1" {
		t.Errorf("ID = %q, want evt-1", ev.ID)
	}
	if ev.Description != seg.Text {
		t.Errorf("Description = %q, want %q", ev.Description, seg.Text)
	}
	if ev.SegmentID != seg.ID {
		t.Errorf("SegmentID = %q, want %q", ev.SegmentID, seg.ID)
	}
	if ev.PartyID != "party-1" {
		t.Errorf("PartyID = %q, want party-1", ev.PartyID)
	}
	if ev.OccurredAt == nil {
		t.Fatal("OccurredAt = nil, want non-nil")
	}
	if ev.OccurredAt.Year() != 2024 || ev.OccurredAt.Month() != time.January || ev.OccurredAt.Day() != 10 {
		t.Errorf("OccurredAt = %v, want 2024-01-10", ev.OccurredAt)
	}
	if ev.Confidence <= 0 {
		t.Errorf("Confidence = %v, want > 0", ev.Confidence)
	}
}

func TestExtractEvent_NoDate(t *testing.T) {
	seg := segmentation.Segment{ID: "seg-2", Text: "The parties dispute who breached the agreement."}

	ev := ExtractEvent("evt-2", seg, "")

	if ev.OccurredAt != nil {
		t.Errorf("OccurredAt = %v, want nil", ev.OccurredAt)
	}
	if ev.Confidence != 0 {
		t.Errorf("Confidence = %v, want 0", ev.Confidence)
	}
}
