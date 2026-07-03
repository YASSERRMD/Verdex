package analytics_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/analytics"
	"github.com/YASSERRMD/verdex/packages/caselifecycle"
)

func TestAggregator_Aggregate_RequiresAuth(t *testing.T) {
	repo := caselifecycle.NewInMemoryRepository()
	agg := analytics.NewAggregator(repo)

	_, err := agg.Aggregate(unauthedContext(), uuid.New(), analytics.Filters{})
	if !errors.Is(err, analytics.ErrUnauthenticated) {
		t.Errorf("Aggregate(unauthed) error = %v, want ErrUnauthenticated", err)
	}
}

func TestAggregator_Aggregate_RequiresTenantID(t *testing.T) {
	repo := caselifecycle.NewInMemoryRepository()
	agg := analytics.NewAggregator(repo)

	_, err := agg.Aggregate(advocateContext(), uuid.Nil, analytics.Filters{})
	if !errors.Is(err, analytics.ErrEmptyTenantID) {
		t.Errorf("Aggregate(nil tenant) error = %v, want ErrEmptyTenantID", err)
	}
}

func TestAggregator_Aggregate_ByStateCategoryJurisdiction(t *testing.T) {
	repo := caselifecycle.NewInMemoryRepository()
	tenantID := uuid.New()
	jurisdictionA := uuid.New()
	jurisdictionB := uuid.New()

	now := time.Now().UTC()
	seedCase(t, repo, tenantID, jurisdictionA, "contract", caselifecycle.StateActive, now)
	seedCase(t, repo, tenantID, jurisdictionA, "contract", caselifecycle.StateActive, now)
	seedCase(t, repo, tenantID, jurisdictionA, "tort", caselifecycle.StateUnderReview, now)
	seedCase(t, repo, tenantID, jurisdictionB, "contract", caselifecycle.StateClosed, now)

	agg := analytics.NewAggregator(repo)
	metrics, err := agg.Aggregate(advocateContext(), tenantID, analytics.Filters{})
	if err != nil {
		t.Fatalf("Aggregate() error = %v", err)
	}

	if metrics.TotalCases != 4 {
		t.Fatalf("TotalCases = %d, want 4", metrics.TotalCases)
	}
	if metrics.TenantID != tenantID {
		t.Errorf("TenantID = %v, want %v", metrics.TenantID, tenantID)
	}

	wantStates := map[caselifecycle.State]int{
		caselifecycle.StateActive:      2,
		caselifecycle.StateUnderReview: 1,
		caselifecycle.StateClosed:      1,
	}
	if len(metrics.ByState) != len(wantStates) {
		t.Fatalf("len(ByState) = %d, want %d", len(metrics.ByState), len(wantStates))
	}
	for _, sc := range metrics.ByState {
		if want := wantStates[sc.State]; sc.Count != want {
			t.Errorf("ByState[%s] = %d, want %d", sc.State, sc.Count, want)
		}
	}

	wantCategories := map[string]int{"contract": 3, "tort": 1}
	for _, cc := range metrics.ByCategory {
		if want := wantCategories[cc.CategoryID]; cc.Count != want {
			t.Errorf("ByCategory[%s] = %d, want %d", cc.CategoryID, cc.Count, want)
		}
	}

	if len(metrics.ByJurisdiction) != 2 {
		t.Fatalf("len(ByJurisdiction) = %d, want 2", len(metrics.ByJurisdiction))
	}
	for _, jb := range metrics.ByJurisdiction {
		switch jb.JurisdictionID {
		case jurisdictionA:
			if jb.Count != 3 {
				t.Errorf("jurisdictionA Count = %d, want 3", jb.Count)
			}
		case jurisdictionB:
			if jb.Count != 1 {
				t.Errorf("jurisdictionB Count = %d, want 1", jb.Count)
			}
		default:
			t.Errorf("unexpected jurisdiction %v in breakdown", jb.JurisdictionID)
		}
	}
}

func TestAggregator_Aggregate_CreatedTrendByDay(t *testing.T) {
	repo := caselifecycle.NewInMemoryRepository()
	tenantID := uuid.New()
	jurisdictionID := uuid.New()

	day1 := time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC)

	seedCase(t, repo, tenantID, jurisdictionID, "contract", caselifecycle.StateActive, day1)
	seedCase(t, repo, tenantID, jurisdictionID, "contract", caselifecycle.StateActive, day1)
	seedCase(t, repo, tenantID, jurisdictionID, "contract", caselifecycle.StateActive, day2)

	agg := analytics.NewAggregator(repo)
	metrics, err := agg.Aggregate(advocateContext(), tenantID, analytics.Filters{})
	if err != nil {
		t.Fatalf("Aggregate() error = %v", err)
	}

	if len(metrics.CreatedTrend) != 2 {
		t.Fatalf("len(CreatedTrend) = %d, want 2", len(metrics.CreatedTrend))
	}
	if metrics.CreatedTrend[0].Date != "2026-06-01" || metrics.CreatedTrend[0].Count != 2 {
		t.Errorf("CreatedTrend[0] = %+v, want {2026-06-01 2}", metrics.CreatedTrend[0])
	}
	if metrics.CreatedTrend[1].Date != "2026-06-02" || metrics.CreatedTrend[1].Count != 1 {
		t.Errorf("CreatedTrend[1] = %+v, want {2026-06-02 1}", metrics.CreatedTrend[1])
	}
}

func TestAggregator_Aggregate_FiltersByState(t *testing.T) {
	repo := caselifecycle.NewInMemoryRepository()
	tenantID := uuid.New()
	jurisdictionID := uuid.New()
	now := time.Now().UTC()

	seedCase(t, repo, tenantID, jurisdictionID, "contract", caselifecycle.StateActive, now)
	seedCase(t, repo, tenantID, jurisdictionID, "contract", caselifecycle.StateClosed, now)

	agg := analytics.NewAggregator(repo)
	metrics, err := agg.Aggregate(advocateContext(), tenantID, analytics.Filters{State: caselifecycle.StateActive})
	if err != nil {
		t.Fatalf("Aggregate() error = %v", err)
	}
	if metrics.TotalCases != 1 {
		t.Fatalf("TotalCases = %d, want 1", metrics.TotalCases)
	}
}

func TestAggregator_Aggregate_TenantIsolation(t *testing.T) {
	repo := caselifecycle.NewInMemoryRepository()
	tenantA := uuid.New()
	tenantB := uuid.New()
	jurisdictionID := uuid.New()
	now := time.Now().UTC()

	seedCase(t, repo, tenantA, jurisdictionID, "contract", caselifecycle.StateActive, now)
	seedCase(t, repo, tenantB, jurisdictionID, "contract", caselifecycle.StateActive, now)
	seedCase(t, repo, tenantB, jurisdictionID, "contract", caselifecycle.StateActive, now)

	agg := analytics.NewAggregator(repo)

	metricsA, err := agg.Aggregate(advocateContext(), tenantA, analytics.Filters{})
	if err != nil {
		t.Fatalf("Aggregate(tenantA) error = %v", err)
	}
	if metricsA.TotalCases != 1 {
		t.Errorf("tenantA TotalCases = %d, want 1", metricsA.TotalCases)
	}

	metricsB, err := agg.Aggregate(advocateContext(), tenantB, analytics.Filters{})
	if err != nil {
		t.Fatalf("Aggregate(tenantB) error = %v", err)
	}
	if metricsB.TotalCases != 2 {
		t.Errorf("tenantB TotalCases = %d, want 2", metricsB.TotalCases)
	}
}
