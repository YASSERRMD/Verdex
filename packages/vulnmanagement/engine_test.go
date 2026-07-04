package vulnmanagement_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/auditlog"
	"github.com/YASSERRMD/verdex/packages/vulnmanagement"
)

func TestEngine_RecordFinding_RequiresManagePermission(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)

	viewer := auditorUser(tenantID)
	_, err := engine.RecordFinding(ctxWithUser(viewer), tenantID, newTestFinding(tenantID))
	if !errors.Is(err, vulnmanagement.ErrForbidden) {
		t.Fatalf("RecordFinding(auditor) error = %v, want ErrForbidden", err)
	}
}

func TestEngine_RecordFinding_RequiresAuthentication(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)

	_, err := engine.RecordFinding(t.Context(), tenantID, newTestFinding(tenantID))
	if !errors.Is(err, vulnmanagement.ErrUnauthenticated) {
		t.Fatalf("RecordFinding(no actor) error = %v, want ErrUnauthenticated", err)
	}
}

func TestEngine_RecordFinding_RejectsCrossTenantActor(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	otherTenant := uuid.New()

	admin := adminUser(otherTenant)
	_, err := engine.RecordFinding(ctxWithUser(admin), tenantID, newTestFinding(tenantID))
	if !errors.Is(err, vulnmanagement.ErrCrossTenantAccess) {
		t.Fatalf("RecordFinding(cross-tenant) error = %v, want ErrCrossTenantAccess", err)
	}
}

func TestEngine_RecordFinding_AlwaysStartsOpen(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)

	input := newTestFinding(tenantID)
	input.Status = vulnmanagement.StatusResolved // caller-supplied status must be ignored

	admin := adminUser(tenantID)
	f, err := engine.RecordFinding(ctxWithUser(admin), tenantID, input)
	if err != nil {
		t.Fatalf("RecordFinding: %v", err)
	}
	if f.Status != vulnmanagement.StatusOpen {
		t.Errorf("f.Status = %s, want StatusOpen", f.Status)
	}
	if f.ID == uuid.Nil {
		t.Error("f.ID is uuid.Nil, want a generated ID")
	}
	if f.DiscoveredAt.IsZero() {
		t.Error("f.DiscoveredAt is zero, want a set timestamp")
	}
}

func TestEngine_RecordFinding_RejectsInvalidFinding(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)

	input := newTestFinding(tenantID)
	input.Package = "" // structurally invalid

	admin := adminUser(tenantID)
	_, err := engine.RecordFinding(ctxWithUser(admin), tenantID, input)
	if !errors.Is(err, vulnmanagement.ErrInvalidFinding) {
		t.Fatalf("RecordFinding(blank package) error = %v, want ErrInvalidFinding", err)
	}
}

func TestEngine_GetFinding_RoundTrips(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)

	recorded := recordTestFinding(t, engine, tenantID)

	viewer := auditorUser(tenantID)
	got, err := engine.GetFinding(ctxWithUser(viewer), tenantID, recorded.ID)
	if err != nil {
		t.Fatalf("GetFinding: %v", err)
	}
	if got.ID != recorded.ID {
		t.Errorf("got.ID = %s, want %s", got.ID, recorded.ID)
	}
}

func TestEngine_GetFinding_RequiresViewPermission(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	recorded := recordTestFinding(t, engine, tenantID)

	noPerms := newTestUser(tenantID) // no roles at all
	_, err := engine.GetFinding(ctxWithUser(noPerms), tenantID, recorded.ID)
	if !errors.Is(err, vulnmanagement.ErrForbidden) {
		t.Fatalf("GetFinding(no perms) error = %v, want ErrForbidden", err)
	}
}

func TestEngine_ListFindings_TenantScoped(t *testing.T) {
	t.Parallel()
	engine, tenantA := newTestEngine(t)
	tenantB := uuid.New()

	recordTestFinding(t, engine, tenantA)

	adminB := adminUser(tenantB)
	_, err := engine.RecordFinding(ctxWithUser(adminB), tenantB, newTestFinding(tenantB))
	if err != nil {
		t.Fatalf("RecordFinding(tenantB): %v", err)
	}

	viewerA := auditorUser(tenantA)
	listA, err := engine.ListFindings(ctxWithUser(viewerA), tenantA)
	if err != nil {
		t.Fatalf("ListFindings(tenantA): %v", err)
	}
	if len(listA) != 1 {
		t.Fatalf("len(listA) = %d, want 1", len(listA))
	}

	// A tenantA-scoped actor can never list tenantB's findings.
	_, err = engine.ListFindings(ctxWithUser(viewerA), tenantB)
	if !errors.Is(err, vulnmanagement.ErrCrossTenantAccess) {
		t.Fatalf("ListFindings(viewerA, tenantB) error = %v, want ErrCrossTenantAccess", err)
	}
}

