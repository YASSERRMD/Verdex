# Notifications

`packages/notifications` implements Phase 072: a central, persisted
notification inbox — the real delivery sink for four upstream
packages that already emit notification-shaped events, but each
previously had only a logging stub.

## Why this package exists now

Several earlier phases anticipated a central notification pipeline and
left a seam for it, but never built the sink itself:

| Upstream package        | Interface                          | Fired on                              |
| ------------------------ | ----------------------------------- | -------------------------------------- |
| `packages/signoff`       | `NotificationSink.Notify`           | A case enters/remains "awaiting sign-off" (`PendingSignoffEvent`) |
| `packages/annotations`   | `MentionSink.Notify`                | An annotation body contains an `@<userID>` mention (`Mention`) |
| `packages/reasoningeval` | `AlertSink.Send`                    | A reasoning-quality regression crosses its threshold (`Alert`) |
| `packages/accounting`    | `AlertSink.Send`                    | A tenant's token/cost budget crosses a warning or exceeded threshold (`AlertEvent`) |

Until this phase, every one of those interfaces had only a
`Logging*Sink`/`NoOp*Sink` implementation — the event fired, a log
line was written (or nothing at all), and no user ever saw it. This
package is the real sink.

## The model

```
Notification
  ID, TenantID, RecipientID, Kind, Title, Body
  CaseID           *uuid.UUID  (optional back-reference)
  RelatedEntityID  *uuid.UUID  (optional back-reference)
  CreatedAt, ReadAt *time.Time (nil while unread)

Preference
  TenantID, UserID, Kind
  Enabled   bool        (opt-out, not opt-in — see below)
  Channels  []Channel   (additional delivery channels beyond in-app)
```

`Kind` is a small closed enum (`types.go`):

- `KindIngestionComplete` — an evidence ingestion run finished.
- `KindPendingSignoff` — a case needs sign-off.
- `KindMention` — a user was `@`-mentioned in a discussion.
- `KindQualityAlert` — a reasoning-quality regression fired.
- `KindBudgetAlert` — a budget threshold was crossed.
- `KindTaskAssignment` — a user was assigned to review/act on a case.

`Channel` names an *additional* delivery channel beyond the always-on
in-app inbox: `ChannelEmail`, `ChannelPush`. Both are logged, no-op
stubs today (`channel.go`'s `LoggingChannelDeliverer` — see
`EmailChannel`/`PushChannel`) behind the `ChannelDeliverer` interface,
the seam a later phase wires a real transport into.

## Repository pattern

Mirrors `packages/caseversioning`'s repository pattern exactly:
`Repository` and `PreferenceRepository` interfaces, each with an
`InMemoryRepository`/`InMemoryPreferenceRepository` (tests, fixtures)
and a `PostgresRepository`/`PostgresPreferenceRepository` +
`TenantScopedRepository`/`TenantScopedPreferenceRepository` pair
(production, backed by `notifications` and `notification_preferences`
tables — `packages/persistence/migrations/000016_create_notifications.up.sql`,
RLS enabled in `000017_enable_rls_notifications.up.sql`). Every method
takes an explicit `tenantID` and refuses cross-tenant access via
`ErrCrossTenantAccess`; `TenantScopedRepository`/
`TenantScopedPreferenceRepository` additionally open an RLS-scoped
transaction per call via `packages/tenancy.WithTenantScope`, so tenant
isolation is enforced at the database layer too (proven in
`postgres_integration_test.go`).

## Service: the single write path

`Service.Notify` (`service.go`) is the only path that persists a
`Notification`. Every adapter below funnels through it, so preference
and persistence semantics stay uniform regardless of which upstream
event triggered the call.

**Preferences are opt-out, not opt-in.** `Service.isEnabled` treats
the *absence* of an explicit `Preference` row as "enabled, in-app
only" — a user must actively write `Preference{Enabled: false}` to
suppress a `Kind`. When suppressed, `Notify` returns `(nil, nil)`: no
row is written, no channel fires — this is the behavior
`TestNotify_SuppressedKindIsNotStoredOrDelivered` proves.

**Access control.** `Notify` itself is not actor-gated — it is called
by trusted server-side event hooks (the adapters below), not directly
by an end user. Every inbox-reading or preference-writing method
(`List`, `UnreadCount`, `MarkRead`, `MarkAllRead`, `SetPreference`,
`Preferences`) is gated by `authorizeSelf` (`access.go`): the `ctx`
actor must be the same user whose inbox is being read or changed —
there is no "view teammate's notifications" permission in this
system.

