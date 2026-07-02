package evidence_test

import (
	"errors"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/evidence"
)

func TestApplyOverride_TakesPrecedence(t *testing.T) {
	original := evidence.Classification{
		SegmentID:  "seg-1",
		Type:       evidence.TypeArgument,
		Party:      evidence.PartyUnattributed,
		Confidence: 0.55,
	}

	override := evidence.ManualOverride{
		SegmentID:  "seg-1",
		Type:       evidence.TypeWitnessStatement,
		Party:      evidence.PartyFirst,
		Reason:     "misclassified; is sworn testimony",
		ReviewedBy: "reviewer-1",
	}

	got, err := evidence.ApplyOverride(original, override)
	if err != nil {
		t.Fatalf("ApplyOverride() error = %v", err)
	}

	if got.Type != evidence.TypeWitnessStatement {
		t.Errorf("Type = %q, want %q", got.Type, evidence.TypeWitnessStatement)
	}
	if got.Party != evidence.PartyFirst {
		t.Errorf("Party = %q, want %q", got.Party, evidence.PartyFirst)
	}
	if got.Confidence != 1.0 {
		t.Errorf("Confidence = %v, want 1.0", got.Confidence)
	}
	if got.Override == nil {
		t.Fatal("Override is nil, want populated")
	}
	if got.Override.Previous == nil {
		t.Fatal("Override.Previous is nil, want the original classification preserved")
	}
	if got.Override.Previous.Type != evidence.TypeArgument {
		t.Errorf("Override.Previous.Type = %q, want %q (original determination preserved distinctly)", got.Override.Previous.Type, evidence.TypeArgument)
	}
	if got.Override.Previous.Confidence != 0.55 {
		t.Errorf("Override.Previous.Confidence = %v, want 0.55", got.Override.Previous.Confidence)
	}
	if got.Override.ReviewedAt.IsZero() {
		t.Error("Override.ReviewedAt is zero, want defaulted to now")
	}
}

func TestApplyOverride_PreservesExplicitReviewedAt(t *testing.T) {
	original := evidence.Classification{SegmentID: "seg-2", Type: evidence.TypeOther}
	when := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	got, err := evidence.ApplyOverride(original, evidence.ManualOverride{
		SegmentID:  "seg-2",
		Type:       evidence.TypeDocumentaryEvidence,
		ReviewedAt: when,
	})
	if err != nil {
		t.Fatalf("ApplyOverride() error = %v", err)
	}
	if !got.Override.ReviewedAt.Equal(when) {
		t.Errorf("Override.ReviewedAt = %v, want %v", got.Override.ReviewedAt, when)
	}
}

func TestApplyOverride_MismatchedSegmentID(t *testing.T) {
	original := evidence.Classification{SegmentID: "seg-1", Type: evidence.TypeArgument}
	override := evidence.ManualOverride{SegmentID: "seg-2", Type: evidence.TypeWitnessStatement}

	_, err := evidence.ApplyOverride(original, override)
	if !errors.Is(err, evidence.ErrInvalidOverride) {
		t.Fatalf("ApplyOverride() error = %v, want ErrInvalidOverride", err)
	}
}

func TestApplyOverride_InvalidOverride(t *testing.T) {
	tests := []struct {
		name     string
		override evidence.ManualOverride
	}{
		{"empty segment ID", evidence.ManualOverride{SegmentID: "", Type: evidence.TypeArgument}},
		{"unknown evidence type", evidence.ManualOverride{SegmentID: "seg-1", Type: evidence.EvidenceType("bogus")}},
		{"unknown party role", evidence.ManualOverride{SegmentID: "seg-1", Type: evidence.TypeArgument, Party: evidence.PartyRole("bogus")}},
	}

	original := evidence.Classification{SegmentID: "seg-1", Type: evidence.TypeOther}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := evidence.ApplyOverride(original, tt.override)
			if !errors.Is(err, evidence.ErrInvalidOverride) {
				t.Errorf("ApplyOverride() error = %v, want ErrInvalidOverride", err)
			}
		})
	}
}
