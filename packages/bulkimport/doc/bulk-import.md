# Bulk import & migration tools (Phase 088)

This phase adds `packages/bulkimport`: the batch-onboarding path for a
large historical case corpus, distinct from the live, one-document-at-
a-time intake pipeline added in Phase 029 (`packages/ingestion`).

## Goal

A deployment migrating onto Verdex typically arrives with years of
existing case records -- a CSV export from a legacy case-management
system, a DMS export API, a stream of rows from an old database. This
phase provides the tools to onboard that corpus safely: in bounded
batches, with real per-record validation and deduplication, resumable
after a crash, rolled back cleanly if something goes wrong, and fully
audited.

## What this package composes from, versus what is new

| Existing piece | What it already provides | What this phase adds |
|---|---|---|
| `packages/ingestion` (Phase 029) | A checkpointed state-machine pipeline with bounded retries and resumability, built for one uploaded document moving through OCR/STT/segmentation/classification | `Engine.RunBatch`'s Cursor-based resumability follows the same *shape* (checkpoint after every unit of work, resume from the last checkpoint), but models its own `ImportJob`/`ImportRecord` state machine for a different unit of work -- a batch of N corpus rows, not one document's pipeline stage. No import of `packages/ingestion`. |
| `packages/auditlog` (Phase 077) | Hash-chained, tenant-scoped, queryable `Event` store | `AuditSink` projects every job registration, batch run, and rollback into that same store; no parallel log |
| `packages/identity` (Phase 006) | `Role`, `Permission`, `PermissionMatrix`, `HasPermission` | `PermViewBulkImport`/`PermManageBulkImport`: the two fine-grained permissions this package's `Engine` gates every operation on |
| `packages/caselifecycle` (Phase 063) | `Case` -- the canonical case record | `ImportRecord.CreatedCaseID` references a case a deployment's downstream integration created from a successfully imported record, by `uuid.UUID` only -- reference, never an import |
| `packages/tenancy` / `packages/persistence` (Phase 004/005) | `WithTenantScope`, Row-Level Security, the three-layer repository pattern | `TenantScopedJobRepository`/`TenantScopedRecordRepository` follow the identical pattern every phase since Phase 005 has used |

## Why this is not built on `packages/ingestion`

Phase 029's `ingestion.Job`/`ingestion.WorkflowState` model a single
uploaded binary moving through a fixed pipeline of stages (intake ->
extraction -> normalize -> segment -> classify), each stage calling out
to a sibling service (`packages/stt`, `packages/ocr`,
`packages/multilingual`, `packages/segmentation`, `packages/evidence`).
A bulk-corpus import job has a different shape entirely:

- The unit of work is a *batch of already-digitized records* read from
  a `RecordSource`, not a single binary going through OCR/STT.
- Resumability granularity is per-batch (`ImportJob.Cursor`, an index
  into the source corpus), not per-pipeline-stage.
- There is no OCR/STT/segmentation/classification step at all -- a
  historical corpus row already carries structured fields
  (case number, jurisdiction, party names), it just needs
  validating, deduplicating, and importing.
- Failure handling is dedup/reject-at-the-record-level plus
  rollback-the-whole-job, not a dead-letter queue per pipeline stage.

Reusing `ingestion.Job`/`WorkflowState`/`RunWithRetry` directly would
force this package to either leave most of those types' fields unused
or bend its own state machine to fit a five-stage pipeline it does not
have. Instead, `packages/bulkimport` borrows the *idea* --
checkpoint after every completed unit of work, resume from the
checkpoint, bound retries -- and implements it against its own
`ImportJob`/`ImportRecord` types.

## Import job state machine (task 1)

```
Pending --> Running --> Paused --> Running
              |            |
              v            v
         Completed     Failed
              |            |
              v            v
         RolledBack   RolledBack
              ^
              |
           Failed --> Running (retry)
```

Every transition is validated by `Status.CanTransitionTo`, backed by
the single `statusTransitions` adjacency map in types.go -- mirroring
`packages/caselifecycle.State`'s transition-table approach rather than
scattering ad hoc `if` statements across call sites. `Engine.RunBatch`,
`Pause`, `ResumeJob`, `FailJob`, and `Rollback` are the only operations
that move a job between states; nothing else mutates `ImportJob.Status`
directly.

