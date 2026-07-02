package graph_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
)

func TestWithTransaction_CommitsOnSuccess(t *testing.T) {
	t.Parallel()

	store := graph.NewInMemoryGraphStore()
	ctx := context.Background()

	err := graph.WithTransaction(ctx, store, func(txStore graph.GraphStore) error {
		return txStore.CreateNode(ctx, testNode("n1", "case1", irac.NodeIssue))
	})
	if err != nil {
		t.Fatalf("WithTransaction: %v", err)
	}

	if _, err := store.GetNode(ctx, "n1"); err != nil {
		t.Fatalf("expected n1 to be committed, got %v", err)
	}
}

func TestWithTransaction_RollsBackOnError(t *testing.T) {
	t.Parallel()

	store := graph.NewInMemoryGraphStore()
	ctx := context.Background()

	// Seed a node before the transaction, so we can also verify
	// rollback does not touch state that existed prior to the
	// transaction.
	if err := store.CreateNode(ctx, testNode("pre-existing", "case1", irac.NodeFact)); err != nil {
		t.Fatalf("seed CreateNode: %v", err)
	}

	sentinel := errors.New("boom")
	err := graph.WithTransaction(ctx, store, func(txStore graph.GraphStore) error {
		if err := txStore.CreateNode(ctx, testNode("partial1", "case1", irac.NodeIssue)); err != nil {
			return err
		}
		if err := txStore.CreateNode(ctx, testNode("partial2", "case1", irac.NodeRule)); err != nil {
			return err
		}
		return sentinel
	})
	if err == nil {
		t.Fatal("expected WithTransaction to return an error, got nil")
	}
	if !errors.Is(err, graph.ErrTransactionFailed) {
		t.Fatalf("expected ErrTransactionFailed, got %v", err)
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected the sentinel error to be wrapped, got %v", err)
	}

	// Neither partial write must have survived the rollback.
	if _, err := store.GetNode(ctx, "partial1"); !errors.Is(err, graph.ErrNodeNotFound) {
		t.Fatalf("expected partial1 to have been rolled back, got %v", err)
	}
	if _, err := store.GetNode(ctx, "partial2"); !errors.Is(err, graph.ErrNodeNotFound) {
		t.Fatalf("expected partial2 to have been rolled back, got %v", err)
	}

	// The pre-existing node must still be present.
	if _, err := store.GetNode(ctx, "pre-existing"); err != nil {
		t.Fatalf("expected pre-existing node to survive rollback, got %v", err)
	}
}

func TestWithTransaction_RollsBackOnPanic(t *testing.T) {
	t.Parallel()

	store := graph.NewInMemoryGraphStore()
	ctx := context.Background()

	if err := store.CreateNode(ctx, testNode("pre-existing", "case1", irac.NodeFact)); err != nil {
		t.Fatalf("seed CreateNode: %v", err)
	}

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic to propagate out of WithTransaction")
			}
		}()
		_ = graph.WithTransaction(ctx, store, func(txStore graph.GraphStore) error {
			if err := txStore.CreateNode(ctx, testNode("panicking-write", "case1", irac.NodeIssue)); err != nil {
				return err
			}
			panic("boom")
		})
	}()

	if _, err := store.GetNode(ctx, "panicking-write"); !errors.Is(err, graph.ErrNodeNotFound) {
		t.Fatalf("expected panicking-write to have been rolled back, got %v", err)
	}
	if _, err := store.GetNode(ctx, "pre-existing"); err != nil {
		t.Fatalf("expected pre-existing node to survive panic rollback, got %v", err)
	}
}

func TestWithTransaction_NilArgs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	if err := graph.WithTransaction(ctx, nil, func(graph.GraphStore) error { return nil }); err == nil {
		t.Fatal("expected error for nil store, got nil")
	}

	store := graph.NewInMemoryGraphStore()
	if err := graph.WithTransaction(ctx, store, nil); err == nil {
		t.Fatal("expected error for nil fn, got nil")
	}
}
