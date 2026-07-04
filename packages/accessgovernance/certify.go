package accessgovernance

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"strconv"

	"github.com/google/uuid"
)

// Certify produces a structured access-certification Report (task 7):
// every CaseGrant, Grant (elevation), and Review recorded for tenantID
// whose relevant timestamp falls within period. Requires
// reviewPermission (the same read-oriented permission ListDueReviews
// uses) and tenant match.
func (e *Engine) Certify(ctx context.Context, tenantID uuid.UUID, period Period) (Report, error) {
	if e.grants == nil || e.elevate == nil {
		return Report{}, ErrNilStore
	}
	if err := period.Validate(); err != nil {
		return Report{}, err
	}
	user, err := authorizeActor(ctx)
	if err != nil {
		return Report{}, err
	}
	if !user.HasPermission(reviewPermission) {
		return Report{}, ErrForbidden
	}
	if err := requireMatchingUserTenant(user, tenantID); err != nil {
		return Report{}, err
	}

	allGrants, err := e.grants.ListAll(ctx, tenantID)
	if err != nil {
		return Report{}, wrapf("Certify", err)
	}
	caseGrants := make([]CaseGrant, 0)
	for _, g := range allGrants {
		if period.Contains(g.GrantedAt) {
			caseGrants = append(caseGrants, g)
		}
	}

	allElevations, err := e.elevate.ListAll(ctx, tenantID)
	if err != nil {
		return Report{}, wrapf("Certify", err)
	}
	elevations := make([]Grant, 0)
	for _, g := range allElevations {
		if period.Contains(g.GrantedAt) {
			elevations = append(elevations, g)
		}
	}

	reviews := make([]Review, 0)
	if e.reviews != nil {
		allReviews, err := e.reviews.ListAll(ctx, tenantID)
		if err != nil {
			return Report{}, wrapf("Certify", err)
		}
		for _, r := range allReviews {
			if period.Contains(r.CreatedAt) {
				reviews = append(reviews, r)
			}
		}
	}

	return Report{
		TenantID:    tenantID,
		Period:      period,
		GeneratedAt: e.now(),
		CaseGrants:  caseGrants,
		Elevations:  elevations,
		Reviews:     reviews,
	}, nil
}

// ExportReport renders report in format (CSV or JSON), mirroring
// packages/auditlog.Export / packages/dataresidency's export
// conventions. CSV output uses one section per record kind
// (case_grants, elevations, reviews), each with its own header row,
// since the three record kinds have different columns.
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

	if err := w.Write([]string{"section", "id", "case_id", "grantee_or_actor", "granted_or_created_at", "expires_or_due_at", "detail"}); err != nil {
		return nil, wrapf("exportReportCSV", err)
	}

	for _, g := range report.CaseGrants {
		caseID := ""
		if g.CaseID != uuid.Nil {
			caseID = g.CaseID.String()
		}
		detail := "deny=" + strconv.FormatBool(g.Deny) + " permissions=" + strconv.Itoa(len(g.Permissions))
		if err := w.Write([]string{
			"case_grant", g.ID.String(), caseID, g.GranteeUserID.String(),
			g.GrantedAt.UTC().Format("2006-01-02T15:04:05Z"),
			g.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z"),
			detail,
		}); err != nil {
			return nil, wrapf("exportReportCSV", err)
		}
	}

	for _, g := range report.Elevations {
		caseID := ""
		if g.CaseID != uuid.Nil {
			caseID = g.CaseID.String()
		}
		if err := w.Write([]string{
			"elevation", g.ID.String(), caseID, g.GranteeUserID.String(),
			g.GrantedAt.UTC().Format("2006-01-02T15:04:05Z"),
			g.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z"),
			"action=" + string(g.Action) + " justification=" + g.Justification,
		}); err != nil {
			return nil, wrapf("exportReportCSV", err)
		}
	}

	for _, r := range report.Reviews {
		detail := "decision=" + string(r.Decision) + " subject_kind=" + string(r.SubjectKind)
		if err := w.Write([]string{
			"review", r.ID.String(), r.SubjectID.String(), r.RequestedBy.String(),
			r.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
			r.DueAt.UTC().Format("2006-01-02T15:04:05Z"),
			detail,
		}); err != nil {
			return nil, wrapf("exportReportCSV", err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, wrapf("exportReportCSV", err)
	}
	return buf.Bytes(), nil
}
