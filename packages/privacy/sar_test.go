package privacy_test

import (
	"errors"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/auditlog"
	"github.com/YASSERRMD/verdex/packages/privacy"
)

// TestCanTransitionSAR_AllowedMoves table-drives every transition
// allowedSARTransitions permits and every one it must reject,
// mirroring how packages/caselifecycle tests CanTransition.
func TestCanTransitionSAR_AllowedMoves(t *testing.T) {
	t.Parallel()

	tests := []struct {
		from, to privacy.SARStatus
		want     bool
	}{
		{privacy.SARStatusReceived, privacy.SARStatusInProgress, true},
		{privacy.SARStatusReceived, privacy.SARStatusRejected, true},
		{privacy.SARStatusReceived, privacy.SARStatusFulfilled, false},
		{privacy.SARStatusInProgress, privacy.SARStatusFulfilled, true},
		{privacy.SARStatusInProgress, privacy.SARStatusRejected, true},
		{privacy.SARStatusInProgress, privacy.SARStatusReceived, false},
		{privacy.SARStatusFulfilled, privacy.SARStatusInProgress, false},
		{privacy.SARStatusFulfilled, privacy.SARStatusReceived, false},
		{privacy.SARStatusRejected, privacy.SARStatusInProgress, false},
		{privacy.SARStatusRejected, privacy.SARStatusFulfilled, false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			t.Parallel()
			got := privacy.CanTransitionSAR(tt.from, tt.to)
			if got != tt.want {
				t.Fatalf("CanTransitionSAR(%q, %q) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestSARStatus_IsTerminal(t *testing.T) {
	t.Parallel()

	terminal := []privacy.SARStatus{privacy.SARStatusFulfilled, privacy.SARStatusRejected}
	for _, s := range terminal {
		if !s.IsTerminal() {
			t.Errorf("SARStatus(%q).IsTerminal() = false, want true", s)
		}
	}

	nonTerminal := []privacy.SARStatus{privacy.SARStatusReceived, privacy.SARStatusInProgress}
	for _, s := range nonTerminal {
		if s.IsTerminal() {
			t.Errorf("SARStatus(%q).IsTerminal() = true, want false", s)
		}
	}
}

func TestTransitionSAR_IllegalMoveRejected(t *testing.T) {
	t.Parallel()

	req := &privacy.SubjectAccessRequest{Status: privacy.SARStatusReceived}
	err := privacy.TransitionSAR(req, privacy.SARStatusFulfilled, time.Now(), "")
	if !errors.Is(err, privacy.ErrIllegalSARTransition) {
		t.Fatalf("TransitionSAR() error = %v, want ErrIllegalSARTransition", err)
	}
	// The illegal transition must not have mutated req.
	if req.Status != privacy.SARStatusReceived {
		t.Fatalf("req.Status = %q after rejected transition, want unchanged %q", req.Status, privacy.SARStatusReceived)
	}
}

func TestTransitionSAR_TerminalStampsResolvedAt(t *testing.T) {
	t.Parallel()

	req := &privacy.SubjectAccessRequest{Status: privacy.SARStatusReceived}
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	if err := privacy.TransitionSAR(req, privacy.SARStatusInProgress, now, ""); err != nil {
		t.Fatalf("TransitionSAR to in_progress: %v", err)
	}
	if req.ResolvedAt != nil {
		t.Fatal("ResolvedAt set on a non-terminal transition")
	}

	later := now.Add(time.Hour)
	if err := privacy.TransitionSAR(req, privacy.SARStatusFulfilled, later, "delivered via secure portal"); err != nil {
		t.Fatalf("TransitionSAR to fulfilled: %v", err)
	}
	if req.ResolvedAt == nil || !req.ResolvedAt.Equal(later) {
		t.Fatalf("ResolvedAt = %v, want %v", req.ResolvedAt, later)
	}
	if req.ResolutionNotes != "delivered via secure portal" {
		t.Fatalf("ResolutionNotes = %q, want the supplied notes", req.ResolutionNotes)
	}
}

func TestEngine_SubmitSAR_And_AdvanceSAR(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	created, err := engine.SubmitSAR(ctxWithUser(admin), tenantID, privacy.SubjectAccessRequest{SubjectID: "subject-1"})
	if err != nil {
		t.Fatalf("SubmitSAR: %v", err)
	}
	if created.Status != privacy.SARStatusReceived {
		t.Fatalf("created.Status = %q, want %q", created.Status, privacy.SARStatusReceived)
	}
	if created.DueAt.Before(created.ReceivedAt) || created.DueAt.Equal(created.ReceivedAt) {
		t.Fatalf("DueAt = %v, want strictly after ReceivedAt = %v", created.DueAt, created.ReceivedAt)
	}

	inProgress, err := engine.AdvanceSAR(ctxWithUser(admin), tenantID, created.ID, privacy.SARStatusInProgress, "")
	if err != nil {
		t.Fatalf("AdvanceSAR to in_progress: %v", err)
	}
	if inProgress.Status != privacy.SARStatusInProgress {
		t.Fatalf("inProgress.Status = %q, want %q", inProgress.Status, privacy.SARStatusInProgress)
	}

	fulfilled, err := engine.AdvanceSAR(ctxWithUser(admin), tenantID, created.ID, privacy.SARStatusFulfilled, "exported and delivered")
	if err != nil {
		t.Fatalf("AdvanceSAR to fulfilled: %v", err)
	}
	if fulfilled.Status != privacy.SARStatusFulfilled {
		t.Fatalf("fulfilled.Status = %q, want %q", fulfilled.Status, privacy.SARStatusFulfilled)
	}
	if fulfilled.ResolvedAt == nil {
		t.Fatal("fulfilled.ResolvedAt is nil, want non-nil after reaching a terminal status")
	}

	list, err := engine.ListSARsForSubject(ctxWithUser(admin), tenantID, "subject-1")
	if err != nil {
		t.Fatalf("ListSARsForSubject: %v", err)
	}
	if len(list) != 1 || list[0].Status != privacy.SARStatusFulfilled {
		t.Fatalf("ListSARsForSubject = %v, want exactly the fulfilled request", list)
	}
}

func TestEngine_AdvanceSAR_IllegalTransitionRejected(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	admin := adminUser(tenantID)

	created, err := engine.SubmitSAR(ctxWithUser(admin), tenantID, privacy.SubjectAccessRequest{SubjectID: "subject-1"})
	if err != nil {
		t.Fatalf("SubmitSAR: %v", err)
	}

	_, err = engine.AdvanceSAR(ctxWithUser(admin), tenantID, created.ID, privacy.SARStatusFulfilled, "")
	if !errors.Is(err, privacy.ErrIllegalSARTransition) {
		t.Fatalf("AdvanceSAR() error = %v, want ErrIllegalSARTransition", err)
	}
}

func TestEngine_AdvanceSAR_RecordsAuditEvent(t *testing.T) {
	t.Parallel()
	engine, auditStore, tenantID := newTestEngineWithAudit(t)
	admin := adminUser(tenantID)

	created, err := engine.SubmitSAR(ctxWithUser(admin), tenantID, privacy.SubjectAccessRequest{SubjectID: "subject-1"})
	if err != nil {
		t.Fatalf("SubmitSAR: %v", err)
	}
	if _, err := engine.AdvanceSAR(ctxWithUser(admin), tenantID, created.ID, privacy.SARStatusInProgress, ""); err != nil {
		t.Fatalf("AdvanceSAR: %v", err)
	}

	events, err := auditStore.Query(ctxWithUser(admin), tenantID, auditlog.Filter{})
	if err != nil {
		t.Fatalf("auditStore.Query: %v", err)
	}
	found := false
	for _, ev := range events {
		if ev.Target == created.ID.String() {
			found = true
		}
	}
	if !found {
		t.Fatalf("no audit event found targeting SAR %s among %d events", created.ID, len(events))
	}
}

func TestEngine_SubmitSAR_RequiresManagePermission(t *testing.T) {
	t.Parallel()
	engine, tenantID := newTestEngine(t)
	auditor := auditorUser(tenantID)

	_, err := engine.SubmitSAR(ctxWithUser(auditor), tenantID, privacy.SubjectAccessRequest{SubjectID: "subject-1"})
	if !errors.Is(err, privacy.ErrForbidden) {
		t.Fatalf("SubmitSAR() error = %v, want ErrForbidden", err)
	}
}
