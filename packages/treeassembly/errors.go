package treeassembly

import "errors"

// Sentinel errors that callers can test with errors.Is.
var (
	// ErrEmptyInput is returned when AssemblyInput.CaseID is blank, or
	// when an assembly is attempted with no Issues, Facts, or
	// Applications at all (nothing to assemble).
	ErrEmptyInput = errors.New("treeassembly: empty assembly input")

	// ErrCriticalIntegrityFailure is returned by TreeAssemblyService when
	// the composed tree fails structural validation with issues that
	// make it untrustworthy to persist or reason over further (e.g.
	// dangling edges, illegal edge triples, a ConclusionNode missing its
	// mandatory draft_analysis guardrail label). Mirrors
	// CONTRIBUTING.md's guardrail spirit: a reasoning artifact must be
	// trustworthy before it is used, so assembly refuses to persist a
	// tree in this state rather than silently persisting a broken one.
	ErrCriticalIntegrityFailure = errors.New("treeassembly: assembled tree failed critical integrity validation")

	// ErrNilPrevTree is returned by ReassembleIncremental when prev is
	// nil — there is no prior tree to incrementally extend.
	ErrNilPrevTree = errors.New("treeassembly: previous tree must not be nil")

	// ErrNilStore is returned by persistence operations when no
	// graph.GraphStore is supplied.
	ErrNilStore = errors.New("treeassembly: graph store must not be nil")

	// ErrSnapshotNotFound is returned by SnapshotStore.GetSnapshot when
	// no snapshot exists under the requested SnapshotKey.
	ErrSnapshotNotFound = errors.New("treeassembly: snapshot not found")
)
