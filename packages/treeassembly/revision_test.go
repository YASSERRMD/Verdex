package treeassembly

import (
	"context"
	"testing"
)

func TestNextRevision_NilTree(t *testing.T) {
	rev := NextRevision(nil)
	if rev.CaseID != "" || rev.RevisionNumber != 0 {
		t.Fatalf("expected zero-value revision for nil tree, got %+v", rev)
	}
}

func TestNextRevision_BumpsFromInitial(t *testing.T) {
	input := syntheticInput("case-1")
	tree, err := ComposeTree(context.Background(), input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tree.Revision.RevisionNumber != 1 {
		t.Fatalf("expected initial revision 1, got %d", tree.Revision.RevisionNumber)
	}
	if !tree.Revision.IsInitial() {
		t.Fatal("expected initial revision")
	}

	next := NextRevision(tree)
	if next.RevisionNumber != 2 {
		t.Fatalf("expected revision 2, got %d", next.RevisionNumber)
	}
	if next.ParentRevision == nil || *next.ParentRevision != 1 {
		t.Fatalf("expected parent revision 1, got %v", next.ParentRevision)
	}
	if !next.IsValidSuccessorOf(tree.Revision) {
		t.Fatal("expected next to be a valid successor of tree.Revision")
	}
}

func TestNextRevision_CaseIDPreserved(t *testing.T) {
	input := syntheticInput("case-42")
	tree, err := ComposeTree(context.Background(), input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	next := NextRevision(tree)
	if next.CaseID != "case-42" {
		t.Fatalf("expected case-42, got %q", next.CaseID)
	}
}
