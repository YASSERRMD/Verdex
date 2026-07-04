# Statute/precedent corpus updater (Phase 089)

This phase draws together five earlier threads -- the structured
statute corpus added in Phase 035 (`packages/statute`), the structured
precedent corpus added in Phase 036 (`packages/precedent`), the
embedding computation/versioning machinery added in Phase 015
(`packages/embedding`), the notification pipeline shape established in
Phase 072 (`packages/notifications`), and the durable, hash-chained
audit trail added in Phase 077 (`packages/auditlog`) -- into a single,
tenant- and jurisdiction-scoped workflow for keeping those two corpora
current as amendments and new rulings are published:
`packages/corpusupdater`.

## Goal

Keep legal corpora current per jurisdiction: ingest an amendment or new
precedent, stage it, validate it, hold it until its effective date
actually arrives, apply it (writing the new text, re-embedding it, and
notifying the cases that reference it), and be able to roll the whole
batch back if something was wrong -- all durably audited.

## What this package composes from, versus what is new

| Existing piece | What it already provides | What this phase adds |
|---|---|---|
| `packages/statute` (Phase 035) | The `RuleNode`/`Citation`/hierarchy model for statute text, plus its own in-corpus `Amendment`/`AmendmentRecord` history tracking | This package's `Amendment` is a distinct, higher-level *staged change awaiting a workflow* (validate/apply/rollback/audit/notify), referencing a `packages/statute` rule by its `RuleNode.ID` string -- it does not import `packages/statute` or replace its own amendment-history model |
| `packages/precedent` (Phase 036) | The holding/precedent node model | Referenced the same way, by node ID string, for `CorpusPrecedent`-targeted jobs |
| `packages/embedding` (Phase 015) | `EmbeddingService.Embed`/`EmbedChunked`, the real embedding computation, caching, and model-version tracking | `Embedder`: a small interface shaped like `EmbeddingService.Embed`. `Engine.ApplyAmendment` calls it exactly once per changed rule/precedent; this package never computes a vector itself |
| `packages/notifications` (Phase 072) | The persisted, user-facing notification inbox and its adapter-per-upstream-interface pattern (`signoff.NotificationSink`, `annotations.MentionSink`, ...) | `NotificationSink`: this package's own outbound event shape (`ChangeNotification`), composing conceptually by shape, not by import -- a notifications-side adapter is the real sink, exactly like the four adapters `packages/notifications` already wires up |
| `packages/auditlog` (Phase 077) | Hash-chained, tenant-scoped, queryable `Event` store; `Kind` taxonomy | `AuditSink` projects every job creation/transition, amendment stage/apply, and rollback into that same store; no parallel log |
| `packages/identity` (Phase 006) | `Role`, `Permission`, `PermissionMatrix`, `HasPermission` | `PermViewCorpusUpdater`/`PermManageCorpusUpdater`: the two fine-grained permissions this package's `Engine` gates every operation on |

## Corpus update workflow (task 1)

`CorpusUpdateJob` (types.go) is a jurisdiction-scoped unit of work
targeting exactly one corpus (`CorpusStatute` or `CorpusPrecedent`),
moving through a closed `JobStatus` state machine enforced by
`IsValidTransition`:

```
StatusPending -> StatusValidating -> StatusApplying -> StatusApplied
                       |                     |               |
                       v                     v               v
                 StatusFailed          StatusFailed   StatusRolledBack
```

A job owns zero or more staged `Amendment` records
(`AmendmentRepository.ListForJob`). `Engine.CreateJob` creates a job in
`StatusPending`; `Engine.StageAmendment` only accepts new amendments
while a job is `StatusPending` or `StatusValidating` -- once it moves
to `StatusApplying` the batch is locked.

## Amendment ingestion (task 2)

`Amendment` (types.go) is a single staged change: a `TargetCorpus` +
`TargetID` (the `packages/statute`/`packages/precedent` node ID this
amendment changes, referenced by string -- this package does not
import either corpus package), a `ChangeType`
(`ChangeTypeAdd`/`ChangeTypeAmend`/`ChangeTypeRepeal`), `NewText`, a
`Citation` for the amending instrument itself, and an `EffectiveDate`.
`ChangeTypeAdd` may leave `TargetID` blank (a brand-new entry has no
existing ID yet); `ChangeTypeAmend`/`ChangeTypeRepeal` require one.

## Effective-date handling (task 3)

`Amendment.IsEffective(now)` is a real check, not a status field: an
amendment is "live" only once `EffectiveDate <= now`. Two query paths
respect this:

- `Engine.EffectiveAmendments(ctx, tenantID, jobID, now)` filters a
  job's staged amendments down to just the currently effective ones.
- `Engine.ApplyJob` only calls `ApplyAmendment` for amendments that are
  both effective and not yet applied -- an amendment staged with a
  future `EffectiveDate` stays untouched until a later `ApplyJob` call
  (after that date has passed) picks it up.

Tested with both past dates (effective) and future dates (not
effective yet) in types_test.go and engine_test.go.

## Re-embedding on updates (task 4)

