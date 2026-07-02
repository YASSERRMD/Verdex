package graph

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Migration is a single idempotent schema-setup step: a name (for
// logging/diagnostics) and a Cypher statement to run against Neo4j.
// Every statement in this package's built-in migrations is written to
// be safely re-runnable (CREATE CONSTRAINT IF NOT EXISTS / CREATE INDEX
// IF NOT EXISTS), so applying the same Migration twice is a no-op the
// second time.
type Migration struct {
	// Name identifies this migration for logging.
	Name string

	// Cypher is the statement to execute. Must be idempotent.
	Cypher string
}

// coreMigrations are the baseline schema-setup statements every
// Neo4j-backed deployment needs: a uniqueness constraint on node IDs (so
// CreateNode's upsert semantics map onto Neo4j's own MERGE guarantees)
// plus the indexes registered in indexMigrations (index.go) for
// traversal performance.
func coreMigrations() []Migration {
	migrations := []Migration{
		{
			Name:   "irac_node_id_unique",
			Cypher: "CREATE CONSTRAINT irac_node_id_unique IF NOT EXISTS FOR (n:IracNode) REQUIRE n.id IS UNIQUE",
		},
	}
	return append(migrations, indexMigrations()...)
}

// Migrator applies a sequence of Migration values against a Neo4j
// database. It holds its own neo4j.DriverWithContext, opened from the
// same target/username/password shape as persistence.NewGraphDriver
// (see packages/persistence/neo4j.go): persistence.GraphDriver
// intentionally exposes only connectivity-check and health-probe
// primitives (its doc comment defers "real graph operations (queries,
// sessions, transactions)" to this phase), so this package opens its
// own session-capable driver for the actual migration/query/transaction
// work rather than reaching into persistence.GraphDriver's unexported
// fields. persistence.GraphDriver / persistence.GraphChecker remain the
// integration point for connectivity verification and health probing
// (see health.go).
//
// Applying the same Migrator twice is safe: every built-in Migration is
// expressed as idempotent Cypher (IF NOT EXISTS), so a second Apply call
// is a no-op.
type Migrator struct {
	driver     neo4j.DriverWithContext
	migrations []Migration
}

// NewMigrator opens a Neo4j driver against target ("neo4j://" or
// "bolt://") using basic auth and builds a Migrator that applies
// migrations (defaulting to coreMigrations when migrations is empty).
func NewMigrator(target, username, password string, migrations ...Migration) (*Migrator, error) {
	if target == "" {
		return nil, fmt.Errorf("graph: NewMigrator: target must not be empty")
	}

	driver, err := neo4j.NewDriverWithContext(target, neo4j.BasicAuth(username, password, ""))
	if err != nil {
		return nil, fmt.Errorf("graph: NewMigrator: %w", err)
	}

	if len(migrations) == 0 {
		migrations = coreMigrations()
	}
	return &Migrator{driver: driver, migrations: migrations}, nil
}

// Close releases the Migrator's underlying driver.
func (m *Migrator) Close(ctx context.Context) error {
	if m == nil || m.driver == nil {
		return nil
	}
	return m.driver.Close(ctx)
}

// Apply runs every migration in order, using an auto-commit session per
// statement (schema statements like CREATE CONSTRAINT/CREATE INDEX are
// not valid inside an explicit transaction in Neo4j). It stops and
// returns an error at the first failing migration.
func (m *Migrator) Apply(ctx context.Context) error {
	if m == nil || m.driver == nil {
		return fmt.Errorf("graph: Migrator.Apply: driver is not initialized")
	}

	session := m.driver.NewSession(ctx, neo4j.SessionConfig{})
	defer func() { _ = session.Close(ctx) }()

	for _, mig := range m.migrations {
		if _, err := session.Run(ctx, mig.Cypher, nil); err != nil {
			return fmt.Errorf("graph: Migrator.Apply: migration %q: %w", mig.Name, err)
		}
	}
	return nil
}

// inMemoryMigrator is a no-op Migrator-shaped type for the in-memory
// store: InMemoryGraphStore has no schema to set up, so applying
// migrations against it always succeeds trivially. Exposed so callers
// that select a GraphStore implementation at runtime can treat migration
// application uniformly regardless of backend.
type inMemoryMigrator struct{}

// NewInMemoryMigrator returns a Migrator-shaped no-op suitable for use
// with InMemoryGraphStore, whose Apply always returns nil immediately.
func NewInMemoryMigrator() *inMemoryMigrator {
	return &inMemoryMigrator{}
}

// Apply is a no-op: InMemoryGraphStore has no schema.
func (*inMemoryMigrator) Apply(_ context.Context) error {
	return nil
}
