package analytics

import (
	"context"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/accounting"
	"github.com/YASSERRMD/verdex/packages/caselifecycle"
	"github.com/YASSERRMD/verdex/packages/reasoningeval"
)

// Dashboard is a single, stable, tenant-scoped facade over caseload
// aggregation (Aggregator), reasoning-quality trends (QualityComposer),
// and usage/cost views (UsageComposer), mirroring
// packages/reasoningeval.Dashboard's and
// packages/knowledgeapi.KnowledgeAPI's "thin, stable facade" style.
// Callers building an HTTP surface on top of this package (a future
// phase's concern, same as reasoningeval.Dashboard's own doc comment)
// wrap Dashboard the same way packages/knowledgeapi/http.go wraps
// KnowledgeAPI.
type Dashboard struct {
	aggregator *Aggregator
	quality    *QualityComposer
	usage      *UsageComposer
}

// NewDashboard constructs a Dashboard from its three components. Any
// of quality/usage may be nil if the caller does not wire up
// reasoning-quality or usage/cost views (Caseload still works); calling
// QualityTrend or UsageView on a Dashboard built without the
// corresponding composer returns ErrComposerNotConfigured.
func NewDashboard(aggregator *Aggregator, quality *QualityComposer, usage *UsageComposer) *Dashboard {
	return &Dashboard{aggregator: aggregator, quality: quality, usage: usage}
}

// Caseload returns the caseload Metrics for tenantID matching filters.
// Delegates entirely to Aggregator.Aggregate, including its access
// check.
func (d *Dashboard) Caseload(ctx context.Context, tenantID uuid.UUID, filters Filters) (*Metrics, error) {
	return d.aggregator.Aggregate(ctx, tenantID, filters)
}

// QualityTrend returns the reasoning-quality trend for runID.
// Delegates entirely to QualityComposer.QualityTrend, including the
// access check enforced by the underlying reasoningeval.Dashboard.
//
// Returns ErrComposerNotConfigured if this Dashboard was built without
// a QualityComposer.
func (d *Dashboard) QualityTrend(ctx context.Context, runID string) (QualityTrend, error) {
	if d.quality == nil {
		return QualityTrend{}, ErrComposerNotConfigured
	}
	return d.quality.QualityTrend(ctx, runID)
}

// UsageView returns the usage/cost dashboard for tenantID. Delegates
// entirely to UsageComposer.UsageView, including its costPermission
// access check.
//
// Returns ErrComposerNotConfigured if this Dashboard was built without
// a UsageComposer.
func (d *Dashboard) UsageView(ctx context.Context, tenantID uuid.UUID) (*accounting.TenantDashboard, error) {
	if d.usage == nil {
		return nil, ErrComposerNotConfigured
	}
	return d.usage.UsageView(ctx, tenantID)
}

// NewDashboardFromStores is a convenience constructor building a fully
// wired Dashboard directly from a caselifecycle.Repository (via
// Aggregator), a reasoningeval.Store (via reasoningeval.Dashboard and
// QualityComposer), and an accounting.InMemoryRepository usage-record
// fetcher (via accounting.DashboardAPI and UsageComposer — the
// concrete type is required here because accounting.DashboardAPI's
// fetcher parameter type is unexported). Kept separate from
// NewDashboard so callers that already hold constructed
// reasoningeval.Dashboard/accounting.DashboardAPI values (e.g. shared
// across multiple features, or backed by a non-in-memory store) can
// use NewDashboard directly instead of constructing fresh ones here.
func NewDashboardFromStores(
	repo caselifecycle.Repository,
	qualityStore reasoningeval.Store,
	usageFetcher *accounting.InMemoryRepository,
) *Dashboard {
	return NewDashboard(
		NewAggregator(repo),
		NewQualityComposer(reasoningeval.NewDashboard(qualityStore)),
		NewUsageComposer(accounting.NewDashboardAPI(usageFetcher)),
	)
}