`ImportRecord` (also task 1) is one row of the source corpus: a
`PayloadRef` (a reference to the raw source payload -- a row number, an
object-storage key -- never dereferenced by this package, mirroring
`packages/privacy.DataInventoryEntry.SourceTag`'s convention), the
structural fields this package validates and dedups on
(`CaseNumber`/`Jurisdiction`/`PartyNames`), a computed `DedupKey`, a
`ValidationStatus`/`ValidationErrors` pair, and a final `Outcome`
(`Imported` / `SkippedDuplicate` / `Rejected` + reason / `RolledBack`).

## Batched ingestion pipeline (task 2)

`RecordSource` (source.go) is the pluggable read side of a job: a
source corpus being onboarded. `Engine.RunBatch(ctx, tenantID, jobID,
source, batchSize)` reads up to `batchSize` records starting at
`ImportJob.Cursor`, processes each one (validate -> dedup -> import),
and persists the updated `Cursor`/counters/`Status` once per call.

A `RecordSource` implementation must be **deterministic by index**:
`ReadAt(index, count)` must return the same records for the same
`index` across calls, including after a process restart. This is what
makes `Cursor`-based resumability correct -- `RunBatch` has no other
way to know it is re-reading exactly the records it already processed
versus skipping or duplicating them. `InMemoryRecordSource` (the
test/small-corpus implementation shipped here) satisfies this trivially
because it wraps a fixed slice; a real deployment's `RecordSource`
implementation against a CSV file, an object-storage prefix, or a
paginated export API is responsible for the same guarantee (e.g. by
reading rows in a stable sort order, or by using a stable pagination
cursor from the source system rather than an arbitrary API-side
ordering that can change between calls).

## Validation (task 3)

`Validator` (validator.go) is a one-method interface:
`Validate(rec ImportRecord) []ValidationError`. Each `ValidationError`
names the failing `Field` and a human-readable `Reason` -- not a bare
bool, because a corpus owner reconciling a failed import needs to know
*which* rows failed and *why*, not just a pass/fail count.
`DefaultValidator` checks required-field presence
(`case_number`/`jurisdiction`/`payload_ref`) and blank-entry structural
well-formedness of `party_names`; `Engine.SetValidator` lets a
deployment with a richer source schema (additional required fields,
format checks on a court-specific case-number pattern, etc.) supply its
own implementation without this package needing a new phase.

A record that fails validation is recorded as `OutcomeRejected` with
its `ValidationErrors` attached and counted toward
`ImportJob.FailedRecords` -- this does **not** fail the batch or the
job. A single malformed row in a 50,000-row corpus should not block the
other 49,999 legitimate ones; this mirrors
`packages/caselifecycle.BulkTransition`'s partial-failure-safe
contract.

## Resumable imports (task 4)

`ImportJob.Cursor` is persisted via `JobRepository.Update` after every
`RunBatch` call, whether that batch's records all imported cleanly or
included rejections/duplicates. If the process crashes between
`RunBatch` calls, the job's `Cursor` on disk reflects the last
successfully processed record. A fresh `Engine` instance (backed by the
same persisted `JobRepository`/`RecordRepository`, e.g. after a
process restart) calling `RunBatch` again with the job's ID reads the
job, sees its current `Cursor`, and resumes `ReadAt` from exactly that
index -- no re-processing of already-imported records, no skipped
records.

`engine_test.go`'s `TestEngine_RunBatch_ResumesFromMidJobCursor` proves
this explicitly: it processes 6 of 10 records, constructs a **new**
`*Engine` value (simulating a fresh process with no shared in-memory
state, only the same repositories), and verifies the resumed
`RunBatch` call reads exactly the remaining 4 records and produces
exactly 10 total `ImportRecord`s, not 14 or 16.

`Pause`/`ResumeJob` provide an explicit, operator-triggered version of
the same guarantee: `Pause` moves a `Running` job to `Paused` without
touching `Cursor`, and `ResumeJob` moves it back to `Running` so the
next `RunBatch` call continues exactly where it left off.

## Deduplication (task 5)

`ComputeDedupKey(caseNumber, jurisdiction, partyNames)` (dedup.go)
derives a SHA-256 digest of the case number, jurisdiction, and
sorted, normalized (lower-cased, trimmed) party names. Normalizing
before hashing means two records referring to the same case never fail
to dedup just because of a formatting difference in the source data
(different capitalization, extra whitespace, party names listed in a
different order).

`Engine.RunBatch` looks a valid record's `DedupKey` up against already
-imported records in the *same job and tenant* via
`RecordRepository.FindByDedupKey` -- a real lookup, not a heuristic or
a client-side cache that could go stale. A match is recorded as
`OutcomeSkippedDuplicate` (counted toward `ImportJob.SkippedRecords`)
rather than re-imported.

## Progress tracking (task 6)

`Progress` (progress.go) is a processed/total/failed/skipped snapshot
with a computed `PercentComplete` (0 when `Total` is unknown, rather
than misleadingly reporting 0% or 100%) and `StartedAt`/
`LastUpdatedAt` timestamps. `Progress.EstimatedTimeRemaining()`
linearly extrapolates an ETA from the observed processing rate between
those two timestamps, returning `ok=false` when there isn't enough data
yet (job not started, nothing processed). `Engine.GetProgress` is the
getter, requiring `viewPermission`.

## Rollback on failure (task 7)

`Engine.Rollback(ctx, tenantID, jobID)` is a real state transition, not
a no-op: every `ImportRecord` with `Outcome == OutcomeImported` moves
to `OutcomeRolledBack` (clearing any `CreatedCaseID` reference), and
the `ImportJob` itself moves to `StatusRolledBack`, decrementing
`ImportedRecords` by the number actually reversed. Only reachable from
`StatusCompleted` or `StatusFailed` (`ErrJobNotRollbackable`
otherwise) -- a still-running job should be paused or allowed to finish,
not rolled back mid-flight.

Because `ImportRecord.CreatedCaseID` is a reference-only field (this
package does not import `packages/caselifecycle` to create or delete a
`Case`), `Rollback` clears the reference but does not itself reach into
another package's storage to delete/flag a case. A deployment that
wires bulk import to real downstream case creation is responsible for
reacting to a record's `OutcomeRolledBack` transition (e.g. via the
audit trail this phase writes) and retracting or flagging the case it
created -- exactly the same reference-by-convention discipline
`packages/compliance.Control.MappedTo` and
`packages/privacy.DataInventoryEntry.SourceTag` already established for
this codebase.

