# Comprehensive audit trail

`packages/auditlog` implements Phase 077's centralized, immutable,
queryable audit trail: the durable persisted store and query/export API
that `packages/observability`'s `AuditEvent` (Phase 003) explicitly
deferred to a later phase.

## Why this package exists now

`packages/observability/audit.go`'s doc comment on `AuditEvent` says it
directly:

> Phase 077 owns the full audit trail (richer event taxonomy,
> retention/storage guarantees, tamper evidence, query interfaces,
> etc.); this type only establishes that audit events flow through a
> channel separate from application logs...

`AuditLogger` in that package only writes one JSON line per event to an
`io.Writer` — a process-local log sink, not a durable, queryable store.
Nothing before this phase persisted an audit event anywhere a compliance
officer or regulator could query it after the process exits. This
package supplies that missing piece.

## The pieces

```
Event                -- the canonical audit record (embeds
                         observability.AuditEvent, adds TenantID, Kind,
                         CaseID, Detail, PrevHash, ChainHash)

Repository            -- persists Event rows (task 4)
  InMemoryRepository   -- tests and fixtures
  PostgresRepository    -- `audit_events` table, UPDATE revoked,
                            trigger-enforced append-only + retention
                            floor
  TenantScopedRepository -- RLS-enforced tenant isolation

Store                 -- write path (Append, builds the hash chain) and
                          read path (Query, Export, Purge,
                          VerifyTenantChain), all gated on
                          identity.PermAuditRead except Append

ChainBuilder           -- SHA-256 hash chain over PrevHash + ID +
                           content digest (task 4), mirroring
                           packages/provenance's ChainBuilder idiom

SignoffAuditSink       -- projects packages/signoff.AuditEntry into
                           Event (task 3)
DataAccessSink         -- records case/document read access as Event
                           (task 2)
```

## Reused, not reinvented: the event schema

`Event` (types.go) embeds `observability.AuditEvent` directly rather
than duplicating its `Time`/`Actor`/`Action`/`Target`/`Outcome` fields.
It adds exactly what `AuditEvent`'s own doc comment named as
out-of-scope for Phase 003:

- `TenantID` — tenant scoping, required on every event.
- `Kind` — a small, closed taxonomy (`data_access`, `reasoning`,
  `signoff`, `data_change`, `admin`, `export`, `system`) so `Query` can
  filter meaningfully, independent of the free-form `Action` string.
- `CaseID` — a stable case-scoped filter distinct from `Target` (which
  may hold a document ID, a key ID, a tenant ID, etc.).
- `Detail` — a short, free-form elaboration (reviewer notes, a
  break-glass justification) — never raw document content.
- `PrevHash` / `ChainHash` — the tamper-evidence fields (see below).

Any package that already produces an `observability.AuditEvent` (e.g.
`packages/caselifecycle.TransitionRecord.ToAuditEvent`) can embed that
value directly into an `Event` and set the additional fields — no
schema translation layer is needed.

## Tamper evidence: a per-tenant hash chain

`chain.go` mirrors `packages/provenance`'s `ChainBuilder` /
`BuildChain` / `VerifyChain` idiom exactly, applied to audit events
instead of provenance (chain-of-custody) records:

```go
ChainHash = SHA-256(PrevHash + ID + contentDigest(Event))
```

`Store.Append` is the only place `ChainHash` is computed: it reads the
tenant's current chain tail (`Repository.Last`) and links the new event
to it before persisting. `Store`'s public API has no `Update` and no
unconditional `Delete` — `Purge` (see Retention below) is the sole,
retention-window-bounded exception.

`VerifyChain` recomputes every event's expected hash and reports the
first index where a stored `PrevHash` or `ChainHash` no longer matches
— catching field tampering, chain-hash tampering, and silent row
deletion alike. It anchors on `events[0].PrevHash` rather than assuming
the empty string, so a legitimately `Purge`-truncated tail still
verifies as an internally consistent segment.

`VerifyGenesisChain` is the stricter form: it additionally requires the
first event's `PrevHash` to be exactly `""` (a true, never-purged chain
start), for full end-to-end verification of a tenant's complete history
or of a pre-purge archival export.

## Storage: Postgres, append-only by grant and by trigger

`PostgresRepository` persists `Event` rows in the `audit_events` table
(`packages/persistence/migrations/000020_create_auditlog.up.sql`,
RLS in `000021_enable_rls_auditlog.up.sql`). The migration:

- **Revokes `UPDATE`** on `audit_events` from the `verdex_app` role
  unconditionally — no code path in this package ever needs to modify
  a persisted row, so there is no legitimate `UPDATE` to carve an
  exception for. A `BEFORE UPDATE` trigger additionally rejects every
  update attempt outright, as defense-in-depth against a future
  migration accidentally re-granting the privilege.
- **Keeps `DELETE` granted** (it is part of `verdex_app`'s default
  privileges from `000005_create_app_role.up.sql`), because `Store.Purge`
  legitimately deletes events past their retention window. A
  `BEFORE DELETE` trigger enforces a floor in the database itself: no
  row less than one hour old can ever be deleted, regardless of what
  application code requests — making an accidental "delete everything"
  or "delete recent rows" mistake structurally impossible, not merely
  a matter of application discipline.

`TenantScopedRepository` wraps `PostgresRepository` with
`packages/tenancy.WithTenantScope`, exactly as
`packages/keymanagement.TenantScopedRepository` does, so Row-Level
Security enforces tenant isolation at the database layer in addition to
this package's application-level checks.

