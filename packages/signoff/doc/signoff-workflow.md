# Sign-off workflow

`packages/signoff` implements Phase 068's mandatory human sign-off workflow:
the first real, persisted implementation of `packages/guardrail.SignoffGate`.
Until this phase, `guardrail.NoSignoffRecordedGate` was the only
implementation — it always reports `SignoffPending`, fail-closed, because
there was no mechanism anywhere in the codebase by which a case could ever
have an approved sign-off. This package supplies that mechanism.

## Why this package exists now

`packages/guardrail/signoff.go`'s doc comment on `SignoffGate` says it
directly: "once Phase 068 exists, it need only implement `SignoffGate` and
be wired into `CanFinalize`'s caller; no change to this package is
required." That is exactly what this package does — `packages/guardrail`
is untouched by this phase.

## The entity

```go
type SignoffRecord struct {
    ID          uuid.UUID
    CaseID      uuid.UUID
    TenantID    uuid.UUID
    Status      guardrail.SignoffStatus // Pending, Approved, or Rejected
    ReviewerID  uuid.UUID               // who made the most recent decision
    Notes       string                  // required on Reject, optional on Approve
    CaseVersion int                     // caselifecycle Case.MetadataVersion reviewed
    Source      DecisionSource          // reviewer, re_review, or initial
    DecidedAt   time.Time
    CreatedAt   time.Time
}
```

Exactly one current `SignoffRecord` exists per case (enforced by a `UNIQUE
(case_id)` constraint on the `signoff_records` table). Every decision and
every automatic re-review trigger additionally appends an immutable
`AuditEntry` to `signoff_audit_entries`, mirroring
`packages/caselifecycle.TransitionRecord`'s append-only role for case state
transitions.

## Explicit human acknowledgement

`Service.Approve` and `Service.Reject` both take a `DecisionInput` that
must carry:

- an authenticated `identity.User` on `ctx` holding `identity.PermSignOff`
  (already defined on `identity.RoleJudge` in the `PermissionMatrix` — no
  new role was needed for this phase);
- `Acknowledgement` equal to the exact literal
  `signoff.AcknowledgementConfirmation` ("I acknowledge and approve this
  review decision") — a caller cannot satisfy this by accident with some
  other truthy value; a UI is expected to require the reviewer to
  deliberately confirm this phrase before the request is even built;
- `CaseVersion` matching the case's *live* `MetadataVersion` (via
  `CaseVersionReader`) — a reviewer can never approve or reject content
  other than what they actually reviewed.

`Reject` additionally requires non-blank `Notes` (`ErrNotesRequired`
otherwise); `Approve`'s `Notes` are optional.

```go
rec, err := svc.Approve(ctx, signoff.DecisionInput{
    TenantID:        tenantID,
    CaseID:          caseID,
    CaseVersion:     currentCase.MetadataVersion,
    Acknowledgement: signoff.AcknowledgementConfirmation,
    Notes:           "Reviewed reasoning tree and evidence citations; concur.",
})
```

## Wiring example: swapping `NoSignoffRecordedGate` for `GateImpl`

This is the concrete example `packages/guardrail/signoff.go` anticipates.
Before Phase 068, a caller finalizing a case looked like:

```go
ok, err := guardrail.CanFinalize(ctx, caseID.String(), guardrail.NoSignoffRecordedGate{})
```

That gate can never return `SignoffApproved` — `CanFinalize` always blocks.
After this phase, swap in `signoff.GateImpl`, backed by a real
`signoff.Repository`:

```go
// Built once, e.g. at service startup.
signoffRepo := signoff.NewTenantScopedRepository(pool) // or signoff.NewInMemoryRepository() in tests

// Per finalization request, scoped to the caller's tenant.
gate, err := signoff.NewGate(signoffRepo, tenantID)
if err != nil {
    return err
}

ok, err := guardrail.CanFinalize(ctx, caseID.String(), gate)
if !ok {
    // err wraps guardrail.ErrSignoffNotApproved until Approve has been
    // called for this case.
    return err
}
```

No other change to `guardrail.CanFinalize` or its callers is required —
`GateImpl` implements `guardrail.SignoffGate` exactly, and reports the same
fail-closed `SignoffPending` status as `NoSignoffRecordedGate` for any case
that has never entered the sign-off workflow (see
`TestGateImpl_MatchesNoSignoffRecordedGate_ForUnknownCase`).

## Case-version reader: connecting to `packages/caselifecycle`

This package does not import `packages/caselifecycle.Repository` directly
in its core API — `CaseVersionReader` is a narrow, one-method interface
(`CaseVersion(ctx, tenantID, caseID) (int, error)`) that keeps the
dependency surface minimal and easy to fake in tests. A ready-made adapter
is provided so callers who already have a `caselifecycle.Repository` do not
need to hand-write one:

