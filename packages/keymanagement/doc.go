// Package keymanagement provides Verdex's centralized, rotatable
// secret and key handling: the real implementation Phase 075
// (packages/encryption) anticipated when it defined KeySource as an
// extension point and shipped only LocalKeySource, a provisional
// default explicitly documented as pending this phase.
//
// # Why this package exists
//
// packages/encryption/keysource.go says it directly: "Phase 076 (Key
// management & secrets) is expected to replace it with a real
// KMS-backed KeySource implementing the same interface, at which
// point no caller of Encrypt/Decrypt needs to change." This package
// is that replacement — Adapter (adapter.go) implements
// encryption.KeySource (and encryption.Rotator) by delegating to a
// Provider backed by real key lifecycle management, tenant isolation,
// access control, audit, and a break-glass procedure, without any
// change to packages/encryption itself.
//
// # Provider: the pluggable backend abstraction
//
// Provider (provider.go) is this package's own extension point,
// mirroring the model-agnostic, no-hardcoded-backend convention
// packages/provider already established for LLM adapters: business
// logic depends only on the Provider interface, never on a concrete
// backend. Two implementations exist:
//
//   - FileProvider (fileprovider.go) — a real, file-backed provider
//     suitable for air-gapped deployments (task 8): it reads and
//     writes AES-256-GCM-wrapped key material under a local root
//     directory with zero network calls. This is the offline key
//     store Phase 079's air-gapped deployment tier builds on.
//   - A cloud KMS backend is a documented extension point, not a
//     hardcoded dependency: anything implementing Provider (using a
//     real KMS SDK's key-version semantics for CurrentKey/Key/Rotate)
//     plugs in without touching Service, Adapter, or the repository
//     layer. See provider.go's doc comment for the exact shape a
//     cloud implementation must satisfy.
//
// # Key lifecycle and metadata
//
// KeyMetadata (types.go) is the persisted record of a key version's
// identity, tenant, version number, KeyState (Active, Rotating,
// Retired, Revoked), and validity window. Repository (repository.go)
// persists KeyMetadata rows only — never key material, which always
// stays behind a Provider (see types.go, "Metadata vs. material").
// Repository follows the same three-implementation split as
// packages/notifications and packages/caseversioning:
// InMemoryRepository for tests, PostgresRepository for the
// `key_metadata` table (see
// packages/persistence/migrations/000018_create_keymanagement.up.sql),
// and TenantScopedRepository, which wraps PostgresRepository with
// packages/tenancy.WithTenantScope so Row-Level Security enforces
// tenant isolation at the database layer in addition to this
// package's application-level requireMatchingTenant guard.
//
// # Rotation
//
// Service.Rotate (service.go) creates a new Active key version and
// demotes the tenant's prior Active version to Retired — never
// deleting or forgetting it, so ciphertext written under the old key
// stays decryptable, matching Phase 075's rotation-preserves-old-key
// expectation (packages/encryption.Rotator's doc comment) exactly.
//
// # Per-tenant isolation
//
// Every KeyMetadata row and every Provider call is scoped by
// TenantID; FileProvider partitions key material into
// per-tenant subdirectories, and PostgresRepository/
// TenantScopedRepository refuse cross-tenant reads/writes
// (ErrCrossTenantAccess). See tenant_isolation_test.go for a real
// cross-tenant-leakage test proving one tenant's CurrentKey can never
// resolve to another tenant's key.
//
// # Access policies and break-glass
//
// access.go gates Rotate/Revoke/view-metadata operations on
// identity.Permission (PermManageKeys, added to identity's
// PermissionMatrix by this phase — see access.go's doc comment for
// why it was genuinely missing). breakglass.go implements the
// emergency-access path (task 6): admin-only
// (identity.PermBreakGlassKeys), requires an explicit non-blank
// justification string, is heavily audited (a distinct audit action
// per grant/use), and is time-bound — a grant expires and becomes
// unusable after its TTL, checked on every use via
// ErrBreakGlassExpired.
//
// # Audit
//
// Every CurrentKey/Key/Rotate/break-glass call is recorded via
// AuditRecorder (audit.go), which wraps
// packages/observability.AuditLogger and additionally persists a
// queryable AuditEntry (task 7) through Repository — a caller can
// list a tenant's key-access history, not just tail a log file.
//
// # Wiring example
//
// See doc/key-management.md for the full wiring example showing how a
// caller swaps encryption.LocalKeySource for Adapter, mirroring how
// packages/signoff/doc/signoff-workflow.md documented swapping
// guardrail.NoSignoffRecordedGate for signoff.GateImpl.
package keymanagement
