package evidence_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/evidence"
	"github.com/YASSERRMD/verdex/packages/segmentation"
)

func TestIsStatutoryCitation(t *testing.T) {
	tests := []struct {
		name string
		seg  segmentation.Segment
		want bool
	}{
		{
			name: "SegmentCitation is always a statutory citation",
			seg: segmentation.Segment{
				Type: segmentation.SegmentCitation,
				Text: "Section 302 IPC",
			},
			want: true,
		},
		{
			name: "USC citation shape in plain paragraph",
			seg: segmentation.Segment{
				Type: segmentation.SegmentParagraph,
				Text: "The claim arises under 42 U.S.C. § 1983.",
			},
			want: true,
		},
		{
			name: "case-law citation shape",
			seg: segmentation.Segment{
				Type: segmentation.SegmentParagraph,
				Text: "As held in Smith v. Jones, the duty of care extends further.",
			},
			want: true,
		},
		{
			name: "Article citation shape",
			seg: segmentation.Segment{
				Type: segmentation.SegmentParagraph,
				Text: "Article 21 guarantees the right to life and liberty.",
			},
			want: true,
		},
		{
			name: "plain paragraph without citation shape",
			seg: segmentation.Segment{
				Type: segmentation.SegmentParagraph,
				Text: "The witness described the events leading up to the incident.",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, conf := evidence.IsStatutoryCitation(tt.seg)
			if got != tt.want {
				t.Errorf("IsStatutoryCitation() = %v, want %v", got, tt.want)
			}
			if got && (conf <= 0 || conf > 1) {
				t.Errorf("IsStatutoryCitation() confidence = %v, want within (0, 1]", conf)
			}
			if !got && conf != 0 {
				t.Errorf("IsStatutoryCitation() confidence = %v, want 0 when false", conf)
			}
		})
	}
}
