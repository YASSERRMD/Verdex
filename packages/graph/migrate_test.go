package graph_test

import (
	"context"
	"testing"

	"github.com/YASSERRMD/verdex/packages/graph"
)

func TestNewMigrator_EmptyTarget(t *testing.T) {
	t.Parallel()

	_, err := graph.NewMigrator("", "neo4j", "password")
	if err == nil {
		t.Fatal("expected error for empty target, got nil")
	}
}

func TestInMemoryMigrator_ApplyIsNoOp(t *testing.T) {
	t.Parallel()

	m := graph.NewInMemoryMigrator()
	if err := m.Apply(context.Background()); err != nil {
		t.Fatalf("expected in-memory migrator Apply to always succeed, got %v", err)
	}
}
