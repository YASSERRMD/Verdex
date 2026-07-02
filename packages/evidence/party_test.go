package evidence_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/evidence"
	"github.com/YASSERRMD/verdex/packages/segmentation"
)

func TestAttributeParty(t *testing.T) {
	tests := []struct {
		name string
		seg  segmentation.Segment
		want evidence.PartyRole
	}{
		{
			name: "first party from speaker label",
			seg: segmentation.Segment{
				SpeakerLabel: "Plaintiff",
				Text:         "The plaintiff filed the motion on Monday.",
			},
			want: evidence.PartyFirst,
		},
		{
			name: "second party from speaker label",
			seg: segmentation.Segment{
				SpeakerLabel: "Defendant",
				Text:         "The defendant denies all allegations.",
			},
			want: evidence.PartySecond,
		},
		{
			name: "first party from explicit marker",
			seg: segmentation.Segment{
				Text: "Counsel for the petitioner argued that the deadline was missed.",
			},
			want: evidence.PartyFirst,
		},
		{
			name: "second party from explicit marker",
			seg: segmentation.Segment{
				Text: "On behalf of the respondent, we deny the claim in full.",
			},
			want: evidence.PartySecond,
		},
		{
			name: "prosecution maps to first party",
			seg: segmentation.Segment{
				SpeakerLabel: "prosecution",
				Text:         "The state presented its opening statement.",
			},
			want: evidence.PartyFirst,
		},
		{
			name: "accused maps to second party",
			seg: segmentation.Segment{
				SpeakerLabel: "accused",
				Text:         "I was not present at the scene.",
			},
			want: evidence.PartySecond,
		},
		{
			name: "unattributed when no signal present",
			seg: segmentation.Segment{
				Text: "The hearing was adjourned to next week.",
			},
			want: evidence.PartyUnattributed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evidence.AttributeParty(tt.seg)
			if got != tt.want {
				t.Errorf("AttributeParty() = %q, want %q", got, tt.want)
			}
		})
	}
}
