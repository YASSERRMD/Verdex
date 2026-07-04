package compliance_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/compliance"
)

func TestBuildDashboard_CountsByFrameworkAndStatus(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	now := time.Now()

	report := compliance.GapAnalysisReport{
		TenantID:    tenantID,
		GeneratedAt: now,
		Results: []compliance.ControlGapResult{
			{Control: compliance.Control{Code: "UAE-A", Framework: compliance.FrameworkUAEDataProtection}, Status: compliance.StatusSatisfied},
			{Control: compliance.Control{Code: "UAE-B", Framework: compliance.FrameworkUAEDataProtection}, Status: compliance.StatusGap},
			{Control: compliance.Control{Code: "JRH-A", Framework: compliance.FrameworkJudicialRecordsHandling}, Status: compliance.StatusPartiallyMet},
		},
	}

	dashboard := compliance.BuildDashboard(report, nil, now)

	if dashboard.TotalControls != 3 {
		t.Fatalf("TotalControls = %d, want 3", dashboard.TotalControls)
	}
	if dashboard.OverallStatusCounts[compliance.StatusSatisfied] != 1 ||
		dashboard.OverallStatusCounts[compliance.StatusGap] != 1 ||
		dashboard.OverallStatusCounts[compliance.StatusPartiallyMet] != 1 {
		t.Fatalf("OverallStatusCounts = %v, want 1 each", dashboard.OverallStatusCounts)
	}
	if len(dashboard.ByFramework) != 2 {
		t.Fatalf("len(ByFramework) = %d, want 2", len(dashboard.ByFramework))
	}

	// ByFramework is sorted ascending lexicographically:
	// "judicial_records_handling" < "uae_data_protection".
	if dashboard.ByFramework[0].Framework != compliance.FrameworkJudicialRecordsHandling {
		t.Fatalf("ByFramework[0].Framework = %q, want %q (deterministic ordering)", dashboard.ByFramework[0].Framework, compliance.FrameworkJudicialRecordsHandling)
	}
	uaeSummary := dashboard.ByFramework[1]
	if uaeSummary.TotalControls != 2 {
		t.Fatalf("uaeSummary.TotalControls = %d, want 2", uaeSummary.TotalControls)
	}
	if uaeSummary.StatusCounts[compliance.StatusSatisfied] != 1 || uaeSummary.StatusCounts[compliance.StatusGap] != 1 {
		t.Fatalf("uaeSummary.StatusCounts = %v, want satisfied=1 gap=1", uaeSummary.StatusCounts)
	}
}

// TestBuildDashboard_RecentEvidenceCount proves the trailing-window
// evidence count only counts records within the window and not after
// the evaluation instant.
func TestBuildDashboard_RecentEvidenceCount(t *testing.T) {
	t.Parallel()
	now := time.Now()
	report := compliance.GapAnalysisReport{TenantID: uuid.New(), GeneratedAt: now}

	evidence := []compliance.ControlEvidence{
		{CollectedAt: now.Add(-24 * time.Hour)},      // within window
		{CollectedAt: now.Add(-40 * 24 * time.Hour)}, // outside window (>30 days)
		{CollectedAt: now.Add(24 * time.Hour)},       // future-dated, must not count
	}

	dashboard := compliance.BuildDashboard(report, evidence, now)
	if dashboard.RecentEvidenceCount != 1 {
		t.Fatalf("RecentEvidenceCount = %d, want 1", dashboard.RecentEvidenceCount)
	}
}

// TestEngine_BuildDashboardReport exercises task 8's engine-level
// entry point end to end.
func TestEngine_BuildDashboardReport(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)
	control := registerTestControl(t, engine, tenantID)

	if _, err := engine.RecordEvidence(ctxWithUser(admin), tenantID, compliance.ControlEvidence{
		ControlID: control.ID, Kind: compliance.EvidenceKindTestName, Reference: "TestFoo",
	}); err != nil {
		t.Fatalf("RecordEvidence: %v", err)
	}

	dashboard, err := engine.BuildDashboardReport(ctxWithUser(admin), tenantID)
	if err != nil {
		t.Fatalf("BuildDashboardReport: %v", err)
	}
	if dashboard.TotalControls != 1 {
		t.Fatalf("TotalControls = %d, want 1", dashboard.TotalControls)
	}
	if dashboard.OverallStatusCounts[compliance.StatusPartiallyMet] != 1 {
		t.Fatalf("OverallStatusCounts = %v, want partially_met=1 (one test-name evidence, below the 2-kind bar)", dashboard.OverallStatusCounts)
	}
	if dashboard.RecentEvidenceCount != 1 {
		t.Fatalf("RecentEvidenceCount = %d, want 1", dashboard.RecentEvidenceCount)
	}
}
