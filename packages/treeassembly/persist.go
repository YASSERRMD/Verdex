package treeassembly

import (
	"context"
	"fmt"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
)

// SnapshotKey identifies one persisted tree snapshot: a case at a
// specific revision. Snapshots are keyed this way (rather than just by
// CaseID) because packages/irac's TreeRevision model treats every
// assembly as an immutable, versioned snapshot rather than an in-place
// mutation (see packages/irac/version.go) — persisting under CaseID
// alone would silently overwrite the previous revision's snapshot.
type SnapshotKey struct {
	CaseID         string
	RevisionNumber int
}

// SnapshotStore persists tree snapshot bytes (as produced by
// graph.Export / irac.MarshalTree) keyed by SnapshotKey. A minimal
// interface so PersistTree can be exercised in tests without a real
// backing store; production callers can back this with any key-value
// store keyed by fmt.Sprintf("%s@%d", CaseID, RevisionNumber) or
// equivalent.
type SnapshotStore interface {
	// PutSnapshot stores data under key, overwriting any existing
	// snapshot at that key.
	PutSnapshot(ctx context.Context, key SnapshotKey, data []byte) error

	// GetSnapshot retrieves the snapshot bytes stored under key, or
	// returns an error if none exists.
	GetSnapshot(ctx context.Context, key SnapshotKey) ([]byte, error)
}

// InMemorySnapshotStore is a SnapshotStore backed by an in-memory map.
// The default used when no other SnapshotStore is supplied.
type InMemorySnapshotStore struct {
	snapshots map[SnapshotKey][]byte
}

// NewInMemorySnapshotStore constructs an empty InMemorySnapshotStore.
func NewInMemorySnapshotStore() *InMemorySnapshotStore {
	return &InMemorySnapshotStore{snapshots: make(map[SnapshotKey][]byte)}
}

// PutSnapshot stores data under key.
func (s *InMemorySnapshotStore) PutSnapshot(_ context.Context, key SnapshotKey, data []byte) error {
	cp := make([]byte, len(data))
	copy(cp, data)
	s.snapshots[key] = cp
	return nil
}

// GetSnapshot retrieves the snapshot stored under key, or
// ErrSnapshotNotFound if none exists.
func (s *InMemorySnapshotStore) GetSnapshot(_ context.Context, key SnapshotKey) ([]byte, error) {
	data, ok := s.snapshots[key]
	if !ok {
		return nil, ErrSnapshotNotFound
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	return cp, nil
}

// PersistTree writes every node and edge in tree into store via
// CreateNode/CreateEdge, then snapshots the full tree (via graph.Export)
// into snapshots keyed by tree's CaseID and RevisionNumber.
//
// CreateNode is treated as an idempotent upsert by every GraphStore
// implementation (see packages/graph/store.go's doc comment), so nodes
// already persisted by an upstream service (packages/issue,
// packages/fact, packages/application) are safely re-written rather than
// duplicated — this function does not attempt to detect "already
// persisted" itself, it relies on that upsert contract. Edge creation is
// similarly safe to repeat for an edge that already exists, since
// GraphStore's in-memory implementation simply appends and downstream
// consumers (Traverse, Export) treat the edge list as a set of directed
// relationships rather than requiring uniqueness.
//
// Returns ErrNilStore if store or snapshots is nil, or ErrEmptyInput if
// tree is nil.
func PersistTree(ctx context.Context, store graph.GraphStore, snapshots SnapshotStore, tree *Tree) error {
	if store == nil || snapshots == nil {
		return ErrNilStore
	}
	if tree == nil {
		return ErrEmptyInput
	}

	for _, n := range tree.Nodes {
		if err := store.CreateNode(ctx, toBaseNode(n)); err != nil {
			return fmt.Errorf("treeassembly: persist node %q: %w", n.GetID(), err)
		}
	}
	for _, e := range tree.Edges {
		if err := store.CreateEdge(ctx, e); err != nil {
			return fmt.Errorf("treeassembly: persist edge %s->%s: %w", e.FromID, e.ToID, err)
		}
	}

	data, err := graph.Export(ctx, store, tree.Revision.CaseID)
	if err != nil {
		return fmt.Errorf("treeassembly: export snapshot: %w", err)
	}

	key := SnapshotKey{CaseID: tree.Revision.CaseID, RevisionNumber: tree.Revision.RevisionNumber}
	if err := snapshots.PutSnapshot(ctx, key, data); err != nil {
		return fmt.Errorf("treeassembly: put snapshot: %w", err)
	}
	return nil
}

// toBaseNode flattens any irac.NodeLike down to the base irac.Node shape
// GraphStore persists, mirroring packages/graph/backup.go's toNode
// helper (unexported there, so reimplemented here rather than imported).
func toBaseNode(nl irac.NodeLike) irac.Node {
	switch v := nl.(type) {
	case irac.IssueNode:
		return v.Node
	case irac.RuleNode:
		return v.Node
	case irac.FactNode:
		return v.Node
	case irac.ApplicationNode:
		return v.Node
	case irac.ConclusionNode:
		return v.Node
	case irac.Node:
		return v
	default:
		return irac.Node{ID: nl.GetID(), Type: nl.GetType()}
	}
}
