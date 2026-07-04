package pilot

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"strconv"
	"time"

	"github.com/google/uuid"
)

// Period is a half-open time range [Start, End) a pilot Report covers,
// mirroring packages/accessgovernance.Period exactly.
type Period struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// Validate checks that p.End is strictly after p.Start.
func (p Period) Validate() error {
	if !p.End.After(p.Start) {
		return ErrInvalidPeriod
	}
	return nil
}

// Contains reports whether t falls within [Start, End).
func (p Period) Contains(t time.Time) bool {
	return !t.Before(p.Start) && t.Before(p.End)
}

// Report is the structured output of CaptureReport (task 9):
// deployment status, case count, quality summary, findings by
// priority, refinements applied, and non-binding-compliance rate for
// one PilotDeployment within Period -- exportable as CSV/JSON,
// mirroring packages/accessgovernance.Report's shape and
// packages/vulnmanagement.Report's real-generator convention.
type Report struct {
	// TenantID is the tenant this report was generated for.
	TenantID uuid.UUID `json:"tenant_id"`

	// DeploymentID is the PilotDeployment this report covers.
	DeploymentID uuid.UUID `json:"deployment_id"`

	// Period is the reporting window this report covers.
	Period Period `json:"period"`

	// GeneratedAt is when this report was computed.
	GeneratedAt time.Time `json:"generated_at"`

	// DeploymentStatus is the deployment's Status as of GeneratedAt.
	DeploymentStatus DeploymentStatus `json:"deployment_status"`

	// CaseCount is the number of PilotCases assigned under the
	// deployment.
	CaseCount int `json:"case_count"`

	// ObservedOutcomeCount is how many of those PilotCases have
	// OutcomeObserved set true.
	ObservedOutcomeCount int `json:"observed_outcome_count"`

	// Quality is the deployment's aggregated reasoning-quality-and-trust
	// summary (task 5), computed over feedback recorded within Period.
	Quality QualitySummary `json:"quality"`

	// FindingsByPriority counts PilotFindings recorded within Period,
	// keyed by Priority.
	FindingsByPriority map[Priority]int `json:"findings_by_priority"`

	// FindingsByStatus counts PilotFindings recorded within Period,
	// keyed by FindingStatus.
	FindingsByStatus map[FindingStatus]int `json:"findings_by_status"`

	// RefinementsApplied is how many RefinementRecords were applied
	// within Period.
	RefinementsApplied int `json:"refinements_applied"`

	// RefinementsVerified is how many of those RefinementRecords have
	// VerifiedFixed set true.
	RefinementsVerified int `json:"refinements_verified"`

	// ComplianceChecksRun is how many ValidateNonBindingCompliance
	// results are folded into ComplianceRate (task 8), supplied by the
	// caller of CaptureReport -- see BuildReport.
	ComplianceChecksRun int `json:"compliance_checks_run"`

	// ComplianceRate is the fraction (in [0,1]) of
	// ComplianceChecksRun-many ValidateNonBindingCompliance results
	// that had Passed set true. 0 when ComplianceChecksRun is 0.
	ComplianceRate float64 `json:"compliance_rate"`
}

// ExportFormat selects the rendering ExportReport produces, mirroring
// packages/accessgovernance.ExportFormat / packages/auditlog.ExportFormat
// exactly.
type ExportFormat string

const (
	// ExportFormatCSV renders a Report as CSV (one section per
	// sub-aggregate, each with its own header row).
	ExportFormatCSV ExportFormat = "csv"

	// ExportFormatJSON renders a Report as a single JSON object.
	ExportFormatJSON ExportFormat = "json"
)

// IsValid reports whether f is a recognized ExportFormat.
func (f ExportFormat) IsValid() bool {
	return f == ExportFormatCSV || f == ExportFormatJSON
}

