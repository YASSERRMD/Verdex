package compliance

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// FrameworkSummary is one Framework's aggregate standing within a
// Dashboard: how many applicable controls it contributes and how many
// of those resolve to each ComplianceStatus.
type FrameworkSummary struct {
	// Framework is the framework this summary aggregates.
	Framework Framework `json:"framework"`

	// TotalControls is how many applicable controls are mapped to
	// Framework.
	TotalControls int `json:"total_controls"`

	// StatusCounts is how many of those controls resolve to each
	// ComplianceStatus, keyed by status.
	StatusCounts map[ComplianceStatus]int `json:"status_counts"`
}

// Dashboard is a compliance-mapping aggregation report for one tenant
// (task 8): counts by framework and status, mirroring
// packages/accessgovernance.Report / packages/analytics's aggregation
// shape. This is a report/query type, not a UI -- a caller renders it
// however its own presentation layer requires.
type Dashboard struct {
	// TenantID is the tenant this dashboard was generated for.
	TenantID uuid.UUID `json:"tenant_id"`

	// GeneratedAt is when this dashboard was computed.
	GeneratedAt time.Time `json:"generated_at"`

	// TotalControls is the total number of applicable controls across
	// every framework.
	TotalControls int `json:"total_controls"`

	// OverallStatusCounts is how many applicable controls resolve to
	// each ComplianceStatus, keyed by status, across every framework
	// combined.
	OverallStatusCounts map[ComplianceStatus]int `json:"overall_status_counts"`

	// ByFramework is one FrameworkSummary per distinct Framework
	// represented among the applicable controls, ordered by Framework
	// name for deterministic output.
	ByFramework []FrameworkSummary `json:"by_framework"`

	// RecentEvidenceCount is how many ControlEvidence records were
	// collected for this tenant within the trailing evidence window
	// (see BuildDashboard), giving a dashboard viewer a sense of
	// evidence-collection velocity, not just a static snapshot.
	RecentEvidenceCount int `json:"recent_evidence_count"`
}

// recentEvidenceWindow is how far back BuildDashboard looks when
// counting RecentEvidenceCount: 30 days, a reporting-cadence window
// consistent with packages/privacy's defaultSARWindow choice of a
// common, non-jurisdiction-specific baseline that a caller may
// override by post-processing the returned GapAnalysisReport/evidence
// directly if a different cadence is needed.
const recentEvidenceWindow = 30 * 24 * time.Hour

// BuildDashboard aggregates report (a GapAnalysisReport, typically
// fresh from Engine.RunGapAnalysis) and evidence (typically fresh from
// Engine.ListAllEvidence) into a Dashboard (task 8): counts by
// framework and status. This is a pure aggregation function over
// already-fetched data, mirroring how
// packages/accessgovernance.ExportReport operates on an
// already-assembled Report rather than re-querying storage itself.
func BuildDashboard(report GapAnalysisReport, evidence []ControlEvidence, now time.Time) Dashboard {
	frameworkOrder := make([]Framework, 0)
	summaries := make(map[Framework]*FrameworkSummary)
	overall := map[ComplianceStatus]int{
		StatusSatisfied:    0,
		StatusPartiallyMet: 0,
		StatusGap:          0,
	}

	for _, res := range report.Results {
		fw := res.Control.Framework
		summary, ok := summaries[fw]
		if !ok {
			summary = &FrameworkSummary{
				Framework: fw,
				StatusCounts: map[ComplianceStatus]int{
					StatusSatisfied:    0,
					StatusPartiallyMet: 0,
					StatusGap:          0,
				},
			}
			summaries[fw] = summary
			frameworkOrder = append(frameworkOrder, fw)
		}
		summary.TotalControls++
		summary.StatusCounts[res.Status]++
		overall[res.Status]++
	}

	sortFrameworks(frameworkOrder)
	byFramework := make([]FrameworkSummary, 0, len(frameworkOrder))
	for _, fw := range frameworkOrder {
		byFramework = append(byFramework, *summaries[fw])
	}

	cutoff := now.Add(-recentEvidenceWindow)
	recentCount := 0
	for _, e := range evidence {
		if e.CollectedAt.After(cutoff) && !e.CollectedAt.After(now) {
			recentCount++
		}
	}

	return Dashboard{
		TenantID:            report.TenantID,
		GeneratedAt:         now,
		TotalControls:       len(report.Results),
		OverallStatusCounts: overall,
		ByFramework:         byFramework,
		RecentEvidenceCount: recentCount,
	}
}

// sortFrameworks sorts fws in place, ascending lexicographically, for
// deterministic Dashboard.ByFramework ordering.
func sortFrameworks(fws []Framework) {
	for i := 1; i < len(fws); i++ {
		for j := i; j > 0 && fws[j-1] > fws[j]; j-- {
			fws[j-1], fws[j] = fws[j], fws[j-1]
		}
	}
}

// BuildDashboardReport runs RunGapAnalysis and ListAllEvidence for
// tenantID and aggregates the results into a Dashboard (task 8's
// engine-level entry point), requiring viewPermission and tenant match
// (enforced by the two calls it composes).
func (e *Engine) BuildDashboardReport(ctx context.Context, tenantID uuid.UUID) (Dashboard, error) {
	report, err := e.RunGapAnalysis(ctx, tenantID)
	if err != nil {
		return Dashboard{}, wrapf("BuildDashboardReport", err)
	}
	evidence, err := e.ListAllEvidence(ctx, tenantID)
	if err != nil {
		return Dashboard{}, wrapf("BuildDashboardReport", err)
	}
	return BuildDashboard(report, evidence, e.now()), nil
}