## Import audit (task 8)

`AuditSink` (audit.go) records every `RegisterJob` call, `RunBatch`
call, and `Rollback` call into `packages/auditlog.Store` -- the single
durable, hash-chained audit sink every phase since 077 composes with.
There is no second audit table anywhere in this package.

## Storage

`ImportJob` and `ImportRecord` rows live in the `bulkimport_jobs` and
`bulkimport_records` tables. Both carry a `tenant_id` column and a
`tenant_isolation` Row-Level Security policy, exactly like every
tenant-scoped table added since Phase 005. `bulkimport_records` also
carries a unique `(job_id, source_index)` index (so re-inserting the
same source position for a job is rejected at the database layer, a
second line of defense alongside the application-level Cursor
checkpoint) and a `(tenant_id, job_id, dedup_key)` index supporting
`FindByDedupKey`'s lookup.

This phase's migration files were authored against the tip of `main`
at the time; since several phases land in parallel, the exact migration
numbers may be renumbered by a coordinator process after this phase's
PR opens, to avoid colliding with a sibling phase's migration. The
table names and schema are unaffected by any such renumbering.

## Reused, not duplicated

- `packages/auditlog.Store` remains the only durable event sink this
  package writes to.
- `identity.Role`/`identity.Permission`/`identity.HasPermission` remain
  the coarse RBAC gate every `Engine` method authorizes through.
- `packages/caselifecycle.Case` remains the only case record type in
  this codebase; `ImportRecord.CreatedCaseID` references it by ID only.
- `packages/ingestion`'s checkpointed-state-machine *shape* is the
  reference `Engine.RunBatch`'s resumability follows, not a dependency
  -- this package does not import `packages/ingestion`.
- `packages/tenancy.WithTenantScope` and the three-layer repository
  pattern (in-memory / Postgres / tenant-scoped) remain the only
  persistence pattern this package uses, identical to every phase since
  004.
