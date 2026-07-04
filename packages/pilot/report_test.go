package pilot_test

import (
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/pilot"
)

func TestEngine_CaptureReport_AggregatesEverything(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	d := provisionAndActivate(t, te)
	admin := adminUser(te.tenantID)

	pc1 := assignTestCase(t, te, d.ID)
	pc2 := assignTestCase(t, te, d.ID)
	if _, err := te.engine.MarkOutcomeObserved(ctxWithUser(admin), te.tenantID, pc1.ID); err != nil {
		t.Fatalf("MarkOutcomeObserved: %v", err)
	}

	entry1 := submitTestFeedback(t, te, pc1.ID)
	submitTestFeedback(t, te, pc2.ID)

	f := recordTestFinding(t, te, d.ID, entry1.ID)
	triaged, err := te.engine.TriageFinding(ctxWithUser(admin), te.tenantID, f.ID, pilot.PriorityCritical, pilot.FindingStatusTriaged, "Critical pilot-blocking issue.")
	if err != nil {
		t.Fatalf("TriageFinding: %v", err)
	}
	refinement, err := te.engine.RecordRefinement(ctxWithUser(admin), te.tenantID, pilot.RefinementRecord{
		FindingID:   triaged.ID,
		Description: "Applied a targeted fix.",
	})
	if err != nil {
		t.Fatalf("RecordRefinement: %v", err)
	}
	if _, err := te.engine.VerifyRefinement(ctxWithUser(admin), te.tenantID, refinement.ID, "Re-tested, confirmed fixed."); err != nil {
		t.Fatalf("VerifyRefinement: %v", err)
	}

	compliance, err := te.engine.ValidateNonBindingCompliance(ctxWithUser(admin), te.tenantID, pc1.ID, properlyLabeledDraftAnalysis)
	if err != nil {
		t.Fatalf("ValidateNonBindingCompliance: %v", err)
	}

	period := pilot.Period{Start: pilotNow().Add(-24 * time.Hour), End: pilotNow().Add(24 * time.Hour)}
	report, err := te.engine.CaptureReport(ctxWithUser(admin), te.tenantID, d.ID, period, nil, []pilot.ComplianceResult{compliance})
	if err != nil {
		t.Fatalf("CaptureReport: %v", err)
	}

	if report.DeploymentStatus != pilot.DeploymentStatusActive {
		t.Fatalf("DeploymentStatus = %q, want %q", report.DeploymentStatus, pilot.DeploymentStatusActive)
	}
	if report.CaseCount != 2 {
		t.Fatalf("CaseCount = %d, want 2", report.CaseCount)
	}
	if report.ObservedOutcomeCount != 1 {
		t.Fatalf("ObservedOutcomeCount = %d, want 1", report.ObservedOutcomeCount)
	}
	if report.Quality.FeedbackCount != 2 {
		t.Fatalf("Quality.FeedbackCount = %d, want 2", report.Quality.FeedbackCount)
	}
	if report.FindingsByPriority[pilot.PriorityCritical] != 1 {
		t.Fatalf("FindingsByPriority[critical] = %d, want 1", report.FindingsByPriority[pilot.PriorityCritical])
	}
	if report.RefinementsApplied != 1 || report.RefinementsVerified != 1 {
		t.Fatalf("RefinementsApplied=%d RefinementsVerified=%d, want 1 and 1", report.RefinementsApplied, report.RefinementsVerified)
	}
	if report.ComplianceChecksRun != 1 || !floatsClose(report.ComplianceRate, 1.0) {
		t.Fatalf("ComplianceChecksRun=%d ComplianceRate=%v, want 1 and 1.0", report.ComplianceChecksRun, report.ComplianceRate)
	}
}

func TestEngine_CaptureReport_RejectsInvalidPeriod(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	d := provisionAndActivate(t, te)
	admin := adminUser(te.tenantID)

	now := pilotNow()
	_, err := te.engine.CaptureReport(ctxWithUser(admin), te.tenantID, d.ID, pilot.Period{Start: now, End: now.Add(-1 * time.Hour)}, nil, nil)
	if err == nil {
		t.Fatal("expected an error for End before Start")
	}
}

func TestExportReport_JSONRoundTrips(t *testing.T) {
	t.Parallel()
	report := pilot.Report{
		DeploymentStatus: pilot.DeploymentStatusActive,
		CaseCount:        3,
		FindingsByPriority: map[pilot.Priority]int{
			pilot.PriorityHigh: 2,
		},
	}
	out, err := pilot.ExportReport(report, pilot.ExportFormatJSON)
	if err != nil {
		t.Fatalf("ExportReport (json): %v", err)
	}
	var roundTripped pilot.Report
	if err := json.Unmarshal(out, &roundTripped); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if roundTripped.CaseCount != 3 {
		t.Fatalf("CaseCount = %d, want 3", roundTripped.CaseCount)
	}
	if roundTripped.FindingsByPriority[pilot.PriorityHigh] != 2 {
		t.Fatalf("FindingsByPriority[high] = %d, want 2", roundTripped.FindingsByPriority[pilot.PriorityHigh])
	}
}

func TestExportReport_CSVIsValidAndHasExpectedSections(t *testing.T) {
	t.Parallel()
	report := pilot.Report{
		DeploymentStatus: pilot.DeploymentStatusActive,
		CaseCount:        5,
		FindingsByPriority: map[pilot.Priority]int{
			pilot.PriorityCritical: 1, pilot.PriorityHigh: 0, pilot.PriorityMedium: 0, pilot.PriorityLow: 0,
		},
		FindingsByStatus: map[pilot.FindingStatus]int{
			pilot.FindingStatusOpen: 0, pilot.FindingStatusTriaged: 0, pilot.FindingStatusInProgress: 0,
			pilot.FindingStatusResolved: 0, pilot.FindingStatusWontFix: 0,
		},
	}
	out, err := pilot.ExportReport(report, pilot.ExportFormatCSV)
	if err != nil {
		t.Fatalf("ExportReport (csv): %v", err)
	}

	r := csv.NewReader(strings.NewReader(string(out)))
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatalf("csv.ReadAll: %v", err)
	}
	if len(rows) < 2 {
		t.Fatalf("expected at least a header row and one data row, got %d rows", len(rows))
	}
	if rows[0][0] != "section" {
		t.Fatalf("header row = %v, want first column %q", rows[0], "section")
	}

	sections := make(map[string]bool)
	for _, row := range rows[1:] {
		sections[row[0]] = true
	}
	for _, want := range []string{"summary", "findings_by_priority", "findings_by_status"} {
		if !sections[want] {
			t.Errorf("expected a %q section in CSV output, got sections %v", want, sections)
		}
	}
}

func TestExportReport_RejectsInvalidFormat(t *testing.T) {
	t.Parallel()
	_, err := pilot.ExportReport(pilot.Report{}, pilot.ExportFormat("xml"))
	if err == nil {
		t.Fatal("expected an error for an unrecognized ExportFormat")
	}
}
