# Backup, restore & disaster recovery (Phase 085)

This phase draws together four earlier threads -- the encrypted-backup
primitive added in Phase 075 (`packages/encryption`), the durable,
hash-chained audit trail added in Phase 077 (`packages/auditlog`), the
hash-verification pattern established in Phase 020
(`packages/provenance`), and the static role/permission model added in
Phase 006 (`packages/identity`) -- into a single, tenant-scoped,
auditable backup-policy, restore-drill, and DR-orchestration layer:
`packages/backupdr`.

## Goal

Make this platform's own data resilient to loss: define what gets
backed up and how often, prove backups are encrypted and their
integrity is verifiable, resolve "what's the nearest good backup as of
time T", regularly *prove* restores actually work via drills, define
and evaluate concrete recovery objectives (RPO/RTO), and give
responders a runbook rather than institutional memory.

## What this package composes from, versus what is new

| Existing piece | What it already provides | What this phase adds |
|---|---|---|
| `packages/encryption` (Phase 075) | `EncryptBackup`/`DecryptBackup` -- versioned envelope encryption for whole backup artifacts | `BackupPolicy.EncryptionRequired` records the requirement per `DataClass`; `BackupRecord.Encrypted` records whether a specific backup satisfied it. Neither calls `EncryptBackup` directly -- referenced by name, enforced by backup-execution tooling |
| `packages/auditlog` (Phase 077) | Hash-chained, tenant-scoped, queryable `Event` store; `Kind` taxonomy | `AuditSink` projects every policy set, backup, drill, and target change into that same store; no parallel log |
| `packages/provenance` (Phase 020, by reference only) | `ComputeChainHash`'s stored-vs-recomputed hash comparison pattern | `VerifyIntegrity` follows the identical shape for a single artifact's content hash, without importing `packages/provenance` |
| `packages/identity` (Phase 006) | `Role`, `Permission`, `PermissionMatrix`, `HasPermission` | `PermViewBackupDR`/`PermManageBackupDR`: the two fine-grained permissions this package's `Engine` gates every operation on |

## Backup policy per data class (task 1)

`DataClass` (types.go) is a closed, four-value taxonomy --
`DataClassCaseData`, `DataClassCorpusPrecedent`, `DataClassAuditLog`,
`DataClassConfig` -- unlike `packages/compliance.Framework`'s
deliberately open string type. The set of data classes this platform
stores is a property of its own schema, not something a customer or
jurisdiction varies, so a fixed enum is the right shape here (and
`exhaustive`-lint-friendly switches stay meaningful).

`BackupPolicy` binds a `DataClass` to:

- `Frequency`: how often a backup must be taken.
- `RetentionWindow`: how long a completed backup is kept.
- `EncryptionRequired`: whether every backup of this class must be
  wrapped via `packages/encryption`'s `EncryptBackup` -- referenced by
  name, not imported. This package records the requirement; it is not
  itself the code path that calls `EncryptBackup`.
- `CrossRegionRequired`: whether a copy must additionally land outside
  `LocationPrimaryRegion` (task 4).

One `BackupPolicy` exists per tenant/`DataClass` pair, mirroring
`packages/privacy.Engine`'s `retentionPolicies` registry shape and,
at the persistence layer, `packages/compliance`'s per-tenant profile
row shape.

## Encrypted automated backups (task 2)

`packages/encryption`'s `EncryptBackup`/`DecryptBackup` (added in
Phase 075 as a distinct, self-documenting entry point over the same
AES-256-GCM envelope `Encrypt`/`Decrypt` use) remain the only backup-
encryption mechanism in this codebase. This phase does not reimplement
envelope encryption: `BackupRecord.Encrypted` simply records whether a
completed backup was wrapped that way, and `BackupPolicy.EncryptionRequired`
records whether it was supposed to be. Automated scheduling itself
(the "automated" half of task 2) is deployment-tooling's job -- a cron
job or scheduler calls `Engine.RecordBackup` once a backup completes,
the same way a restore drill calls `Engine.RunDrill`.

## Point-in-time recovery (task 3)