`Embedder` (embedder.go) is a small interface shaped like
`packages/embedding.EmbeddingService`'s own `Embed(ctx, texts
[]string) ([]EmbeddedText, error)` method -- just enough surface for
`Engine.ApplyAmendment` to trigger a recomputation after a text change
lands, without importing `packages/embedding` or reimplementing
embedding. A production caller wires a small adapter around a real
`embedding.EmbeddingService`; `engine_test.go`'s `fakeEmbedder` proves
`ApplyAmendment` calls it exactly once per changed rule/precedent
(and not at all for `ChangeTypeRepeal`, which leaves no surviving text
to embed).

## Change notification to affected cases (task 5)

`NotificationSink` (notification.go) receives a `ChangeNotification`
whenever an applied amendment goes live, listing the case IDs an
`AffectedCaseResolver` callback names as referencing the changed
target. This package does not query the knowledge graph itself --
callers supply the resolver (backed by `packages/graph` or
`packages/knowledgeapi` in production) -- keeping this package's
dependency footprint thin, the same way `TargetResolver` (validation.go)
keeps target-existence checking out of this package's import graph.
Composing conceptually with `packages/notifications` by shape, exactly
as `packages/signoff.NotificationSink` and
`packages/annotations.MentionSink` already establish that seam: this
package fires the event, a notifications-side adapter is the real
sink. Delivery failure is non-fatal to the amendment apply itself.

## Update validation (task 6)

`Validate(amendment, resolve, now)` (validation.go) performs the
structural checks `Engine.ValidateJob` runs over every staged
amendment before a job may progress to `StatusApplying`: a recognized
`ChangeType`, a non-blank `Citation`, a `TargetID` present when
`ChangeType` requires one, a `TargetID` that resolves via the
caller-supplied `TargetResolver` (skipped when `resolve` is `nil`), and
an `EffectiveDate` within a decade each way of `now` (generous enough
for a real long-lead legislative date, tight enough to catch an
obviously wrong value). The first amendment to fail moves the whole
job to `StatusFailed` with `FailureReason` set to that error.

## Rollback support (task 7)

`Engine.Rollback(ctx, tenantID, jobID, store)` (engine.go) reverts
every `Applied`, not-yet-`RolledBack` amendment in a job back to its
`PreviousText`/`PreviousCitation` snapshot -- captured by
`ApplyAmendment` at apply time, before the new text overwrote it -- and
transitions the job `StatusApplied -> StatusRolledBack`. This is real
revert logic operating through the same `CorpusTextStore` seam
`ApplyAmendment` uses to write the original change, not a job-status
flip alone: `engine_test.go` proves the underlying store's text is
actually restored.

## Update audit (task 8)

`AuditSink` (audit.go) composes with `packages/auditlog.Store` --
exactly the pattern `packages/compliance`'s and `packages/privacy`'s
own `AuditSink` established -- recording every job creation, job status
transition, amendment stage, amendment apply, and job rollback. No
second audit table.

## Access control

Two new `identity.Permission` constants gate every `Engine` operation,
added following `permission.go`'s exact
`PermViewIntegration`/`PermManageIntegration` precedent from Phase 087:

- `corpusupdater:view` (`identity.PermViewCorpusUpdater`): read-only
  access to `CorpusUpdateJob` records, staged/effective `Amendment`s,
  and this package's audit trail.
- `corpusupdater:manage` (`identity.PermManageCorpusUpdater`): stage
  amendments, validate/apply a job, and roll one back.

`RoleAdmin` holds both; `RoleAuditor` holds only the view permission,
consistent with its read-only, compliance-facing posture elsewhere in
the matrix (see `packages/identity/doc/rbac-matrix.md`).

## Storage

Two new migration pairs, continuing directly after
`000035_enable_rls_integration`:

- `packages/persistence/migrations/000036_create_corpusupdater.up.sql`
  / `.down.sql` create two tenant-scoped tables:
  `corpusupdater_jobs` and `corpusupdater_amendments` (with a foreign
  key to its owning job and `previous_text`/`previous_citation`
  snapshot columns for rollback). `CHECK` constraints mirror the
  Go-side `JobStatus`/`ChangeType`/`CorpusTarget` enums and
  `Amendment.Validate`'s `requiresTarget` guard at the database layer.
- `packages/persistence/migrations/000037_enable_rls_corpusupdater.up.sql`
  / `.down.sql` enable and force row-level security with the standard
  `tenant_isolation` policy on both tables.

Each table follows the same `Repository` / `PostgresXRepository` /
`TenantScopedXRepository` three-layer pattern established by
`packages/privacy` and `packages/compliance`, with Row-Level Security
enforcing tenant isolation at the database layer in addition to each
repository's own application-level `requireMatchingTenant` guard.

## What is explicitly reused, not duplicated

- `packages/statute` and `packages/precedent` remain the only corpus
  node models in this codebase. `Amendment.TargetID` references a rule/
  precedent by string ID, exactly as
  `packages/compliance.Control.MappedTo` references platform features
  by string tag rather than importing them.
- `packages/embedding.EmbeddingService` remains the only embedding
  computation in this codebase; `Embedder` is a thin, locally-declared
  interface a real adapter wraps around it.
- `packages/notifications` remains the only persisted, user-facing
  notification inbox; `NotificationSink` is this package's outbound
  event shape, not a dependency on it.
- `packages/auditlog.Store` is the only durable event sink this
  package writes to, via `AuditSink`.
- `identity.Role`/`identity.Permission`/`identity.HasPermission` remain
  the coarse RBAC gate every `Engine` method calls through before doing
  anything corpus-update-specific.
