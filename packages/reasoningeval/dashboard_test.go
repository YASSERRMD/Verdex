package reasoningeval_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/reasoningeval"
)

func seededDashboard(t *testing.T) *reasoningeval.Dashboard {
	t.Helper()
	store := reasoningeval.NewInMemoryStore()
	ctx := context.Background()

	scores := []reasoningeval.QualityScore{
		{CaseID: "c1", RunID: "v1", JurisdictionCode: "AE-DXB", LegalFamily: "civil_law", Overall: 0.9, ScoredAt: time.Now()},
		{CaseID: "c2", RunID: "v1", JurisdictionCode: "US-NY", LegalFamily: "common_law", Overall: 0.7, ScoredAt: time.Now()},
	}
	for _, s := range scores {
		if err := store.SaveScore(ctx, s); err != nil {
			t.Fatalf("SaveScore() error = %v", err)
		}
	}

	review := reasoningeval.ExpertReview{
		ReviewID: "r1", CaseID: testCaseID, ReviewerID: "judge-1", Score: 0.8, ReviewedAt: time.Now(),
	}
	if err := store.SaveReview(ctx, review); err != nil {
		t.Fatalf("SaveReview() error = %v", err)
	}

	return reasoningeval.NewDashboard(store)
}

func TestDashboard_JurisdictionTrend_RequiresAuth(t *testing.T) {
	d := seededDashboard(t)
	_, err := d.JurisdictionTrend(unauthedContext(), reasoningeval.JurisdictionTrendRequest{})
	if !errors.Is(err, reasoningeval.ErrUnauthenticated) {
		t.Errorf("JurisdictionTrend(unauthed) error = %v, want ErrUnauthenticated", err)
	}
}

func TestDashboard_JurisdictionTrend_ReturnsAggregates(t *testing.T) {
	d := seededDashboard(t)
	resp, err := d.JurisdictionTrend(authedContext(), reasoningeval.JurisdictionTrendRequest{})
	if err != nil {
		t.Fatalf("JurisdictionTrend() error = %v", err)
	}
	if len(resp.Summaries) != 2 {
		t.Fatalf("len(Summaries) = %d, want 2", len(resp.Summaries))
	}
	if resp.Summaries["AE-DXB"].AvgOverall != 0.9 {
		t.Errorf("AE-DXB AvgOverall = %.4f, want 0.9", resp.Summaries["AE-DXB"].AvgOverall)
	}
}

func TestDashboard_LegalFamilyTrend_ReturnsAggregates(t *testing.T) {
	d := seededDashboard(t)
	resp, err := d.LegalFamilyTrend(authedContext(), reasoningeval.LegalFamilyTrendRequest{})
	if err != nil {
		t.Fatalf("LegalFamilyTrend() error = %v", err)
	}
	if len(resp.Summaries) != 2 {
		t.Fatalf("len(Summaries) = %d, want 2", len(resp.Summaries))
	}
}

func TestDashboard_RecentAlerts_RequiresAuth(t *testing.T) {
	d := seededDashboard(t)
	_, err := d.RecentAlerts(unauthedContext())
	if !errors.Is(err, reasoningeval.ErrUnauthenticated) {
		t.Errorf("RecentAlerts(unauthed) error = %v, want ErrUnauthenticated", err)
	}
}

func TestDashboard_CaseReviews_ReturnsMatchingReviews(t *testing.T) {
	d := seededDashboard(t)
	reviews, err := d.CaseReviews(authedContext(), testCaseID)
	if err != nil {
		t.Fatalf("CaseReviews() error = %v", err)
	}
	if len(reviews) != 1 {
		t.Fatalf("len(reviews) = %d, want 1", len(reviews))
	}
}

func TestDashboard_CaseReviews_RequiresCaseID(t *testing.T) {
	d := seededDashboard(t)
	_, err := d.CaseReviews(authedContext(), "")
	if !errors.Is(err, reasoningeval.ErrEmptyCaseID) {
		t.Errorf("CaseReviews(empty) error = %v, want ErrEmptyCaseID", err)
	}
}
