# Case lifecycle

`packages/caselifecycle` defines the canonical `Case` entity and its lifecycle
state machine — the first time a case is modeled as a first-class, persisted
entity anywhere in Verdex. Every other package built through Phase 062
(`packages/ingestion`, `packages/category`, `packages/timeline`,
`packages/grounding`, `packages/reasoningeval`, and others) already threads a
bare `CaseID string` through its own API. This package is what that string
has always implicitly referred to.

## The entity

```go
type Case struct {
    ID              uuid.UUID
    TenantID        uuid.UUID
    JurisdictionID  uuid.UUID
    CategoryID      string // a packages/category.CategoryCode value, stored as a string
    Title           string
    Reference       string // optional external/docket reference
    State           State
    Metadata        map[string]string
    MetadataVersion int
    CreatedBy       uuid.UUID
    CreatedAt       time.Time
    UpdatedAt       time.Time
    ArchivedAt      *time.Time // nil until Archive
}
```

`ID`, `TenantID`, `JurisdictionID`, and `Title` are required (`Case.Validate`
enforces this). `CategoryID` may be blank at creation and assigned later
during intake — this package does not own categorization logic, it only
stores the reference. Every `Case` belongs to exactly one tenant; see
"Tenant isolation" below.

## The state machine

```
              Transition (draft->active)
      draft ────────────────────────────► active
                                            │  ▲
                  Transition (active->under_review)
                                            │  │
                                            ▼  │ Transition (under_review->active)
                                     under_review
                                            │
                  Transition (under_review->closed)
                                            │
                                            ▼
                                         closed
                                            │
                        Reopen (closed->active, requires justification)
                                            │
                                            ▼
                                         active
                                            │
                         Archive (closed->archived, terminal)
                                            ▼
                                        archived
```

States:

| State          | Meaning                                                                 |
|----------------|--------------------------------------------------------------------------|
| `draft`        | Intake in progress. Entry state for every new case (see `NewCase`).      |
| `active`       | Normal working state. Every case-scoped action is permitted.             |
| `under_review` | Submitted for judicial review of draft reasoning output. Case-scoped data is frozen except review and metadata. |
| `closed`       | Reached a disposition. Read-only except via `Reopen`.                    |
| `archived`     | Terminal. Distinct from `closed` — an archived case can never be reopened. |

The allowed-transitions table (`allowedTransitions` in `transition.go`) is
the single authoritative source of truth:

```go
var allowedTransitions = map[State][]State{
    StateDraft:       {StateActive},
    StateActive:      {StateUnderReview},
    StateUnderReview: {StateClosed, StateActive},
    StateClosed:      {},
    StateArchived:    {},
}
```

`Transition(ctx, repo, TransitionInput{...})` enforces this table and rejects
anything not listed with `ErrIllegalTransition`. Every successful transition
appends a `TransitionRecord` to the audit log (see "Audit log" below).

`Reopen` and `Archive` are **not** represented in `allowedTransitions`, even
though they also change `State` from/to `closed`/`archived`. They are
distinct, more heavily guarded operations (see next section) — this keeps a
caller from accidentally reopening or archiving a case via a plain
`Transition` call.

## Reopen and archive

- **`Reopen(ctx, repo, tenantID, caseID, justification)`** moves a case from
  `closed` back to `active`. `justification` must be non-blank
  (`ErrReasonRequired` otherwise) — reopening a closed case must always be
  self-documenting in the audit log.
- **`Archive(ctx, repo, tenantID, caseID, reason)`** moves a case from
  `closed` to `archived`, a terminal state distinct from `closed`, and stamps
  `Case.ArchivedAt`. `reason` is optional context, not a hard requirement,
  since archiving an already-closed case is a lower-stakes, often-batch
  administrative action.

Both require an authenticated actor (`identity.UserFromContext(ctx)`) and
both reject if the case is not currently `closed`.

## Per-state permitted actions

`actions.go` maps each `State` to the set of `Action`s downstream packages
may perform against a case in that state:

| Action                    | draft | active | under_review | closed | archived |
|---------------------------|:-----:|:------:|:-------------:|:------:|:--------:|
| `ingest_evidence`         |  yes  |  yes   |               |        |          |
| `edit_category`           |  yes  |  yes   |               |        |          |
| `edit_timeline`           |  yes  |  yes   |               |        |          |
| `generate_reasoning`      |       |  yes   |               |        |          |
| `review_opinion`          |       |  yes   |      yes      |        |          |
| `edit_metadata`           |  yes  |  yes   |      yes      |        |          |

`CanPerform(state, action) bool` and `RequireAction(state, action) error` are
the guard other packages call before mutating case-scoped data, e.g.:

```go
if err := caselifecycle.RequireAction(c.State, caselifecycle.ActionIngestEvidence); err != nil {
    return err // ErrActionNotPermitted
}
```

