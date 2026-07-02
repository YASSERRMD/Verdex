package graph

import (
	"context"

	"github.com/YASSERRMD/verdex/packages/irac"
)

// TraversalQuery describes a traversal starting from a case's reasoning
// tree in a GraphStore. Traverse (see GraphStore) returns every node
// reachable from CaseID that matches the optional filters below.
type TraversalQuery struct {
	// CaseID restricts the traversal to nodes belonging to a single
	// case's reasoning tree. Required.
	CaseID string

	// NodeType, if non-empty, restricts the returned nodes to this
	// irac.NodeType. Empty means "any node type".
	NodeType irac.NodeType

	// FromNodeID, if non-empty, restricts the traversal to nodes
	// reachable by following edges outward from this node id (a
	// breadth-first walk over irac.Edge relationships). Empty means
	// "every node in the case", i.e. no edge-walk restriction.
	FromNodeID string

	// MaxDepth bounds how many edge hops the traversal will follow from
	// FromNodeID. Zero (the default) means "unbounded" when FromNodeID
	// is set. Ignored when FromNodeID is empty.
	MaxDepth int
}

// GraphStore persists and queries IRAC reasoning trees (packages/irac
// Node/Edge values). Implementations:
//
//   - InMemoryGraphStore (inmemory.go): a fully in-memory implementation
//     backed by maps. This is the default used by unit tests and by
//     downstream packages (033-040) that do not need a live Neo4j
//     instance.
//   - a Neo4j-backed implementation built on persistence.GraphDriver:
//     Neo4j was already selected and pinned as this project's graph
//     database in Phase 004 (see packages/persistence/neo4j.go, whose
//     doc comment explicitly defers "real graph operations (queries,
//     sessions, transactions)" to this phase). Neo4j's native property
//     graph model — nodes, directed typed relationships, Cypher
//     traversal queries — maps directly onto irac.Node/irac.Edge without
//     an impedance mismatch, and persistence.GraphDriver already
//     provides connection management, so this phase builds query,
//     session, and transaction logic on top of that pinned driver
//     rather than re-deriving connection handling. migrate.go,
//     index.go, tx.go, and health.go each document the Neo4j-specific
//     half of their concern (Cypher statements, session/transaction
//     usage, connectivity probing) even though the only fully
//     implemented, exercised-by-default store in this phase is
//     InMemoryGraphStore; a live Neo4j-backed GraphStore is exercised
//     only by the testcontainers-gated tests in
//     neo4j_integration_test.go, consistent with most deployments not
//     having a Neo4j endpoint configured yet.
type GraphStore interface {
	// CreateNode persists a single irac.Node. Implementations should
	// treat creating a node with an ID that already exists as an
	// overwrite (idempotent upsert), mirroring how irac trees are
	// produced as immutable revisions (see irac.TreeRevision) rather
	// than mutated in place.
	CreateNode(ctx context.Context, node irac.Node) error

	// CreateEdge persists a single irac.Edge. Implementations are not
	// required to validate the edge against irac.ValidateTree; callers
	// that want tree-integrity guarantees should validate before
	// calling CreateEdge.
	CreateEdge(ctx context.Context, edge irac.Edge) error

	// GetNode returns the node with the given id, or ErrNodeNotFound if
	// no such node exists.
	GetNode(ctx context.Context, id string) (irac.Node, error)

	// Traverse returns every node matching query. See TraversalQuery for
	// the supported filters.
	Traverse(ctx context.Context, query TraversalQuery) ([]irac.Node, error)

	// DeleteTree removes every node and edge belonging to caseID. It is
	// not an error to delete a case with no nodes.
	DeleteTree(ctx context.Context, caseID string) error
}
