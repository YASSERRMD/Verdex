package evidence_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/evidence"
	"github.com/YASSERRMD/verdex/packages/segmentation"
)

func TestIsDocumentaryEvidence(t *testing.T) {
	tests := []struct {
		name string
		seg  segmentation.Segment
		want bool
	}{
		{
			name: "SegmentExhibit is documentary",
			seg: segmentation.Segment{
				Type: segmentation.SegmentExhibit,
				Text: "Exhibit A: the signed lease agreement.",
			},
			want: true,
		},
		{
			name: "document reference in plain paragraph",
			seg: segmentation.Segment{
				Type: segmentation.SegmentParagraph,
				Text: "The contract dated 12 March 2019 was marked and attached as evidence.",
			},
			want: true,
		},
		{
			name: "SegmentExhibit describing a physical object defers to physical exhibit",
			seg: segmentation.Segment{
				Type: segmentation.SegmentExhibit,
				Text: "Exhibit B: the recovered weapon used in the assault.",
			},
			want: false,
		},
		{
			name: "plain paragraph with no document reference",
			seg: segmentation.Segment{
				Type: segmentation.SegmentParagraph,
				Text: "The judge asked the parties to approach the bench.",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, conf := evidence.IsDocumentaryEvidence(tt.seg)
			if got != tt.want {
				t.Errorf("IsDocumentaryEvidence() = %v, want %v", got, tt.want)
			}
			if got && (conf <= 0 || conf > 1) {
				t.Errorf("IsDocumentaryEvidence() confidence = %v, want within (0, 1]", conf)
			}
		})
	}
}

func TestIsPhysicalExhibit(t *testing.T) {
	tests := []struct {
		name string
		seg  segmentation.Segment
		want bool
	}{
		{
			name: "weapon reference in exhibit segment",
			seg: segmentation.Segment{
				Type: segmentation.SegmentExhibit,
				Text: "Exhibit B: the recovered weapon used in the assault.",
			},
			want: true,
		},
		{
			name: "DNA sample reference in plain paragraph",
			seg: segmentation.Segment{
				Type: segmentation.SegmentParagraph,
				Text: "The lab confirmed the DNA sample matched the defendant.",
			},
			want: true,
		},
		{
			name: "document reference is not a physical exhibit",
			seg: segmentation.Segment{
				Type: segmentation.SegmentExhibit,
				Text: "Exhibit A: the signed lease agreement.",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, conf := evidence.IsPhysicalExhibit(tt.seg)
			if got != tt.want {
				t.Errorf("IsPhysicalExhibit() = %v, want %v", got, tt.want)
			}
			if got && (conf <= 0 || conf > 1) {
				t.Errorf("IsPhysicalExhibit() confidence = %v, want within (0, 1]", conf)
			}
		})
	}
}
