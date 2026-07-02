package fact

import (
	"context"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
)

// PersistFacts persists every node in facts via store.CreateNode, then,
// for each fact whose ID appears as a key in supportsApplicationIDs,
// creates an irac.EdgeSupports edge from that FactNode to every
// ApplicationNode ID listed — the Fact --supports--> Application triple
// is the only legal edge triple in irac's constraint table
// (packages/irac/edge.go's legalEdgeTriples) with a FactNode as its
// source, so that is the only edge type this function ever creates.
//
// applicationIDs must list every irac.ApplicationNode.ID that already
// exists in store for this case; PersistFacts does not create
// ApplicationNodes itself (mirroring packages/issue/persist.go's
// PersistIssues, which persists nodes only when the edge's other
// endpoint type does not yet exist at that stage of the pipeline — here
// the endpoint, ApplicationNode, may or may not exist yet depending on
// how far the case's IRAC tree has progressed). Fact IDs whose
// supportsApplicationIDs entry references an ID not present in
// applicationIDs are skipped for edge creation (but the fact node itself
// is still persisted) since creating an edge to a nonexistent node would
// violate GraphStore.CreateEdge's dangling-reference expectations.
//
// Returns the persisted nodes (in the same order as facts) and
// ErrPersistFailed (wrapping the underlying store error) if any
// CreateNode or CreateEdge call fails; nodes/edges already persisted
// before the failing call are not rolled back.
func PersistFacts(ctx context.Context, store graph.GraphStore, facts []irac.FactNode, applicationIDs []string, supportsApplicationIDs map[string][]string) ([]irac.FactNode, error) {
	knownApplications := make(map[string]struct{}, len(applicationIDs))
	for _, id := range applicationIDs {
		knownApplications[id] = struct{}{}
	}

	persisted := make([]irac.FactNode, 0, len(facts))
	for _, f := range facts {
		if err := store.CreateNode(ctx, f.Node); err != nil {
			return persisted, wrapPersistError(err)
		}
		persisted = append(persisted, f)

		for _, appID := range supportsApplicationIDs[f.ID] {
			if _, ok := knownApplications[appID]; !ok {
				continue
			}
			edge := irac.Edge{
				FromID: f.ID,
				ToID:   appID,
				Type:   irac.EdgeSupports,
			}
			if err := store.CreateEdge(ctx, edge); err != nil {
				return persisted, wrapPersistError(err)
			}
		}
	}
	return persisted, nil
}

// wrapPersistError wraps err with ErrPersistFailed so callers can test
// errors.Is(err, ErrPersistFailed) regardless of the underlying
// graph.GraphStore implementation's own error value, mirroring
// packages/issue/persist.go's wrapPersistError.
func wrapPersistError(err error) error {
	return &persistError{underlying: err}
}

// persistError implements error and errors.Unwrap so both
// ErrPersistFailed and the underlying store error can be matched via
// errors.Is.
type persistError struct {
	underlying error
}

func (e *persistError) Error() string {
	return ErrPersistFailed.Error() + ": " + e.underlying.Error()
}

func (e *persistError) Unwrap() []error {
	return []error{ErrPersistFailed, e.underlying}
}
