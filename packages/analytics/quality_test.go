package analytics_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/analytics"
	"github.com/YASSERRMD/verdex/packages/reasoningeval"
)

func seededQualityComposer(t *testing.T) *analytics.QualityComposer {
	t.Helper()
	store := reasoningeval.NewInMemoryStore()
	ctx := context.Background()

	scores := []reasoningeval.QualityScore{
		{CaseID: "c1", RunID: "v1", JurisdictionCode: "AE-DXB", LegalFamily: "civil_law", Overall: 0.9, ScoredAt: time.Now()},
		{CaseID: "c2", RunID: "v1", JurisdictionCode: "AE-DXB", LegalFamily: "civil_law", Overall: 0.7, ScoredAt: time.Now()},
		{CaseID: "c3", RunID: "v1", JurisdictionCode: "US-NY", LegalFamily: "common_law", Overall: 0.6, ScoredAt: time.Now()},
	}
	for _, s := range scores {
		if err := store.SaveScore(ctx, s); err != nil {
			t.Fatalf("SaveScore() error = %v", err)
		}
	}

	return analytics.NewQualityComposer(reasoningeval.NewDashboard(store))
}

func TestQualityComposer_QualityTrend_ComposesReasoningEvalDashboard(t *testing.T) {
	c := seededQualityComposer(t)

	trend, err := c.QualityTrend(advocateContext(), "v1")
	if err != nil {
		t.Fatalf("QualityTrend() error = %v", err)
	}
	if len(trend.Points) != 2 {
		t.Fatalf("len(Points) = %d, want 2", len(trend.Points))
	}

	// Sorted by JurisdictionCode ascending.
	if trend.Points[0].JurisdictionCode != "AE-DXB" {
		t.Errorf("Points[0].JurisdictionCode = %q, want AE-DXB", trend.Points[0].JurisdictionCode)
	}
	if trend.Points[0].Count != 2 {
		t.Errorf("AE-DXB Count = %d, want 2", trend.Points[0].Count)
	}
	wantAvg := (0.9 + 0.7) / 2
	if diff := trend.Points[0].AvgOverall - wantAvg; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("AE-DXB AvgOverall = %.4f, want %.4f", trend.Points[0].AvgOverall, wantAvg)
	}

	if trend.Points[1].JurisdictionCode != "US-NY" {
		t.Errorf("Points[1].JurisdictionCode = %q, want US-NY", trend.Points[1].JurisdictionCode)
	}
	if trend.Points[1].Count != 1 {
		t.Errorf("US-NY Count = %d, want 1", trend.Points[1].Count)
	}
}

func TestQualityComposer_QualityTrend_RequiresAuth(t *testing.T) {
	c := seededQualityComposer(t)

	_, err := c.QualityTrend(unauthedContext(), "v1")
	if !errors.Is(err, reasoningeval.ErrUnauthenticated) {
		t.Errorf("QualityTrend(unauthed) error = %v, want reasoningeval.ErrUnauthenticated", err)
	}
}

func TestQualityComposer_QualityTrend_UnknownRunReturnsEmpty(t *testing.T) {
	c := seededQualityComposer(t)

	trend, err := c.QualityTrend(advocateContext(), "does-not-exist")
	if err != nil {
		t.Fatalf("QualityTrend() error = %v", err)
	}
	if len(trend.Points) != 0 {
		t.Errorf("len(Points) = %d, want 0", len(trend.Points))
	}
}
