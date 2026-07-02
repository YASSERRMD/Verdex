package treeassembly

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
)

func TestPersistTree_NilArgs(t *testing.T) {
	ctx := context.Background()
	input := syntheticInput("case-1")
	tree, err := ComposeTree(ctx, input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := PersistTree(ctx, nil, NewInMemorySnapshotStore(), tree); !errors.Is(err, ErrNilStore) {
		t.Fatalf("expected ErrNilStore for nil store, got %v", err)
	}
	if err := PersistTree(ctx, graph.NewInMemoryGraphStore(), nil, tree); !errors.Is(err, ErrNilStore) {
		t.Fatalf("expected ErrNilStore for nil snapshots, got %v", err)
	}
	if err := PersistTree(ctx, graph.NewInMemoryGraphStore(), NewInMemorySnapshotStore(), nil); !errors.Is(err, ErrEmptyInput) {
		t.Fatalf("expected ErrEmptyInput for nil tree, got %v", err)
	}
}

func TestPersistTree_RoundTripsViaGraphImport(t *testing.T) {
	ctx := context.Background()
	input := syntheticInput("case-1")
	tree, err := ComposeTree(ctx, input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	store := graph.NewInMemoryGraphStore()
	snapshots := NewInMemorySnapshotStore()

	if err := PersistTree(ctx, store, snapshots, tree); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Confirm every node was persisted into the GraphStore.
	for _, n := range tree.Nodes {
		got, err := store.GetNode(ctx, n.GetID())
		if err != nil {
			t.Fatalf("expected node %q to be persisted: %v", n.GetID(), err)
		}
		if got.Type != n.GetType() {
			t.Errorf("node %q: got type %q, want %q", n.GetID(), got.Type, n.GetType())
		}
	}

	// Confirm the snapshot round-trips via a fresh store + graph.Import.
	key := SnapshotKey{CaseID: tree.Revision.CaseID, RevisionNumber: tree.Revision.RevisionNumber}
	data, err := snapshots.GetSnapshot(ctx, key)
	if err != nil {
		t.Fatalf("expected snapshot to exist: %v", err)
	}

	freshStore := graph.NewInMemoryGraphStore()
	if err := graph.Import(ctx, freshStore, data); err != nil {
		t.Fatalf("expected snapshot to import cleanly: %v", err)
	}

	imported, err := freshStore.Traverse(ctx, graph.TraversalQuery{CaseID: tree.Revision.CaseID})
	if err != nil {
		t.Fatalf("unexpected error traversing imported store: %v", err)
	}
	if len(imported) != len(tree.Nodes) {
		t.Fatalf("expected %d imported nodes, got %d", len(tree.Nodes), len(imported))
	}
}

func TestPersistTree_SkipsDuplicateNodesIdempotently(t *testing.T) {
	ctx := context.Background()
	input := syntheticInput("case-1")
	tree, err := ComposeTree(ctx, input, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	store := graph.NewInMemoryGraphStore()
	snapshots := NewInMemorySnapshotStore()

	// Pre-persist one node via the store directly, simulating an
	// upstream service (issue/fact/application) having already
	// persisted it.
	preExisting := tree.Nodes[0]
	baseNode := irac.Node{ID: preExisting.GetID(), Type: preExisting.GetType(), CaseID: input.CaseID}
	if err := store.CreateNode(ctx, baseNode); err != nil {
		t.Fatalf("unexpected error pre-seeding store: %v", err)
	}

	if err := PersistTree(ctx, store, snapshots, tree); err != nil {
		t.Fatalf("unexpected error persisting tree with pre-existing node: %v", err)
	}

	got, err := store.GetNode(ctx, preExisting.GetID())
	if err != nil {
		t.Fatalf("expected node to still be retrievable: %v", err)
	}
	if got.ID != preExisting.GetID() {
		t.Fatalf("expected upsert to keep the same ID, got %q", got.ID)
	}
}

func TestInMemorySnapshotStore_NotFound(t *testing.T) {
	s := NewInMemorySnapshotStore()
	_, err := s.GetSnapshot(context.Background(), SnapshotKey{CaseID: "nope", RevisionNumber: 1})
	if !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("expected ErrSnapshotNotFound, got %v", err)
	}
}