// CaptureReport aggregates deployment status, case count, quality
// summary, findings by priority, refinements applied, and
// non-binding-compliance rate for deploymentID within period (task 9),
// requiring viewPermission and tenant match. complianceResults is the
// caller-supplied set of ValidateNonBindingCompliance outcomes to
// fold into ComplianceRate (this package does not re-run those checks
// itself here, since the opinion texts they were computed against are
// not persisted by this package -- see ValidateNonBindingCompliance);
// may be nil or empty. automatedScores is forwarded to AggregateQuality
// unchanged -- see its doc comment. Every capture is recorded via
// AuditSink regardless of outcome.
func (e *Engine) CaptureReport(ctx context.Context, tenantID, deploymentID uuid.UUID, period Period, automatedScores []QualityScoreLike, complianceResults []ComplianceResult) (Report, error) {
	user, err := authorizeView(ctx)
	if err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordReportCapture(ctx, tenantID, actorFromCtx(ctx), deploymentID, err)
		}
		return Report{}, err
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordReportCapture(ctx, tenantID, user.ID, deploymentID, err)
		}
		return Report{}, err
	}
	if err := period.Validate(); err != nil {
		if e.audit != nil {
			_, _ = e.audit.RecordReportCapture(ctx, tenantID, user.ID, deploymentID, err)
		}
		return Report{}, err
	}

	deployment, err := e.deployments.Get(ctx, tenantID, deploymentID)
	if err != nil {
		wrapped := wrapf("CaptureReport", err)
		if e.audit != nil {
			_, _ = e.audit.RecordReportCapture(ctx, tenantID, user.ID, deploymentID, wrapped)
		}
		return Report{}, wrapped
	}

	cases, err := e.cases.ListForDeployment(ctx, tenantID, deploymentID)
	if err != nil {
		wrapped := wrapf("CaptureReport", err)
		if e.audit != nil {
			_, _ = e.audit.RecordReportCapture(ctx, tenantID, user.ID, deploymentID, wrapped)
		}
		return Report{}, wrapped
	}

	quality, err := e.AggregateQuality(ctx, tenantID, deploymentID, automatedScores)
	if err != nil {
		wrapped := wrapf("CaptureReport", err)
		if e.audit != nil {
			_, _ = e.audit.RecordReportCapture(ctx, tenantID, user.ID, deploymentID, wrapped)
		}
		return Report{}, wrapped
	}

	findings, err := e.findings.ListForDeployment(ctx, tenantID, deploymentID)
	if err != nil {
		wrapped := wrapf("CaptureReport", err)
		if e.audit != nil {
			_, _ = e.audit.RecordReportCapture(ctx, tenantID, user.ID, deploymentID, wrapped)
		}
		return Report{}, wrapped
	}

	refinements, err := e.ListRefinementsForDeployment(ctx, tenantID, deploymentID)
	if err != nil {
		wrapped := wrapf("CaptureReport", err)
		if e.audit != nil {
			_, _ = e.audit.RecordReportCapture(ctx, tenantID, user.ID, deploymentID, wrapped)
		}
		return Report{}, wrapped
	}

	report := BuildReport(tenantID, *deployment, cases, quality, findings, refinements, complianceResults, period, e.now())

	if e.audit != nil {
		_, _ = e.audit.RecordReportCapture(ctx, tenantID, user.ID, deploymentID, nil)
	}
	return report, nil
}

