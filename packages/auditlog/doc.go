// Package auditlog provides Verdex's centralized, immutable, queryable
// audit trail: the durable persisted store and query/export API that
// packages/observability's AuditEvent (Phase 003) explicitly deferred
// to a later phase.
//
// # Why this package exists
//
// packages/observability/audit.go's AuditEvent doc comment says it
// directly: "Phase 077 owns the full audit trail (richer event
// taxonomy, retention/storage guarantees, tamper evidence, query
// interfaces, etc.)". AuditLogger (observability package) only writes
// JSON lines to an io.Writer — an in-process log sink, not a durable,
// queryable store. This package is that missing piece.
//
// # Reused, not reinvented: the event schema
//
// Event (types.go) embeds observability.AuditEvent directly rather
// than duplicating its Time/Actor/Action/Target/Outcome fields. It
// adds only what observability.AuditEvent's doc comment named as
// out-of-scope for Phase 003: TenantID (tenant scoping), Kind (a
// closed taxonomy for filtering — data_access, reasoning, signoff,
// data_change, admin, export, system), CaseID (a stable case-scoped
// filter distinct from the free-form Target), Detail (a short
// human-readable elaboration), and the hash-chain fields PrevHash/
// ChainHash. Any package that already produces an observability.AuditEvent
// (e.g. packages/caselifecycle.TransitionRecord.ToAuditEvent) can
// project straight into an Event by embedding that value and setting
// the additional fields.
//
// # Tamper evidence: a per-tenant hash chain
//
// chain.go mirrors packages/provenance's ChainBuilder/BuildChain/
// VerifyChain idiom exactly: each Event's ChainHash is
// SHA-256(PrevHash + ID + a content digest of its own fields), so
// altering any field of any stored event — or reordering/deleting one
// without going through the sanctioned Purge path — changes that
// event's recomputed ChainHash and breaks the link the next event
// depends on. VerifyChain detects this and reports the first broken
// index; VerifyGenesisChain additionally requires the chain's first
// event to have PrevHash == "" (true chain start), for full
// end-to-end verification of a never-purged or archived history.
//
// Store.Append (store.go) is the only place ChainHash is computed: it
// reads the tenant's current chain tail (Repository.Last) and links
// the new event to it before persisting. There is no Update or Delete
// method on Store's public API — Purge (retention.go) is the sole,
// retention-window-bounded exception, implemented as a delete of a
// contiguous prefix that never mutates or removes any surviving event.
//
// # Storage: Postgres, append-only by convention and by grant
//
// PostgresRepository (postgres_repository.go) persists Event rows in
// the `audit_events` table (see
// packages/persistence/migrations/000020_create_auditlog.up.sql). The
// migration revokes UPDATE and DELETE on that table from the
// application role entirely (deny-by-grant, not just "we promise not
// to call it"), and grants a narrow DELETE-with-a-WHERE-clause-only
// path is not possible in plain SQL grants, so PurgeBefore instead
// runs as the same application role using a parameterized DELETE ...
// WHERE occurred_at < $cutoff statement — the grant permits DELETE at
// all only because retention purging is a legitimate, audited
// operation this package itself performs and records; ad hoc
// UPDATE is revoked unconditionally. See the migration file for the
// exact GRANT/REVOKE statements and rationale.
//
// TenantScopedRepository (tenant_scoped_repository.go) wraps
// PostgresRepository with packages/tenancy.WithTenantScope, exactly as
// packages/keymanagement.TenantScopedRepository does, so Row-Level
// Security enforces tenant isolation at the database layer in addition
// to this package's application-level checks.
//
// # Query, retention, and export
//
// Store.Query (query.go) filters by actor, case, kind, action, and
// time range (task 5). Store.Purge (retention.go) deletes events older
// than a configurable RetentionPolicy.Window, itself producing a new
// KindSystem audit event recording the purge. Store.Export (export.go)
// renders Query's result as CSV or JSON for a regulator/compliance
// handoff (task 7).
//
// # Access control
//
// Query, Export, Purge, and VerifyTenantChain all require the
// authenticated actor (via identity.UserFromContext) to hold
// identity.PermAuditRead — already granted to RoleJudge, RoleAdmin,
// and RoleAuditor by packages/identity's PermissionMatrix — and to
// belong to the tenant being queried. See access.go.
//
// # What this package wires into, and what it does not
//
// This phase's real net-new contribution is the durable store +
// query/export API + retention policy, not a wholesale migration of
// every package's existing audit mechanism. As a concrete, tested
// proof this is not a parallel unused system, adapter.go projects
// packages/signoff.AuditEntry values (sign-off approvals, rejections,
// and re-review triggers — task 3) into Event and appends them to a
// Store. accessadapter.go does the same for case-view/read access
// events (task 2), using packages/caselifecycle's existing
// TransitionRecord.ToAuditEvent projection as its starting point where
// applicable and a direct Event construction for pure-read (no state
// transition) access.
//
// packages/observability (audit.go), packages/guardrail (audit.go),
// packages/category (audit.go), packages/caseversioning (Snapshot
// attribution), packages/annotations (AuditRecord), and
// packages/keymanagement (key_audit_entries) all retain their own
// existing, independent audit-shaped mechanisms unchanged by this
// phase — see doc/audit-trail.md for the full composition map and an
// explicit list of what remains independent versus what has a wired
// adapter today.
package auditlog
