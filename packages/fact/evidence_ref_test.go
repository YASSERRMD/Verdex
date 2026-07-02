package fact_test

import (
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/evidence"
	"github.com/YASSERRMD/verdex/packages/fact"
)

func TestNewEvidenceRef(t *testing.T) {
	tests := []struct {
		name             string
		factID           string
		classification   evidence.Classification
		classificationID string
		wantErr          error
		wantClassID      string
	}{
		{
			name:             "valid ref with explicit classification id",
			factID:           "fact-1",
			classification:   evidence.Classification{SegmentID: "seg-1", Type: evidence.TypeWitnessStatement, Party: evidence.PartyFirst, Confidence: 0.7},
			classificationID: "class-1",
			wantClassID:      "class-1",
		},
		{
			name:           "falls back to segment id when classification id empty",
			factID:         "fact-1",
			classification: evidence.Classification{SegmentID: "seg-1", Confidence: 0.5},
			wantClassID:    "seg-1",
		},
		{
			name:           "empty fact id is invalid",
			factID:         "",
			classification: evidence.Classification{SegmentID: "seg-1"},
			wantErr:        fact.ErrClassificationInvalid,
		},
		{
			name:           "empty segment id is invalid",
			factID:         "fact-1",
			classification: evidence.Classification{SegmentID: ""},
			wantErr:        fact.ErrClassificationInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := fact.NewEvidenceRef(tt.factID, tt.classification, tt.classificationID)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ref.FactID != tt.factID {
				t.Errorf("expected FactID %q, got %q", tt.factID, ref.FactID)
			}
			if ref.ClassificationID != tt.wantClassID {
				t.Errorf("expected ClassificationID %q, got %q", tt.wantClassID, ref.ClassificationID)
			}
			if ref.EvidenceType != tt.classification.Type {
				t.Errorf("expected EvidenceType %v, got %v", tt.classification.Type, ref.EvidenceType)
			}
		})
	}
}

func TestEvidenceRef_IsTestimonialAndIsExhibit(t *testing.T) {
	testimonial := fact.EvidenceRef{EvidenceType: evidence.TypeWitnessStatement}
	if !testimonial.IsTestimonial() {
		t.Errorf("expected witness statement to be testimonial")
	}
	if testimonial.IsExhibit() {
		t.Errorf("expected witness statement to not be an exhibit")
	}

	documentary := fact.EvidenceRef{EvidenceType: evidence.TypeDocumentaryEvidence}
	if !documentary.IsExhibit() {
		t.Errorf("expected documentary evidence to be an exhibit")
	}

	physical := fact.EvidenceRef{EvidenceType: evidence.TypePhysicalExhibit}
	if !physical.IsExhibit() {
		t.Errorf("expected physical exhibit to be an exhibit")
	}

	other := fact.EvidenceRef{EvidenceType: evidence.TypeArgument}
	if other.IsTestimonial() || other.IsExhibit() {
		t.Errorf("expected argument evidence to be neither testimonial nor exhibit")
	}
}