// BuildReport is CaptureReport's pure aggregation function, mirroring
// packages/vulnmanagement.BuildReport and packages/compliance.BuildDashboard's
// "operate on already-fetched data" convention.
func BuildReport(
	tenantID uuid.UUID,
	deployment PilotDeployment,
	cases []PilotCase,
	quality QualitySummary,
	findings []PilotFinding,
	refinements []RefinementRecord,
	complianceResults []ComplianceResult,
	period Period,
	now time.Time,
) Report {
	r := Report{
		TenantID:         tenantID,
		DeploymentID:     deployment.ID,
		Period:           period,
		GeneratedAt:      now,
		DeploymentStatus: deployment.Status,
		CaseCount:        len(cases),
		Quality:          quality,
		FindingsByPriority: map[Priority]int{
			PriorityLow: 0, PriorityMedium: 0, PriorityHigh: 0, PriorityCritical: 0,
		},
		FindingsByStatus: map[FindingStatus]int{
			FindingStatusOpen: 0, FindingStatusTriaged: 0, FindingStatusInProgress: 0,
			FindingStatusResolved: 0, FindingStatusWontFix: 0,
		},
	}

	for _, c := range cases {
		if c.OutcomeObserved {
			r.ObservedOutcomeCount++
		}
	}

	for _, f := range findings {
		if !period.Contains(f.DiscoveredAt) {
			continue
		}
		r.FindingsByPriority[f.Priority]++
		r.FindingsByStatus[f.Status]++
	}

	for _, rec := range refinements {
		if !period.Contains(rec.AppliedAt) {
			continue
		}
		r.RefinementsApplied++
		if rec.VerifiedFixed {
			r.RefinementsVerified++
		}
	}

	for _, cr := range complianceResults {
		if !period.Contains(cr.CheckedAt) {
			continue
		}
		r.ComplianceChecksRun++
		if cr.Passed {
			r.ComplianceRate++
		}
	}
	if r.ComplianceChecksRun > 0 {
		r.ComplianceRate /= float64(r.ComplianceChecksRun)
	}

	return r
}

// ExportReport renders report in format (CSV or JSON), mirroring
// packages/accessgovernance.ExportReport / packages/auditlog.Export's
// conventions. CSV output uses one section per sub-aggregate
// (summary, findings_by_priority, findings_by_status), each with its
// own header row, since the sections have different columns.
func ExportReport(report Report, format ExportFormat) ([]byte, error) {
	if !format.IsValid() {
		return nil, ErrInvalidExportFormat
	}
	switch format {
	case ExportFormatJSON:
		return exportReportJSON(report)
	case ExportFormatCSV:
		return exportReportCSV(report)
	default:
		return nil, ErrInvalidExportFormat
	}
}

func exportReportJSON(report Report) ([]byte, error) {
	out, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, wrapf("exportReportJSON", err)
	}
	return out, nil
}

func exportReportCSV(report Report) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	if err := w.Write([]string{"section", "key", "value"}); err != nil {
		return nil, wrapf("exportReportCSV", err)
	}

	summaryRows := [][2]string{
		{"deployment_id", report.DeploymentID.String()},
		{"deployment_status", report.DeploymentStatus.String()},
		{"case_count", strconv.Itoa(report.CaseCount)},
		{"observed_outcome_count", strconv.Itoa(report.ObservedOutcomeCount)},
		{"avg_overall_feedback_score", strconv.FormatFloat(report.Quality.AvgOverallFeedbackScore, 'f', 4, 64)},
		{"avg_trust", strconv.FormatFloat(report.Quality.AvgTrust, 'f', 4, 64)},
		{"refinements_applied", strconv.Itoa(report.RefinementsApplied)},
		{"refinements_verified", strconv.Itoa(report.RefinementsVerified)},
		{"compliance_checks_run", strconv.Itoa(report.ComplianceChecksRun)},
		{"compliance_rate", strconv.FormatFloat(report.ComplianceRate, 'f', 4, 64)},
	}
	for _, row := range summaryRows {
		if err := w.Write([]string{"summary", row[0], row[1]}); err != nil {
			return nil, wrapf("exportReportCSV", err)
		}
	}

	for _, p := range []Priority{PriorityLow, PriorityMedium, PriorityHigh, PriorityCritical} {
		if err := w.Write([]string{"findings_by_priority", string(p), strconv.Itoa(report.FindingsByPriority[p])}); err != nil {
			return nil, wrapf("exportReportCSV", err)
		}
	}

	for _, s := range []FindingStatus{FindingStatusOpen, FindingStatusTriaged, FindingStatusInProgress, FindingStatusResolved, FindingStatusWontFix} {
		if err := w.Write([]string{"findings_by_status", string(s), strconv.Itoa(report.FindingsByStatus[s])}); err != nil {
			return nil, wrapf("exportReportCSV", err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, wrapf("exportReportCSV", err)
	}
	return buf.Bytes(), nil
}
