package vulnmanagement_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/vulnmanagement"
)

func TestBuildReport_Aggregates(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	findings := []vulnmanagement.Finding{
		{
			ID: uuid.New(), TenantID: tenantID, Source: vulnmanagement.ScannerSourceSCA,
			Severity: vulnmanagement.SeverityCritical, Status: vulnmanagement.StatusOpen,
			DiscoveredAt: now.Add(-30 * 24 * time.Hour), // overdue for a 7-day critical SLA
		},
		{
			ID: uuid.New(), TenantID: tenantID, Source: vulnmanagement.ScannerSourceSAST,
			Severity: vulnmanagement.SeverityMedium, Status: vulnmanagement.StatusTriaged,
			DiscoveredAt: now.Add(-1 * 24 * time.Hour),
		},
		{
			ID: uuid.New(), TenantID: tenantID, Source: vulnmanagement.ScannerSourceContainer,
			Severity: vulnmanagement.SeverityHigh, Status: vulnmanagement.StatusResolved,
			DiscoveredAt: now.Add(-90 * 24 * time.Hour),
		},
	}

	report := vulnmanagement.BuildReport(tenantID, findings, now)

	if report.TotalFindings != 3 {
		t.Errorf("TotalFindings = %d, want 3", report.TotalFindings)
	}
	if report.BySeverity[vulnmanagement.SeverityCritical] != 1 {
		t.Errorf("BySeverity[Critical] = %d, want 1", report.BySeverity[vulnmanagement.SeverityCritical])
	}
	if report.ByStatus[vulnmanagement.StatusResolved] != 1 {
		t.Errorf("ByStatus[Resolved] = %d, want 1", report.ByStatus[vulnmanagement.StatusResolved])
	}
	if report.BySource[vulnmanagement.ScannerSourceSAST] != 1 {
		t.Errorf("BySource[SAST] = %d, want 1", report.BySource[vulnmanagement.ScannerSourceSAST])
	}
	if len(report.SLABreaches) != 1 {
		t.Fatalf("len(SLABreaches) = %d, want 1", len(report.SLABreaches))
	}
	if report.OpenCount() != 2 {
		t.Errorf("OpenCount() = %d, want 2 (open + triaged)", report.OpenCount())
	}
}

func TestReport_SLABreachesBySeverityDesc(t *testing.T) {
	t.Parallel()
	tenantID := uuid.New()
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	// All three breach their respective SLA; ordering should come out
	// Critical, High, Medium regardless of input order.
	findings := []vulnmanagement.Finding{
		{ID: uuid.New(), TenantID: tenantID, Severity: vulnmanagement.SeverityMedium, Status: vulnmanagement.StatusOpen, DiscoveredAt: now.Add(-200 * 24 * time.Hour)},
		{ID: uuid.New(), TenantID: tenantID, Severity: vulnmanagement.SeverityCritical, Status: vulnmanagement.StatusOpen, DiscoveredAt: now.Add(-200 * 24 * time.Hour)},
		{ID: uuid.New(), TenantID: tenantID, Severity: vulnmanagement.SeverityHigh, Status: vulnmanagement.StatusOpen, DiscoveredAt: now.Add(-200 * 24 * time.Hour)},
	}

	report := vulnmanagement.BuildReport(tenantID, findings, now)
	ordered := report.SLABreachesBySeverityDesc()
	if len(ordered) != 3 {
		t.Fatalf("len(ordered) = %d, want 3", len(ordered))
	}
	want := []vulnmanagement.Severity{vulnmanagement.SeverityCritical, vulnmanagement.SeverityHigh, vulnmanagement.SeverityMedium}
	for i, sev := range want {
		if ordered[i].Severity != sev {
			t.Errorf("ordered[%d].Severity = %s, want %s", i, ordered[i].Severity, sev)
		}
	}
}

func TestEngine_BuildDashboard(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	if _, err := engine.RecordFinding(ctxWithUser(admin), tenantID, newTestFinding(tenantID)); err != nil {
		t.Fatalf("RecordFinding: %v", err)
	}

	viewer := auditorUser(tenantID)
	report, err := engine.BuildDashboard(ctxWithUser(viewer), tenantID)
	if err != nil {
		t.Fatalf("BuildDashboard: %v", err)
	}
	if report.TotalFindings != 1 {
		t.Errorf("report.TotalFindings = %d, want 1", report.TotalFindings)
	}
	if report.TenantID != tenantID {
		t.Errorf("report.TenantID = %s, want %s", report.TenantID, tenantID)
	}
}

func TestEngine_BuildDashboard_RequiresViewPermission(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)

	noPerms := newTestUser(tenantID)
	_, err := engine.BuildDashboard(ctxWithUser(noPerms), tenantID)
	if !errors.Is(err, vulnmanagement.ErrForbidden) {
		t.Fatalf("BuildDashboard(no perms) error = %v, want ErrForbidden", err)
	}
}
