package pilot_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/pilot"
)

func TestEngine_AggregateQuality_ComputesRealAggregates(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	d := provisionAndActivate(t, te)
	admin := adminUser(te.tenantID)

	pc1 := assignTestCase(t, te, d.ID)
	pc2 := assignTestCase(t, te, d.ID)

	// Two feedback entries per case with different trust levels, so the
	// aggregate is a real computed mean, not just a pass-through of a
	// single value.
	mustSubmitFeedback(t, te, pc1.ID, pilot.TrustHigh, 0.9, 0.7)
	mustSubmitFeedback(t, te, pc2.ID, pilot.TrustLow, 0.3, 0.5)

	summary, err := te.engine.AggregateQuality(ctxWithUser(admin), te.tenantID, d.ID, nil)
	if err != nil {
		t.Fatalf("AggregateQuality: %v", err)
	}
	if summary.FeedbackCount != 2 {
		t.Fatalf("FeedbackCount = %d, want 2", summary.FeedbackCount)
	}

	wantOverall := ((0.9+0.7)/2 + (0.3+0.5)/2) / 2
	if !floatsClose(summary.AvgOverallFeedbackScore, wantOverall) {
		t.Fatalf("AvgOverallFeedbackScore = %v, want %v", summary.AvgOverallFeedbackScore, wantOverall)
	}

	wantTrust := float64(int(pilot.TrustHigh)+int(pilot.TrustLow)) / 2
	if !floatsClose(summary.AvgTrust, wantTrust) {
		t.Fatalf("AvgTrust = %v, want %v", summary.AvgTrust, wantTrust)
	}

	if summary.TrustDistribution[pilot.TrustHigh] != 1 || summary.TrustDistribution[pilot.TrustLow] != 1 {
		t.Fatalf("TrustDistribution = %+v, want exactly one TrustHigh and one TrustLow", summary.TrustDistribution)
	}

	groundingAvg, ok := summary.AvgPerDimension[pilot.DimensionGrounding]
	if !ok {
		t.Fatal("AvgPerDimension missing DimensionGrounding")
	}
	wantGrounding := (0.9 + 0.3) / 2
	if !floatsClose(groundingAvg, wantGrounding) {
		t.Fatalf("AvgPerDimension[grounding] = %v, want %v", groundingAvg, wantGrounding)
	}
}

func TestEngine_AggregateQuality_FoldsInAutomatedScores(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	d := provisionAndActivate(t, te)
	admin := adminUser(te.tenantID)
	pc := assignTestCase(t, te, d.ID)

	automated := []pilot.QualityScoreLike{
		pilot.ReasoningEvalQualityScoreAdapter{PilotCaseID: pc.ID, Overall: 0.6},
		pilot.ReasoningEvalQualityScoreAdapter{PilotCaseID: pc.ID, Overall: 0.8},
	}

	summary, err := te.engine.AggregateQuality(ctxWithUser(admin), te.tenantID, d.ID, automated)
	if err != nil {
		t.Fatalf("AggregateQuality: %v", err)
	}
	if summary.AutomatedScoreCount != 2 {
		t.Fatalf("AutomatedScoreCount = %d, want 2", summary.AutomatedScoreCount)
	}
	if !floatsClose(summary.AvgAutomatedOverall, 0.7) {
		t.Fatalf("AvgAutomatedOverall = %v, want 0.7", summary.AvgAutomatedOverall)
	}
	// Automated scores never overwrite the feedback-derived aggregate --
	// with zero FeedbackEntry records collected, AvgOverallFeedbackScore
	// stays at its zero value rather than picking up the automated mean.
	if summary.AvgOverallFeedbackScore != 0 {
		t.Fatalf("AvgOverallFeedbackScore = %v, want 0 (no feedback collected)", summary.AvgOverallFeedbackScore)
	}
}

func TestEngine_AggregateQuality_EmptyDeploymentIsZeroNotError(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	d := provisionAndActivate(t, te)
	admin := adminUser(te.tenantID)

	summary, err := te.engine.AggregateQuality(ctxWithUser(admin), te.tenantID, d.ID, nil)
	if err != nil {
		t.Fatalf("AggregateQuality: %v", err)
	}
	if summary.FeedbackCount != 0 || summary.AvgOverallFeedbackScore != 0 || summary.AvgTrust != 0 {
		t.Fatalf("expected all-zero summary for empty deployment, got %+v", summary)
	}
}

// mustSubmitFeedback submits a FeedbackEntry against pilotCaseID with
// the given trust and grounding/citation scores, failing the test on
// error.
func mustSubmitFeedback(t *testing.T, te *testEngine, pilotCaseID uuid.UUID, trust pilot.TrustRating, grounding, citation float64) pilot.FeedbackEntry {
	t.Helper()
	admin := adminUser(te.tenantID)
	entry, err := te.engine.SubmitFeedback(ctxWithUser(admin), te.tenantID, pilot.FeedbackEntry{
		PilotCaseID: pilotCaseID,
		Ratings: []pilot.DimensionRating{
			{Dimension: pilot.DimensionGrounding, Score: grounding},
			{Dimension: pilot.DimensionCitation, Score: citation},
		},
		Trust: trust,
	})
	if err != nil {
		t.Fatalf("SubmitFeedback: %v", err)
	}
	return entry
}