## Adapters: one per upstream sink interface

Each adapter is a genuine implementation of its upstream interface —
verified in `adapters_test.go`/`accounting_adapter.go`'s tests with a
`var _ upstream.Interface = (*Adapter)(nil)` assertion — that
translates the upstream event into a `NotifyInput` and calls
`Service.Notify`:

| Adapter (file) | Implements | Recipient resolution |
| --- | --- | --- |
| `SignoffNotificationSink` (`adapters.go`) | `signoff.NotificationSink` | `PendingSignoffEvent` is per-case, not per-user, so the adapter takes a `RecipientResolver` callback: `(tenantID, caseID) -> []userID`. |
| `AnnotationsMentionSink` (`adapters.go`) | `annotations.MentionSink` | `Mention.MentionedUserID` names the recipient directly — no resolution needed. |
| `ReasoningEvalAlertSink` (`adapters.go`) | `reasoningeval.AlertSink` | `Alert` carries no tenant or recipient (a pipeline-quality signal, not a per-user event), so the adapter is constructed with a fixed `TenantID` and `Recipients` list — typically the tenant's admins/auditors. |
| `AccountingAlertSink` (`accounting_adapter.go`) | `accounting.AlertSink` | `AlertEvent` carries `TenantID` but no recipient, so the adapter is constructed with a fixed `Recipients` list — typically the tenant's admins. |

`TestSignoffIntegration_ReReviewFiresRealNotification`
(`integration_test.go`) drives the actual `packages/signoff.Service`
end to end (`Approve` → simulate a case-content change →
`ReReviewOnCaseUpdate`) with `SignoffNotificationSink` as its live
`NotificationSink`, proving the adapter works against real upstream
behavior, not a hand-built event value.

### Wiring example

```go
notifSvc, _ := notifications.NewService(notifRepo, prefRepo)

// signoff
sink := notifications.NewSignoffNotificationSink(notifSvc, myRecipientResolver)
signoffSvc, _ := signoff.NewService(signoffRepo, caseVersionReader, sink)

// annotations
mentionSink := notifications.NewAnnotationsMentionSink(notifSvc)
annotationsSvc := annotations.NewService(annotationsRepo, caseRepo, mentionSink)

// reasoningeval
alertSink := notifications.NewReasoningEvalAlertSink(notifSvc, tenantID, adminIDs)
checker := reasoningeval.NewQualityAlertChecker(threshold, alertSink)

// accounting
budgetSink := notifications.NewAccountingAlertSink(notifSvc, adminIDs)
accountingSvc := accounting.NewAccountingService(accountingRepo, budgetChecker, budgetSink)
```

## Entrypoints that are not adapters

Two `Kind` values have no upstream sink interface to satisfy, per
task scope — they are plain functions wrapping `Service.Notify`,
documented as the call site future work should use:

- **`NotifyIngestionComplete`** (`ingestion.go`) — `packages/ingestion`
  has no existing sink-style hook (unlike the four packages above),
  and wiring one in was out of scope for this phase ("don't modify
  ingestion unless trivial and justified"). Once
  `packages/ingestion`'s orchestration is ready to notify, its call
  site is exactly `notifications.NotifyIngestionComplete(ctx, notifSvc, tenantID, caseID, recipientID, documentCount)`.
- **`NotifyTaskAssignment`** (`taskassignment.go`) — no new assignment
  engine is introduced; reviewer/assignee selection remains
  `packages/caselifecycle`'s/`packages/signoff`'s concern. This is
  simply the entrypoint those (or any future) call sites invoke once
  they have decided who is assigned.

## Web UI

`apps/web/src/components/layout/NotificationBell.tsx` is a dropdown
bell in `TopBar` showing the unread count (polled via
`GET /api/v1/notifications/unread-count`), the recent notification
list (`GET /api/v1/notifications`), and mark-read/mark-all-read
actions (`POST /api/v1/notifications/:id/read`,
`POST /api/v1/notifications/mark-all-read`) — see
`apps/web/src/types/index.ts`'s `NotificationEntry` for the wire shape
and `apps/web/__tests__/NotificationBell.test.tsx` for coverage of the
empty state, unread badge, and mark-read interactions.