## Query API

```go
func (s *Store) Query(ctx context.Context, tenantID uuid.UUID, filter Filter) ([]Event, error)
```

`Filter` supports `Actor`, `CaseID`, `Kinds`, `Action`, `Since`/`Until`,
and `Limit` (defaulting to 1000 when unset, so a forgotten limit cannot
return an unbounded result set). Results are always ordered by `Time`
ascending (chain order), so a caller can run `VerifyChain` directly on
a `Query` result.

## Retention policy

```go
type RetentionPolicy struct {
    Window time.Duration
}

func (s *Store) Purge(ctx context.Context, tenantID uuid.UUID, policy RetentionPolicy) (int, error)
```

`Purge` deletes every event older than `policy.Window` (measured from
now) for the tenant, and nothing else — it can never remove an event
still inside the retention window, and it never mutates a surviving
event. Because chain links are computed forward from each event's
immediate predecessor, removing a contiguous prefix does not change any
surviving event's stored `PrevHash`/`ChainHash` — `VerifyChain` over
the post-purge history still succeeds (see
`TestStore_Purge_PreservesChainIntegrityOfSurvivors`). `Purge` itself
appends a new `KindSystem` `"audit.purged"` event recording what it
did, so the purge action is itself part of the durable trail.

## Export for regulators

```go
func (s *Store) Export(ctx context.Context, tenantID uuid.UUID, filter Filter, format ExportFormat) ([]byte, error)
```

`Export` runs `Query` (inheriting its access control) and renders the
result as `ExportFormatCSV` or `ExportFormatJSON` for a
compliance/regulator handoff. Both formats preserve chain order, so a
recipient can independently run `VerifyChain` (or `VerifyGenesisChain`,
for a complete-history export) over the parsed result to confirm
nothing was altered between generation and receipt.

## Access control

`Query`, `Export`, `Purge`, and `VerifyTenantChain` all require the
authenticated actor (via `identity.UserFromContext`) to hold
`identity.PermAuditRead` — already granted to `RoleJudge`, `RoleAdmin`,
and `RoleAuditor` by `packages/identity`'s `PermissionMatrix` — and to
belong to the tenant being queried (`ErrCrossTenantAccess` otherwise,
even for an auditor). `Store.Append` itself carries no read-permission
gate: writing an audit event must never be gated behind read access,
since the caller recording the event has already authorized the
underlying action through that action's own permission check (e.g.
`identity.PermSignOff` for a sign-off decision).

## What this composes with today

This phase's real net-new contribution is the durable store + query/
export API + retention policy — not a wholesale migration of every
package's existing audit mechanism. Two concrete, tested adapters prove
the store is a real sink other packages can write into, not a parallel
unused system:

- **`SignoffAuditSink`** (`adapter.go`) projects a real
  `packages/signoff.AuditEntry` — produced by the real
  `signoff.Service.Approve`/`Reject` — into an `Event` and appends it.
  `TestSignoffAdapter_EndToEnd` drives a real `signoff.Service.Approve`
  call, reads the resulting `AuditEntry` back out of
  `signoff.Repository.ListAudit`, projects it, and queries it back out
  through `Store.Query` end-to-end.
- **`DataAccessSink`** (`accessadapter.go`) records case/document read
  access. `TestDataAccessSink_RecordCaseView_EndToEnd` exercises a real
  `packages/caselifecycle.RequireViewPermission` + `Repository.Get`
  read path, then records and queries the resulting `KindDataAccess`
  event.

## What remains independent (honest scope)

The following packages' own audit-shaped mechanisms are **not**
migrated or replaced by this phase — each continues to own its
existing storage and API exactly as before:

- **`packages/observability`** (`audit.go`) — the in-process
  `AuditLogger` JSON-line sink remains as-is; it is a useful
  low-overhead operational log independent of this durable store, and
  `Event` embeds its `AuditEvent` type rather than replacing it.
- **`packages/guardrail`** (`audit.go`) — guardrail enforcement audit
  stays local to that package.
- **`packages/category`** (`audit.go`) — category audit stays local.
- **`packages/caselifecycle`** (`TransitionRecord`) — case-transition
  history remains `caselifecycle`'s own durable record (its
  `ToAuditEvent` projection into `observability.AuditEvent` predates
  this phase and is unchanged); this phase's `DataAccessSink` adds a
  *separate*, additional read-access trail rather than replacing
  `TransitionRecord`.
- **`packages/annotations`** (`AuditRecord`) — unchanged.
- **`packages/caseversioning`** (`Snapshot` attribution) — unchanged.
- **`packages/reportexport`** (export audit) — unchanged; a future
  wiring could feed `reportexport`'s own export events into this
  store's `KindExport` category the same way `SignoffAuditSink` does
  for sign-off, but that wiring is not implemented in this phase.
- **`packages/keymanagement`** (`key_audit_entries`) — unchanged; its
  own Postgres-backed, tenant-scoped audit table already independently
  satisfies the "durable, queryable, tenant-isolated" bar this phase
  sets as a general pattern, and migrating it into `auditlog.Event`
  is left as a follow-up, not claimed as done here.

A future phase (or a follow-up to this one) that wants full
consolidation should add adapters for the packages above the same way
`SignoffAuditSink` and `DataAccessSink` do here — project the
package-local record into `auditlog.Event` and call `Store.Append` —
without needing to change this package's schema or storage layer.