```go
caseRepo := caselifecycle.NewTenantScopedRepository(pool)
reader := signoff.NewCaselifecycleVersionReader(caseRepo)

svc, err := signoff.NewService(signoffRepo, reader, notifier)
```

## Re-review on case update

If a case's content changes after approval — modeled here as its
`caselifecycle.Case.MetadataVersion` advancing — a stale approval must not
survive. `Service.ReReviewOnCaseUpdate` checks the live version against the
version recorded on the current `SignoffRecord`:

- if the record is `Approved` and the version has changed, it reverts to
  `Pending`, appends an `AuditEntry` (`Source: DecisionSourceReReview`)
  explaining exactly what changed (e.g. "case metadata version changed from
  3 to 4 after approval"), and fires a `PendingSignoffEvent` notification;
- if the record is `Rejected`, a version change does not revert it — only
  an `Approved` sign-off is ever automatically reverted, since a rejection
  already means "not ready," and the reviewer would want to see it again
  regardless of what changed;
- if the version has not changed, or no record exists yet, this is a no-op.

Callers are expected to invoke `ReReviewOnCaseUpdate` after any operation
that bumps a case's `MetadataVersion` (`caselifecycle.SetMetadata` /
`MergeMetadata`), or on a schedule/webhook, so a stale approval can never
survive a case content change silently.

## Notifications

`NotificationSink` mirrors `packages/accounting.AlertSink`'s idiom exactly:

```go
type NotificationSink interface {
    Notify(ctx context.Context, event PendingSignoffEvent) error
}
```

`LoggingNotificationSink` (writes to the standard logger),
`NoOpNotificationSink` (discards silently), and `MultiNotificationSink`
(fans out to several sinks) are provided. A `PendingSignoffEvent` fires
from two places:

- `Service.MarkAwaitingSignoff` — call this when a case enters the state
  that requires judicial review (e.g. transitioning into
  `caselifecycle.StateUnderReview`);
- `Service.ReReviewOnCaseUpdate` — fires automatically on a re-review
  reversion (see above).

## Access control

- `RequireSignoffPermission` (used internally by `Approve`/`Reject`)
  requires `identity.PermSignOff`, which the existing `PermissionMatrix`
  already grants to `identity.RoleJudge`. No new role or permission was
  added by this phase.
- `RequireViewPermission` (used internally by `Get`/`History`) requires
  `identity.PermViewCase`, mirroring `packages/caselifecycle`'s read guard.

## Persistence and tenant isolation

Two `Repository` implementations are provided, mirroring
`packages/caselifecycle`'s split exactly:

- `PostgresRepository` — backed by the `signoff_records` and
  `signoff_audit_entries` tables (see
  `packages/persistence/migrations/000008_create_signoff.up.sql` and
  `000009_enable_rls_signoff.up.sql`), takes a `persistence.Executor` per
  call so it composes inside a larger transaction.
- `TenantScopedRepository` — wraps `PostgresRepository` with
  `packages/tenancy.WithTenantScope`, so Row-Level Security enforces
  tenant isolation at the database layer in addition to the
  application-level `requireMatchingTenant` guard. This is the type
  production code should use against a live `*pgxpool.Pool`.
- `InMemoryRepository` — for tests and other packages' fixtures.

`signoff_records.case_id` carries a `UNIQUE` constraint (one current record
per case) and a `CHECK` constraint requiring non-blank `notes` whenever
`status = 'rejected'`, enforcing "Reject requires notes" at the database
layer as defense-in-depth alongside the application-level check in
`Service.Reject`.

## Relationship to `packages/guardrail`

`guardrail.CanFinalize` is unchanged by this phase — it still just calls
`gate.Status(ctx, caseID)` and requires `SignoffApproved`. This package
supplies the gate; it does not, and should not, need to modify
`packages/guardrail` itself.

## Testing

- Unit tests (`decision_test.go`, `rereview_test.go`, `notification_test.go`,
  `tenant_isolation_test.go`) run against `InMemoryRepository` and a fake
  `CaseVersionReader`, covering: acknowledgement enforcement, permission
  enforcement, case-version matching, notes-required-on-reject, re-review
  reversion (and its non-reversion cases), notification firing, and tenant
  isolation.
- `gate_test.go` proves `CanFinalize` enforcement **end-to-end using the
  real `guardrail.CanFinalize` function**, not a mock: no record blocks, an
  explicit `Reject` blocks, and only a genuine `Approve` call unblocks
  finalization.
- `integration_test.go` is a testcontainers-backed suite (skipped under
  `-short`, matching `packages/caselifecycle`'s pattern) that exercises the
  same flows against real Postgres, including RLS tenant isolation and a
  real `caselifecycle.SetMetadata` version bump driving
  `ReReviewOnCaseUpdate`.
