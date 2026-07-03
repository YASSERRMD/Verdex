package analytics_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/accounting"
	"github.com/YASSERRMD/verdex/packages/analytics"
	"github.com/YASSERRMD/verdex/packages/caselifecycle"
	"github.com/YASSERRMD/verdex/packages/reasoningeval"
)

func TestNewDashboardFromStores_Caseload(t *testing.T) {
	caseRepo := caselifecycle.NewInMemoryRepository()
	tenantID := uuid.New()
	jurisdictionID := uuid.New()
	seedCase(t, caseRepo, tenantID, jurisdictionID, "contract", caselifecycle.StateActive, time.Now())

	usageRepo := accounting.NewInMemoryRepository()
	qualityStore := reasoningeval.NewInMemoryStore()

	dash := analytics.NewDashboardFromStores(caseRepo, qualityStore, usageRepo)

	metrics, err := dash.Caseload(advocateContext(), tenantID, analytics.Filters{})
	if err != nil {
		t.Fatalf("Caseload() error = %v", err)
	}
	if metrics.TotalCases != 1 {
		t.Errorf("TotalCases = %d, want 1", metrics.TotalCases)
	}
}

func TestNewDashboardFromStores_QualityTrend(t *testing.T) {
	caseRepo := caselifecycle.NewInMemoryRepository()
	usageRepo := accounting.NewInMemoryRepository()
	qualityStore := reasoningeval.NewInMemoryStore()
	if err := qualityStore.SaveScore(context.Background(), reasoningeval.QualityScore{
		CaseID: "c1", RunID: "v1", JurisdictionCode: "AE-DXB", Overall: 0.8, ScoredAt: time.Now(),
	}); err != nil {
		t.Fatalf("SaveScore() error = %v", err)
	}

	dash := analytics.NewDashboardFromStores(caseRepo, qualityStore, usageRepo)

	trend, err := dash.QualityTrend(advocateContext(), "v1")
	if err != nil {
		t.Fatalf("QualityTrend() error = %v", err)
	}
	if len(trend.Points) != 1 {
		t.Fatalf("len(Points) = %d, want 1", len(trend.Points))
	}
}

func TestNewDashboardFromStores_UsageView_RoleGated(t *testing.T) {
	caseRepo := caselifecycle.NewInMemoryRepository()
	usageRepo := accounting.NewInMemoryRepository()
	qualityStore := reasoningeval.NewInMemoryStore()
	tenantID := uuid.New()

	dash := analytics.NewDashboardFromStores(caseRepo, qualityStore, usageRepo)

	if _, err := dash.UsageView(judgeContext(), tenantID); err != nil {
		t.Errorf("UsageView(judge) error = %v, want nil", err)
	}
	if _, err := dash.UsageView(advocateContext(), tenantID); !errors.Is(err, analytics.ErrForbidden) {
		t.Errorf("UsageView(advocate) error = %v, want ErrForbidden", err)
	}
}

func TestDashboard_QualityTrend_ComposerNotConfigured(t *testing.T) {
	caseRepo := caselifecycle.NewInMemoryRepository()
	dash := analytics.NewDashboard(analytics.NewAggregator(caseRepo), nil, nil)

	_, err := dash.QualityTrend(advocateContext(), "v1")
	if !errors.Is(err, analytics.ErrComposerNotConfigured) {
		t.Errorf("QualityTrend() error = %v, want ErrComposerNotConfigured", err)
	}
}

func TestDashboard_UsageView_ComposerNotConfigured(t *testing.T) {
	caseRepo := caselifecycle.NewInMemoryRepository()
	dash := analytics.NewDashboard(analytics.NewAggregator(caseRepo), nil, nil)

	_, err := dash.UsageView(judgeContext(), uuid.New())
	if !errors.Is(err, analytics.ErrComposerNotConfigured) {
		t.Errorf("UsageView() error = %v, want ErrComposerNotConfigured", err)
	}
}
