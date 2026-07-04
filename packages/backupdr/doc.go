// Package backupdr is Phase 085: resilient backup and disaster
// recovery for this platform's own data. It draws on the encrypted-
// backup primitive added in Phase 075 (packages/encryption's
// EncryptBackup/DecryptBackup), the durable, hash-chained audit trail
// added in Phase 077 (packages/auditlog), the hash-verification
// pattern established in Phase 020 (packages/provenance), and the
// role/permission model added in Phase 006 (packages/identity),
// composing them into a tenant-scoped, auditable backup-policy,
// restore-drill, and DR-orchestration layer rather than duplicating
// any of them.
//
// # What is new here
//
//   - DataClass / BackupPolicy (types.go): a closed taxonomy of what
//     this platform backs up (case data, corpus/precedent, audit log,
//     config), each with its own per-tenant BackupPolicy: Frequency,
//     RetentionWindow, EncryptionRequired, and CrossRegionRequired.
//     EncryptionRequired references packages/encryption's
//     EncryptBackup by name, not by import -- this package records the
//     requirement, backup-execution tooling enforces it (task 1).
//   - BackupRecord (types.go): a completed (or attempted) backup's
//     metadata -- DataClass, TakenAt, BackupLocation (primary-region,
//     cross-region, or offline), Reference, IntegrityHash, SizeBytes,
//     Encrypted, and BackupStatus. Never stores the backup's actual
//     bytes, mirroring packages/provenance.ProvenanceRecord's
//     hash-not-content shape (tasks 2-4).
//   - RecoveryPoint / ResolveRecoveryPoint (pitr.go): point-in-time
//     recovery. Real selection logic over a []BackupRecord -- the
//     latest succeeded record at-or-before a requested instant -- not
//     a stub (task 3).
//   - RestoreDrill / Engine.RunDrill (drill.go, engine.go): a
//     scheduled/executed drill record (DataClass, ExecutedAt,
//     Executor, DrillOutcome, Duration, Notes) plus real
//     restore-and-verify simulation logic (simulateRestore) whose
//     outcome genuinely depends on the source BackupRecord's status
//     and integrity, not a hardcoded success (task 5).
//   - Target / EvaluateRPO / EvaluateRTO (target.go): per-tenant,
//     per-DataClass RPO/RTO targets, plus real evaluation of whether a
//     given BackupRecord's age satisfies its class's RPO and whether a
//     RestoreDrill's duration satisfied its class's RTO (task 6).
//   - Runbook / RunbookStep / DefaultDRRunbook (runbook.go) plus
//     doc/dr-runbook.md: a structured, ordered DR procedure (each step
//     with a Description and an OwnerRole) and its human-readable
//     counterpart, kept in the same step order deliberately (task 7).
//   - VerifyIntegrity / ComputeIntegrityHash (integrity.go): compares a
//     BackupRecord's stored hash against a freshly computed one,
//     mirroring packages/provenance's ComputeChainHash
//     stored-vs-recomputed comparison pattern by reference (task 8).
//   - AuditSink (audit.go): records every policy set, backup, drill,
//     and target change into packages/auditlog.Store -- the same
//     durable, hash-chained sink the rest of the platform already
//     writes to and queries. No second audit table.
//   - identity.PermViewBackupDR / identity.PermManageBackupDR
//     (packages/identity/permission.go): the fine-grained permissions
//     this package's Engine gates every operation on, following the
//     exact PermViewThreatmodel/PermManageThreatmodel precedent from
//     Phase 083.
//
// # What is explicitly reused, not duplicated
//
//   - packages/encryption's EncryptBackup/DecryptBackup (Phase 075)
//     remain the only backup-encryption mechanism in this codebase;
//     BackupPolicy.EncryptionRequired and BackupRecord.Encrypted
//     reference that capability by name, never reimplementing
//     envelope encryption here.
//   - packages/auditlog.Store is the only durable event sink this
//     package writes to, via AuditSink -- exactly the composition
//     pattern packages/compliance's and packages/privacy's own
//     AuditSink established.
//   - packages/provenance's hash-verification shape (ComputeChainHash's
//     stored-vs-recomputed comparison) is the reference
//     VerifyIntegrity follows, not a dependency -- this package does
//     not import packages/provenance.
//   - identity.Role / identity.Permission / identity.HasPermission
//     (Phase 006) remain the coarse RBAC gate every Engine method
//     calls through authorizeManage/authorizeView before doing
//     anything backup/DR-specific.
//
// See doc/backup-dr.md for the full write-up.
package backupdr
