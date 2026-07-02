// Package graph persists IRAC reasoning trees (packages/irac nodes and
// edges — see Phase 031) in a graph database, with an in-memory
// implementation for tests and environments without a live Neo4j
// instance. This is the storage layer packages/irac's own doc comment
// explicitly defers: "This phase defines TYPES and VALIDATION only —
// there is no storage backend here; that is Phase 032 (graph store
// integration), which builds on this schema."
//
// Core concepts:
//
//   - GraphStore: the storage-agnostic interface every implementation
//     in this package satisfies — CreateNode, CreateEdge, GetNode,
//     Traverse, DeleteTree (store.go). Neo4j was already selected and
//     pinned as this project's graph database in Phase 004
//     (packages/persistence/neo4j.go); this phase builds the query,
//     session, and migration logic Neo4j needs on top of that pinned
//     dependency.
//   - InMemoryGraphStore: a fully in-memory GraphStore implementation
//     backed by maps (inmemory.go). This is the default used by unit
//     tests and by downstream reasoning phases (033-040) that do not
//     need a live Neo4j instance.
//   - TenantScopedStore / TenantID: wraps any GraphStore and enforces
//     that every node/edge write and read is scoped to a single tenant,
//     rejecting cross-tenant access with ErrCrossTenantAccess rather
//     than silently filtering it (tenant.go). TenantID is a local
//     string type with no hard dependency on packages/tenancy.
//   - Migration / Migrator: applies idempotent Neo4j schema setup
//     (uniqueness constraints, indexes) expressed as Cypher statement
//     strings (migrate.go). NewInMemoryMigrator is a no-op stand-in for
//     InMemoryGraphStore, which has no schema.
//   - Secondary indexes: InMemoryGraphStore keeps NodeType and CaseID
//     indexes so Traverse resolves its filters without scanning every
//     node; the Neo4j-backed path registers the equivalent CREATE INDEX
//     statements as migrations (index.go).
//   - WithTransaction: gives a caller the illusion of an atomic
//     sequence of GraphStore writes. The in-memory implementation
//     snapshots state before running the caller's function and restores
//     it on error or panic (tx.go); a Neo4j-backed store should instead
//     use real Neo4j sessions/transactions.
//   - Export / Import: serializes a case's nodes and edges to and from
//     a lossless JSON envelope, reusing irac.MarshalTree/UnmarshalTree
//     as the wire format (backup.go).
//   - HealthCheck / Neo4jHealthChecker: a GraphStore-agnostic health
//     probe, wired to persistence.GraphChecker for the Neo4j-backed
//     path (health.go).
//
// Design principles:
//
//   - GraphStore is the seam. Everything else in this package
//     (TenantScopedStore, WithTransaction, Export/Import, HealthCheck)
//     is written purely in terms of the GraphStore interface, so it
//     behaves identically whether the underlying store is
//     InMemoryGraphStore or a future full Neo4j-backed implementation.
//   - In-memory first. Downstream reasoning phases (033-040) should be
//     able to depend on this package and pass all their own tests
//     using only InMemoryGraphStore, without needing Docker or a
//     network connection.
//   - Tenant isolation rejects, never silently filters cross-tenant
//     writes — a caller must never mistake "no access" for "no data".
//   - No hard dependency on packages/tenancy. TenantID is a local
//     opaque string type, mirroring packages/irac's convention of
//     keeping cross-cutting identifiers local rather than importing the
//     defining package (see packages/irac/jurisdiction.go).
//
// See doc/graph-layer.md for a fuller write-up, including the custody
// model for tenant ownership tracking and the Neo4j Cypher shapes this
// package's Migrator installs.
package graph
