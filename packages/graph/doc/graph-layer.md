# Graph Store Integration

`packages/graph` persists IRAC reasoning trees (`packages/irac` nodes and
edges — Phase 031) in a graph database. This phase (032) is the storage
layer `packages/irac` explicitly deferred: that package defines the schema
and validation only, with no persistence backend.

## Why Neo4j

Neo4j was already selected and pinned as this project's graph database in
Phase 004 (`packages/persistence/neo4j.go`). Its doc comment is explicit
about the split of responsibility:

> "Phase 032 owns real graph operations (queries, sessions, transactions);
> this phase only pins the driver dependency and provides a
> connectivity-check primitive."

Neo4j's native property graph model — nodes, directed typed relationships,
Cypher traversal queries — maps directly onto `irac.Node` / `irac.Edge`
without an impedance mismatch: an IRAC reasoning tree already *is* a graph
(issues, rules, facts, applications, and conclusions connected by
`governs` / `applies_to` / `supports` / `concludes_from` edges), so no
relational normalization or document-modeling compromise is needed.

`persistence.GraphDriver` intentionally exposes only connectivity-check and
health-probe primitives — it has no session, query, or transaction
surface, and its internal driver field is unexported. This package
therefore opens its own `neo4j.DriverWithContext` (via the same
target/username/password shape `persistence.NewGraphDriver` accepts) for
the actual query/migration/transaction work, while still delegating
connectivity verification and health probing to
`persistence.GraphDriver` / `persistence.GraphChecker` (see `health.go`
and `Neo4jHealthChecker`). This keeps `packages/persistence`
untouched — this phase only imports and depends on it, per the module
boundary — while giving `packages/graph` the session-capable driver
access it actually needs.

## GraphStore: the storage-agnostic interface

Every capability in this package is written purely in terms of the
`GraphStore` interface (`store.go`):

```go
type GraphStore interface {
    CreateNode(ctx context.Context, node irac.Node) error
    CreateEdge(ctx context.Context, edge irac.Edge) error
    GetNode(ctx context.Context, id string) (irac.Node, error)
    Traverse(ctx context.Context, query TraversalQuery) ([]irac.Node, error)
    DeleteTree(ctx context.Context, caseID string) error
}
```

`CreateNode` is an idempotent upsert (overwriting a node with the same ID),
matching how IRAC trees are produced as immutable revisions
(`irac.TreeRevision`) rather than mutated in place.

`TraversalQuery` supports three composable filters: `CaseID` (mandatory —
every traversal is scoped to one case's tree), `NodeType` (optional, e.g.
"only conclusion nodes"), and `FromNodeID` + `MaxDepth` (optional,
breadth-first walk outward along edges, bounded by hop count).

## InMemoryGraphStore

`InMemoryGraphStore` (`inmemory.go`) is a fully in-memory implementation
backed by maps, safe for concurrent use via a `sync.RWMutex`. It is the
default implementation used by:

- this package's own unit tests (everything except
  `neo4j_integration_test.go`),
- downstream reasoning phases (033-040) that don't need a live Neo4j
  instance to test their own logic.

Because `TenantScopedStore`, `WithTransaction`, `Export`/`Import`, and
`HealthCheck` are all written against the `GraphStore` interface, they
work identically over `InMemoryGraphStore` today and would work
identically over a future full Neo4j-backed `GraphStore` implementation
without any changes to their own logic.

## Tenant isolation

`TenantScopedStore` (`tenant.go`) wraps any `GraphStore` and scopes every
read and write to a single `TenantID` — a locally-defined string type with
no hard dependency on `packages/tenancy`, mirroring `packages/irac`'s own
convention of keeping cross-cutting identifiers (`JurisdictionCode`,
`LegalFamily`) as opaque local types rather than importing their defining
package.

Ownership is tracked by node ID in a map (`owners map[string]TenantID`)
that can be shared across multiple `TenantScopedStore` values wrapping the
same inner store — one per tenant — via `NewTenantScopedStore`'s `owners`
parameter, so a write made by one tenant's view is visible to another
tenant's cross-access checks.

