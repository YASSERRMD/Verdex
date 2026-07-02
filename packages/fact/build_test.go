package fact_test

import (
	"errors"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/evidence"
	"github.com/YASSERRMD/verdex/packages/fact"
	"github.com/YASSERRMD/verdex/packages/irac"
)

func TestBuildFactNode(t *testing.T) {
	createdAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name            string
		classification  evidence.Classification
		segmentText     string
		wantErr         error
		wantText        string
		wantConfidence  float64
		wantSpansLength int
	}{
		{
			name:            "valid classification builds a fact node",
			classification:  evidence.Classification{SegmentID: "seg-1", Type: evidence.TypeWitnessStatement, Confidence: 0.8},
			segmentText:     "The witness stated the light was red.",
			wantText:        "The witness stated the light was red.",
			wantConfidence:  0.8,
			wantSpansLength: 1,
		},
		{
			name:           "empty segment id is invalid",
			classification: evidence.Classification{SegmentID: "", Confidence: 0.5},
			segmentText:    "some text",
			wantErr:        fact.ErrClassificationInvalid,
		},
		{
			name:           "empty segment text is invalid",
			classification: evidence.Classification{SegmentID: "seg-1", Confidence: 0.5},
			segmentText:    "   ",
			wantErr:        fact.ErrClassificationInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := fact.BuildFactNode(tt.classification, tt.segmentText, fact.SourceSpan{Start: 0, End: 10}, "fact-1", "case-1", createdAt)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if node.Type != irac.NodeFact {
				t.Errorf("expected NodeFact type, got %v", node.Type)
			}
			if node.ID != "fact-1" || node.CaseID != "case-1" {
				t.Errorf("unexpected node shape: %+v", node)
			}
			if node.Text != tt.wantText {
				t.Errorf("expected text %q, got %q", tt.wantText, node.Text)
			}
			if node.Confidence != tt.wantConfidence {
				t.Errorf("expected confidence %v, got %v", tt.wantConfidence, node.Confidence)
			}
			if !node.Provenance.IsValid() {
				t.Errorf("expected valid provenance, got %+v", node.Provenance)
			}
			if len(node.Spans) != tt.wantSpansLength {
				t.Errorf("expected %d spans, got %d", tt.wantSpansLength, len(node.Spans))
			}
		})
	}
}
