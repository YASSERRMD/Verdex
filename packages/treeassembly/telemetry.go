package treeassembly

import (
	"sync"
	"time"
)

// AssemblyTelemetry captures the observable outcome of a single
// assembly run: how large the resulting tree was, how many structural
// and semantic problems it had, and how long assembly took. Recorded by
// TreeAssemblyService (service.go) after every ComposeTree /
// ReassembleIncremental call, and readable back via a Recorder for
// callers that want to inspect assembly health over time (e.g. an
// operator dashboard, or a test asserting counts match).
type AssemblyTelemetry struct {
	// CaseID identifies the case this assembly run was for.
	CaseID string

	// RevisionNumber is the RevisionNumber of the Tree produced by this
	// run (see irac.TreeRevision).
	RevisionNumber int

	// NodeCount is the number of nodes in the resulting Tree.
	NodeCount int

	// EdgeCount is the number of edges in the resulting Tree.
	EdgeCount int

	// ValidationIssueCount is the number of irac.ValidationIssues found
	// by ValidateIntegrity for this run.
	ValidationIssueCount int

	// GapCount is the number of Gaps found by DetectGaps for this run.
	GapCount int

	// Duration is how long this assembly run took, start to finish.
	Duration time.Duration

	// RecordedAt is when this telemetry entry was recorded.
	RecordedAt time.Time
}

// Recorder records and retrieves AssemblyTelemetry entries. Implemented
// by InMemoryRecorder (the default, no-external-backend implementation);
// a future phase could add a Recorder backed by packages/observability
// without changing TreeAssemblyService's call sites.
type Recorder interface {
	// Record appends a new AssemblyTelemetry entry.
	Record(entry AssemblyTelemetry)

	// ForCase returns every recorded entry for caseID, in the order they
	// were recorded.
	ForCase(caseID string) []AssemblyTelemetry

	// All returns every recorded entry across every case, in the order
	// they were recorded.
	All() []AssemblyTelemetry
}

// InMemoryRecorder is a Recorder backed by an in-memory slice. Safe for
// concurrent use. The default Recorder used by TreeAssemblyService when
// none is supplied.
type InMemoryRecorder struct {
	mu      sync.RWMutex
	entries []AssemblyTelemetry
}

// NewInMemoryRecorder constructs an empty InMemoryRecorder.
func NewInMemoryRecorder() *InMemoryRecorder {
	return &InMemoryRecorder{}
}

// Record appends entry to the recorder.
func (r *InMemoryRecorder) Record(entry AssemblyTelemetry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, entry)
}

// ForCase returns every recorded entry for caseID, in recorded order.
func (r *InMemoryRecorder) ForCase(caseID string) []AssemblyTelemetry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]AssemblyTelemetry, 0)
	for _, e := range r.entries {
		if e.CaseID == caseID {
			out = append(out, e)
		}
	}
	return out
}

// All returns every recorded entry, in recorded order.
func (r *InMemoryRecorder) All() []AssemblyTelemetry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]AssemblyTelemetry, len(r.entries))
	copy(out, r.entries)
	return out
}

// NewAssemblyTelemetry constructs an AssemblyTelemetry entry from an
// assembled tree and the validation/gap results computed over it,
// stamping Duration as the elapsed time since start and RecordedAt as
// now.
func NewAssemblyTelemetry(tree *Tree, validationIssueCount, gapCount int, start time.Time) AssemblyTelemetry {
	entry := AssemblyTelemetry{
		ValidationIssueCount: validationIssueCount,
		GapCount:             gapCount,
		Duration:             time.Since(start),
		RecordedAt:           time.Now(),
	}
	if tree != nil {
		entry.CaseID = tree.Revision.CaseID
		entry.RevisionNumber = tree.Revision.RevisionNumber
		entry.NodeCount = len(tree.Nodes)
		entry.EdgeCount = len(tree.Edges)
	}
	return entry
}
