package pilot_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/pilot"
)

func TestEngine_AssignCase_RequiresActiveDeployment(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	admin := adminUser(te.tenantID)

	d, err := te.engine.ProvisionDeployment(ctxWithUser(admin), te.tenantID, pilot.PilotDeployment{
		Name:             "Test pilot",
		JurisdictionCode: "AE-DXB-COMM",
		StartDate:        pilotNow(),
	})
	if err != nil {
		t.Fatalf("ProvisionDeployment: %v", err)
	}

	// Still Provisioning, not Active -- AssignCase should refuse.
	_, err = te.engine.AssignCase(ctxWithUser(admin), te.tenantID, d.ID, uuid.New(), uuid.New())
	if !errors.Is(err, pilot.ErrIllegalStatusTransition) {
		t.Fatalf("error = %v, want ErrIllegalStatusTransition", err)
	}
}

func TestEngine_AssignCase_SucceedsOnceActive(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	d := provisionAndActivate(t, te)

	caseID := uuid.New()
	supervisorID := uuid.New()
	admin := adminUser(te.tenantID)

	pc, err := te.engine.AssignCase(ctxWithUser(admin), te.tenantID, d.ID, caseID, supervisorID)
	if err != nil {
		t.Fatalf("AssignCase: %v", err)
	}
	if pc.CaseID != caseID {
		t.Fatalf("CaseID = %v, want %v", pc.CaseID, caseID)
	}
	if pc.SupervisorUserID != supervisorID {
		t.Fatalf("SupervisorUserID = %v, want %v", pc.SupervisorUserID, supervisorID)
	}
	if pc.OutcomeObserved {
		t.Fatal("OutcomeObserved should start false")
	}
}

func TestEngine_MarkOutcomeObserved_IsIdempotent(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	d := provisionAndActivate(t, te)
	pc := assignTestCase(t, te, d.ID)
	admin := adminUser(te.tenantID)

	first, err := te.engine.MarkOutcomeObserved(ctxWithUser(admin), te.tenantID, pc.ID)
	if err != nil {
		t.Fatalf("MarkOutcomeObserved (first): %v", err)
	}
	if !first.OutcomeObserved || first.ObservedAt == nil {
		t.Fatal("expected OutcomeObserved=true and ObservedAt set")
	}

	second, err := te.engine.MarkOutcomeObserved(ctxWithUser(admin), te.tenantID, pc.ID)
	if err != nil {
		t.Fatalf("MarkOutcomeObserved (second): %v", err)
	}
	if !second.ObservedAt.Equal(*first.ObservedAt) {
		t.Fatalf("ObservedAt changed on second call: first=%v second=%v", first.ObservedAt, second.ObservedAt)
	}
}

func TestEngine_ListCasesForDeployment(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	d := provisionAndActivate(t, te)
	admin := adminUser(te.tenantID)

	assignTestCase(t, te, d.ID)
	assignTestCase(t, te, d.ID)

	list, err := te.engine.ListCasesForDeployment(ctxWithUser(admin), te.tenantID, d.ID)
	if err != nil {
		t.Fatalf("ListCasesForDeployment: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len(list) = %d, want 2", len(list))
	}
}
