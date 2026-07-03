# Key management & secrets

`packages/keymanagement` implements Phase 076's mandatory, centralized,
rotatable secret and key handling: the first real, persisted
implementation of `packages/encryption.KeySource`. Until this phase,
`encryption.LocalKeySource` was the only implementation — an explicitly
provisional default that resolves keys from an environment variable or
a local file and forgets rotated-away keys on process restart. This
package supplies the real mechanism.

## Why this package exists now

`packages/encryption/keysource.go`'s doc comment on `KeySource` says it
directly: "Phase 076 (Key management & secrets) is expected to replace
it with a real KMS-backed KeySource implementing the same interface,
at which point no caller of Encrypt/Decrypt needs to change." That is
exactly what this package does — `packages/encryption` is untouched by
this phase.

## The pieces

```
Provider            -- pluggable secrets/KMS backend (task 1)
  FileProvider       -- real, file-backed, air-gapped implementation (task 8)
  (cloud KMS)        -- documented extension point, not shipped here

KeyMetadata          -- key lifecycle record: ID, TenantID, Version,
                        State, CreatedAt, ExpiresAt (task 2)
Repository           -- persists KeyMetadata only, never key material

Service              -- role-gated orchestration: CurrentKey, Key,
                        Rotate, Revoke, ListKeys, AuditHistory,
                        GrantBreakGlass, UseBreakGlass (tasks 3, 5, 6, 7)

Adapter              -- implements encryption.KeySource + Rotator by
                        delegating to a Provider, bound to one tenant
```

## Metadata vs. material

`KeyMetadata` (types.go) is a lifecycle record — ID, tenant, version
number, `KeyState` (`active`, `rotating`, `retired`, `revoked`),
timestamps — and is the only thing `Repository` ever persists to
Postgres. The actual key bytes never touch a `KeyMetadata` row or the
`key_metadata` table; they live behind a `Provider`, sourced from an
offline/file-backed store (`FileProvider`) or a future cloud KMS
backend. This split mirrors why `packages/encryption.Key.Material` is
never logged: key material is sensitive in a way metadata is not, and
keeping them in physically separate storage paths means a Postgres
backup or a `SELECT * FROM key_metadata` can never leak a key.

## Provider: the pluggable backend

`Provider` (provider.go) mirrors `packages/provider.LLMProvider`'s
model-agnostic, no-hardcoded-backend convention exactly:

```go
type Provider interface {
    CurrentKey(ctx context.Context, tenantID string) (KeyMaterial, error)
    Key(ctx context.Context, tenantID, keyID string) (KeyMaterial, error)
    Rotate(ctx context.Context, tenantID string) (KeyMetadata, error)
}
```

`FileProvider` (fileprovider.go) is the real, file-backed
implementation for air-gapped deployments (task 8): every operation
reads/writes local files under a root directory with zero network
calls, partitioned per tenant (`<root>/<tenantID>/<keyID>.key`), with
each file's key material wrapped in AES-256-GCM under an
operator-supplied master key (`DeriveMasterKey` turns an arbitrary
passphrase into a valid one). This is the offline key store Phase 079's
air-gapped deployment tier builds on.

A cloud KMS backend is a documented extension point, not a hardcoded
dependency — see `Provider`'s doc comment for the exact contract a
cloud implementation must satisfy (tenant-scoped key resources,
historical-version resolution via `Key`, no import of this package
into the KMS adapter).

## Wiring example: swapping `LocalKeySource` for `Adapter`

This is the concrete example `packages/encryption/keysource.go`
anticipates. Before Phase 076, a caller needing an `encryption.KeySource`
looked like:

```go
keySource, err := encryption.NewLocalKeySourceFromEnv("VERDEX_ENCRYPTION_KEY")
if err != nil {
    return err
}
```

After this phase, swap in `keymanagement.Adapter`, backed by a real
`keymanagement.Provider`:

```go
// Built once per tenant, e.g. at request scope or service startup.
repo := keymanagement.NewTenantScopedRepository(pool) // or keymanagement.NewInMemoryRepository() in tests
provider, err := keymanagement.NewFileProvider(
    "/var/lib/verdex/keys",             // air-gapped-friendly root; a cloud Provider needs no root at all
    keymanagement.DeriveMasterKey(masterPassphrase),
    repo,
)
if err != nil {
    return err
}

keySource, err := keymanagement.NewAdapter(provider, tenantID)
if err != nil {
    return err
}

ciphertext, err := encryption.Encrypt(ctx, keySource, plaintext)
```

No other change to `encryption.Encrypt`/`Decrypt`/`EncryptBackup`/
`DecryptBackup` or their callers is required — `Adapter` implements
`encryption.KeySource` exactly (`var _ encryption.KeySource =
(*Adapter)(nil)`, proven against the real `Encrypt`/`Decrypt` functions
in `adapter_test.go`, not merely asserted), and additionally implements
`encryption.Rotator` (`var _ encryption.Rotator = (*Adapter)(nil)`), so
any caller that type-asserts a `KeySource` for `Rotator` — as
`packages/encryption`'s own doc comment on `Rotator` anticipates —
gets real, `Provider`-backed rotation instead of `LocalKeySource`'s
in-process-only rotation.

## Rotation (task 3)

