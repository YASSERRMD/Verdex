package graph_test

import (
	"context"
	"errors"
	"testing"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/irac"
)

var errBoom = errors.New("boom")

// checkableStore is a minimal GraphStore test double that also
// implements the optional HealthCheck(ctx) error method HealthCheck
// (health.go) type-asserts for, so tests can verify HealthCheck
// delegates to a store-provided checker instead of only handling
// InMemoryGraphStore specially.
type checkableStore struct {
	err error
}

func (c *checkableStore) CreateNode(context.Context, irac.Node) error { return nil }
func (c *checkableStore) CreateEdge(context.Context, irac.Edge) error { return nil }
func (c *checkableStore) GetNode(context.Context, string) (irac.Node, error) {
	return irac.Node{}, nil
}
func (c *checkableStore) Traverse(context.Context, graph.TraversalQuery) ([]irac.Node, error) {
	return nil, nil
}
func (c *checkableStore) DeleteTree(context.Context, string) error { return nil }
func (c *checkableStore) HealthCheck(context.Context) error        { return c.err }

func TestHealthCheck_InMemoryStoreAlwaysHealthy(t *testing.T) {
	t.Parallel()

	store := graph.NewInMemoryGraphStore()
	if err := graph.HealthCheck(context.Background(), store); err != nil {
		t.Fatalf("expected in-memory store to always be healthy, got %v", err)
	}
}

func TestHealthCheck_NilStore(t *testing.T) {
	t.Parallel()

	if err := graph.HealthCheck(context.Background(), nil); err == nil {
		t.Fatal("expected error for nil store, got nil")
	}
}

// TestHealthCheck_DelegatesToStoreProvidedChecker verifies HealthCheck
// prefers a store's own HealthCheck method when present, so a future
// Neo4j-backed GraphStore that implements one gets used instead of the
// generic in-memory-only fallback.
func TestHealthCheck_DelegatesToStoreProvidedChecker(t *testing.T) {
	t.Parallel()

	healthy := &checkableStore{}
	if err := graph.HealthCheck(context.Background(), healthy); err != nil {
		t.Fatalf("expected delegated HealthCheck to succeed, got %v", err)
	}

	unhealthy := &checkableStore{err: errBoom}
	if err := graph.HealthCheck(context.Background(), unhealthy); err == nil {
		t.Fatal("expected delegated HealthCheck to surface the store's error, got nil")
	}
}

func TestNeo4jHealthChecker_NoEndpointConfigured(t *testing.T) {
	t.Parallel()

	checker := graph.Neo4jHealthChecker("", "", "")
	if err := checker(context.Background()); err != nil {
		t.Fatalf("expected nil error when no Neo4j endpoint is configured, got %v", err)
	}
}
