package analytics

import (
	"context"
	"sort"

	"github.com/YASSERRMD/verdex/packages/reasoningeval"
)

// QualityTrendPoint is one jurisdiction's reasoning-quality summary
// within a QualityTrend, reshaping
// reasoningeval.JurisdictionSummary for display alongside this
// package's other per-jurisdiction breakdowns (JurisdictionBreakdown).
// The underlying scores are unchanged — this is a projection, not a
// recomputation.
type QualityTrendPoint struct {
	JurisdictionCode string                                  `json:"jurisdiction_code"`
	LegalFamily      string                                  `json:"legal_family,omitempty"`
	Count            int                                     `json:"count"`
	AvgOverall       float64                                 `json:"avg_overall"`
	AvgPerDimension  map[reasoningeval.DimensionName]float64 `json:"avg_per_dimension"`
}

// QualityTrend is the reshaped result of QualityTrend, sorted by
// JurisdictionCode ascending for stable display.
type QualityTrend struct {
	Points []QualityTrendPoint `json:"points"`
}

// QualityComposer produces reasoning-quality trend views by calling
// into an already-constructed reasoningeval.Dashboard, rather than
// reimplementing QualityScore aggregation. See doc.go, "What this
// package composes rather than duplicates".
type QualityComposer struct {
	dashboard *reasoningeval.Dashboard
}

// NewQualityComposer constructs a QualityComposer backed by dashboard.
func NewQualityComposer(dashboard *reasoningeval.Dashboard) *QualityComposer {
	return &QualityComposer{dashboard: dashboard}
}

// QualityTrend returns a per-jurisdiction reasoning-quality trend by
// delegating to reasoningeval.Dashboard.JurisdictionTrend and
// reshaping its map result into a stably sorted slice. Access control
// is enforced entirely by the delegated call: JurisdictionTrend itself
// requires ctx to carry identity.PermViewCase (see
// reasoningeval.RequireViewPermission), so this method does not
// duplicate that check — it surfaces reasoningeval's own
// ErrUnauthenticated/ErrForbidden unchanged if the caller lacks
// access.
func (c *QualityComposer) QualityTrend(ctx context.Context, runID string) (QualityTrend, error) {
	resp, err := c.dashboard.JurisdictionTrend(ctx, reasoningeval.JurisdictionTrendRequest{RunID: runID})
	if err != nil {
		return QualityTrend{}, err
	}

	points := make([]QualityTrendPoint, 0, len(resp.Summaries))
	for code, summary := range resp.Summaries {
		points = append(points, QualityTrendPoint{
			JurisdictionCode: code,
			LegalFamily:      summary.LegalFamily,
			Count:            summary.Count,
			AvgOverall:       summary.AvgOverall,
			AvgPerDimension:  summary.AvgPerDimension,
		})
	}
	sort.Slice(points, func(i, j int) bool { return points[i].JurisdictionCode < points[j].JurisdictionCode })

	return QualityTrend{Points: points}, nil
}
