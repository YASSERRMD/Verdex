// Package persistence provides the Verdex data layer: connection
// pooling and lifecycle management for PostgreSQL, schema migrations,
// repository implementations, transaction helpers, and health probes
// for both the relational store (PostgreSQL/pgvector) and the graph
// store (Neo4j, wired out fully in a later phase). See the package
// README for driver choices, pool configuration, migration workflow,
// the repository pattern, the transaction helper, the health checker,
// and how to run the integration test suite.
package persistence