This package does not and cannot enforce that callers actually make this
check — it only defines the table.

## Case metadata

`Case.Metadata` is a free-form `map[string]string` for fields this package
does not model explicitly (docket numbers, external system references,
jurisdiction-specific flags). Mutate only via:

- `SetMetadata(ctx, repo, input)` — replaces the entire map.
- `MergeMetadata(ctx, repo, input)` — overlays `input.Values` onto the
  existing map (new keys added, existing keys overwritten, other keys
  untouched).

Both validate every key (non-blank) and bump `Case.MetadataVersion` on
success. Pass `MetadataUpdateInput.ExpectedVersion` to get optimistic
concurrency: a mismatched version returns `ErrMetadataVersionConflict`
instead of silently clobbering a concurrent writer's change.

## Transition audit log

Every successful `Transition`, `Reopen`, and `Archive` call appends a
`TransitionRecord`:

```go
type TransitionRecord struct {
    ID         uuid.UUID
    CaseID     uuid.UUID
    TenantID   uuid.UUID
    FromState  State
    ToState    State
    Actor      uuid.UUID // identity.User.ID
    Reason     string
    OccurredAt time.Time
}
```

`Repository.AppendTransition`/`ListTransitions` persist and query these
records (append-only; never updated or deleted). `TransitionRecord.ToAuditEvent()`
projects a record into `packages/observability.AuditEvent`, so transition
history flows through the same audit channel as the rest of the system
rather than a second, parallel logging path.

## Bulk operations

`BulkTransition`, `BulkSetMetadata`, and `BulkMergeMetadata` apply one
operation across many case IDs and return one `BulkResult` per case:

```go
type BulkResult struct {
    CaseID uuid.UUID
    Case   *Case // non-nil on success
    Err    error // non-nil on failure
}
```

A single case failing (not found, cross-tenant, illegal transition, invalid
metadata) does not abort the batch for any other case — this is
partial-failure-safe by default, not all-or-nothing. Callers that need
all-or-nothing semantics should wrap their own call in a database
transaction at the `Repository` implementation level.

## Persistence and tenant isolation

Every `Repository` method takes a `tenantID` and refuses (via
`ErrCrossTenantAccess`, checked before any storage access) to operate on a
`Case` whose `TenantID` does not match.

Two production implementations exist:

- **`PostgresRepository`** — `Executor`-based (works directly against a pool
  or inside a transaction started by `persistence.WithTx`), backed by the
  `cases` and `case_transitions` tables
  (`packages/persistence/migrations/000006_create_cases.up.sql`). Enforces
  tenant isolation at the application level.
- **`TenantScopedRepository`** — composes `PostgresRepository` with
  `packages/tenancy.WithTenantScope`, so Row-Level Security
  (`packages/persistence/migrations/000007_enable_rls_cases.up.sql`) enforces
  the same isolation at the database layer as defense-in-depth. This is what
  production code should use against a live `*pgxpool.Pool`, mirroring
  `packages/tenancy.TenantScopedDeploymentRepository`.

**`InMemoryRepository`** is a third, in-process implementation for tests and
for other packages' test fixtures — it enforces the same
`ErrCrossTenantAccess`/`ErrNotFound` semantics without a live database.

## Relationship to downstream packages

- **`packages/ingestion`** — its own `status.go`/`progress.go` track the
  internal state of a single ingestion job (queued, transcribing,
  extracting, ...): a fine-grained, per-artifact pipeline concern. This
  package's `State` tracks the case's own coarse lifecycle and is not
  derived from ingestion status. A case can sit in `active` while ingestion
  jobs for it come and go; `PermittedActions`/`CanPerform` is what governs
  whether a *new* ingestion job may even start against a case in a given
  state.
- **`packages/category`** — a `Case.CategoryID` links to a
  `packages/category.CategoryCode` value. This package does not re-derive
  categorization/taxonomy logic; it stores the reference only.
- **`packages/timeline`** — parties and events reference a case by ID
  (`CaseID string`, matching `Case.ID.String()`). This package does not
  duplicate party/event modeling.
- **`packages/jurisdiction`** — `Case.JurisdictionID` links to a
  `packages/jurisdiction.Jurisdiction` by its UUID primary key.

## Access control

`access.go` exposes `RequireViewPermission`, `RequireEditPermission`, and
`RequireDeletePermission`, gating on `identity.PermViewCase`,
`identity.PermEditCase`, and `identity.PermDeleteCase` respectively,
mirroring `packages/grounding.RequireCheckPermission`. `Transition`,
`Reopen`, `Archive`, `SetMetadata`, and `MergeMetadata` only require *some*
authenticated user on `ctx` (to attribute the resulting `TransitionRecord`'s
`Actor`) — callers building an API layer on top of this package are expected
to call `RequireEditPermission` explicitly before invoking those functions.
