package vulnmanagement_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/vulnmanagement"
)

func TestRemediationDeadlineFor(t *testing.T) {
	t.Parallel()

	cases := []struct {
		severity vulnmanagement.Severity
		want     time.Duration
	}{
		{vulnmanagement.SeverityCritical, 7 * 24 * time.Hour},
		{vulnmanagement.SeverityHigh, 30 * 24 * time.Hour},
		{vulnmanagement.SeverityMedium, 90 * 24 * time.Hour},
		{vulnmanagement.SeverityLow, 180 * 24 * time.Hour},
	}
	for _, c := range cases {
		got, ok := vulnmanagement.RemediationDeadlineFor(c.severity)
		if !ok {
			t.Errorf("RemediationDeadlineFor(%s) ok = false, want true", c.severity)
		}
		if got != c.want {
			t.Errorf("RemediationDeadlineFor(%s) = %s, want %s", c.severity, got, c.want)
		}
	}

	if _, ok := vulnmanagement.RemediationDeadlineFor("bogus"); ok {
		t.Error(`RemediationDeadlineFor("bogus") ok = true, want false`)
	}
}

func TestIsSLABreached(t *testing.T) {
	t.Parallel()
	discovered := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	criticalFinding := vulnmanagement.Finding{
		TenantID:     uuid.New(),
		Severity:     vulnmanagement.SeverityCritical,
		Status:       vulnmanagement.StatusOpen,
		DiscoveredAt: discovered,
	}

	withinSLA := discovered.Add(6 * 24 * time.Hour)
	if vulnmanagement.IsSLABreached(criticalFinding, withinSLA) {
		t.Error("IsSLABreached at 6 days for a 7-day critical SLA = true, want false")
	}

	pastSLA := discovered.Add(8 * 24 * time.Hour)
	if !vulnmanagement.IsSLABreached(criticalFinding, pastSLA) {
		t.Error("IsSLABreached at 8 days for a 7-day critical SLA = false, want true")
	}

	// A finding in a terminal status is never breached, even long past
	// what would otherwise be its deadline.
	resolved := criticalFinding
	resolved.Status = vulnmanagement.StatusResolved
	if vulnmanagement.IsSLABreached(resolved, discovered.Add(365*24*time.Hour)) {
		t.Error("IsSLABreached(resolved finding) = true, want false (terminal status is never breached)")
	}
}

func TestFindingsPastSLA(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	tenantID := uuid.New()

	overdueCritical := vulnmanagement.Finding{
		ID: uuid.New(), TenantID: tenantID, Severity: vulnmanagement.SeverityCritical,
		Status: vulnmanagement.StatusOpen, DiscoveredAt: now.Add(-30 * 24 * time.Hour),
	}
	freshHigh := vulnmanagement.Finding{
		ID: uuid.New(), TenantID: tenantID, Severity: vulnmanagement.SeverityHigh,
		Status: vulnmanagement.StatusTriaged, DiscoveredAt: now.Add(-1 * 24 * time.Hour),
	}
	resolvedOldCritical := vulnmanagement.Finding{
		ID: uuid.New(), TenantID: tenantID, Severity: vulnmanagement.SeverityCritical,
		Status: vulnmanagement.StatusResolved, DiscoveredAt: now.Add(-365 * 24 * time.Hour),
	}

	breaches := vulnmanagement.FindingsPastSLA([]vulnmanagement.Finding{overdueCritical, freshHigh, resolvedOldCritical}, now)
	if len(breaches) != 1 {
		t.Fatalf("len(breaches) = %d, want 1", len(breaches))
	}
	if breaches[0].ID != overdueCritical.ID {
		t.Errorf("breaches[0].ID = %s, want %s", breaches[0].ID, overdueCritical.ID)
	}
}

func TestEngine_ListSLABreaches(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	overdue := newTestFinding(tenantID)
	overdue.Severity = vulnmanagement.SeverityCritical
	overdue.AdvisoryID = "CVE-2024-OVERDUE"
	overdue.DiscoveredAt = time.Now().Add(-30 * 24 * time.Hour)
	if _, err := engine.RecordFinding(ctxWithUser(admin), tenantID, overdue); err != nil {
		t.Fatalf("RecordFinding(overdue): %v", err)
	}

	fresh := newTestFinding(tenantID)
	fresh.Severity = vulnmanagement.SeverityLow
	fresh.AdvisoryID = "CVE-2024-FRESH"
	if _, err := engine.RecordFinding(ctxWithUser(admin), tenantID, fresh); err != nil {
		t.Fatalf("RecordFinding(fresh): %v", err)
	}

	viewer := auditorUser(tenantID)
	breaches, err := engine.ListSLABreaches(ctxWithUser(viewer), tenantID)
	if err != nil {
		t.Fatalf("ListSLABreaches: %v", err)
	}
	if len(breaches) != 1 {
		t.Fatalf("len(breaches) = %d, want 1", len(breaches))
	}
	if breaches[0].AdvisoryID != "CVE-2024-OVERDUE" {
		t.Errorf("breaches[0].AdvisoryID = %s, want CVE-2024-OVERDUE", breaches[0].AdvisoryID)
	}
}
