# packages/tenancy

Multi-tenant isolation, resolution, and enforcement machinery keyed to
a court/organisation deployment. The `Tenant` and `Deployment` storage
entities themselves live in `packages/persistence` (Phase 004); this
package builds the isolation, resolution, and enforcement layers
around them.

## Context propagation pattern

`context.go` mirrors the pattern
`packages/observability/context.go` already uses for correlation IDs:
an unexported context key type (`tenantContextKey`) so values stored
here can never collide with keys set by other packages, plus a
`With*`/`*FromContext` accessor pair:

```go
ctx = tenancy.WithTenant(ctx, tenant)
tenant, ok := tenancy.TenantFromContext(ctx)
```

`TenantContext` wraps `*persistence.Tenant` rather than storing it
directly so this package can grow request-scoped fields later without
changing `persistence.Tenant`'s shape.

### Middleware and composition order

`tenancy.Middleware` expects a tenant to already be resolved (attached
via `WithResolvedTenant`, e.g. by `ResolveMiddleware`) and re-attaches
it as the active tenant via `WithTenant`, so downstream handlers can
call `TenantFromContext` unconditionally. A request with no resolved
tenant gets a 401.

Compose it **inside** `observability.CorrelationMiddleware`:

```go
handler = observability.CorrelationMiddleware(logger)(
    tenancy.ResolveMiddleware(resolver)(
        tenancy.Middleware(next)))
```

Correlation IDs must exist for every request, including ones rejected
for having no resolvable tenant, so `CorrelationMiddleware` stays
outermost. Tenant resolution and its rejection path are only useful to
debug if a correlation ID is already on the context, so
`tenancy.Middleware`/`ResolveMiddleware` nest inside it, never outside.

## Row-Level Security strategy

Migration `000003_enable_rls_deployments` (in
`packages/persistence/migrations/`) enables Postgres Row-Level
Security on `deployments`:

```sql
ALTER TABLE deployments ENABLE ROW LEVEL SECURITY;
ALTER TABLE deployments FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON deployments
    USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid);
```

`FORCE ROW LEVEL SECURITY` makes the policy apply even to the table
owner (Postgres otherwise exempts owners from RLS by default).
`current_setting('app.current_tenant_id', true)` (`true` = the
`missing_ok` argument) returns `NULL` instead of raising when the
setting has never been set on the current session/transaction.
Comparing the `NOT NULL` `tenant_id` column to a `NULL`-valued cast
`uuid` evaluates to `NULL` under standard SQL three-valued logic, and a
policy's `USING` clause treats `NULL` as "row does not match" — so a
connection that never sets `app.current_tenant_id` sees **zero rows**,
not an error. This is verified directly by
`TestIntegration_UnscopedQuery_SeesZeroRowsNotError` in
`integration_test.go`.

### The connecting role must not be a superuser — `FORCE` is not enough

**This is the second most important thing a future maintainer must not
get wrong, after the `SET LOCAL` requirement below.** PostgreSQL never
applies Row-Level Security to a role with the `BYPASSRLS` attribute —
**and every superuser has `BYPASSRLS`, unconditionally, regardless of
`FORCE ROW LEVEL SECURITY`**. `FORCE` only removes the *table owner*
exemption; it cannot and does not override `BYPASSRLS`.

This is not theoretical: this phase's isolation tests first ran
against a live Postgres in CI while `pgxpool` was authenticated as
`testcontainers`' bootstrap user — a superuser, exactly like the
default admin account on many managed Postgres providers — and every
single cross-tenant assertion failed, because RLS was silently
providing zero isolation the whole time.

The fix, in migration `000005_create_app_role`: a dedicated
`verdex_app` role, created `NOSUPERUSER ... NOBYPASSRLS`, with grants
on all current and (via `ALTER DEFAULT PRIVILEGES`) future tables.
**`cfg.Database.DSN` must authenticate as this role — or an equivalent
non-superuser, non-`BYPASSRLS` role — in every real deployment.** A
DSN pointed at an admin/superuser account, however convenient for
local development, silently disables tenant isolation.

`packages/tenancy/role.go` provides the supporting machinery:

