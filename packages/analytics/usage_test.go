package analytics_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/accounting"
	"github.com/YASSERRMD/verdex/packages/analytics"
)

func seededUsageComposer(t *testing.T, tenantID uuid.UUID) *analytics.UsageComposer {
	t.Helper()
	repo := accounting.NewInMemoryRepository()
	ctx := context.Background()

	cost1 := 1.25
	cost2 := 2.50
	records := []accounting.UsageRecord{
		{
			ID: uuid.New(), TenantID: tenantID, ProviderID: "anthropic", TaskType: "reason",
			InputTokens: 100, OutputTokens: 50, TotalTokens: 150, CostUSD: &cost1, CreatedAt: time.Now(),
		},
		{
			ID: uuid.New(), TenantID: tenantID, ProviderID: "anthropic", TaskType: "embed",
			InputTokens: 200, OutputTokens: 0, TotalTokens: 200, CostUSD: &cost2, CreatedAt: time.Now(),
		},
	}
	for _, r := range records {
		if err := repo.SaveRecord(ctx, r); err != nil {
			t.Fatalf("SaveRecord() error = %v", err)
		}
	}

	return analytics.NewUsageComposer(accounting.NewDashboardAPI(repo))
}

func TestUsageComposer_UsageView_ComposesAccountingDashboard(t *testing.T) {
	tenantID := uuid.New()
	c := seededUsageComposer(t, tenantID)

	view, err := c.UsageView(judgeContext(), tenantID)
	if err != nil {
		t.Fatalf("UsageView() error = %v", err)
	}
	if view.TotalTokens != 350 {
		t.Errorf("TotalTokens = %d, want 350", view.TotalTokens)
	}
	if view.RequestCount != 2 {
		t.Errorf("RequestCount = %d, want 2", view.RequestCount)
	}
	wantCost := 3.75
	if diff := view.EstimatedCostUSD - wantCost; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("EstimatedCostUSD = %.4f, want %.4f", view.EstimatedCostUSD, wantCost)
	}
}

func TestUsageComposer_UsageView_RequiresAuth(t *testing.T) {
	tenantID := uuid.New()
	c := seededUsageComposer(t, tenantID)

	_, err := c.UsageView(unauthedContext(), tenantID)
	if !errors.Is(err, analytics.ErrUnauthenticated) {
		t.Errorf("UsageView(unauthed) error = %v, want ErrUnauthenticated", err)
	}
}

func TestUsageComposer_UsageView_ForbidsRolesWithoutAuditPermission(t *testing.T) {
	tenantID := uuid.New()
	c := seededUsageComposer(t, tenantID)

	// RoleAdvocate holds PermViewCase but not PermAuditRead, per
	// identity.PermissionMatrix: advocates can see caseload/quality
	// metrics but must not see cost/usage.
	_, err := c.UsageView(advocateContext(), tenantID)
	if !errors.Is(err, analytics.ErrForbidden) {
		t.Errorf("UsageView(advocate) error = %v, want ErrForbidden", err)
	}
}

func TestUsageComposer_UsageView_RequiresTenantID(t *testing.T) {
	c := seededUsageComposer(t, uuid.New())

	_, err := c.UsageView(judgeContext(), uuid.Nil)
	if !errors.Is(err, analytics.ErrEmptyTenantID) {
		t.Errorf("UsageView(nil tenant) error = %v, want ErrEmptyTenantID", err)
	}
}
