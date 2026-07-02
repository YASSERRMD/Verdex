package graph

import (
	"context"
	"fmt"

	"github.com/YASSERRMD/verdex/packages/persistence"
)

// HealthCheck reports whether store is currently able to serve reads.
// For InMemoryGraphStore this is always healthy (a nil, non-nil map-
// backed store has nothing external to fail against); for a Neo4j-
// backed store, health should be answered by the same connectivity
// probe persistence.GraphChecker already wires up for the /readyz
// endpoint (see packages/persistence/neo4j.go and
// packages/observability/health.go's Checker/NamedChecker shape).
func HealthCheck(ctx context.Context, store GraphStore) error {
	if store == nil {
		return fmt.Errorf("graph: HealthCheck: store must not be nil")
	}

	if _, ok := store.(*InMemoryGraphStore); ok {
		// The in-memory store has no external dependency to fail
		// against: it is healthy as long as it exists.
		return nil
	}

	// Any other GraphStore implementation is expected to know how to
	// check its own health; if it also implements a Neo4j-style
	// connectivity probe, prefer that.
	if checkable, ok := store.(interface {
		HealthCheck(ctx context.Context) error
	}); ok {
		return checkable.HealthCheck(ctx)
	}

	return nil
}

// Neo4jHealthChecker returns an observability.Checker-compatible
// function (context.Context) error that verifies a Neo4j-backed
// GraphStore's connectivity, by delegating directly to
// persistence.GraphChecker. This is the wiring point a service using
// this package's Neo4j-backed store should register alongside its other
// NamedChecker values on packages/observability's ReadinessHandler.
//
// If target is empty (no Neo4j endpoint configured), the returned
// checker is a graceful no-op that always reports healthy, mirroring
// persistence.GraphChecker's own behavior for the same case.
func Neo4jHealthChecker(target, username, password string) func(ctx context.Context) error {
	return persistence.GraphChecker(target, username, password)
}
