package caseversioning_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/YASSERRMD/verdex/packages/caseversioning"
	"github.com/YASSERRMD/verdex/packages/identity"
)

// TestService_Diff_CaseMetadata_FieldLevelChanges proves Diff produces a
// real field-by-field comparison for two ArtifactCaseMetadata
// snapshots: title, state, and a metadata key change are each reported.
func TestService_Diff_CaseMetadata_FieldLevelChanges(t *testing.T) {
	svc, c, caseRepo, tenantID := newTestService(t)
	user := newTestUser(tenantID, identity.RoleClerk)
	ctx := ctxWithUser(user)

	before, err := svc.SnapshotCaseMetadata(ctx, tenantID, c.ID, "initial", "")
	if err != nil {
		t.Fatalf("SnapshotCaseMetadata (before): %v", err)
	}

	c.Title = "Doe v. Acme Corp (Amended)"
	c.Metadata = map[string]string{"docket_number": "2026-CV-001"}
	if err := caseRepo.Update(ctx, tenantID, c); err != nil {
		t.Fatalf("caseRepo.Update: %v", err)
	}

	after, err := svc.SnapshotCaseMetadata(ctx, tenantID, c.ID, "manual edit", "")
	if err != nil {
		t.Fatalf("SnapshotCaseMetadata (after): %v", err)
	}

	diff, err := svc.Diff(ctx, tenantID, before.ID, after.ID)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if diff.Identical {
		t.Fatal("Identical = true, want false")
	}

	fields := make(map[string]caseversioning.FieldChange, len(diff.FieldChanges))
	for _, fc := range diff.FieldChanges {
		fields[fc.Field] = fc
	}
	titleChange, ok := fields["title"]
	if !ok {
		t.Fatalf("FieldChanges missing title, got %+v", diff.FieldChanges)
	}
	if titleChange.Before != "Doe v. Acme Corp" || titleChange.After != "Doe v. Acme Corp (Amended)" {
		t.Fatalf("title change = %+v, want before=Doe v. Acme Corp after=...Amended", titleChange)
	}
	docketChange, ok := fields["metadata[docket_number]"]
	if !ok {
		t.Fatalf("FieldChanges missing metadata[docket_number], got %+v", diff.FieldChanges)
	}
	if docketChange.Before != "" || docketChange.After != "2026-CV-001" {
		t.Fatalf("docket change = %+v, want before=\"\" after=2026-CV-001", docketChange)
	}
}

// TestService_Diff_CaseMetadata_Identical proves Diff reports Identical
// when nothing changed between two snapshots.
func TestService_Diff_CaseMetadata_Identical(t *testing.T) {
	svc, c, _, tenantID := newTestService(t)
	user := newTestUser(tenantID, identity.RoleClerk)
	ctx := ctxWithUser(user)

	a, err := svc.SnapshotCaseMetadata(ctx, tenantID, c.ID, "a", "")
	if err != nil {
		t.Fatalf("SnapshotCaseMetadata a: %v", err)
	}
	b, err := svc.SnapshotCaseMetadata(ctx, tenantID, c.ID, "b", "")
	if err != nil {
		t.Fatalf("SnapshotCaseMetadata b: %v", err)
	}

	diff, err := svc.Diff(ctx, tenantID, a.ID, b.ID)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if !diff.Identical {
		t.Fatalf("Identical = false, want true; FieldChanges = %+v", diff.FieldChanges)
	}
}

// TestComputeDiff_Tree_ReferenceLevel proves Diff for ArtifactTree
// snapshots reports which revision ref changed rather than a field
// diff.
func TestComputeDiff_Tree_ReferenceLevel(t *testing.T) {
	caseID := uuid.New()
	tenantID := uuid.New()
	actor := uuid.New()

	a := &caseversioning.Snapshot{
		ID: uuid.New(), CaseID: caseID, TenantID: tenantID,
		ArtifactKind: caseversioning.ArtifactTree, ArtifactRevisionRef: "1", CreatedBy: actor,
	}
	b := &caseversioning.Snapshot{
		ID: uuid.New(), CaseID: caseID, TenantID: tenantID,
		ArtifactKind: caseversioning.ArtifactTree, ArtifactRevisionRef: "2", CreatedBy: actor,
	}

	diff, err := caseversioning.ComputeDiff(a, b)
	if err != nil {
		t.Fatalf("ComputeDiff: %v", err)
	}
	if !diff.RevisionRefChanged {
		t.Fatal("RevisionRefChanged = false, want true")
	}
	if diff.RevisionRefBefore != "1" || diff.RevisionRefAfter != "2" {
		t.Fatalf("revision refs = %q -> %q, want 1 -> 2", diff.RevisionRefBefore, diff.RevisionRefAfter)
	}
	if len(diff.FieldChanges) != 0 {
		t.Fatalf("FieldChanges = %+v, want none for a reference-level artifact", diff.FieldChanges)
	}
	if diff.Identical {
		t.Fatal("Identical = true, want false")
	}
}

// TestComputeDiff_MismatchedCaseOrKind proves ComputeDiff rejects
// snapshots from different cases or of different artifact kinds.
func TestComputeDiff_MismatchedCaseOrKind(t *testing.T) {
	tenantID := uuid.New()
	actor := uuid.New()

	a := &caseversioning.Snapshot{ID: uuid.New(), CaseID: uuid.New(), TenantID: tenantID, ArtifactKind: caseversioning.ArtifactTree, CreatedBy: actor}
	b := &caseversioning.Snapshot{ID: uuid.New(), CaseID: uuid.New(), TenantID: tenantID, ArtifactKind: caseversioning.ArtifactTree, CreatedBy: actor}

	if _, err := caseversioning.ComputeDiff(a, b); !errors.Is(err, caseversioning.ErrMismatchedCase) {
		t.Fatalf("mismatched case error = %v, want ErrMismatchedCase", err)
	}

	sameCase := a.CaseID
	b.CaseID = sameCase
	b.ArtifactKind = caseversioning.ArtifactOpinion
	if _, err := caseversioning.ComputeDiff(a, b); !errors.Is(err, caseversioning.ErrMismatchedArtifactKind) {
		t.Fatalf("mismatched kind error = %v, want ErrMismatchedArtifactKind", err)
	}
}
