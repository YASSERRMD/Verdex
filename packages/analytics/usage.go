package analytics

import (
	"context"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/accounting"
)

// UsageComposer produces usage/cost views by calling into an
// already-constructed accounting.DashboardAPI, rather than
// reimplementing token/cost aggregation. See doc.go, "What this
// package composes rather than duplicates".
//
// Unlike QualityComposer, accounting.DashboardAPI has no built-in
// permission check of its own (packages/accounting does not depend on
// packages/identity), so UsageView enforces costPermission itself
// before delegating.
type UsageComposer struct {
	dashboard *accounting.DashboardAPI
}

// NewUsageComposer constructs a UsageComposer backed by dashboard.
func NewUsageComposer(dashboard *accounting.DashboardAPI) *UsageComposer {
	return &UsageComposer{dashboard: dashboard}
}

// UsageView returns tenantID's token/cost dashboard by delegating to
// accounting.DashboardAPI.GetTenantDashboard unchanged.
//
// Requires ctx to carry an authenticated identity.User holding
// costPermission (identity.PermAuditRead) — checked first via
// RequireCostPermission, so an unauthorized caller never triggers a
// records scan. This is the analytics access-scoping this phase adds:
// only roles with audit/financial oversight may see spend, matching
// "add access-scoped dashboards ... only certain roles see cost/usage
// views" in the phase's task list.
func (c *UsageComposer) UsageView(ctx context.Context, tenantID uuid.UUID) (*accounting.TenantDashboard, error) {
	if err := RequireCostPermission(ctx); err != nil {
		return nil, err
	}
	if tenantID == uuid.Nil {
		return nil, ErrEmptyTenantID
	}
	return c.dashboard.GetTenantDashboard(ctx, tenantID)
}
