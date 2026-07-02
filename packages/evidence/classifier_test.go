package evidence_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/evidence"
	"github.com/YASSERRMD/verdex/packages/segmentation"
)

func TestRuleBasedClassifier_Classify_TypeByPattern(t *testing.T) {
	tests := []struct {
		name     string
		seg      segmentation.Segment
		wantType evidence.EvidenceType
	}{
		{
			name: "witness statement from SegmentStatement with first-person testimony",
			seg: segmentation.Segment{
				ID:           "seg-1",
				Type:         segmentation.SegmentStatement,
				Text:         "I saw the defendant leave the building at midnight.",
				SpeakerLabel: "witness-1",
			},
			wantType: evidence.TypeWitnessStatement,
		},
		{
			name: "documentary evidence from SegmentExhibit",
			seg: segmentation.Segment{
				ID:   "seg-2",
				Type: segmentation.SegmentExhibit,
				Text: "Exhibit A: the signed lease agreement.",
			},
			wantType: evidence.TypeDocumentaryEvidence,
		},
		{
			name: "statutory citation from SegmentCitation",
			seg: segmentation.Segment{
				ID:   "seg-3",
				Type: segmentation.SegmentCitation,
				Text: "Section 302 IPC applies to this offense.",
			},
			wantType: evidence.TypeStatutoryCitation,
		},
		{
			name: "argument from advocacy language",
			seg: segmentation.Segment{
				ID:   "seg-4",
				Type: segmentation.SegmentParagraph,
				Text: "We submit that the contract was validly terminated.",
			},
			wantType: evidence.TypeArgument,
		},
		{
			name: "physical exhibit reference",
			seg: segmentation.Segment{
				ID:   "seg-5",
				Type: segmentation.SegmentExhibit,
				Text: "Exhibit B: the recovered weapon used in the assault.",
			},
			wantType: evidence.TypePhysicalExhibit,
		},
		{
			name: "other for unremarkable paragraph",
			seg: segmentation.Segment{
				ID:   "seg-6",
				Type: segmentation.SegmentParagraph,
				Text: "The hearing was adjourned to the following week.",
			},
			wantType: evidence.TypeOther,
		},
	}

	c := evidence.NewRuleBasedClassifier()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.Classify(context.Background(), tt.seg)
			if err != nil {
				t.Fatalf("Classify() error = %v", err)
			}
			if got.Type != tt.wantType {
				t.Errorf("Classify() Type = %q, want %q", got.Type, tt.wantType)
			}
			if got.SegmentID != tt.seg.ID {
				t.Errorf("Classify() SegmentID = %q, want %q", got.SegmentID, tt.seg.ID)
			}
			if got.Confidence < 0 || got.Confidence > 1 {
				t.Errorf("Classify() Confidence = %v, want within [0, 1]", got.Confidence)
			}
		})
	}
}

func TestRuleBasedClassifier_Classify_EmptyInput(t *testing.T) {
	c := evidence.NewRuleBasedClassifier()

	_, err := c.Classify(context.Background(), segmentation.Segment{ID: "seg-empty", Text: "   "})
	if !errors.Is(err, evidence.ErrEmptyInput) {
		t.Fatalf("Classify() error = %v, want ErrEmptyInput", err)
	}
}

func TestRuleBasedClassifier_Classify_ConfidenceBounds(t *testing.T) {
	c := evidence.NewRuleBasedClassifier()
	segs := []segmentation.Segment{
		{ID: "a", Type: segmentation.SegmentStatement, Text: "I testify that I witnessed the collision."},
		{ID: "b", Type: segmentation.SegmentExhibit, Text: "Exhibit C: bank statement dated 2020."},
		{ID: "c", Type: segmentation.SegmentCitation, Text: "42 U.S.C. § 1983"},
		{ID: "d", Type: segmentation.SegmentParagraph, Text: "Random filler text with no markers at all."},
	}

	for _, seg := range segs {
		got, err := c.Classify(context.Background(), seg)
		if err != nil {
			t.Fatalf("Classify(%q) error = %v", seg.ID, err)
		}
		if got.Confidence < 0 || got.Confidence > 1 {
			t.Errorf("Classify(%q) Confidence = %v, want within [0, 1]", seg.ID, got.Confidence)
		}
	}
}