`Service.Rotate` (and `FileProvider.Rotate` directly) creates a new
`Active` key version and demotes the tenant's prior `Active` version to
`Retired` — never deleting or forgetting it, so ciphertext written
under the old key stays decryptable. This is proven end-to-end against
the real `packages/encryption` `Encrypt`/`Decrypt` functions in both
`TestAdapter_Rotate_PreservesOldKeyDecryptability` (in-memory fixture)
and `TestFileProvider_RotationPreservesOldKeyDecryptability`
(file-backed provider), matching Phase 075's rotation-test expectation
exactly.

## Per-tenant isolation (task 4)

Every `KeyMetadata` row and every `Provider` call is scoped by tenant:
`key_metadata` carries a partial unique index enforcing at most one
`Active` key per tenant, `FileProvider` partitions key material into
per-tenant subdirectories, and `PostgresRepository`/
`TenantScopedRepository` refuse cross-tenant reads/writes
(`ErrCrossTenantAccess`). `TestService_TenantIsolation_CurrentKeyNeverCrossesTenants`
proves two tenants sharing the same `Provider`/`Repository` instances
never resolve each other's `CurrentKey`/`Key`/`ListKeys` results.

## Access control (task 5)

`Service` gates every operation on `identity.Permission`
(`access.go`), added to `identity.PermissionMatrix` by this phase:

- `identity.PermViewKeys` — read key metadata and the audit trail
  (`CurrentKey`, `Key`, `ListKeys`, `AuditHistory`). Granted to
  `RoleAdmin` and `RoleAuditor`.
- `identity.PermManageKeys` — rotate or revoke a key (`Rotate`,
  `Revoke`). Granted to `RoleAdmin` only.
- `identity.PermBreakGlassKeys` — invoke the emergency break-glass
  path (`GrantBreakGlass`, `UseBreakGlass`). Granted to `RoleAdmin`
  only.

Every `Service` method additionally requires the authenticated actor's
own `TenantID` to match the requested scope — an admin from tenant A
can never operate on tenant B's keys, even with the right permission.

## Break-glass procedure (task 6)

`GrantBreakGlass` issues a `BreakGlassGrant`: admin-only, requires a
non-blank `justification` string (`ErrJustificationRequired`
otherwise), and is time-bound (`DefaultBreakGlassTTL` of one hour if no
explicit TTL is given). `UseBreakGlass` fails closed on an expired
grant (`ErrBreakGlassExpired`), an unknown grant
(`ErrBreakGlassNotFound`), or a grant issued to a different user
(`ErrForbidden`). Every grant and every use — success or failure — is
recorded via `AuditRecorder` with the justification attached
(`AuditActionBreakGlassGrant`/`AuditActionBreakGlassUse`).

## Audit of key access (task 7)

`AuditRecorder` (audit.go) writes every `CurrentKey`/`Key`/`Rotate`/
`Revoke`/break-glass call to two places: a structured log line via
`packages/observability.AuditLogger`, and a persisted, queryable
`AuditEntry` row via `AuditRepository` (`key_audit_entries` table).
`Service.AuditHistory` exposes the queryable half directly — a caller
can list a tenant's key-access history, not just tail a log file.

## Persistence and tenant isolation

Three `Repository` implementations, mirroring
`packages/notifications`/`packages/caseversioning` exactly:

- `PostgresRepository` — backed by the `key_metadata` and
  `key_audit_entries` tables (see
  `packages/persistence/migrations/000018_create_keymanagement.up.sql`
  and `000019_enable_rls_keymanagement.up.sql`).
- `TenantScopedRepository` — wraps `PostgresRepository` with
  `packages/tenancy.WithTenantScope`, so Row-Level Security enforces
  tenant isolation at the database layer in addition to the
  application-level `requireMatchingTenant` guard. This is the type
  production code should use against a live `*pgxpool.Pool`.
- `InMemoryRepository` — for tests and other packages' fixtures.

## Testing

- `types_test.go` — `KeyMetadata.Validate` and `KeyState.IsValid`.
- `adapter_test.go` — proves `Adapter` implements `encryption.KeySource`
  (and `encryption.Rotator`) by using it with the real
  `packages/encryption` `Encrypt`/`Decrypt` functions, plus a
  rotation-preserves-decryptability round trip.
- `service_test.go` — rotation demotes the prior key to `Retired`;
  role-gated operations reject unauthorized actors (`RoleAdvocate`,
  `RoleAuditor` where `PermManageKeys` is required) and accept `RoleAdmin`;
  every operation leaves an audit entry, including denials.
- `tenant_isolation_test.go` — the real cross-tenant-leakage test:
  two tenants sharing one `Provider`/`Repository` never resolve each
  other's keys.
- `breakglass_test.go` — permission gating, blank-justification
  rejection, grant+use success with audited justification, expired
  grant rejection, unknown grant rejection, wrong-user rejection.
- `fileprovider_test.go` — offline round-trip with zero network calls
  (`t.TempDir()` only), rotation-preserves-decryptability against the
  real `packages/encryption` path, and per-tenant on-disk isolation.
- `postgres_integration_test.go` — testcontainers-backed (skipped
  under `-short`), exercising `Service` against real Postgres rows and
  proving RLS enforces the tenant boundary independently of the
  application-level guard.
