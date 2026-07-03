package reasoningeval_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/reasoningeval"
)

func TestInMemoryStore_SaveAndListScoresFiltered(t *testing.T) {
	store := reasoningeval.NewInMemoryStore()
	ctx := context.Background()

	scores := []reasoningeval.QualityScore{
		{CaseID: "c1", RunID: "v1", JurisdictionCode: "AE-DXB", Overall: 0.9, ScoredAt: time.Now().Add(-2 * time.Hour)},
		{CaseID: "c2", RunID: "v1", JurisdictionCode: "US-NY", Overall: 0.8, ScoredAt: time.Now().Add(-1 * time.Hour)},
		{CaseID: "c3", RunID: "v2", JurisdictionCode: "AE-DXB", Overall: 0.7, ScoredAt: time.Now()},
	}
	for _, s := range scores {
		if err := store.SaveScore(ctx, s); err != nil {
			t.Fatalf("SaveScore() error = %v", err)
		}
	}

	all, err := store.ListScores(ctx, "", "")
	if err != nil {
		t.Fatalf("ListScores() error = %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("len(all) = %d, want 3", len(all))
	}
	// Most recent first.
	if all[0].CaseID != "c3" {
		t.Errorf("all[0].CaseID = %q, want c3 (most recent)", all[0].CaseID)
	}

	byRun, err := store.ListScores(ctx, "v1", "")
	if err != nil {
		t.Fatalf("ListScores(v1) error = %v", err)
	}
	if len(byRun) != 2 {
		t.Fatalf("len(byRun) = %d, want 2", len(byRun))
	}

	byJurisdiction, err := store.ListScores(ctx, "", "AE-DXB")
	if err != nil {
		t.Fatalf("ListScores(AE-DXB) error = %v", err)
	}
	if len(byJurisdiction) != 2 {
		t.Fatalf("len(byJurisdiction) = %d, want 2", len(byJurisdiction))
	}

	byBoth, err := store.ListScores(ctx, "v2", "AE-DXB")
	if err != nil {
		t.Fatalf("ListScores(v2, AE-DXB) error = %v", err)
	}
	if len(byBoth) != 1 || byBoth[0].CaseID != "c3" {
		t.Fatalf("byBoth = %+v, want exactly [c3]", byBoth)
	}
}

func TestInMemoryStore_SaveReviewRejectsInvalid(t *testing.T) {
	store := reasoningeval.NewInMemoryStore()
	ctx := context.Background()

	err := store.SaveReview(ctx, reasoningeval.ExpertReview{ReviewID: "r1"}) // missing CaseID/ReviewerID
	if err == nil {
		t.Fatal("SaveReview() with missing required fields: want error, got nil")
	}
}

func TestInMemoryStore_SaveAndGetReview(t *testing.T) {
	store := reasoningeval.NewInMemoryStore()
	ctx := context.Background()

	review := reasoningeval.ExpertReview{
		ReviewID:   "r1",
		CaseID:     testCaseID,
		ReviewerID: "judge-1",
		Score:      0.75,
		Comments:   "Solid grounding, thin on issue 2.",
		ReviewedAt: time.Now(),
	}
	if err := store.SaveReview(ctx, review); err != nil {
		t.Fatalf("SaveReview() error = %v", err)
	}

	got, err := store.GetReview(ctx, "r1")
	if err != nil {
		t.Fatalf("GetReview() error = %v", err)
	}
	if got.Comments != review.Comments {
		t.Errorf("GetReview().Comments = %q, want %q", got.Comments, review.Comments)
	}

	_, err = store.GetReview(ctx, "does-not-exist")
	if !errors.Is(err, reasoningeval.ErrReviewNotFound) {
		t.Errorf("GetReview(missing) error = %v, want ErrReviewNotFound", err)
	}

	reviews, err := store.ListReviews(ctx, testCaseID)
	if err != nil {
		t.Fatalf("ListReviews() error = %v", err)
	}
	if len(reviews) != 1 {
		t.Fatalf("len(reviews) = %d, want 1", len(reviews))
	}
}

func TestInMemoryStore_SaveAndListAlerts(t *testing.T) {
	store := reasoningeval.NewInMemoryStore()
	ctx := context.Background()

	alert := reasoningeval.NewRegressionAlert("AE-DXB", reasoningeval.RegressionResult{
		BaselineRunID: "v1", CurrentRunID: "v2", BaselineAvg: 0.9, CurrentAvg: 0.5, Drop: 0.4, Regressed: true,
	})
	if err := store.SaveAlert(ctx, alert); err != nil {
		t.Fatalf("SaveAlert() error = %v", err)
	}

	alerts, err := store.ListAlerts(ctx)
	if err != nil {
		t.Fatalf("ListAlerts() error = %v", err)
	}
	if len(alerts) != 1 {
		t.Fatalf("len(alerts) = %d, want 1", len(alerts))
	}
}
