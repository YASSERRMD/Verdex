# packages/persistence

Verdex's data layer: connection pooling and lifecycle management for
PostgreSQL, schema migrations, repository implementations, transaction
helpers, and health probes for both the relational store
(PostgreSQL/pgvector) and the graph store (Neo4j).

## Driver choices

| Concern | Driver | Rationale |
|---|---|---|
| Relational store | [`github.com/jackc/pgx/v5`](https://github.com/jackc/pgx) via `pgxpool` | The de facto standard high-performance PostgreSQL driver for Go; native pooling, context-aware everywhere, no `database/sql` overhead for application code. |
| Vector storage | [`github.com/pgvector/pgvector-go`](https://github.com/pgvector/pgvector-go) | Go type support for the `pgvector` extension. Pinned now so the dependency and extension are available from day one; Phase 041 (vector index over leaf nodes) is the first phase to actually store and query embeddings with it. |
| Schema migrations | [`github.com/golang-migrate/migrate/v4`](https://github.com/golang-migrate/migrate) | Battle-tested, embeddable, supports up/down migrations and safe dirty-state recovery. Runs over `database/sql` via `pgx`'s `stdlib` shim, independent of the application's `pgxpool.Pool`. |
| Graph store client | [`github.com/neo4j/neo4j-go-driver/v5`](https://github.com/neo4j/neo4j-go-driver) | Selected as the graph store for the IRAC reasoning tree (Phase 032 onward): a mature property-graph model fits issue/rule/fact/application/conclusion nodes and their edges more naturally than a relational schema, and the official driver has first-class context support and connection pooling. **This phase only pins the dependency and wires a connectivity check** (`GraphDriver`, `GraphChecker`) — no graph schema or query logic exists yet; that is Phase 032's responsibility. |

## Connection pool

`Open(ctx, cfg *config.Config) (*Postgres, error)` builds a `pgxpool.Pool`
from `cfg.Database`:

- `DSN` — required, standard PostgreSQL connection string.
- `MaxOpenConns` — caps the pool's maximum size (`pgxpool`'s `MaxConns`).
- `MaxIdleConns` — used as the pool's minimum warm size (`MinConns`).
- `ConnMaxLifetime` — bounds how long any single connection is reused
  before being recycled.

`Open` pings once before returning, so a `*Postgres` is only ever handed
back already known to be reachable. Call `pg.Close()` when done; `Close`
and `Ping` are both safe to call on a `nil` receiver.

```go
pg, err := persistence.Open(ctx, cfg)
if err != nil {
    return err
}
defer pg.Close()
```

## Migrations

Migration files live in `packages/persistence/migrations/*.up.sql` /
`*.down.sql` and are embedded into the compiled binary via
`migrations.FS` (an `embed.FS`), so production services never need the
`migrations/` directory to exist on disk at runtime.

```go
migrator, err := persistence.NewEmbeddedMigrator(dsn) // production
// or: persistence.NewMigrator(os.DirFS("./migrations"), ".", dsn) // tooling/tests

if err := migrator.Up(ctx); err != nil { ... }
defer migrator.Close()
```

- `Up(ctx)` / `Down(ctx)` / `Steps(ctx, n)` apply, revert, or step
  migrations. All three treat "nothing to do" as success, not an error.
- `Version()` reports the current schema version and whether it is
  **dirty** (a previous migration failed partway through).
- **Dirty-state recovery:** if a migration fails mid-way, `golang-migrate`
  marks the schema dirty and refuses further `Up`/`Down`/`Steps` calls
  until an operator resolves it. Inspect the database by hand, determine
  the real schema version, then call `migrator.RecoverDirty(knownGoodVersion)`.
  `RecoverDirty` refuses to act unless the schema is actually dirty (it
  will not silently no-op a clean schema), and otherwise clears the dirty
  flag via `Force`. **You are responsible for verifying `knownGoodVersion`
  matches reality** — this only clears golang-migrate's own bookkeeping,
  it cannot verify your schema for you.

Adding a new migration: drop a new `NNNNNN_description.up.sql` /
`.down.sql` pair into `migrations/`, using the next sequential number.
No other file needs to change: `TestIntegration_MigrationsApplyCleanly`
(in `integration_test.go`) derives its expected post-migration schema
version by counting `*.up.sql` files in the embedded `migrations.FS`
rather than hardcoding a number, so it stays correct as this directory
grows.

## Repository pattern

Verdex uses **explicit per-entity repository interfaces**
(`TenantRepository`, `DeploymentRepository`) rather than a single generic
`Repository[T]`. Each entity's query shape, constraints, and default
values differ enough (e.g. a `Deployment` defaults to
`DeploymentStatusProvisioning` on `Create`, cascades from its parent
`Tenant`) that a shared generic surface would either leak
entity-specific methods back onto a common interface or hide those
differences behind a lowest-common-denominator API. Explicit interfaces
keep each repository's contract self-documenting.

Every repository method takes an `Executor` (the subset of
`*pgxpool.Pool` and `pgx.Tx` that repositories need — `Exec`, `Query`,
`QueryRow`) rather than a concrete pool, so the exact same method calls
work standalone or composed inside a transaction:

```go
tenants := persistence.NewPostgresTenantRepository()

// standalone
tenants.Create(ctx, pg.Pool(), tenant)

// inside a transaction — same call, different Executor
persistence.WithTx(ctx, pg.Pool(), func(ctx context.Context, exec persistence.Executor) error {
    return tenants.Create(ctx, exec, tenant)
})
```

`Get`/`Update`/`Delete` return `persistence.ErrNotFound` (checkable via
`errors.Is`) when no row matches.

## Transaction helper

`WithTx(ctx, pool, fn)` begins a transaction, runs `fn` with a
tx-scoped `Executor`, and:

- commits if `fn` returns `nil`,
- rolls back if `fn` returns a non-nil error (the error is returned to
  the caller, wrapped only if the rollback itself also failed),
- rolls back and **re-panics with the original panic value** if `fn`
  panics, so callers see the same panic they'd get without `WithTx` in
  the stack.

## Health probes

- `PostgresChecker(pg *Postgres) Checker` pings the pool; unhealthy if
  `pg` is `nil`, its pool is uninitialized, or the ping fails.
- `GraphChecker(target, username, password) func(ctx) error` verifies
  Neo4j connectivity. If `target` is empty (no graph store configured
  yet — expected for most deployments until Phase 032), it is a
  graceful no-op that always reports healthy.

`persistence.Checker` is defined structurally as `func(ctx
context.Context) error` to match `packages/observability`'s
health-checker function type without this package needing to import
`packages/observability` just for that signature.

## Running the test suite

- **Unit tests** (no external services required):
  ```sh
  go test ./... -short
  ```
- **Full suite, including integration tests** (spins up a real
  ephemeral PostgreSQL container via
  [`testcontainers-go`](https://golang.testcontainers.org/)):
  ```sh
  go test ./...
  ```
  This requires a working Docker daemon. Integration tests
  (`TestIntegration_*` in `integration_test.go`) bound container
  startup to a 30-second timeout and call `t.Skip` — rather than
  failing or hanging — if Docker is unreachable, so a machine without
  Docker (or with a stuck Docker daemon) still gets a green `go test
  ./...` on everything that doesn't need it. GitHub Actions'
  `ubuntu-latest` runners have a working Docker daemon, so CI always
  exercises the full integration suite for real.

Integration coverage includes: pool connectivity, migrations applying
and rolling back cleanly (with a real re-apply-after-rollback check),
tenant and deployment CRUD round-trips (including FK cascade delete),
`WithTx` commit and rollback semantics, the health checker against both
a live and a closed pool, and the dirty-migration recovery path.
