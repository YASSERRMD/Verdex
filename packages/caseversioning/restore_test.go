package caseversioning_test

import (
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/caseversioning"
	"github.com/YASSERRMD/verdex/packages/identity"
)

// TestService_Restore_RevertsFieldsAndRecordsNewSnapshot proves Restore
// reverts the live Case's fields to a prior ArtifactCaseMetadata
// snapshot's payload, and records a brand-new forward-only Snapshot
// (never rewriting the snapshot history) whose RestoredFromID points
// back at the source snapshot.
func TestService_Restore_RevertsFieldsAndRecordsNewSnapshot(t *testing.T) {
	svc, c, caseRepo, tenantID := newTestService(t)
	user := newTestUser(tenantID, identity.RoleClerk)
	ctx := ctxWithUser(user)

	original, err := svc.SnapshotCaseMetadata(ctx, tenantID, c.ID, "initial", "Original")
	if err != nil {
		t.Fatalf("SnapshotCaseMetadata (original): %v", err)
	}
	originalTitle := c.Title
	originalVersion := c.MetadataVersion

	c.Title = "Doe v. Acme Corp (Amended)"
	c.Reference = "2026-CV-999"
	if err := caseRepo.Update(ctx, tenantID, c); err != nil {
		t.Fatalf("caseRepo.Update: %v", err)
	}
	if _, err := svc.SnapshotCaseMetadata(ctx, tenantID, c.ID, "manual edit", "Amended"); err != nil {
		t.Fatalf("SnapshotCaseMetadata (amended): %v", err)
	}

	beforeHistory, err := svc.History(ctx, tenantID, c.ID, caseversioning.SnapshotFilter{})
	if err != nil {
		t.Fatalf("History (before restore): %v", err)
	}
	if len(beforeHistory) != 2 {
		t.Fatalf("len(beforeHistory) = %d, want 2", len(beforeHistory))
	}

	restored, err := svc.Restore(ctx, tenantID, original.ID)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if restored.RestoredFromID == nil || *restored.RestoredFromID != original.ID {
		t.Fatalf("RestoredFromID = %v, want pointer to %v", restored.RestoredFromID, original.ID)
	}
	if !restored.IsRestore() {
		t.Fatal("IsRestore() = false, want true")
	}

	live, err := caseRepo.Get(ctx, tenantID, c.ID)
	if err != nil {
		t.Fatalf("caseRepo.Get: %v", err)
	}
	if live.Title != originalTitle {
		t.Fatalf("live.Title = %q, want reverted to %q", live.Title, originalTitle)
	}
	if live.Reference != "" {
		t.Fatalf("live.Reference = %q, want reverted to empty", live.Reference)
	}
	if live.MetadataVersion <= originalVersion {
		t.Fatalf("live.MetadataVersion = %d, want > original version %d (restore bumps version)", live.MetadataVersion, originalVersion)
	}

	// History must have grown by exactly one entry — the restore
	// snapshot — never rewriting or removing the two prior entries.
	afterHistory, err := svc.History(ctx, tenantID, c.ID, caseversioning.SnapshotFilter{})
	if err != nil {
		t.Fatalf("History (after restore): %v", err)
	}
	if len(afterHistory) != 3 {
		t.Fatalf("len(afterHistory) = %d, want 3 (original, amended, restore)", len(afterHistory))
	}
	if afterHistory[0].ID != original.ID {
		t.Fatalf("afterHistory[0].ID = %v, want original snapshot %v unchanged", afterHistory[0].ID, original.ID)
	}
	if afterHistory[2].ID != restored.ID {
		t.Fatalf("afterHistory[2].ID = %v, want the new restore snapshot %v", afterHistory[2].ID, restored.ID)
	}
}

// TestService_Restore_RejectsNonMetadataArtifactKinds proves Restore
// refuses to act on a tree/evidence/opinion snapshot: those artifacts
// are owned by their own upstream packages, not reverted by this one.
func TestService_Restore_RejectsNonMetadataArtifactKinds(t *testing.T) {
	svc, c, _, tenantID := newTestService(t)
	user := newTestUser(tenantID, identity.RoleClerk)
	ctx := ctxWithUser(user)

	snap, err := svc.SnapshotEvidence(ctx, tenantID, c.ID, "evidence-1", "ingested", "")
	if err != nil {
		t.Fatalf("SnapshotEvidence: %v", err)
	}

	if _, err := svc.Restore(ctx, tenantID, snap.ID); !errors.Is(err, caseversioning.ErrNotRestorable) {
		t.Fatalf("Restore error = %v, want ErrNotRestorable", err)
	}
}

// TestService_Restore_RequiresEditPermission proves Restore is gated on
// identity.PermEditCase like every other write in this package.
func TestService_Restore_RequiresEditPermission(t *testing.T) {
	svc, c, _, tenantID := newTestService(t)
	writer := newTestUser(tenantID, identity.RoleClerk)
	viewer := newTestUser(tenantID)

	snap, err := svc.SnapshotCaseMetadata(ctxWithUser(writer), tenantID, c.ID, "initial", "")
	if err != nil {
		t.Fatalf("SnapshotCaseMetadata: %v", err)
	}

	if _, err := svc.Restore(ctxWithUser(viewer), tenantID, snap.ID); !errors.Is(err, caseversioning.ErrForbidden) {
		t.Fatalf("Restore (viewer) error = %v, want ErrForbidden", err)
	}
}