- `BootstrapAppRolePassword(ctx, exec, password)` — sets/rotates
  `verdex_app`'s login password, via the `verdex_set_app_role_password`
  SQL function the migration creates. `exec` must be an elevated
  (bootstrap/superuser) connection; the function's grants specifically
  exclude `verdex_app` itself from being able to call it. `password` is
  passed as an ordinary bound query parameter — the function performs
  the DDL's literal-quoting server-side via `format(..., %L, ...)`,
  so no client-side string concatenation of the password into SQL text
  ever happens.
- `GenerateAppRolePassword()` — a cryptographically random hex
  password generator for bootstrap tooling that needs one (tests use
  this; a real deployment's password should come from Phase 076's key
  management once that exists).
- `BuildAppRoleDSN(baseDSN, password)` — rewrites an admin DSN's
  credentials to `verdex_app`'s, preserving host/port/database/query
  parameters.
- `VerifyRLSEnforceable(ctx, pool)` — queries
  `SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user`
  against the given pool and returns an actionable error if either is
  true. **Call this once at service startup**, against the pool the
  service will actually use for tenant-scoped operations, and fail
  startup on error — catching a misconfigured DSN immediately instead
  of silently shipping with no isolation. It is deliberately *not*
  called automatically inside `WithTenantScope` (that runs per
  request; this check's cost is only worth paying once per process).

`integration_test.go` mirrors this split: `migratedPool` returns the
superuser pool (used only for running migrations and for
`SeedSandboxTenant`-style setup that touches no RLS-protected table),
while `migratedAppPool` bootstraps `verdex_app`'s password and returns
the pool every tenant-scoped assertion actually runs against — and
`TestIntegration_VerifyRLSEnforceable_RejectsSuperuserPool` /
`..._AcceptsAppRolePool` pin down the distinction explicitly.

### `WithTenantScope` and why `SET LOCAL` is mandatory

```go
err := tenancy.WithTenantScope(ctx, pool, tenantID, func(ctx context.Context, exec persistence.Executor) error {
    return deploymentRepo.Get(ctx, exec, deploymentID)
})
```

`WithTenantScope` begins a real transaction via `persistence.WithTx`
(transaction handling is not hand-rewritten here), issues
`SET LOCAL app.current_tenant_id = '<tenantID>'` as the **first**
statement inside that transaction, and only then invokes the caller's
function with the tx-scoped `Executor`.

**`SET LOCAL`, never plain `SET`, is mandatory, and this is the single
most important thing a future maintainer must not get wrong:**

- `SET LOCAL`'s effect is scoped to the current transaction. It is
  automatically and unconditionally undone when the transaction ends
  (commit or rollback) — even if the underlying physical connection is
  then returned to `pgxpool`'s pool for reuse.
- Plain `SET` has **session** scope. Its effect persists on the
  physical connection after the transaction ends. Because `pgxpool`
  multiplexes many logical callers over a small set of pooled physical
  connections, a later request for a **different tenant** could
  acquire that same connection next and silently inherit the previous
  tenant's `app.current_tenant_id` — every subsequent RLS check on that
  connection would then scope to the wrong tenant. That is a real
  cross-tenant data leak, not a style nitpick.

Do not use plain `SET` anywhere that touches `app.current_tenant_id`.

## Deployment provisioning records

`ProvisioningRecord` (in `provisioning.go`) captures the history of a
deployment's provisioning attempts: `StartedAt`, `CompletedAt`,
`Outcome` (`started` / `succeeded` / `failed`), and `ErrorDetail`.
Migration `000004_create_deployment_provisioning_records` creates the
backing table with a `deployment_id` FK to `deployments`
(`ON DELETE CASCADE`).

This type lives in `packages/tenancy` rather than as a sibling of
`Deployment` in `packages/persistence`: provisioning-attempt history is
a tenancy/deployment-lifecycle concern layered on top of the storage
entity, not a new storage primitive peer to `Tenant`/`Deployment`
themselves, and this phase intentionally leaves `packages/persistence`'s
existing entities untouched (only `TenantRepository.GetBySlug` was
added there, for resolution — see below).

`PostgresProvisioningRecordRepository` follows `packages/persistence`'s
explicit-per-entity-interface convention (see its README) rather than
a generic `Repository[T]`, for the same reason: its query shape and
defaults (e.g. `Complete` stamping `completed_at = now()`) are specific
enough that a shared generic surface would blur its contract.

## Tenant-scoped repository wrappers

`TenantScopedDeploymentRepository` (in `deployment_repository.go`)
composes `WithTenantScope` with `persistence.DeploymentRepository` so
every method takes a `tenantID` and internally opens the RLS-scoped
transaction itself:

```go
scoped := tenancy.NewTenantScopedDeploymentRepository(pool, persistence.NewPostgresDeploymentRepository())

deployment, err := scoped.Get(ctx, tenantID, deploymentID)
```

There is no way to reach the underlying `persistence.DeploymentRepository`
or `*pgxpool.Pool` from a `TenantScopedDeploymentRepository` — by
design, application code cannot accidentally call the underlying
repository directly and skip tenant scoping for tenant-owned data.

`Create` and `Update` additionally refuse — via the `ErrCrossTenantAccess`
sentinel, **before any database access** — to operate on a `Deployment`
whose `TenantID` is already set to a tenant other than the scope's
`tenantID`. This is defense in depth on top of RLS: RLS alone would
already make a mismatched deployment invisible (so `Update`/`Delete`
against another tenant's row report `persistence.ErrNotFound`), but
failing fast with `ErrCrossTenantAccess` gives a precise, checkable
error for the specific case of an obviously-mismatched `TenantID`
rather than a generic "not found".

At the middleware layer, this phase's resolution design has exactly
one source of tenant identity per request (`X-Tenant-Slug`, see
below), with no independent second source such as a tenant ID embedded
in a path parameter to cross-check against — so there is no
mismatched-source scenario for `Middleware` to reject today. See the
"Cross-tenant access denial at this layer" section of `middleware.go`'s
doc comment for the full reasoning, and revisit it once Phase 006
introduces a second source of tenant identity.

## Placeholder request resolution mechanism

`HeaderResolver` (in `resolve.go`) resolves a tenant from the
`X-Tenant-Slug` request header via `persistence.TenantRepository.GetBySlug`
(added to `packages/persistence/tenant.go` in this phase, since
resolution needed lookup-by-slug and none existed yet).

**This is a placeholder**, not a permanent auth mechanism:
`HeaderResolver` performs no authentication and trusts the header
value outright. It exists only because Phase 006 (Identity & RBAC)
does not exist yet. **Phase 006 is expected to replace `HeaderResolver`
entirely** with resolution derived from an authenticated
identity/session (e.g. a claim or session record naming the tenant) —
at which point this header-based path should be removed rather than
kept as a fallback, to avoid two divergent resolution mechanisms
coexisting.

## Sandbox tenant seed

`SeedSandboxTenant(ctx, exec)` ensures a well-known tenant with slug
`"sandbox"` exists, creating it if absent. It is idempotent
(upsert-by-slug: a second call against a database that already has it
is a no-op returning the existing tenant), including under a
create-vs-create race between two concurrently starting service
instances (the `tenants_slug_unique` constraint makes the loser's
`Create` fail, and `SeedSandboxTenant` falls back to a re-fetch rather
than surfacing that as an error). Phase 008's setup wizard and early
manual testing use this as the default tenant. It is a Go function
callable from a future bootstrap path — not a migration, since it
seeds data, not schema.

## Running the tests

- **Unit tests** (no external services required):
  ```sh
  go test ./... -short
  ```
  Covers the context helpers, middleware composition, resolver error
  paths, and the tenant-scoped repository wrapper's fail-fast
  cross-tenant guard — all without a database.

- **Full suite, including isolation integration tests** (spins up a
  real ephemeral PostgreSQL container via
  [`testcontainers-go`](https://golang.testcontainers.org/), applies
  every `packages/persistence` migration including this phase's RLS,
  provisioning-record, and app-role migrations, then bootstraps
  `verdex_app`'s password and connects as that role for every
  tenant-scoped assertion — see "The connecting role must not be a
  superuser" above for why that distinction matters):
  ```sh
  go test ./...
  ```
  Requires a working Docker daemon; bounded to a 30-second container
  startup timeout and `t.Skip`s (not fails) if Docker is unreachable,
  identical to `packages/persistence`'s policy. Integration coverage
  includes: two tenants' `TenantScopedDeploymentRepository` scopes
  cannot see/update/delete each other's deployments, an unscoped query
  sees zero rows (not an error) per the RLS behavior documented above,
  the cross-tenant guard rejects a mismatched `TenantID` before the
  database is touched, the sandbox tenant seed creates once and is
  idempotent on re-run, and `VerifyRLSEnforceable` correctly
  distinguishes the superuser pool from the `verdex_app` pool.