func TestEngine_ListFindingsBySource(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	sca := newTestFinding(tenantID)
	sca.Source = vulnmanagement.ScannerSourceSCA
	sca.AdvisoryID = "CVE-2024-0001"
	if _, err := engine.RecordFinding(ctxWithUser(admin), tenantID, sca); err != nil {
		t.Fatalf("RecordFinding(sca): %v", err)
	}

	sast := newTestFinding(tenantID)
	sast.Source = vulnmanagement.ScannerSourceSAST
	sast.AdvisoryID = "RULE-0001"
	if _, err := engine.RecordFinding(ctxWithUser(admin), tenantID, sast); err != nil {
		t.Fatalf("RecordFinding(sast): %v", err)
	}

	viewer := auditorUser(tenantID)
	list, err := engine.ListFindingsBySource(ctxWithUser(viewer), tenantID, vulnmanagement.ScannerSourceSAST)
	if err != nil {
		t.Fatalf("ListFindingsBySource: %v", err)
	}
	if len(list) != 1 || list[0].Source != vulnmanagement.ScannerSourceSAST {
		t.Fatalf("ListFindingsBySource = %v, want exactly one SAST finding", list)
	}
}

func TestEngine_ListFindingsByStatus(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	f := recordTestFinding(t, engine, tenantID)
	if _, err := engine.Triage(ctxWithUser(admin), tenantID, f.ID, vulnmanagement.StatusTriaged, "triaging for status filter test", uuid.Nil); err != nil {
		t.Fatalf("Triage: %v", err)
	}
	recordTestFinding(t, engine, tenantID) // stays StatusOpen

	viewer := auditorUser(tenantID)
	open, err := engine.ListFindingsByStatus(ctxWithUser(viewer), tenantID, vulnmanagement.StatusOpen)
	if err != nil {
		t.Fatalf("ListFindingsByStatus(Open): %v", err)
	}
	if len(open) != 1 {
		t.Fatalf("len(open) = %d, want 1", len(open))
	}

	triaged, err := engine.ListFindingsByStatus(ctxWithUser(viewer), tenantID, vulnmanagement.StatusTriaged)
	if err != nil {
		t.Fatalf("ListFindingsByStatus(Triaged): %v", err)
	}
	if len(triaged) != 1 || triaged[0].ID != f.ID {
		t.Fatalf("ListFindingsByStatus(Triaged) = %v, want exactly the triaged finding", triaged)
	}
}

func TestEngine_Activity_SurfacesAuditEvents(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	if _, err := engine.RecordFinding(ctxWithUser(admin), tenantID, newTestFinding(tenantID)); err != nil {
		t.Fatalf("RecordFinding: %v", err)
	}

	events, err := engine.Activity(ctxWithUser(admin), tenantID, auditlog.Filter{})
	if err != nil {
		t.Fatalf("Activity: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("Activity returned no events, want at least the finding_record event")
	}
}

func TestEngine_Activity_RequiresViewPermission(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)

	noPerms := newTestUser(tenantID)
	_, err := engine.Activity(ctxWithUser(noPerms), tenantID, auditlog.Filter{})
	if !errors.Is(err, vulnmanagement.ErrForbidden) {
		t.Fatalf("Activity(no perms) error = %v, want ErrForbidden", err)
	}
}

func TestEngine_RecordFinding_AuditsSuccessAndDenial(t *testing.T) {
	t.Parallel()
	engine, auditStore, tenantID := newTestEngineWithAudit(t)
	admin := adminUser(tenantID)

	if _, err := engine.RecordFinding(ctxWithUser(admin), tenantID, newTestFinding(tenantID)); err != nil {
		t.Fatalf("RecordFinding: %v", err)
	}

	viewer := auditorUser(tenantID)
	if _, err := engine.RecordFinding(ctxWithUser(viewer), tenantID, newTestFinding(tenantID)); err == nil {
		t.Fatal("RecordFinding(viewer) unexpectedly succeeded")
	}

	events, err := auditStore.Query(ctxWithUser(admin), tenantID, auditlog.Filter{})
	if err != nil {
		t.Fatalf("auditStore.Query: %v", err)
	}

	var recorded, denied int
	for _, ev := range events {
		if ev.Action != "vulnmanagement.finding_record" {
			continue
		}
		switch ev.Outcome {
		case "recorded":
			recorded++
		case "denied":
			denied++
		}
	}
	if recorded != 1 {
		t.Errorf("recorded finding_record events = %d, want 1", recorded)
	}
	if denied != 1 {
		t.Errorf("denied finding_record events = %d, want 1", denied)
	}
}