**Cross-tenant access is rejected, not silently filtered**, except for
`Traverse`, where filtering is the correct behavior (a multi-tenant
traversal should return only the caller's own nodes rather than error
just because *other* tenants happen to have nodes in the same query
scope). Every point write (`CreateNode` overwriting another tenant's node,
`CreateEdge` spanning another tenant's node, `GetNode` on another tenant's
node, `DeleteTree` touching another tenant's node) returns
`ErrCrossTenantAccess` — a caller must never mistake "no access" for "no
data".

`DeleteTree` first checks that *every* node in the target case is owned by
the calling tenant (or unowned) before deleting anything, so a rejected
cross-tenant delete never partially removes the calling tenant's own
nodes from a case another tenant also has nodes in.

## Migrations and indexing

`Migration` (`migrate.go`) is a name plus a Cypher statement, written to be
idempotent (`CREATE CONSTRAINT ... IF NOT EXISTS`, `CREATE INDEX ... IF
NOT EXISTS`). `Migrator.Apply` runs every migration in an auto-commit
session (Neo4j requires schema statements outside an explicit
transaction) and stops at the first failure. Applying the same `Migrator`
twice is safe — every built-in migration's idempotent Cypher makes the
second `Apply` call a no-op.

The built-in `coreMigrations()` install:

| Name | Cypher | Purpose |
|---|---|---|
| `irac_node_id_unique` | `CREATE CONSTRAINT ... FOR (n:IracNode) REQUIRE n.id IS UNIQUE` | Backs `CreateNode`'s upsert semantics with Neo4j's own uniqueness enforcement |
| `irac_node_type_index` | `CREATE INDEX ... FOR (n:IracNode) ON (n.type)` | Backs `Traverse`'s `NodeType` filter |
| `irac_node_case_id_index` | `CREATE INDEX ... FOR (n:IracNode) ON (n.case_id)` | Backs `Traverse`'s mandatory `CaseID` filter |

`NewInMemoryMigrator()` returns a no-op stand-in: `InMemoryGraphStore` has
no schema, so its `Apply` always succeeds immediately. This lets a caller
that selects a `GraphStore` implementation at runtime treat "apply
migrations" uniformly regardless of backend.

`InMemoryGraphStore` mirrors the same two indexes internally
(`index.go`): a `byCase` map (case ID -> node ID set) and a `typeIndex`
(`inMemoryIndex`, node type -> node ID set), both kept in sync on every
`CreateNode` (including on the "retype" edge case, where an upsert changes
a node's `Type` or `CaseID`) and `DeleteTree`. `Traverse` intersects these
indexed candidate sets rather than scanning every node in the store.

## Transactions

`WithTransaction(ctx, store, fn)` (`tx.go`) gives `fn` the illusion of an
atomic sequence of `GraphStore` writes. For `*InMemoryGraphStore`, it uses
a **snapshot-and-rollback-on-error** strategy: it deep-copies the store's
internal maps before invoking `fn`, and restores that snapshot if `fn`
returns an error or panics (re-panicking with the original value after
restoring). This is sufficient for a single-process in-memory store and
keeps the rollback path simple and obviously correct, without needing to
track and undo individual writes.

**A Neo4j-backed `GraphStore` must not reuse this snapshot strategy.** It
should instead open a real Neo4j transaction (`session.BeginTransaction`
or `session.ExecuteWrite`) via the session-capable driver introduced in
`migrate.go`, run every write against the transaction handle, and
commit/rollback it directly — atomicity enforced by the database itself,
not by an in-process copy of state that database transactions may not
even fully capture (e.g. concurrent writers, other processes).

## Backup and restore

`Export(ctx, store, caseID)` (`backup.go`) serializes every node and edge
in a case into a JSON envelope, reusing `irac.MarshalTree` as the wire
format so a backup produced here is byte-for-byte the same shape
`packages/irac` itself would produce for the same tree contents. `Import`
decodes that envelope via `irac.UnmarshalTree` and replays it into any
`GraphStore`.

`GraphStore` operates on the base `irac.Node` shape, not `packages/irac`'s
concrete typed wrappers (`IssueNode`, `RuleNode`, ...), since those
wrappers carry fields (`Spans`, `JurisdictionCode`, `Label`, ...) this
package's interface does not persist. `Export` bridges that gap by
reconstructing the minimal typed wrapper implied by each node's
`NodeType` before calling `irac.MarshalTree` — for `NodeConclusion` this
means attaching the mandatory `draft_analysis` guardrail label via
`irac.NewConclusionNode`, since `irac.MarshalTree` refuses to encode a
`ConclusionNode` without it. `Import` flattens typed wrappers back down to
the base `irac.Node` shape before calling `CreateNode`.

Edge listing is not part of the `GraphStore` interface (only
`CreateEdge`), so `Export` opportunistically type-asserts for an
`EdgesForCase(caseID) []irac.Edge` accessor, which `InMemoryGraphStore`
implements. A `GraphStore` that doesn't implement this accessor still
exports successfully — just with zero edges, since a node-only backup is
still valid.

## Health checks

`HealthCheck(ctx, store)` (`health.go`) always reports healthy for
`*InMemoryGraphStore` (nothing external to fail against), and falls
through to a store-provided `HealthCheck(ctx) error` method when the
concrete `GraphStore` implementation exposes one.

`Neo4jHealthChecker(target, username, password)` returns an
`observability.Checker`-compatible function by delegating directly to
`persistence.GraphChecker` — the same connectivity probe wired up for
`/readyz` in Phase 004. If `target` is empty (no Neo4j endpoint
configured), the returned checker is a graceful no-op that always reports
healthy, matching most deployments not having a Neo4j endpoint configured
yet.

## Testing strategy

Every test in `store_test.go`, `tenant_test.go`, `migrate_test.go` /
`migrate_internal_test.go`, `index_test.go`, `tx_test.go`,
`backup_test.go`, and `health_test.go` runs unconditionally against
`InMemoryGraphStore` — no Docker, no network required.

`neo4j_integration_test.go` uses `testcontainers-go/modules/neo4j` to spin
up a real Neo4j container and exercises `persistence.GraphDriver`
connectivity, `Migrator.Apply` against a live database (including the
idempotency guarantee on a second `Apply`), and a raw node CRUD round
trip via Cypher. It follows the exact skip pattern established by
`packages/persistence/integration_test.go`:

- `t.Skip(...)` unconditionally when `testing.Short()` is true, so
  `go test -short` never attempts to start a container;
- `t.Skipf(...)` (not `t.Fatalf`) if the container fails to start, so a
  missing/unreachable Docker daemon degrades to a skip rather than a
  hard failure.
