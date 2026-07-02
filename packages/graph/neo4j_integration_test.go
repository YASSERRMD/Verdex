package graph_test

import (
	"context"
	"testing"
	"time"

	neo4jdriver "github.com/neo4j/neo4j-go-driver/v5/neo4j"
	tcneo4j "github.com/testcontainers/testcontainers-go/modules/neo4j"

	"github.com/YASSERRMD/verdex/packages/graph"
	"github.com/YASSERRMD/verdex/packages/persistence"
)

// containerStartTimeout bounds how long we wait for Docker to pull and
// start the Neo4j container. Mirrors
// packages/persistence/integration_test.go's containerStartTimeout.
const containerStartTimeout = 30 * time.Second

const neo4jTestPassword = "verdex-test-password"

// requireNeo4jContainer starts an ephemeral Neo4j container for the
// duration of the test and returns its bolt URL. It skips the test
// (rather than failing) if Docker is not reachable, and skips
// unconditionally in -short mode, mirroring
// packages/persistence/integration_test.go's requirePostgresContainer
// exactly: -short must never hang or require Docker/network.
func requireNeo4jContainer(t *testing.T) string {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping testcontainers-backed integration test in -short mode")
	}

	startCtx, cancel := context.WithTimeout(context.Background(), containerStartTimeout)
	defer cancel()

	ctr, err := tcneo4j.Run(startCtx, "neo4j:5-community",
		tcneo4j.WithAdminPassword(neo4jTestPassword),
	)
	if err != nil {
		t.Skipf("skipping: could not start Neo4j testcontainer (Docker unavailable?): %v", err)
	}

	t.Cleanup(func() {
		tearCtx, tearCancel := context.WithTimeout(context.Background(), containerStartTimeout)
		defer tearCancel()
		if err := ctr.Terminate(tearCtx); err != nil {
			t.Logf("warning: failed to terminate neo4j container: %v", err)
		}
	})

	boltURL, err := ctr.BoltUrl(startCtx)
	if err != nil {
		t.Fatalf("BoltUrl: %v", err)
	}
	return boltURL
}

// TestIntegration_Neo4jConnectivity verifies persistence.GraphDriver
// (pinned in Phase 004) can actually connect to a live Neo4j instance,
// establishing that the connectivity primitive this package's
// Neo4jHealthChecker delegates to works end-to-end.
func TestIntegration_Neo4jConnectivity(t *testing.T) {
	boltURL := requireNeo4jContainer(t)

	ctx, cancel := context.WithTimeout(context.Background(), containerStartTimeout)
	defer cancel()

	driver, err := persistence.NewGraphDriver(boltURL, "neo4j", neo4jTestPassword)
	if err != nil {
		t.Fatalf("NewGraphDriver: %v", err)
	}
	t.Cleanup(func() { _ = driver.Close(context.Background()) })

	if err := driver.VerifyConnectivity(ctx); err != nil {
		t.Fatalf("VerifyConnectivity: %v", err)
	}
}

// TestIntegration_MigratorApply verifies Migrator.Apply successfully
// runs the built-in idempotent schema migrations (uniqueness
// constraint, indexes) against a live Neo4j database, and that applying
// them a second time is still a no-op success (not an error), per
// Migrator's documented idempotency guarantee.
func TestIntegration_MigratorApply(t *testing.T) {
	boltURL := requireNeo4jContainer(t)

	ctx, cancel := context.WithTimeout(context.Background(), containerStartTimeout)
	defer cancel()

	migrator, err := graph.NewMigrator(boltURL, "neo4j", neo4jTestPassword)
	if err != nil {
		t.Fatalf("NewMigrator: %v", err)
	}
	t.Cleanup(func() { _ = migrator.Close(context.Background()) })

	if err := migrator.Apply(ctx); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Re-applying must be a no-op, not an error.
	if err := migrator.Apply(ctx); err != nil {
		t.Fatalf("second Apply call: %v", err)
	}
}

// TestIntegration_Neo4jNodeCRUD writes and reads a node directly via
// the neo4j Go driver against a live container, proving out the Cypher
// shape a future full Neo4j-backed GraphStore implementation of
// CreateNode/GetNode would use (MERGE by id, matching the uniqueness
// constraint Migrator.Apply installs).
func TestIntegration_Neo4jNodeCRUD(t *testing.T) {
	boltURL := requireNeo4jContainer(t)

	ctx, cancel := context.WithTimeout(context.Background(), containerStartTimeout)
	defer cancel()

	migrator, err := graph.NewMigrator(boltURL, "neo4j", neo4jTestPassword)
	if err != nil {
		t.Fatalf("NewMigrator: %v", err)
	}
	t.Cleanup(func() { _ = migrator.Close(context.Background()) })
	if err := migrator.Apply(ctx); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	driver, err := neo4jdriver.NewDriverWithContext(boltURL, neo4jdriver.BasicAuth("neo4j", neo4jTestPassword, ""))
	if err != nil {
		t.Fatalf("NewDriverWithContext: %v", err)
	}
	t.Cleanup(func() { _ = driver.Close(context.Background()) })

	session := driver.NewSession(ctx, neo4jdriver.SessionConfig{})
	t.Cleanup(func() { _ = session.Close(context.Background()) })

	_, err = session.Run(ctx,
		"MERGE (n:IracNode {id: $id}) SET n.type = $type, n.case_id = $caseID, n.text = $text",
		map[string]any{"id": "n1", "type": "issue", "caseID": "case1", "text": "does the contract bind?"},
	)
	if err != nil {
		t.Fatalf("MERGE: %v", err)
	}

	result, err := session.Run(ctx, "MATCH (n:IracNode {id: $id}) RETURN n.text AS text", map[string]any{"id": "n1"})
	if err != nil {
		t.Fatalf("MATCH: %v", err)
	}
	record, err := result.Single(ctx)
	if err != nil {
		t.Fatalf("Single: %v", err)
	}
	text, ok := record.Get("text")
	if !ok || text != "does the contract bind?" {
		t.Fatalf("expected round-tripped text, got %v (ok=%v)", text, ok)
	}
}
