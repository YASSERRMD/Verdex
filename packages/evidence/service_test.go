package evidence_test

import (
	"context"
	"testing"

	"github.com/YASSERRMD/verdex/packages/evidence"
	"github.com/YASSERRMD/verdex/packages/segmentation"
)

func TestEvidenceService_ClassifySegments_EndToEnd(t *testing.T) {
	svc := evidence.NewEvidenceService()

	segs := []segmentation.Segment{
		{
			ID:           "seg-1",
			Type:         segmentation.SegmentStatement,
			Text:         "I saw the defendant flee the scene.",
			SpeakerLabel: "witness-1",
		},
		{
			ID:   "seg-2",
			Type: segmentation.SegmentExhibit,
			Text: "Exhibit A: the signed lease agreement.",
		},
		{
			ID:   "seg-3",
			Type: segmentation.SegmentCitation,
			Text: "Section 302 IPC",
		},
	}

	results, err := svc.ClassifySegments(context.Background(), evidence.ClassifyRequest{Segments: segs})
	if err != nil {
		t.Fatalf("ClassifySegments() error = %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("ClassifySegments() returned %d results, want 3", len(results))
	}

	if results[0].Type != evidence.TypeWitnessStatement {
		t.Errorf("results[0].Type = %q, want %q", results[0].Type, evidence.TypeWitnessStatement)
	}
	if results[1].Type != evidence.TypeDocumentaryEvidence {
		t.Errorf("results[1].Type = %q, want %q", results[1].Type, evidence.TypeDocumentaryEvidence)
	}
	if results[2].Type != evidence.TypeStatutoryCitation {
		t.Errorf("results[2].Type = %q, want %q", results[2].Type, evidence.TypeStatutoryCitation)
	}

	// Every classification must be persisted to the store.
	for _, seg := range segs {
		stored, err := svc.Store.Get(context.Background(), seg.ID)
		if err != nil {
			t.Errorf("Store.Get(%q) error = %v", seg.ID, err)
		}
		if stored.SegmentID != seg.ID {
			t.Errorf("stored.SegmentID = %q, want %q", stored.SegmentID, seg.ID)
		}
	}
}

func TestEvidenceService_ClassifySegments_SkipsEmptySegments(t *testing.T) {
	svc := evidence.NewEvidenceService()

	segs := []segmentation.Segment{
		{ID: "seg-1", Type: segmentation.SegmentParagraph, Text: "   "},
		{ID: "seg-2", Type: segmentation.SegmentParagraph, Text: "The hearing was adjourned."},
	}

	results, err := svc.ClassifySegments(context.Background(), evidence.ClassifyRequest{Segments: segs})
	if err != nil {
		t.Fatalf("ClassifySegments() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("ClassifySegments() returned %d results, want 1 (empty segment skipped)", len(results))
	}
	if results[0].SegmentID != "seg-2" {
		t.Errorf("results[0].SegmentID = %q, want %q", results[0].SegmentID, "seg-2")
	}
}

func TestEvidenceService_ClassifySegments_AppliesOverride(t *testing.T) {
	svc := evidence.NewEvidenceService()

	segs := []segmentation.Segment{
		{ID: "seg-1", Type: segmentation.SegmentParagraph, Text: "The hearing was adjourned to next week."},
	}
	overrides := map[string]evidence.ManualOverride{
		"seg-1": {
			SegmentID:  "seg-1",
			Type:       evidence.TypeWitnessStatement,
			Party:      evidence.PartyFirst,
			ReviewedBy: "reviewer-1",
		},
	}

	results, err := svc.ClassifySegments(context.Background(), evidence.ClassifyRequest{Segments: segs, Overrides: overrides})
	if err != nil {
		t.Fatalf("ClassifySegments() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("ClassifySegments() returned %d results, want 1", len(results))
	}
	if results[0].Type != evidence.TypeWitnessStatement {
		t.Errorf("Type = %q, want %q (override should take precedence)", results[0].Type, evidence.TypeWitnessStatement)
	}
	if results[0].Override == nil {
		t.Fatal("Override is nil, want populated")
	}
	if results[0].Override.Previous == nil {
		t.Fatal("Override.Previous is nil, want original determination preserved")
	}

	stored, err := svc.Store.Get(context.Background(), "seg-1")
	if err != nil {
		t.Fatalf("Store.Get() error = %v", err)
	}
	if stored.Type != evidence.TypeWitnessStatement {
		t.Errorf("stored.Type = %q, want %q (persisted record should reflect override)", stored.Type, evidence.TypeWitnessStatement)
	}
}

func TestEvidenceService_ClassifySegment_Single(t *testing.T) {
	svc := evidence.NewEvidenceService()

	seg := segmentation.Segment{ID: "seg-1", Type: segmentation.SegmentCitation, Text: "Article 21"}
	got, err := svc.ClassifySegment(context.Background(), seg, nil)
	if err != nil {
		t.Fatalf("ClassifySegment() error = %v", err)
	}
	if got.Type != evidence.TypeStatutoryCitation {
		t.Errorf("Type = %q, want %q", got.Type, evidence.TypeStatutoryCitation)
	}
}
