package reasoningeval

import "context"

// Dashboard is a stable, read-only facade over a Store exposing
// aggregated quality scores and trends, mirroring
// packages/knowledgeapi.KnowledgeAPI's own "thin, stable facade over
// lower-level stores" style. Callers building an actual HTTP surface
// (a future phase's concern, per this package's doc.go) wrap Dashboard
// the same way packages/knowledgeapi/http.go wraps KnowledgeAPI.
type Dashboard struct {
	store Store
}

// NewDashboard constructs a Dashboard backed by store.
func NewDashboard(store Store) *Dashboard {
	return &Dashboard{store: store}
}

// JurisdictionTrendRequest selects which QualityScores
// JurisdictionTrend aggregates.
type JurisdictionTrendRequest struct {
	// RunID restricts the aggregation to a single run, if non-empty.
	RunID string
	// JurisdictionCode restricts the aggregation to a single
	// jurisdiction, if non-empty. Empty means "all jurisdictions".
	JurisdictionCode string
}

// JurisdictionTrendResponse is Dashboard.JurisdictionTrend's result: one
// JurisdictionSummary per distinct jurisdiction found among the matching
// QualityScores.
type JurisdictionTrendResponse struct {
	Summaries map[string]JurisdictionSummary
}

// JurisdictionTrend returns per-jurisdiction quality aggregates.
//
// Requires ctx to carry an authenticated identity.User holding
// identity.PermViewCase (see RequireViewPermission) — checked first, so
// an unauthorized caller never triggers a store read.
func (d *Dashboard) JurisdictionTrend(ctx context.Context, req JurisdictionTrendRequest) (JurisdictionTrendResponse, error) {
	if err := RequireViewPermission(ctx); err != nil {
		return JurisdictionTrendResponse{}, err
	}
	scores, err := d.store.ListScores(ctx, req.RunID, req.JurisdictionCode)
	if err != nil {
		return JurisdictionTrendResponse{}, err
	}
	return JurisdictionTrendResponse{Summaries: AggregateByJurisdiction(scores)}, nil
}

// LegalFamilyTrendRequest selects which QualityScores
// LegalFamilyTrend aggregates.
type LegalFamilyTrendRequest struct {
	// RunID restricts the aggregation to a single run, if non-empty.
	RunID string
}

// LegalFamilyTrendResponse is Dashboard.LegalFamilyTrend's result.
type LegalFamilyTrendResponse struct {
	Summaries map[string]LegalFamilySummary
}

// LegalFamilyTrend returns per-legal-family quality aggregates.
//
// Requires ctx to carry an authenticated identity.User holding
// identity.PermViewCase, checked first via RequireViewPermission.
func (d *Dashboard) LegalFamilyTrend(ctx context.Context, req LegalFamilyTrendRequest) (LegalFamilyTrendResponse, error) {
	if err := RequireViewPermission(ctx); err != nil {
		return LegalFamilyTrendResponse{}, err
	}
	scores, err := d.store.ListScores(ctx, req.RunID, "")
	if err != nil {
		return LegalFamilyTrendResponse{}, err
	}
	return LegalFamilyTrendResponse{Summaries: AggregateByLegalFamily(scores)}, nil
}

// RecentAlerts returns every persisted Alert, most recent first.
//
// Requires ctx to carry an authenticated identity.User holding
// identity.PermViewCase, checked first via RequireViewPermission.
func (d *Dashboard) RecentAlerts(ctx context.Context) ([]Alert, error) {
	if err := RequireViewPermission(ctx); err != nil {
		return nil, err
	}
	return d.store.ListAlerts(ctx)
}

// CaseReviews returns every ExpertReview recorded for caseID, most recent
// first.
//
// Requires ctx to carry an authenticated identity.User holding
// identity.PermViewCase, checked first via RequireViewPermission.
func (d *Dashboard) CaseReviews(ctx context.Context, caseID string) ([]ExpertReview, error) {
	if err := RequireViewPermission(ctx); err != nil {
		return nil, err
	}
	if caseID == "" {
		return nil, ErrEmptyCaseID
	}
	return d.store.ListReviews(ctx, caseID)
}
