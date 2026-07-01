package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// defaultNeo4jVerifyTimeout bounds how long a Neo4j connectivity check
// is allowed to take when the caller does not already supply a
// context deadline.
const defaultNeo4jVerifyTimeout = 5 * time.Second

// GraphDriver is a minimal wrapper around the Neo4j driver used to
// confirm the graph store is reachable. Phase 032 owns real graph
// operations (queries, sessions, transactions); this phase only pins
// the driver dependency and provides a connectivity-check primitive
// for the health probe wired up in commit 7.
type GraphDriver struct {
	driver neo4j.DriverWithContext
}

// NewGraphDriver opens a Neo4j driver against target (a "neo4j://" or
// "bolt://" URI) using basic auth. It does not verify connectivity by
// itself; call VerifyConnectivity to do that.
func NewGraphDriver(target, username, password string) (*GraphDriver, error) {
	if target == "" {
		return nil, fmt.Errorf("persistence: NewGraphDriver: target must not be empty")
	}

	driver, err := neo4j.NewDriverWithContext(target, neo4j.BasicAuth(username, password, ""))
	if err != nil {
		return nil, fmt.Errorf("persistence: NewGraphDriver: %w", err)
	}

	return &GraphDriver{driver: driver}, nil
}

// VerifyConnectivity checks that the graph store is reachable,
// bounding the check with defaultNeo4jVerifyTimeout unless ctx
// already carries an earlier deadline.
func (g *GraphDriver) VerifyConnectivity(ctx context.Context) error {
	if g == nil || g.driver == nil {
		return fmt.Errorf("persistence: VerifyConnectivity: driver is not initialized")
	}

	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultNeo4jVerifyTimeout)
		defer cancel()
	}

	return g.driver.VerifyConnectivity(ctx)
}

// Close releases the underlying driver's resources. It is safe to
// call on a nil receiver.
func (g *GraphDriver) Close(ctx context.Context) error {
	if g == nil || g.driver == nil {
		return nil
	}
	return g.driver.Close(ctx)
}

// GraphChecker returns an observability.Checker-compatible function
// (context.Context) error that verifies Neo4j connectivity. If
// target is empty (no Neo4j endpoint configured), the returned
// checker is a graceful no-op that always reports healthy, since
// Phase 032 owns real graph-store usage and most deployments will not
// have one configured yet.
func GraphChecker(target, username, password string) func(ctx context.Context) error {
	if target == "" {
		return func(_ context.Context) error { return nil }
	}

	return func(ctx context.Context) error {
		driver, err := NewGraphDriver(target, username, password)
		if err != nil {
			return err
		}
		defer func() { _ = driver.Close(ctx) }()

		return driver.VerifyConnectivity(ctx)
	}
}
