package pilot_test

import (
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/pilot"
)

func TestEngine_RecordFinding_StartsAtOpen(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	d := provisionAndActivate(t, te)
	pc := assignTestCase(t, te, d.ID)
	entry := submitTestFeedback(t, te, pc.ID)

	f := recordTestFinding(t, te, d.ID, entry.ID)
	if f.Status != pilot.FindingStatusOpen {
		t.Fatalf("Status = %q, want %q", f.Status, pilot.FindingStatusOpen)
	}
}

func TestEngine_TriageFinding_RequiresNonBlankNotes(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	d := provisionAndActivate(t, te)
	pc := assignTestCase(t, te, d.ID)
	entry := submitTestFeedback(t, te, pc.ID)
	f := recordTestFinding(t, te, d.ID, entry.ID)
	admin := adminUser(te.tenantID)

	_, err := te.engine.TriageFinding(ctxWithUser(admin), te.tenantID, f.ID, pilot.PriorityHigh, pilot.FindingStatusTriaged, "   ")
	if !errors.Is(err, pilot.ErrNotesRequired) {
		t.Fatalf("error = %v, want ErrNotesRequired", err)
	}
}

func TestEngine_TriageFinding_AppliesLegalTransition(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	d := provisionAndActivate(t, te)
	pc := assignTestCase(t, te, d.ID)
	entry := submitTestFeedback(t, te, pc.ID)
	f := recordTestFinding(t, te, d.ID, entry.ID)
	admin := adminUser(te.tenantID)

	triaged, err := te.engine.TriageFinding(ctxWithUser(admin), te.tenantID, f.ID, pilot.PriorityHigh, pilot.FindingStatusTriaged, "Confirmed via three independent reviewer reports.")
	if err != nil {
		t.Fatalf("TriageFinding: %v", err)
	}
	if triaged.Priority != pilot.PriorityHigh {
		t.Fatalf("Priority = %q, want %q", triaged.Priority, pilot.PriorityHigh)
	}
	if triaged.Status != pilot.FindingStatusTriaged {
		t.Fatalf("Status = %q, want %q", triaged.Status, pilot.FindingStatusTriaged)
	}
	if triaged.TriagedBy == nil || *triaged.TriagedBy != admin.ID {
		t.Fatalf("TriagedBy = %v, want %v", triaged.TriagedBy, admin.ID)
	}
	if triaged.TriagedAt == nil {
		t.Fatal("TriagedAt should be set")
	}
}

func TestEngine_TriageFinding_RejectsIllegalTransition(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	d := provisionAndActivate(t, te)
	pc := assignTestCase(t, te, d.ID)
	entry := submitTestFeedback(t, te, pc.ID)
	f := recordTestFinding(t, te, d.ID, entry.ID)
	admin := adminUser(te.tenantID)

	// Open -> InProgress directly (skipping Triaged) is not a legal
	// move.
	_, err := te.engine.TriageFinding(ctxWithUser(admin), te.tenantID, f.ID, pilot.PriorityHigh, pilot.FindingStatusInProgress, "attempting an illegal skip")
	if !errors.Is(err, pilot.ErrIllegalStatusTransition) {
		t.Fatalf("error = %v, want ErrIllegalStatusTransition", err)
	}
}

func TestEngine_TriageFinding_RequiresManagePermission(t *testing.T) {
	t.Parallel()
	te := newTestEngine(t)
	d := provisionAndActivate(t, te)
	pc := assignTestCase(t, te, d.ID)
	entry := submitTestFeedback(t, te, pc.ID)
	f := recordTestFinding(t, te, d.ID, entry.ID)
	auditor := auditorUser(te.tenantID)

	_, err := te.engine.TriageFinding(ctxWithUser(auditor), te.tenantID, f.ID, pilot.PriorityHigh, pilot.FindingStatusTriaged, "should be denied")
	if !errors.Is(err, pilot.ErrForbidden) {
		t.Fatalf("error = %v, want ErrForbidden", err)
	}
}

func TestCanTransitionFinding_Table(t *testing.T) {
	t.Parallel()
	cases := []struct {
		from, to pilot.FindingStatus
		want     bool
	}{
		{pilot.FindingStatusOpen, pilot.FindingStatusTriaged, true},
		{pilot.FindingStatusOpen, pilot.FindingStatusWontFix, true},
		{pilot.FindingStatusTriaged, pilot.FindingStatusInProgress, true},
		{pilot.FindingStatusInProgress, pilot.FindingStatusResolved, true},
		{pilot.FindingStatusInProgress, pilot.FindingStatusTriaged, true},
		{pilot.FindingStatusOpen, pilot.FindingStatusInProgress, false},
		{pilot.FindingStatusOpen, pilot.FindingStatusResolved, false},
		{pilot.FindingStatusResolved, pilot.FindingStatusOpen, false},
		{pilot.FindingStatusWontFix, pilot.FindingStatusTriaged, false},
	}
	for _, c := range cases {
		got := pilot.CanTransitionFinding(c.from, c.to)
		if got != c.want {
			t.Errorf("CanTransitionFinding(%q, %q) = %v, want %v", c.from, c.to, got, c.want)
		}
	}
}

func TestSortFindingsByPriorityDesc(t *testing.T) {
	t.Parallel()
	findings := []pilot.PilotFinding{
		{Title: "low", Priority: pilot.PriorityLow},
		{Title: "critical", Priority: pilot.PriorityCritical},
		{Title: "medium", Priority: pilot.PriorityMedium},
		{Title: "high", Priority: pilot.PriorityHigh},
	}
	sorted := pilot.SortFindingsByPriorityDesc(findings)
	wantOrder := []string{"critical", "high", "medium", "low"}
	if len(sorted) != len(wantOrder) {
		t.Fatalf("len(sorted) = %d, want %d", len(sorted), len(wantOrder))
	}
	for i, want := range wantOrder {
		if sorted[i].Title != want {
			t.Errorf("sorted[%d].Title = %q, want %q", i, sorted[i].Title, want)
		}
	}
	// The original slice must be untouched.
	if findings[0].Title != "low" {
		t.Errorf("SortFindingsByPriorityDesc mutated its input slice: findings[0].Title = %q, want %q", findings[0].Title, "low")
	}
}
