package accessgovernance_test

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/accessgovernance"
	"github.com/YASSERRMD/verdex/packages/identity"
)

// TestEngine_Certify_AggregatesGrantsElevationsAndReviews proves task
// 7: Certify aggregates every CaseGrant, Grant (elevation), and Review
// recorded for a tenant within a Period.
func TestEngine_Certify_AggregatesGrantsElevationsAndReviews(t *testing.T) {
	engine, tenantID := newTestEngine(t)
	admin := newTestUser(tenantID, identity.RoleAdmin)
	reviewer := newTestUser(tenantID)
	caseID := uuid.New()

	start := time.Now().Add(-time.Hour)

	grant, err := engine.GrantCaseAccess(ctxWithUser(admin), tenantID, accessgovernance.CaseGrant{
		CaseID:        caseID,
		GranteeUserID: reviewer.ID,
		Permissions:   []identity.Permission{identity.PermViewCase},
		ExpiresAt:     time.Now().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("GrantCaseAccess: %v", err)
	}
	if _, err := engine.Elevate(ctxWithUser(admin), tenantID, admin.ID, "case:delete", uuid.Nil, "incident", time.Hour); err != nil {
		t.Fatalf("Elevate: %v", err)
	}
	if _, err := engine.ScheduleReview(ctxWithUser(admin), tenantID, accessgovernance.GrantKindCase, grant.ID, reviewer.ID, time.Now()); err != nil {
		t.Fatalf("ScheduleReview: %v", err)
	}

	end := time.Now().Add(time.Hour)
	report, err := engine.Certify(ctxWithUser(admin), tenantID, accessgovernance.Period{Start: start, End: end})
	if err != nil {
		t.Fatalf("Certify: %v", err)
	}
	if len(report.CaseGrants) != 1 {
		t.Errorf("report.CaseGrants = %d, want 1", len(report.CaseGrants))
	}
	if len(report.Elevations) != 1 {
		t.Errorf("report.Elevations = %d, want 1", len(report.Elevations))
	}
	if len(report.Reviews) != 1 {
		t.Errorf("report.Reviews = %d, want 1", len(report.Reviews))
	}
	if report.TotalEntries() != 3 {
		t.Errorf("report.TotalEntries() = %d, want 3", report.TotalEntries())
	}
}

// TestEngine_Certify_ExcludesEntriesOutsidePeriod proves Certify
// respects the Period boundary rather than returning every record
// ever created.
func TestEngine_Certify_ExcludesEntriesOutsidePeriod(t *testing.T) {
	engine, tenantID := newTestEngine(t)
	admin := newTestUser(tenantID, identity.RoleAdmin)
	reviewer := newTestUser(tenantID)
	caseID := uuid.New()

	if _, err := engine.GrantCaseAccess(ctxWithUser(admin), tenantID, accessgovernance.CaseGrant{
		CaseID:        caseID,
		GranteeUserID: reviewer.ID,
		Permissions:   []identity.Permission{identity.PermViewCase},
		ExpiresAt:     time.Now().Add(24 * time.Hour),
	}); err != nil {
		t.Fatalf("GrantCaseAccess: %v", err)
	}

	// A period entirely before the grant was created.
	pastPeriod := accessgovernance.Period{
		Start: time.Now().Add(-48 * time.Hour),
		End:   time.Now().Add(-24 * time.Hour),
	}
	report, err := engine.Certify(ctxWithUser(admin), tenantID, pastPeriod)
	if err != nil {
		t.Fatalf("Certify: %v", err)
	}
	if len(report.CaseGrants) != 0 {
		t.Fatalf("report.CaseGrants for a past period = %d, want 0", len(report.CaseGrants))
	}
}

// TestExportReport_JSONRoundTrips proves ExportReport's JSON format
// produces valid, re-parseable JSON.
func TestExportReport_JSONRoundTrips(t *testing.T) {
	report := accessgovernance.Report{
		TenantID:    uuid.New(),
		Period:      accessgovernance.Period{Start: time.Now().Add(-time.Hour), End: time.Now()},
		GeneratedAt: time.Now(),
		CaseGrants: []accessgovernance.CaseGrant{
			{ID: uuid.New(), CaseID: uuid.New(), GranteeUserID: uuid.New(), ExpiresAt: time.Now().Add(time.Hour), Permissions: []identity.Permission{identity.PermViewCase}},
		},
	}

	out, err := accessgovernance.ExportReport(report, accessgovernance.ExportFormatJSON)
	if err != nil {
		t.Fatalf("ExportReport (json): %v", err)
	}

	var parsed accessgovernance.Report
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("json.Unmarshal(export output): %v", err)
	}
	if len(parsed.CaseGrants) != 1 {
		t.Fatalf("parsed.CaseGrants = %d, want 1", len(parsed.CaseGrants))
	}
	if parsed.CaseGrants[0].ID != report.CaseGrants[0].ID {
		t.Fatalf("parsed grant ID = %v, want %v", parsed.CaseGrants[0].ID, report.CaseGrants[0].ID)
	}
}

