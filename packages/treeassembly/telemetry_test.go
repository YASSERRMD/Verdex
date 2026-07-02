package treeassembly

import (
	"context"
	"testing"
	"time"
)

func TestInMemoryRecorder_RecordAndRetrieve(t *testing.T) {
	rec := NewInMemoryRecorder()

	e1 := AssemblyTelemetry{CaseID: "case-1", NodeCount: 3}
	e2 := AssemblyTelemetry{CaseID: "case-2", NodeCount: 5}
	e3 := AssemblyTelemetry{CaseID: "case-1", NodeCount: 4}

	rec.Record(e1)
	rec.Record(e2)
	rec.Record(e3)

	all := rec.All()
	if len(all) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(all))
	}

	case1 := rec.ForCase("case-1")
	if len(case1) != 2 {
		t.Fatalf("expected 2 entries for case-1, got %d", len(case1))
	}
	if case1[0].NodeCount != 3 || case1[1].NodeCount != 4 {
		t.Fatalf("unexpected entries for case-1: %+v", case1)
	}
}

func TestInMemoryRecorder_ForCase_Empty(t *testing.T) {
	rec := NewInMemoryRecorder()
	got := rec.ForCase("nonexistent")
	if len(got) != 0 {
		t.Fatalf("expected no entries, got %d", len(got))
	}
}

func TestNewAssemblyTelemetry_CountsMatch(t *testing.T) {
	input := syntheticInput("case-1")
	tree, err := ComposeTree(context.Background(), input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	issues := ValidateIntegrity(tree)
	gaps := DetectGaps(tree)
	start := time.Now().Add(-10 * time.Millisecond)

	entry := NewAssemblyTelemetry(tree, len(issues), len(gaps), start)

	if entry.CaseID != tree.Revision.CaseID {
		t.Errorf("CaseID: got %q, want %q", entry.CaseID, tree.Revision.CaseID)
	}
	if entry.RevisionNumber != tree.Revision.RevisionNumber {
		t.Errorf("RevisionNumber: got %d, want %d", entry.RevisionNumber, tree.Revision.RevisionNumber)
	}
	if entry.NodeCount != len(tree.Nodes) {
		t.Errorf("NodeCount: got %d, want %d", entry.NodeCount, len(tree.Nodes))
	}
	if entry.EdgeCount != len(tree.Edges) {
		t.Errorf("EdgeCount: got %d, want %d", entry.EdgeCount, len(tree.Edges))
	}
	if entry.ValidationIssueCount != len(issues) {
		t.Errorf("ValidationIssueCount: got %d, want %d", entry.ValidationIssueCount, len(issues))
	}
	if entry.GapCount != len(gaps) {
		t.Errorf("GapCount: got %d, want %d", entry.GapCount, len(gaps))
	}
	if entry.Duration <= 0 {
		t.Errorf("expected positive duration, got %v", entry.Duration)
	}
}

func TestNewAssemblyTelemetry_NilTree(t *testing.T) {
	entry := NewAssemblyTelemetry(nil, 2, 1, time.Now())
	if entry.CaseID != "" || entry.NodeCount != 0 || entry.EdgeCount != 0 {
		t.Fatalf("expected zero-value tree fields, got %+v", entry)
	}
	if entry.ValidationIssueCount != 2 || entry.GapCount != 1 {
		t.Fatalf("expected counts to still be set, got %+v", entry)
	}
}
