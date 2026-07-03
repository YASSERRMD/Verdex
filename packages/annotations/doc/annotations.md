# Annotations & collaboration

`packages/annotations` implements Phase 070: multi-user notes,
highlights, and threaded discussion on a case, anchored optionally to
a single reasoning-tree node or evidence segment within it.

## Why this package exists now

Every prior phase's data (the case itself, its `irac` reasoning tree,
its evidence segments, its draft opinion) is read-and-reason content —
none of it lets more than one reviewer leave a note the others can
see, reply to, or resolve. `packages/annotations` is the thin
collaboration layer over that existing data: it does not duplicate the
tree, the evidence, or the case record, it only stores pointers into
them (`AnchorType` + `AnchorID`) alongside the discussion itself.

## The model

```
Annotation
  ID, CaseID, TenantID, AuthorID, Body
  AnchorType: case | tree_node | evidence_segment
  AnchorID:   "" | irac node ID | evidence segment ID
  ParentID:   nil (thread root) | root's ID (reply)
  Resolved, ResolvedBy, ResolvedAt
  CreatedAt, UpdatedAt
```

- **Case-level** (`AnchorType: case`, `AnchorID: ""`) — a general note on
  the case as a whole.
- **Tree-node-anchored** (`AnchorType: tree_node`) — `AnchorID` is a
  `packages/irac` node's string ID (the same ID space
  `irac.Node.ID`/`irac.NodeLike.GetID()` use), e.g. "this issue node's
  conclusion looks unsupported."
- **Evidence-segment-anchored** (`AnchorType: evidence_segment`) —
  `AnchorID` is an evidence segment's string ID, the same ID space
  `apps/web/src/components/workspace/EvidenceReviewPanel.tsx` already
  renders segment cards against.

Threading is exactly one level deep: an `Annotation` with `ParentID ==
nil` is a thread root, one with `ParentID` set is a reply to that root.
`Create` rejects a reply-to-a-reply with `ErrParentIsReply` — keeping
"root + flat list of replies" rather than arbitrary nesting, matching
how `ReasoningOpinionPanel`'s existing per-issue comment box models
discussion (one flat list per issue, not a nested tree of replies).

## The composition

```
Service.Create(ctx, tenantID, *Annotation)
  1. authorizeWrite(ctx)                 -> identity.PermEditCase
  2. checkCaseAccess(ctx, tenantID, a.CaseID)
       -> caselifecycle.Repository.Get   -> ErrForbidden if not visible
  3. repo.Create                          -> validates, persists,
                                              derives+stores Mentions
  4. repo.AppendAudit(AuditCreated)
  5. notifyMentions -> MentionSink.Notify per @<userID> token
```

`Service` never reimplements case lookup or permission checking: it
composes `packages/caselifecycle.Repository` (case-accessibility) and
`packages/identity` (permission gates), mirroring
`packages/casesearch.Engine`'s composition of the same
`caselifecycle.Repository` seam.

Every read (`Get`, `ListByCase`, `Thread`, `MentionsFor`) requires
`identity.PermViewCase`; every write (`Create`, `UpdateBody`, `Delete`,
`Resolve`, `Reopen`) requires `identity.PermEditCase`. `UpdateBody` and
`Delete` additionally require the acting user to be the annotation's
own author (`ErrNotAuthor` otherwise) — `Resolve`/`Reopen` do not,
since marking a discussion resolved is a case-workflow action any
edit-permitted reviewer may perform, not an authorship privilege.
`AuditTrail` additionally requires `identity.PermAuditRead`.

## Repository

Two implementations, mirroring `packages/casesearch`'s split exactly:

- `InMemoryRepository` — tests and fixtures, safe for concurrent use.
- `PostgresRepository` / `TenantScopedRepository` — backed by the
  `annotations`, `annotation_mentions`, and `annotation_audit_events`
  tables (`packages/persistence/migrations/000012_create_annotations.up.sql`,
  `000013_enable_rls_annotations.up.sql`). `TenantScopedRepository`
  wraps every call in `packages/tenancy.WithTenantScope` so Row-Level
  Security enforces the tenant boundary at the database layer, in
  addition to `PostgresRepository`'s own application-level
  `requireMatchingTenant` guard — defense in depth verified directly in
  `tenant_isolation_test.go` and, against a real Postgres container,
  in `integration_test.go`.

Deleting a thread root cascades to its replies (`parent_id`'s `ON
DELETE CASCADE`), but `annotation_audit_events` intentionally carries
no foreign key back to `annotations` — audit history must survive the
deletion of the annotation it describes, mirroring how
`packages/caselifecycle`'s `case_transitions` table is independent
append-only history.

## Mentions

`ExtractMentions(body string) []uuid.UUID` parses `@<userID>` tokens
(a hyphenated UUID, matching `packages/identity.User.ID`'s type) out
of an annotation's `Body`. Two things happen with a mention, serving
two different consumers:

1. **Storage.** `Repository.Create`/`UpdateBody` persist one `Mention`
   row per token found, queryable via `Repository.MentionsFor` /
   `Service.MentionsFor(ctx, tenantID, userID)` — "every mention of
   me," answerable without re-parsing every annotation body.
2. **Delivery hook.** `Service` also pushes each `Mention` through an
   injected `MentionSink` at write time — `NoOpMentionSink` by default,
   or `MultiMentionSink` to fan out to several. This mirrors
   `packages/signoff.NotificationSink` and
   `packages/accounting.AlertSink`'s idiom exactly. Phase 072 owns the
   full notification pipeline (delivery, batching, read/unread state,
   preferences); this package only needs to emit the event cleanly, so
   `MentionSink` intentionally stays this small.

## Audit

`Service.Create`/`UpdateBody`/`Delete`/`Resolve`/`Reopen` each append
exactly one `AuditRecord` (`AuditCreated` / `AuditEdited` /
`AuditDeleted` / `AuditResolved` / `AuditReopened`) via
`Repository.AppendAudit`. `AuditRecord.ToAuditEvent()` projects a
record into `observability.AuditEvent`, mirroring
`packages/caselifecycle.TransitionRecord.ToAuditEvent` exactly, so
annotation history flows through the same audit channel as case
transitions rather than a second, parallel logging path.
`Repository.ListAudit` / `Service.AuditTrail` return a single
annotation's full history, oldest first.

## Access control / tenant isolation

Every `Service` method re-confirms the target case is visible to the
caller's tenant via `caselifecycle.Repository.Get` before touching
annotation storage — a bare `Repository` call could otherwise let a
tenant-scoped actor read or write an annotation whose `CaseID` belongs
to a different tenant's case, as long as it guessed a valid annotation
ID. `checkCaseAccess` collapses "case does not exist" and "case
belongs to another tenant" into the same `ErrForbidden`, the same
information-hiding posture `packages/casesearch` takes.
`tenant_isolation_test.go` proves this directly: a tenant-B actor
cannot `Get`, cannot see the annotation via `ListByCase`, cannot
`Create` against tenant A's case, and `MentionsFor` never crosses the
tenant boundary even for the same mentioned-user UUID value.

## The `apps/web` UI

`apps/web/src/components/workspace/AnnotationsPanel.tsx` is a
"Discussion" tab in the case workspace
(`apps/web/src/app/cases/[caseId]/page.tsx`): a flat list of
case-level annotations plus a compose box, each with its replies shown
inline and a resolve/reopen toggle. See
`apps/web/docs/annotations-ui.md` for the UI-specific data shape and
component structure.
