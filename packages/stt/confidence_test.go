package stt_test

import (
	"testing"

	"github.com/YASSERRMD/verdex/packages/stt"
)

func TestClampConfidence(t *testing.T) {
	tests := []struct {
		in   float64
		want float64
	}{
		{-1.5, 0},
		{0, 0},
		{0.5, 0.5},
		{1, 1},
		{1.5, 1},
	}
	for _, tt := range tests {
		if got := stt.ClampConfidence(tt.in); got != tt.want {
			t.Errorf("ClampConfidence(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestAverageConfidence(t *testing.T) {
	segments := []stt.TranscriptSegment{
		{Confidence: 0.5},
		{Confidence: 1.0},
		{Confidence: 0.5},
	}
	if got, want := stt.AverageConfidence(segments), 2.0/3.0; got != want {
		t.Errorf("AverageConfidence() = %v, want %v", got, want)
	}
	if got := stt.AverageConfidence(nil); got != 0 {
		t.Errorf("AverageConfidence(nil) = %v, want 0", got)
	}
}

func TestWeightedConfidence(t *testing.T) {
	segments := []stt.TranscriptSegment{
		{StartMS: 0, EndMS: 1000, Confidence: 1.0},
		{StartMS: 1000, EndMS: 2000, Confidence: 0.0},
	}
	// Equal weights (1000ms each) => average of 1.0 and 0.0 = 0.5.
	if got, want := stt.WeightedConfidence(segments), 0.5; got != want {
		t.Errorf("WeightedConfidence() = %v, want %v", got, want)
	}

	weighted := []stt.TranscriptSegment{
		{StartMS: 0, EndMS: 3000, Confidence: 1.0},
		{StartMS: 3000, EndMS: 4000, Confidence: 0.0},
	}
	// 3000ms at 1.0 + 1000ms at 0.0 => 3000/4000 = 0.75
	if got, want := stt.WeightedConfidence(weighted), 0.75; got != want {
		t.Errorf("WeightedConfidence() = %v, want %v", got, want)
	}
}

func TestWeightedConfidence_FallsBackWhenNoPositiveDuration(t *testing.T) {
	segments := []stt.TranscriptSegment{
		{StartMS: 100, EndMS: 100, Confidence: 0.4},
		{StartMS: 200, EndMS: 200, Confidence: 0.6},
	}
	if got, want := stt.WeightedConfidence(segments), 0.5; got != want {
		t.Errorf("WeightedConfidence() = %v, want %v", got, want)
	}
}

func TestLowConfidenceSegments(t *testing.T) {
	segments := []stt.TranscriptSegment{
		{Text: "a", Confidence: 0.9},
		{Text: "b", Confidence: 0.2},
		{Text: "c", Confidence: 0.5},
	}
	low := stt.LowConfidenceSegments(segments, 0.6)
	if len(low) != 2 {
		t.Fatalf("LowConfidenceSegments() returned %d segments, want 2", len(low))
	}
	if low[0].Text != "b" || low[1].Text != "c" {
		t.Errorf("LowConfidenceSegments() = %v, want [b, c]", low)
	}
}