`RecoveryPoint` (pitr.go) answers "if I restore `Class` for `TenantID`
as of `RequestedAt`, which `BackupRecord` do I actually recover from,
and how stale would that be." `ResolveRecoveryPoint` is real selection
logic over a `[]BackupRecord`:

- Filters to the given tenant/class.
- Filters to `BackupStatusSucceeded` records only -- a failed or
  still-verifying backup is never a valid recovery point.
- Filters to records with `TakenAt` at-or-before the requested instant
  -- PITR only ever recovers data as fresh as the nearest prior
  backup, never data from after the requested point.
- Selects the single *latest* qualifying record and reports
  `AgeAtRequest` (`RequestedAt - Record.TakenAt`), the exact figure
  `EvaluateRPO` (task 6) judges against the tenant's tolerance.

Returns `ErrNoRecoveryPoint` when nothing qualifies (e.g. every backup
postdates the request, or none exist yet for that tenant/class).

## Cross-region and offline backup options (task 4)

`BackupLocation` is a closed three-value type:
`LocationPrimaryRegion`, `LocationCrossRegion`, and `LocationOffline`.
`BackupPolicy.CrossRegionRequired` marks which data classes must
maintain a copy outside the primary region; `BackupRecord.Location`
records where a specific completed backup actually landed.
`LocationOffline` is deliberately described as "referenced by name
only" to `packages/airgapped`'s export flow in the type's doc comment
-- this package does not import `packages/airgapped` to produce an
offline bundle, it only records that a `BackupRecord` is one.

## Restore drills (task 5)

`RestoreDrill` (drill.go) is a scheduled/executed drill record: which
`DataClass`, which source `BackupRecord`, `ExecutedAt`, `Executor`,
`DrillOutcome` (`Success`/`Failure`/`Partial`), `Duration`, and
`Notes`. `Engine.RunDrill` is real state tracking, not a type with no
logic:

1. Resolves the source `BackupRecord` by ID.
2. Calls `simulateRestore`, which -- in this in-memory-test context --
   simulates the restore-and-verify cycle: a record whose `Status`
   isn't `BackupStatusSucceeded` cannot be restored from at all
   (`DrillOutcomeFailure`); a record that fails `VerifyIntegrity`
   against the caller-supplied recomputed hash restored but didn't
   verify (`DrillOutcomePartial`); otherwise the drill succeeded
   (`DrillOutcomeSuccess`).
3. Persists the resulting `RestoreDrill`, timestamped and durationed
   from the actual simulated cycle, and audits it via `AuditSink`.

A real backup-execution environment would swap `simulateRestore`'s
caller-supplied `recomputedHash` for a hash computed from bytes an
actual restore step read back -- the decision logic (status check,
integrity check, outcome resolution) does not change.

## RPO/RTO targets (task 6)

`Target{TenantID, Class, RPO, RTO}` (target.go) is a per-tenant,
per-`DataClass` pair of recovery objectives. Two evaluation functions,
both real comparisons rather than stubs:

- `EvaluateRPO(record, target, asOf)`: computes `asOf - record.TakenAt`
  and reports `Met` iff that age is within `target.RPO`. Rejects a
  `Class`-mismatched `Target` (`ErrInvalidTarget`) rather than
  silently comparing against the wrong data class's tolerance.
- `EvaluateRTO(drill, target)`: reports `Met` iff `drill.Duration` is
  within `target.RTO`. Same mismatch guard.

`Engine.CheckRPO`/`Engine.CheckRTO` are the permission-gated,
tenant-scoped entry points that resolve the registered `Target` and
delegate to these two functions.

### Starter targets

A deployment registers its own `Target` per tenant/`DataClass` via
`Engine.SetTarget`; this package does not hardcode a single global
default (different tenants' contractual/regulatory obligations
differ). As a *reference point only* (not enforced by this package),
a reasonable starting posture per `DataClass`:

| DataClass | Suggested RPO | Suggested RTO | Rationale |
|---|---|---|---|
| `case_data` | 1 hour | 4 hours | Actively written; a judge/clerk depends on it being current |
| `corpus_precedent` | 24 hours | 8 hours | Read-mostly reference corpus; changes are batch ingestions, not live edits |
| `audit_log` | 15 minutes | 4 hours | The audit trail is itself the evidence of what happened during an incident -- cannot tolerate much loss |
| `config` | 1 hour | 2 hours | Small, infrequently-changed, but blocks everything else from coming back up until restored |

