package segmentation_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/segmentation"
)

func TestSegmentationService_Segment_PlainText(t *testing.T) {
	svc := segmentation.NewSegmentationService()

	text := "STATEMENT OF FACTS\n\n" +
		"The petitioner filed the suit on 1 January 2020. Mr. Smith appeared as counsel. " +
		"Refer to Exhibit A for the signed agreement. The claim arises under Section 302 IPC."

	segs, err := svc.Segment(context.Background(), segmentation.SegmentRequest{
		DocumentID: "doc-1",
		Text:       text,
		Language:   "en",
	})
	if err != nil {
		t.Fatalf("Segment() unexpected error: %v", err)
	}
	if len(segs) == 0 {
		t.Fatal("Segment() returned no segments")
	}

	var sawHeading, sawExhibit, sawCitation bool
	for i, s := range segs {
		if s.ID == "" {
			t.Errorf("segment[%d].ID is empty", i)
		}
		if s.Language != "en" {
			t.Errorf("segment[%d].Language = %q, want \"en\"", i, s.Language)
		}
		if s.Confidence != segmentation.DefaultConfidence {
			t.Errorf("segment[%d].Confidence = %v, want default %v", i, s.Confidence, segmentation.DefaultConfidence)
		}
		switch s.Type {
		case segmentation.SegmentHeading:
			sawHeading = true
		case segmentation.SegmentExhibit:
			sawExhibit = true
		case segmentation.SegmentCitation:
			sawCitation = true
		}
	}
	if !sawHeading {
		t.Error("expected at least one SegmentHeading")
	}
	if !sawExhibit {
		t.Error("expected at least one SegmentExhibit")
	}
	if !sawCitation {
		t.Error("expected at least one SegmentCitation")
	}

	if err := segmentation.ValidateOrder(segs); err != nil {
		t.Errorf("ValidateOrder() = %v, want nil", err)
	}

	// IDs must be unique.
	seen := map[string]bool{}
	for _, s := range segs {
		if seen[s.ID] {
			t.Errorf("duplicate segment ID %q", s.ID)
		}
		seen[s.ID] = true
	}
}

func TestSegmentationService_Segment_Transcript(t *testing.T) {
	svc := segmentation.NewSegmentationService()

	turns := []segmentation.TranscriptTurn{
		{Speaker: "speaker-0", Text: "Please state your name for the record.", StartMS: 0, EndMS: 2500, Confidence: 0.9},
		{Speaker: "speaker-1", Text: "My name is Jane Doe.", StartMS: 2600, EndMS: 4000, Confidence: 0.85},
	}

	segs, err := svc.Segment(context.Background(), segmentation.SegmentRequest{
		DocumentID: "doc-2",
		Language:   "en",
		Turns:      turns,
	})
	if err != nil {
		t.Fatalf("Segment() unexpected error: %v", err)
	}
	if len(segs) != 2 {
		t.Fatalf("Segment() = %d segments, want 2", len(segs))
	}

	for i, s := range segs {
		if s.Type != segmentation.SegmentStatement {
			t.Errorf("segment[%d].Type = %v, want SegmentStatement", i, s.Type)
		}
		if s.SpeakerLabel != turns[i].Speaker {
			t.Errorf("segment[%d].SpeakerLabel = %q, want %q", i, s.SpeakerLabel, turns[i].Speaker)
		}
		if s.Confidence != turns[i].Confidence {
			t.Errorf("segment[%d].Confidence = %v, want %v (propagated from STT)", i, s.Confidence, turns[i].Confidence)
		}
	}

	if err := segmentation.ValidateOrder(segs); err != nil {
		t.Errorf("ValidateOrder() = %v, want nil", err)
	}

	// Span offsets must be strictly increasing and non-overlapping across
	// the synthetic joined source text.
	for i := 1; i < len(segs); i++ {
		if segs[i].Span.Start < segs[i-1].Span.End {
			t.Errorf("segment span overlap: segment[%d].Span=%+v segment[%d].Span=%+v", i-1, segs[i-1].Span, i, segs[i].Span)
		}
	}
}

func TestSegmentationService_Segment_EmptyInput(t *testing.T) {
	svc := segmentation.NewSegmentationService()

	tests := []struct {
		name string
		req  segmentation.SegmentRequest
	}{
		{"empty-text", segmentation.SegmentRequest{Text: ""}},
		{"whitespace-text", segmentation.SegmentRequest{Text: "   \n\t "}},
		{"nil-turns", segmentation.SegmentRequest{}},
		{"whitespace-only-turns", segmentation.SegmentRequest{Turns: []segmentation.TranscriptTurn{{Text: "   "}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Segment(context.Background(), tt.req)
			if !errors.Is(err, segmentation.ErrEmptyInput) {
				t.Errorf("Segment() error = %v, want ErrEmptyInput", err)
			}
		})
	}
}

func TestSegmentationService_Segment_DeterministicIDs(t *testing.T) {
	var counter int
	svc := &segmentation.SegmentationService{
		IDGenerator: func() string {
			counter++
			return "seg-" + string(rune('0'+counter))
		},
	}

	segs, err := svc.Segment(context.Background(), segmentation.SegmentRequest{
		Text: "First sentence. Second sentence.",
	})
	if err != nil {
		t.Fatalf("Segment() unexpected error: %v", err)
	}
	if len(segs) != 2 {
		t.Fatalf("Segment() = %d segments, want 2", len(segs))
	}
	if segs[0].ID != "seg-1" || segs[1].ID != "seg-2" {
		t.Errorf("IDs = %q, %q, want seg-1, seg-2", segs[0].ID, segs[1].ID)
	}
}

func TestNewRandomID_Unique(t *testing.T) {
	a := segmentation.NewRandomID()
	b := segmentation.NewRandomID()
	if a == "" || b == "" {
		t.Fatal("NewRandomID() returned empty string")
	}
	if a == b {
		t.Errorf("NewRandomID() returned duplicate IDs: %q", a)
	}
}
