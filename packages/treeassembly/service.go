package treeassembly

import (
	"context"
	"time"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
)

// TreeAssemblyService orchestrates the full tree-assembly pipeline:
//
//	compose -> validate integrity -> detect gaps -> version
//	  -> record telemetry -> persist -> return
//
// This mirrors packages/issue's IssueExtractionService,
// packages/fact's FactConstructionService, and
// packages/application's ApplicationService orchestration pattern: a
// single entry point wires together this package's otherwise
// independent, individually testable building blocks (ComposeTree,
// ValidateIntegrity, DetectGaps, NextRevision, AssemblyTelemetry,
// PersistTree).
type TreeAssemblyService struct {
	// Store persists the assembled Tree's nodes and edges. If nil, a
	// fresh graph.InMemoryGraphStore is used.
	Store graph.GraphStore

	// Snapshots persists full-tree snapshots keyed by case/revision. If
	// nil, a fresh InMemorySnapshotStore is used.
	Snapshots SnapshotStore

	// Conclusions supplies any irac.ConclusionNodes to include in the
	// assembled tree. If nil, NoOpConclusionProvider is used — see
	// compose.go's ConclusionProvider doc comment for why this package
	// does not synthesize conclusions itself.
	Conclusions ConclusionProvider

	// Recorder records AssemblyTelemetry for every Assemble call. If
	// nil, a fresh InMemoryRecorder is used.
	Recorder Recorder

	// prevRevisions tracks the most recently assembled Tree per case, so
	// a subsequent Assemble call for the same case bumps the revision
	// rather than starting over at revision 1.
	prevRevisions map[string]*Tree
}

// AssembleResult bundles everything TreeAssemblyService.Assemble
// produces: the assembled Tree, its structural validation issues, and
// its semantic gaps.
type AssembleResult struct {
	Tree             *Tree
	ValidationIssues []irac.ValidationIssue
	Gaps             []Gap
}

// Assemble runs the full pipeline over input:
//
//  1. compose the tree from input (and s.Conclusions, if any);
//  2. validate its structural integrity;
//  3. detect semantic gaps;
//  4. version it (bumping the revision if this case was already
//     assembled by this service instance);
//  5. record telemetry for this run;
//  6. persist it — unless a critical integrity failure was found, in
//     which case ErrCriticalIntegrityFailure is returned and nothing is
//     persisted, per CONTRIBUTING.md's guardrail spirit that reasoning
//     artifacts must be trustworthy before use.
//
// Returns the AssembleResult (Tree, ValidationIssues, Gaps) alongside
// any error. When ErrCriticalIntegrityFailure is returned, the Tree,
// ValidationIssues, and Gaps in the result are still populated so
// callers can inspect what went wrong.
func (s *TreeAssemblyService) Assemble(ctx context.Context, input AssemblyInput) (AssembleResult, error) {
	start := time.Now()

	store := s.Store
	if store == nil {
		store = graph.NewInMemoryGraphStore()
	}
	snapshots := s.Snapshots
	if snapshots == nil {
		snapshots = NewInMemorySnapshotStore()
	}
	conclusions := s.Conclusions
	if conclusions == nil {
		conclusions = NoOpConclusionProvider{}
	}
	recorder := s.Recorder
	if recorder == nil {
		recorder = NewInMemoryRecorder()
	}

	// 1. compose
	tree, err := ComposeTree(ctx, input, conclusions)
	if err != nil {
		return AssembleResult{}, err
	}

	// 4a. version (bump if this case has a prior revision tracked by
	// this service instance) — done before validation/gap-detection so
	// the recorded telemetry and any persisted snapshot reflect the
	// correct revision number.
	if s.prevRevisions == nil {
		s.prevRevisions = make(map[string]*Tree)
	}
	if prev, ok := s.prevRevisions[input.CaseID]; ok {
		tree.Revision = NextRevision(prev)
	}
	s.prevRevisions[input.CaseID] = tree

	// 2. validate integrity
	issues := ValidateIntegrity(tree)

	// 3. detect gaps
	gaps := DetectGaps(tree)

	result := AssembleResult{Tree: tree, ValidationIssues: issues, Gaps: gaps}

	// 5. record telemetry
	entry := NewAssemblyTelemetry(tree, len(issues), len(gaps), start)
	recorder.Record(entry)

	// 6. persist, unless critical integrity failure
	if HasCriticalIntegrityFailure(issues) {
		return result, ErrCriticalIntegrityFailure
	}

	if err := PersistTree(ctx, store, snapshots, tree); err != nil {
		return result, err
	}

	return result, nil
}
