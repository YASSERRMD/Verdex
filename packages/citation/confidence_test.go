package citation_test

import (
	"math"
	"testing"

	"github.com/YASSERRMD/verdex/packages/citation"
)

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func TestScoreConfidenceWithVerifiedExact(t *testing.T) {
	c := citation.ScoreConfidenceWith(0.9, citation.CertaintyExact, citation.StatusVerified)
	if !almostEqual(c.Score, 0.9) {
		t.Errorf("Score = %v, want 0.9", c.Score)
	}
}

func TestScoreConfidenceWithHeuristicDiscounts(t *testing.T) {
	c := citation.ScoreConfidenceWith(1.0, citation.CertaintyHeuristic, citation.StatusVerified)
	if !almostEqual(c.Score, 0.6) {
		t.Errorf("Score = %v, want 0.6", c.Score)
	}
}

func TestScoreConfidenceWithFailedVerificationIsZero(t *testing.T) {
	cases := []citation.VerificationStatus{
		citation.StatusHallucinated,
		citation.StatusWrongCase,
		citation.StatusBroken,
	}
	for _, status := range cases {
		c := citation.ScoreConfidenceWith(1.0, citation.CertaintyExact, status)
		if c.Score != 0 {
			t.Errorf("Score for status %q = %v, want 0", status, c.Score)
		}
	}
}

func TestScoreConfidenceWithNoneCertaintyIsZero(t *testing.T) {
	c := citation.ScoreConfidenceWith(1.0, citation.CertaintyNone, citation.StatusVerified)
	if c.Score != 0 {
		t.Errorf("Score = %v, want 0", c.Score)
	}
}

func TestScoreConfidenceWithClampsOutOfRangeNodeConfidence(t *testing.T) {
	c := citation.ScoreConfidenceWith(5.0, citation.CertaintyExact, citation.StatusVerified)
	if c.Score != 1 {
		t.Errorf("Score = %v, want clamped to 1", c.Score)
	}

	c = citation.ScoreConfidenceWith(-5.0, citation.CertaintyExact, citation.StatusVerified)
	if c.Score != 0 {
		t.Errorf("Score = %v, want clamped to 0", c.Score)
	}
}

func TestScoreConfidenceCarriesComponents(t *testing.T) {
	result := citation.VerificationResult{Status: citation.StatusVerified}
	c := citation.ScoreConfidence(0.8, citation.CertaintyExact, result)
	if c.NodeConfidence != 0.8 {
		t.Errorf("NodeConfidence = %v, want 0.8", c.NodeConfidence)
	}
	if c.ResolutionCertainty != citation.CertaintyExact {
		t.Errorf("ResolutionCertainty = %q, want %q", c.ResolutionCertainty, citation.CertaintyExact)
	}
	if c.VerificationStatus != citation.StatusVerified {
		t.Errorf("VerificationStatus = %q, want %q", c.VerificationStatus, citation.StatusVerified)
	}
}

func TestScoreConfidenceWithUnknownCertaintyIsZero(t *testing.T) {
	c := citation.ScoreConfidenceWith(1.0, citation.Certainty("bogus"), citation.StatusVerified)
	if c.Score != 0 {
		t.Errorf("Score = %v, want 0 for unrecognized certainty", c.Score)
	}
}
