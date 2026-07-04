package vulnmanagement_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/vulnmanagement"
)

func TestEngine_Triage_MovesStatusAndRecordsDecision(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	finding := recordTestFinding(t, engine, tenantID)
	admin := adminUser(tenantID)

	updated, err := engine.Triage(ctxWithUser(admin), tenantID, finding.ID, vulnmanagement.StatusTriaged, "Reviewed; confirmed applicable, scheduling remediation.", admin.ID)
	if err != nil {
		t.Fatalf("Triage: %v", err)
	}
	if updated.Status != vulnmanagement.StatusTriaged {
		t.Fatalf("updated.Status = %s, want StatusTriaged", updated.Status)
	}

	history, err := engine.TriageHistory(ctxWithUser(admin), tenantID, finding.ID)
	if err != nil {
		t.Fatalf("TriageHistory: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("len(history) = %d, want 1", len(history))
	}
	d := history[0]
	if d.FromStatus != vulnmanagement.StatusOpen || d.ToStatus != vulnmanagement.StatusTriaged {
		t.Errorf("history[0] from/to = %s/%s, want open/triaged", d.FromStatus, d.ToStatus)
	}
	if d.Actor != admin.ID {
		t.Errorf("history[0].Actor = %s, want %s", d.Actor, admin.ID)
	}
	if d.Notes == "" {
		t.Error("history[0].Notes is empty, want the recorded explanation")
	}
}

func TestEngine_Triage_RejectsBlankNotes(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	finding := recordTestFinding(t, engine, tenantID)
	admin := adminUser(tenantID)

	_, err := engine.Triage(ctxWithUser(admin), tenantID, finding.ID, vulnmanagement.StatusTriaged, "   ", uuid.Nil)
	if !errors.Is(err, vulnmanagement.ErrNotesRequired) {
		t.Fatalf("Triage(blank notes) error = %v, want ErrNotesRequired", err)
	}

	// The finding must remain untouched -- a rejected triage attempt
	// never partially applies.
	got, err := engine.GetFinding(ctxWithUser(admin), tenantID, finding.ID)
	if err != nil {
		t.Fatalf("GetFinding: %v", err)
	}
	if got.Status != vulnmanagement.StatusOpen {
		t.Errorf("got.Status = %s, want StatusOpen (unchanged)", got.Status)
	}
}

func TestEngine_Triage_RejectsIllegalTransition(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	finding := recordTestFinding(t, engine, tenantID)
	admin := adminUser(tenantID)

	// Open -> Remediating is not a legal direct transition.
	_, err := engine.Triage(ctxWithUser(admin), tenantID, finding.ID, vulnmanagement.StatusRemediating, "skip ahead", uuid.Nil)
	if !errors.Is(err, vulnmanagement.ErrIllegalStatusTransition) {
		t.Fatalf("Triage(illegal transition) error = %v, want ErrIllegalStatusTransition", err)
	}
}

func TestEngine_Triage_TerminalStatusIsSticky(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	finding := recordTestFinding(t, engine, tenantID)
	admin := adminUser(tenantID)

	if _, err := engine.Triage(ctxWithUser(admin), tenantID, finding.ID, vulnmanagement.StatusFalsePositive, "not actually reachable", uuid.Nil); err != nil {
		t.Fatalf("Triage(false positive): %v", err)
	}

	_, err := engine.Triage(ctxWithUser(admin), tenantID, finding.ID, vulnmanagement.StatusTriaged, "reopen attempt", uuid.Nil)
	if !errors.Is(err, vulnmanagement.ErrIllegalStatusTransition) {
		t.Fatalf("Triage(out of terminal state) error = %v, want ErrIllegalStatusTransition", err)
	}
}

func TestEngine_Triage_RequiresManagePermission(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	finding := recordTestFinding(t, engine, tenantID)

	viewer := auditorUser(tenantID)
	_, err := engine.Triage(ctxWithUser(viewer), tenantID, finding.ID, vulnmanagement.StatusTriaged, "attempted by a viewer", uuid.Nil)
	if !errors.Is(err, vulnmanagement.ErrForbidden) {
		t.Fatalf("Triage(viewer) error = %v, want ErrForbidden", err)
	}
}

func TestEngine_Triage_UnknownFinding(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	_, err := engine.Triage(ctxWithUser(admin), tenantID, uuid.New(), vulnmanagement.StatusTriaged, "no such finding", uuid.Nil)
	if !errors.Is(err, vulnmanagement.ErrFindingNotFound) {
		t.Fatalf("Triage(unknown finding) error = %v, want ErrFindingNotFound", err)
	}
}

func TestEngine_Triage_RemediationToResolvedFullLifecycle(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	finding := recordTestFinding(t, engine, tenantID)
	admin := adminUser(tenantID)

	steps := []vulnmanagement.Status{
		vulnmanagement.StatusTriaged,
		vulnmanagement.StatusRemediating,
		vulnmanagement.StatusResolved,
	}
	for _, next := range steps {
		updated, err := engine.Triage(ctxWithUser(admin), tenantID, finding.ID, next, "progressing lifecycle", uuid.Nil)
		if err != nil {
			t.Fatalf("Triage(%s): %v", next, err)
		}
		if updated.Status != next {
			t.Fatalf("updated.Status = %s, want %s", updated.Status, next)
		}
	}
}
