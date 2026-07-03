package caseversioning_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/caseversioning"
	"github.com/YASSERRMD/verdex/packages/identity"
	"github.com/YASSERRMD/verdex/packages/irac"
	"github.com/YASSERRMD/verdex/packages/synthesisagent"
)

// TestService_SnapshotCaseMetadata_CapturesLiveFields proves
// SnapshotCaseMetadata records a Snapshot whose CaseMetadataPayload
// matches the case's current mutable fields, and that CreatedBy/Reason
// carry the acting user and attribution note.
func TestService_SnapshotCaseMetadata_CapturesLiveFields(t *testing.T) {
	svc, c, _, tenantID := newTestService(t)
	user := newTestUser(tenantID, identity.RoleClerk)

	snap, err := svc.SnapshotCaseMetadata(ctxWithUser(user), tenantID, c.ID, "manual edit", "Initial draft")
	if err != nil {
		t.Fatalf("SnapshotCaseMetadata: %v", err)
	}
	if snap.ArtifactKind != caseversioning.ArtifactCaseMetadata {
		t.Fatalf("ArtifactKind = %v, want ArtifactCaseMetadata", snap.ArtifactKind)
	}
	if snap.CreatedBy != user.ID {
		t.Fatalf("CreatedBy = %v, want %v", snap.CreatedBy, user.ID)
	}
	if snap.Reason != "manual edit" || snap.Label != "Initial draft" {
		t.Fatalf("Reason/Label = %q/%q, want manual edit/Initial draft", snap.Reason, snap.Label)
	}

	payload, err := caseversioning.AsCaseMetadataPayload(snap)
	if err != nil {
		t.Fatalf("AsCaseMetadataPayload: %v", err)
	}
	if payload.Title != c.Title || payload.State != c.State.String() {
		t.Fatalf("payload = %+v, want title=%q state=%q", payload, c.Title, c.State)
	}
}

// TestService_SnapshotTree_ReferencesRevisionNumber proves a tree
// snapshot stores the real irac.TreeRevision.RevisionNumber as its
// ArtifactRevisionRef rather than a copy of the tree.
func TestService_SnapshotTree_ReferencesRevisionNumber(t *testing.T) {
	svc, c, _, tenantID := newTestService(t)
	user := newTestUser(tenantID, identity.RoleClerk)

	rev := irac.NewInitialRevision(c.ID.String(), time.Now())
	snap, err := svc.SnapshotTree(ctxWithUser(user), tenantID, c.ID, rev, "tree assembled", "")
	if err != nil {
		t.Fatalf("SnapshotTree: %v", err)
	}
	if snap.ArtifactKind != caseversioning.ArtifactTree {
		t.Fatalf("ArtifactKind = %v, want ArtifactTree", snap.ArtifactKind)
	}
	if snap.ArtifactRevisionRef != "1" {
		t.Fatalf("ArtifactRevisionRef = %q, want %q", snap.ArtifactRevisionRef, "1")
	}
	if snap.Payload != nil {
		t.Fatalf("Payload = %+v, want nil (tree snapshots reference, not copy)", snap.Payload)
	}
}

// TestService_SnapshotEvidence_ReferencesUpstreamID proves an evidence
// snapshot stores the caller-supplied upstream revision reference (e.g.
// an annotation ID) rather than a copy.
func TestService_SnapshotEvidence_ReferencesUpstreamID(t *testing.T) {
	svc, c, _, tenantID := newTestService(t)
	user := newTestUser(tenantID, identity.RoleClerk)

	snap, err := svc.SnapshotEvidence(ctxWithUser(user), tenantID, c.ID, "annotation-123", "new evidence segment", "")
	if err != nil {
		t.Fatalf("SnapshotEvidence: %v", err)
	}
	if snap.ArtifactKind != caseversioning.ArtifactEvidence {
		t.Fatalf("ArtifactKind = %v, want ArtifactEvidence", snap.ArtifactKind)
	}
	if snap.ArtifactRevisionRef != "annotation-123" {
		t.Fatalf("ArtifactRevisionRef = %q, want annotation-123", snap.ArtifactRevisionRef)
	}
}