## DR runbook (task 7)

`Runbook`/`RunbookStep` (runbook.go) is the structured data model:
`Name`, and an ordered `[]RunbookStep`, each with a `Description` and
an `OwnerRole`. `DefaultDRRunbook()` returns a ten-step region-outage
procedure kept in the *exact same order* as the new
`doc/dr-runbook.md`, deliberately -- the two are meant to be edited
together, never let drift apart. See `doc/dr-runbook.md` for the full,
human-readable procedure; it explicitly ties each step back to
`ResolveRecoveryPoint`, `VerifyIntegrity`, `EvaluateRTO`, and
`Engine.RunDrill` rather than describing a disconnected process.

## Backup integrity verification (task 8)

`VerifyIntegrity(record, computedHash)` (integrity.go) compares
`record.IntegrityHash` (the hash stored at backup time) against
`computedHash` (a hash the caller freshly computed from the actual
backup bytes, e.g. via `ComputeIntegrityHash`) and reports a match or
`ErrIntegrityMismatch`. This mirrors `packages/provenance`'s
`ComputeChainHash` stored-vs-recomputed comparison pattern by
reference -- same "recompute and compare" shape, applied to a single
artifact's content hash rather than a hash-chain link -- without this
package importing `packages/provenance`. `ComputeIntegrityHash`
produces hex-encoded SHA-256, the same format
`packages/provenance.ProvenanceRecord.ContentHash` uses, so the two
are comparable by convention even without a shared type.

## Access control

Two new `identity.Permission` constants gate every `Engine` operation,
added following `permission.go`'s exact
`PermViewThreatmodel`/`PermManageThreatmodel` precedent from Phase
083:

- `backupdr:view` (`identity.PermViewBackupDR`): read-only access to
  backup policies, backup record history, restore-drill history, and
  RPO/RTO evaluation results.
- `backupdr:manage` (`identity.PermManageBackupDR`): set a backup
  policy, record a backup, set an RPO/RTO target, or execute a
  restore drill.

`RoleAdmin` holds both; `RoleAuditor` holds only the view permission,
consistent with its read-only, compliance-facing posture elsewhere in
the matrix (see `packages/identity/doc/rbac-matrix.md`).

## Storage

Two new migration pairs, continuing directly after
`000027_enable_rls_compliance`:

- `packages/persistence/migrations/000028_create_backupdr.up.sql` /
  `.down.sql` create four tenant-scoped tables: `backupdr_policies`,
  `backupdr_records`, `backupdr_drills`, and `backupdr_targets`.
- `packages/persistence/migrations/000029_enable_rls_backupdr.up.sql`
  / `.down.sql` enable and force row-level security with the standard
  `tenant_isolation` policy on all four tables.

Each table follows the same `Repository` / `PostgresXRepository` /
`TenantScopedXRepository` three-layer pattern established by
`packages/privacy` and `packages/compliance`, with Row-Level Security
enforcing tenant isolation at the database layer in addition to each
repository's own application-level `requireMatchingTenant` guard.

## What is explicitly reused, not duplicated

- `packages/encryption`'s `EncryptBackup`/`DecryptBackup` (Phase 075)
  remain the only backup-encryption mechanism in this codebase; this
  package references that capability by name via
  `BackupPolicy.EncryptionRequired`/`BackupRecord.Encrypted`, never
  reimplementing envelope encryption.
- `packages/auditlog.Store` is the only durable event sink this
  package writes to, via `AuditSink`.
- `packages/provenance`'s hash-verification shape
  (`ComputeChainHash`'s stored-vs-recomputed comparison) is the
  reference `VerifyIntegrity` follows, not a dependency -- this package
  does not import `packages/provenance`.
- `identity.Role`/`identity.Permission`/`identity.HasPermission`
  remain the coarse RBAC gate every `Engine` method calls through
  before doing anything backup/DR-specific.
