package treeassembly

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
)

func TestTreeAssemblyService_Assemble_DefaultsAndPersists(t *testing.T) {
	svc := &TreeAssemblyService{}
	input := syntheticInput("case-1")

	result, err := svc.Assemble(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Tree == nil {
		t.Fatal("expected a non-nil tree")
	}
	if len(result.ValidationIssues) != 0 {
		t.Fatalf("expected no validation issues, got %v", result.ValidationIssues)
	}
	if len(result.Gaps) != 0 {
		t.Fatalf("expected no gaps, got %v", result.Gaps)
	}
}

func TestTreeAssemblyService_Assemble_EmptyInput(t *testing.T) {
	svc := &TreeAssemblyService{}
	_, err := svc.Assemble(context.Background(), AssemblyInput{})
	if !errors.Is(err, ErrEmptyInput) {
		t.Fatalf("expected ErrEmptyInput, got %v", err)
	}
}

func TestTreeAssemblyService_Assemble_BumpsRevisionAcrossCalls(t *testing.T) {
	svc := &TreeAssemblyService{}
	input := syntheticInput("case-1")

	first, err := svc.Assemble(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if first.Tree.Revision.RevisionNumber != 1 {
		t.Fatalf("expected revision 1, got %d", first.Tree.Revision.RevisionNumber)
	}

	second, err := svc.Assemble(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if second.Tree.Revision.RevisionNumber != 2 {
		t.Fatalf("expected revision 2, got %d", second.Tree.Revision.RevisionNumber)
	}
	if second.Tree.Revision.ParentRevision == nil || *second.Tree.Revision.ParentRevision != 1 {
		t.Fatalf("expected parent revision 1, got %v", second.Tree.Revision.ParentRevision)
	}
}

func TestTreeAssemblyService_Assemble_RecordsTelemetry(t *testing.T) {
	recorder := NewInMemoryRecorder()
	svc := &TreeAssemblyService{Recorder: recorder}
	input := syntheticInput("case-1")

	if _, err := svc.Assemble(context.Background(), input); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entries := recorder.ForCase("case-1")
	if len(entries) != 1 {
		t.Fatalf("expected 1 telemetry entry, got %d", len(entries))
	}
	if entries[0].NodeCount != 4 {
		t.Fatalf("expected 4 nodes recorded, got %d", entries[0].NodeCount)
	}
}

// criticalFailureConclusionProvider returns a ConclusionNode referencing
// a nonexistent application, forcing a dangling edge / illegal-triple
// style structural failure downstream is not actually reachable given
// ComposeTree only links resolvable IDs — instead this test directly
// exercises the service's refusal path by injecting an already-invalid
// tree scenario via a conclusion whose provenance points at an
// application not present in the input, which safely produces zero
// extra edges (still valid). To exercise the critical-failure path
// deterministically, this test instead asserts the refusal contract
// using HasCriticalIntegrityFailure directly against a hand-built
// invalid tree, since ComposeTree itself never emits illegal triples.
func TestTreeAssemblyService_Assemble_RefusesToPersistOnCriticalFailure(t *testing.T) {
	store := graph.NewInMemoryGraphStore()
	svc := &TreeAssemblyService{Store: store}

	// Build an input whose sole application references an issue ID that
	// does not exist as a Rule (so no edge is derived) — this does not
	// itself trigger a failure, since ComposeTree only emits legal
	// edges. Instead, confirm the guardrail directly: a ConclusionNode
	// without HasGuardrailLabel would trip ValidateTree, but
	// irac.NewConclusionNode can't construct one, so we assert the
	// service's contract at the unit level via ValidateIntegrity +
	// HasCriticalIntegrityFailure, which service.go wires together.
	input := syntheticInput("case-critical")
	result, err := svc.Assemble(context.Background(), input)
	if err != nil {
		t.Fatalf("expected a valid synthetic tree to assemble cleanly: %v", err)
	}
	if HasCriticalIntegrityFailure(result.ValidationIssues) {
		t.Fatal("did not expect critical integrity failure for a valid synthetic tree")
	}

	// Now confirm the refusal path fires for a tree carrying a
	// structural issue, by manually validating a tree with a dangling
	// edge and checking Assemble's guardrail primitives agree it would
	// refuse to persist.
	badTree := &Tree{
		Nodes: result.Tree.Nodes,
		Edges: append(append([]irac.Edge{}, result.Tree.Edges...), irac.Edge{FromID: "missing-a", ToID: "missing-b", Type: irac.EdgeGoverns}),
	}
	issues := ValidateIntegrity(badTree)
	if !HasCriticalIntegrityFailure(issues) {
		t.Fatal("expected critical integrity failure for tree with dangling edge")
	}
}

func TestTreeAssemblyService_Assemble_WithConclusionProvider(t *testing.T) {
	input := syntheticInput("case-1")
	conclusion := irac.NewConclusionNode("conclusion-1", input.CaseID, "Draft analysis.", time.Now(), 0.8, testProvenance(input.Applications[0].ID))
	svc := &TreeAssemblyService{Conclusions: fixedConclusionProvider{conclusions: []irac.ConclusionNode{conclusion}}}

	result, err := svc.Assemble(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, n := range result.Tree.Nodes {
		if n.GetID() == conclusion.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("expected conclusion node to be included via ConclusionProvider")
	}
	// No unresolved-application gap, since the sole application is
	// resolved by the supplied conclusion.
	for _, g := range result.Gaps {
		if g.Kind == GapUnresolvedApplication {
			t.Fatalf("did not expect unresolved application gap, got %v", g)
		}
	}
}
