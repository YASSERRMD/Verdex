// Package bulkimport is Phase 088: onboarding large historical case
// corpora into a tenant in bounded, resumable, auditable batches. It
// borrows the orchestration/state-machine *shape* of the live-intake
// pipeline added in Phase 029 (packages/ingestion) -- a checkpointed
// state machine processed in bounded units with resumability after a
// crash -- but is not built on top of it: batch-corpus onboarding
// (thousands to millions of already-digitized case records read from
// a source description) is a different problem from Phase 029's
// per-document live-intake pipeline (validate/hash/discard one
// uploaded binary through OCR/STT/segmentation/classification), so
// this package models its own ImportJob/ImportRecord state machine
// rather than importing packages/ingestion's Job/WorkflowState types.
// It writes every job registration, batch run, and rollback into the
// durable, hash-chained audit trail added in Phase 077
// (packages/auditlog) via AuditSink, exactly as every phase since 077
// has, and references (by ID only, never by import) the case records
// added in Phase 063 (packages/caselifecycle) that a deployment
// modeling downstream case creation would create from a successfully
// imported record.
//
// # What is new here
//
//   - ImportJob / Status (types.go): a tenant-scoped bulk-import run
//     -- source description, total/processed/failed/skipped/imported
//     counts, and a resumability Cursor -- with a guarded state
//     machine (Pending -> Running -> Paused ->
//     Completed/Failed/RolledBack) enforced by
//     Status.CanTransitionTo, mirroring
//     packages/caselifecycle.State's transition-table approach rather
//     than scattering ad hoc if-statements across call sites (task 1).
//   - ImportRecord / ValidationStatus / Outcome (types.go): one row of
//     the source corpus -- a PayloadRef (reference only, never
//     dereferenced by this package, mirroring
//     packages/privacy.DataInventoryEntry.SourceTag's convention),
//     structural fields (CaseNumber/Jurisdiction/PartyNames), a
//     computed DedupKey, ValidationStatus/ValidationErrors, and a
//     final Outcome (Imported / SkippedDuplicate / Rejected +
//     reason / RolledBack) (task 1).
//   - RecordSource / SourceRecord (source.go): the pluggable read side
//     of a bulk-import job -- ReadAt(index, count) reads a bounded
//     window of the source corpus without requiring the whole corpus
//     in memory, and must be deterministic across calls at the same
//     index so Cursor-based resumability is correct.
//     InMemoryRecordSource is the test/small-corpus implementation;
//     see doc/bulk-import.md for why a real deployment implements its
//     own against durable storage.
//   - Engine.RunBatch (engine.go): real batched-chunking logic (task
//     2) -- reads up to batchSize records from a RecordSource starting
//     at ImportJob.Cursor, runs each through Validate -> dedup-lookup
//     -> import, and persists the updated Cursor/counters/Status once
//     per call. Crash-safe: a failure between RunBatch calls leaves
//     Cursor at the last successfully processed record, so the next
//     call resumes from there rather than reprocessing or skipping
//     records (task 4).
//   - Validator / DefaultValidator (validator.go): an interface
//     producing structured []ValidationError per record (task 3), not
//     a bare bool -- a corpus owner's error report needs to know which
//     field failed and why. DefaultValidator checks required-field
//     presence and blank-entry structural well-formedness;
//     Engine.SetValidator lets a deployment with a richer source
//     schema supply its own.
//   - ComputeDedupKey (dedup.go): a real lookup-backed dedup function
//     (task 5) -- a SHA-256 digest of the normalized (lower-cased,
//     trimmed, party-names-sorted) case number + jurisdiction + party
//     names, so formatting differences and party-name ordering never
//     produce a false negative. RunBatch looks the key up against
//     already-imported records in the same job/tenant via
//     RecordRepository.FindByDedupKey and records a true match as
//     OutcomeSkippedDuplicate rather than re-importing it.
//   - Progress / EstimatedTimeRemaining (progress.go): a
//     processed/total/failed/skipped snapshot with a computed percent
//     and a linear-extrapolation ETA from the observed processing rate
//     (task 6), returned by Engine.GetProgress.
//   - Engine.Rollback (engine.go): a real state transition (task 7),
//     not a no-op -- moves every OutcomeImported ImportRecord in the
//     job to OutcomeRolledBack (clearing any CreatedCaseID reference)
//     and the ImportJob itself to StatusRolledBack, only reachable
//     from StatusCompleted or StatusFailed
//     (ErrJobNotRollbackable otherwise).
//   - AuditSink (audit.go): records every RegisterJob, RunBatch, and
//     Rollback call into packages/auditlog.Store -- the same durable,
//     hash-chained sink the rest of the platform already writes to and
//     queries. No second audit table (task 8).
//   - identity.PermViewBulkImport / identity.PermManageBulkImport
//     (packages/identity/permission.go): the fine-grained permissions
//     this package's Engine gates every operation on, following the
//     exact PermViewIntegration/PermManageIntegration precedent from
//     Phase 087.
//   - JobRepository / RecordRepository, InMemoryJobRepository /
//     InMemoryRecordRepository, PostgresJobRepository /
//     PostgresRecordRepository, TenantScopedJobRepository /
//     TenantScopedRecordRepository (repository.go,
//     inmemory_repository.go, postgres_repository.go,
//     tenant_scoped_repository.go): the same three-layer repository
//     pattern (in-memory for tests, Postgres for the SQL, tenant-scoped
//     wrapping WithTenantScope for RLS enforcement in addition to the
//     application-level tenant guard) every phase since Phase 004 has
//     used.
//
// # Storage
//
// ImportJob and ImportRecord rows live in the `bulkimport_jobs` and
// `bulkimport_records` tables (see
// packages/persistence/migrations for the exact migration numbers --
// this phase's migrations were authored against the tip of main at
// the time and may have been renumbered by a coordinator process to
// avoid colliding with a sibling phase landing in parallel; the tables
// themselves are unaffected by any such renumbering). Both tables
// carry a tenant_id column and get a tenant_isolation Row-Level
// Security policy, exactly like every tenant-scoped table added since
// Phase 005. Cursor lives directly on the bulkimport_jobs row, so a
// resumed job's checkpoint survives a process restart, not just an
// in-memory Engine's lifetime.
//
// # What is explicitly reused, not duplicated
//
//   - packages/auditlog.Store is the only durable event sink this
//     package writes to, via AuditSink -- exactly the composition
//     pattern packages/privacy's, packages/compliance's, and
//     packages/integration's own AuditSink established.
//   - identity.Role / identity.Permission / identity.HasPermission
//     (Phase 006) remain the coarse RBAC gate every Engine method
//     calls through authorizeManage/authorizeView before doing
//     anything bulk-import-specific.
//   - packages/caselifecycle.Case (Phase 063) remains the only case
//     record type in this codebase; ImportRecord.CreatedCaseID
//     references a case by uuid.UUID only, exactly mirroring
//     packages/compliance.Control.MappedTo's and
//     packages/privacy.DataInventoryEntry.SourceTag's
//     reference-by-convention pattern -- this package does not import
//     packages/caselifecycle to create, read, or delete a Case.
//   - packages/ingestion's checkpointed-state-machine-with-retries
//     *shape* (Phase 029) is the reference Engine.RunBatch's
//     Cursor-based resumability follows, not a dependency -- this
//     package does not import packages/ingestion. A bulk-corpus import
//     job and a live per-document intake job have different unit of
//     work, different failure modes, and different resumability
//     granularity (a batch of N records vs. a single document's
//     pipeline stage), so sharing the shape without sharing the types
//     avoids forcing an unrelated abstraction onto either package.
//
// See doc/bulk-import.md for the full write-up, including the state
// diagram and the rationale for RecordSource's ReadAt contract.
package bulkimport