// TestService_SnapshotOpinion_CapturesCompactCopy proves
// SnapshotOpinion records a compact OpinionPayload copy of the given
// synthesisagent.Opinion, since no upstream package versions Opinion
// output.
func TestService_SnapshotOpinion_CapturesCompactCopy(t *testing.T) {
	svc, c, _, tenantID := newTestService(t)
	user := newTestUser(tenantID, identity.RoleClerk)

	op := synthesisagent.Opinion{
		CaseID: c.ID.String(),
		Conclusions: []synthesisagent.TentativeConclusion{
			{IssueNodeID: "issue-1", Text: "Draft analysis", Confidence: 0.8},
		},
		GeneratedAt: time.Now().UTC(),
	}

	snap, err := svc.SnapshotOpinion(ctxWithUser(user), tenantID, c.ID, op, "synthesis run", "")
	if err != nil {
		t.Fatalf("SnapshotOpinion: %v", err)
	}
	if snap.ArtifactKind != caseversioning.ArtifactOpinion {
		t.Fatalf("ArtifactKind = %v, want ArtifactOpinion", snap.ArtifactKind)
	}

	payload, err := caseversioning.AsOpinionPayload(snap)
	if err != nil {
		t.Fatalf("AsOpinionPayload: %v", err)
	}
	if payload.ConclusionCount != 1 || len(payload.Conclusions) != 1 {
		t.Fatalf("payload = %+v, want 1 conclusion", payload)
	}
	if payload.Conclusions[0].IssueNodeID != "issue-1" {
		t.Fatalf("Conclusions[0].IssueNodeID = %q, want issue-1", payload.Conclusions[0].IssueNodeID)
	}
}

// TestService_History_ReturnsChronologicalTimeline proves History
// returns every snapshot for a case across artifact kinds, ordered
// oldest-first.
func TestService_History_ReturnsChronologicalTimeline(t *testing.T) {
	svc, c, _, tenantID := newTestService(t)
	user := newTestUser(tenantID, identity.RoleClerk)
	ctx := ctxWithUser(user)

	if _, err := svc.SnapshotCaseMetadata(ctx, tenantID, c.ID, "initial", ""); err != nil {
		t.Fatalf("SnapshotCaseMetadata: %v", err)
	}
	if _, err := svc.SnapshotTree(ctx, tenantID, c.ID, irac.NewInitialRevision(c.ID.String(), time.Now()), "tree assembled", ""); err != nil {
		t.Fatalf("SnapshotTree: %v", err)
	}

	history, err := svc.History(ctx, tenantID, c.ID, caseversioning.SnapshotFilter{})
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("len(history) = %d, want 2", len(history))
	}
	if history[0].ArtifactKind != caseversioning.ArtifactCaseMetadata || history[1].ArtifactKind != caseversioning.ArtifactTree {
		t.Fatalf("history order = [%v, %v], want [case-metadata, tree]", history[0].ArtifactKind, history[1].ArtifactKind)
	}

	filtered, err := svc.History(ctx, tenantID, c.ID, caseversioning.SnapshotFilter{Kind: caseversioning.ArtifactTree})
	if err != nil {
		t.Fatalf("History filtered: %v", err)
	}
	if len(filtered) != 1 || filtered[0].ArtifactKind != caseversioning.ArtifactTree {
		t.Fatalf("filtered history = %+v, want one tree snapshot", filtered)
	}
}

// TestService_UnauthenticatedAndForbidden_Errors proves the standard
// access-control gate: no user on ctx yields ErrUnauthenticated, a user
// without the required permission yields ErrForbidden.
func TestService_UnauthenticatedAndForbidden_Errors(t *testing.T) {
	svc, c, _, tenantID := newTestService(t)

	if _, err := svc.SnapshotCaseMetadata(context.Background(), tenantID, c.ID, "x", ""); !errors.Is(err, caseversioning.ErrUnauthenticated) {
		t.Fatalf("unauthenticated error = %v, want ErrUnauthenticated", err)
	}

	viewer := newTestUser(tenantID) // no roles => no permissions
	if _, err := svc.SnapshotCaseMetadata(ctxWithUser(viewer), tenantID, c.ID, "x", ""); !errors.Is(err, caseversioning.ErrForbidden) {
		t.Fatalf("forbidden error = %v, want ErrForbidden", err)
	}
}
