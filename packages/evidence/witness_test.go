package evidence_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/evidence"
	"github.com/YASSERRMD/verdex/packages/segmentation"
)

func TestIsWitnessStatement(t *testing.T) {
	tests := []struct {
		name string
		seg  segmentation.Segment
		want bool
	}{
		{
			name: "first-person statement with speaker label",
			seg: segmentation.Segment{
				Type:         segmentation.SegmentStatement,
				Text:         "I saw the defendant enter the store around 9pm.",
				SpeakerLabel: "witness-1",
			},
			want: true,
		},
		{
			name: "speaker-attributed statement without first-person marker",
			seg: segmentation.Segment{
				Type:         segmentation.SegmentStatement,
				Text:         "The store was busy that evening.",
				SpeakerLabel: "witness-2",
			},
			want: true,
		},
		{
			name: "affidavit marker in plain paragraph",
			seg: segmentation.Segment{
				Type: segmentation.SegmentParagraph,
				Text: "Affidavit of Jane Roe, sworn before a notary public.",
			},
			want: true,
		},
		{
			name: "sworn oath language in plain paragraph",
			seg: segmentation.Segment{
				Type: segmentation.SegmentParagraph,
				Text: "I swear that to the best of my knowledge this account is true.",
			},
			want: true,
		},
		{
			name: "plain paragraph with no testimonial language",
			seg: segmentation.Segment{
				Type: segmentation.SegmentParagraph,
				Text: "The court convened at 10am on Tuesday.",
			},
			want: false,
		},
		{
			name: "citation segment is not testimony",
			seg: segmentation.Segment{
				Type: segmentation.SegmentCitation,
				Text: "Section 302 IPC",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, conf := evidence.IsWitnessStatement(tt.seg)
			if got != tt.want {
				t.Errorf("IsWitnessStatement() = %v, want %v", got, tt.want)
			}
			if got && (conf <= 0 || conf > 1) {
				t.Errorf("IsWitnessStatement() confidence = %v, want within (0, 1]", conf)
			}
			if !got && conf != 0 {
				t.Errorf("IsWitnessStatement() confidence = %v, want 0 when false", conf)
			}
		})
	}
}
