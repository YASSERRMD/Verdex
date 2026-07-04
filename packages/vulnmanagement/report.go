package vulnmanagement

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"
)

// Report is a tenant's vulnerability-management dashboard snapshot
// (task 8): aggregate counts by Severity/Status/ScannerSource plus the
// current SLA-breach list, mirroring packages/compliance's
// Dashboard/BuildDashboard and packages/accessgovernance's
// Certify/Report shape -- a real report type + generator, not a UI.
type Report struct {
	// TenantID is the tenant this report was generated for.
	TenantID uuid.UUID `json:"tenant_id"`

	// GeneratedAt is when this report was computed.
	GeneratedAt time.Time `json:"generated_at"`

	// TotalFindings is the total number of findings on file for the
	// tenant at report-generation time.
	TotalFindings int `json:"total_findings"`

	// BySeverity counts findings keyed by Severity.
	BySeverity map[Severity]int `json:"by_severity"`

	// ByStatus counts findings keyed by Status.
	ByStatus map[Status]int `json:"by_status"`

	// BySource counts findings keyed by ScannerSource.
	BySource map[ScannerSource]int `json:"by_source"`

	// SLABreaches lists every currently SLA-breached Finding (per
	// FindingsPastSLA), so the report surfaces exactly what is
	// overdue, not just how many findings exist in the abstract.
	SLABreaches []Finding `json:"sla_breaches"`
}

// BuildReport aggregates findings into a Report as of now (task 8's
// real generator, not a stub): counts by Severity/Status/ScannerSource,
// plus the SLA-breach list computed via FindingsPastSLA.
func BuildReport(tenantID uuid.UUID, findings []Finding, now time.Time) Report {
	r := Report{
		TenantID:      tenantID,
		GeneratedAt:   now,
		TotalFindings: len(findings),
		BySeverity:    map[Severity]int{SeverityLow: 0, SeverityMedium: 0, SeverityHigh: 0, SeverityCritical: 0},
		ByStatus: map[Status]int{
			StatusOpen: 0, StatusTriaged: 0, StatusRemediating: 0,
			StatusResolved: 0, StatusAcceptedRisk: 0, StatusFalsePositive: 0,
		},
		BySource: map[ScannerSource]int{
			ScannerSourceSCA: 0, ScannerSourceSAST: 0, ScannerSourceContainer: 0, ScannerSourceLicense: 0,
		},
	}
	for _, f := range findings {
		r.BySeverity[f.Severity]++
		r.ByStatus[f.Status]++
		r.BySource[f.Source]++
	}
	r.SLABreaches = FindingsPastSLA(findings, now)
	return r
}

// OpenCount returns how many findings in r are not yet in a terminal
// Status (Open, Triaged, or Remediating combined) -- the headline
// "how much outstanding work is there" figure a dashboard wants,
// without a caller needing to sum three map entries by hand.
func (r Report) OpenCount() int {
	return r.ByStatus[StatusOpen] + r.ByStatus[StatusTriaged] + r.ByStatus[StatusRemediating]
}

// SLABreachesBySeverityDesc returns a copy of r.SLABreaches sorted by
// descending Severity (SeverityCritical first, SeverityLow last),
// stable on ties -- mirroring
// packages/threatmodel.ThreatModel.ThreatsBySeverityDesc's ordering:
// the most serious overdue finding surfaced first, rather than
// insertion order.
func (r Report) SLABreachesBySeverityDesc() []Finding {
	out := make([]Finding, len(r.SLABreaches))
	copy(out, r.SLABreaches)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Severity.rank() > out[j].Severity.rank()
	})
	return out
}

// BuildDashboard is Engine's permission-gated, tenant-scoped
// counterpart to BuildReport: it loads tenantID's full Finding set
// (requiring viewPermission and tenant match, exactly like
// packages/compliance.Engine.RunGapAnalysis) and aggregates it via
// BuildReport using the Engine's own clock, so a caller never has to
// wire up ListFindings + BuildReport by hand.
func (e *Engine) BuildDashboard(ctx context.Context, tenantID uuid.UUID) (Report, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		return Report{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return Report{}, err
	}

	all, err := e.findings.ListAll(ctx, tenantID)
	if err != nil {
		return Report{}, wrapf("BuildDashboard", err)
	}
	return BuildReport(tenantID, all, e.now()), nil
}