// TestExportReport_CSVIsValidAndHasExpectedSections proves
// ExportReport's CSV format is well-formed CSV with one row per
// section entry, header first.
func TestExportReport_CSVIsValidAndHasExpectedSections(t *testing.T) {
	caseGrantID := uuid.New()
	elevationID := uuid.New()

	report := accessgovernance.Report{
		TenantID: uuid.New(),
		CaseGrants: []accessgovernance.CaseGrant{
			{ID: caseGrantID, CaseID: uuid.New(), GranteeUserID: uuid.New(), ExpiresAt: time.Now().Add(time.Hour), Permissions: []identity.Permission{identity.PermViewCase}},
		},
		Elevations: []accessgovernance.Grant{
			{ID: elevationID, GranteeUserID: uuid.New(), Action: "case:delete", Justification: "j", ExpiresAt: time.Now().Add(time.Hour)},
		},
	}

	out, err := accessgovernance.ExportReport(report, accessgovernance.ExportFormatCSV)
	if err != nil {
		t.Fatalf("ExportReport (csv): %v", err)
	}

	records, err := csv.NewReader(bytes.NewReader(out)).ReadAll()
	if err != nil {
		t.Fatalf("csv.ReadAll: %v", err)
	}
	// header + 1 case_grant row + 1 elevation row
	if len(records) != 3 {
		t.Fatalf("csv record count = %d, want 3 (header + 2 rows)", len(records))
	}
	if records[0][0] != "section" {
		t.Fatalf("csv header[0] = %q, want section", records[0][0])
	}

	var sawCaseGrant, sawElevation bool
	for _, row := range records[1:] {
		switch row[0] {
		case "case_grant":
			sawCaseGrant = true
			if !strings.Contains(row[1], caseGrantID.String()) {
				t.Errorf("case_grant row id = %q, want to contain %v", row[1], caseGrantID)
			}
		case "elevation":
			sawElevation = true
			if !strings.Contains(row[1], elevationID.String()) {
				t.Errorf("elevation row id = %q, want to contain %v", row[1], elevationID)
			}
		}
	}
	if !sawCaseGrant || !sawElevation {
		t.Fatalf("csv export missing expected sections: case_grant=%v elevation=%v", sawCaseGrant, sawElevation)
	}
}

// TestEngine_Certify_InvalidPeriodRejected proves Certify validates
// its Period argument.
func TestEngine_Certify_InvalidPeriodRejected(t *testing.T) {
	engine, tenantID := newTestEngine(t)
	admin := newTestUser(tenantID, identity.RoleAdmin)

	now := time.Now()
	_, err := engine.Certify(ctxWithUser(admin), tenantID, accessgovernance.Period{Start: now, End: now})
	if err != accessgovernance.ErrInvalidPeriod {
		t.Fatalf("Certify() invalid period error = %v, want ErrInvalidPeriod", err)
	}
}
