// Package notifications provides Verdex's central, persisted
// notification inbox: a Notification entity, delivery-channel stubs,
// read/unread state, and per-user/per-Kind preferences — plus real
// adapters that satisfy the sink interfaces four upstream packages
// already define and fire events into, so those events finally reach
// a user-visible inbox instead of only a log line.
//
// # Why this package exists
//
// Several earlier phases anticipated a central notification pipeline
// and left a seam for it:
//
//   - packages/signoff.NotificationSink fires a PendingSignoffEvent
//     whenever a case enters (or remains in) "awaiting sign-off".
//   - packages/annotations.MentionSink fires a Mention whenever an
//     annotation body contains an "@<userID>" token.
//   - packages/reasoningeval.AlertSink fires an Alert whenever a
//     quality-regression check crosses its threshold.
//   - packages/accounting.AlertSink fires an AlertEvent whenever a
//     tenant's token/cost budget crosses a warning or exceeded
//     threshold.
//
// Until this phase, every one of those interfaces had only a logging
// (or no-op) implementation — the event fired, but nothing durable or
// user-facing happened. This package is the real sink: adapters.go and
// accounting_adapter.go define one adapter per upstream interface,
// each verified with a `var _ upstream.Interface = (*Adapter)(nil)`
// assertion, translating the upstream event into a NotifyInput and
// calling Service.Notify.
//
// # Notification: the persisted entity
//
// A Notification (types.go) is addressed to exactly one RecipientID
// within one TenantID, carries a Kind (one of the Kind constants),
// a Title/Body, and optional CaseID/RelatedEntityID back-references to
// whatever triggered it. ReadAt is nil until MarkRead/MarkAllRead sets
// it — see Repository (repository.go) and its InMemoryRepository/
// PostgresRepository/TenantScopedRepository implementations, which
// mirror packages/caseversioning's repository pattern exactly,
// including its tenant-isolation posture (ErrCrossTenantAccess,
// row-level security in migrations/000017_enable_rls_notifications.up.sql).
//
// # Delivery channels are stubs, in-app is the source of truth
//
// Every Notification is always persisted (in-app delivery). Channel
// (types.go: ChannelEmail, ChannelPush) names *additional* channels a
// user's Preference can opt a Kind into; ChannelDeliverer
// (channel.go) is the seam for those, but for this phase both
// EmailChannel and PushChannel are LoggingChannelDeliverer — a stub
// that logs and returns nil, never a real transport. A stub channel
// failing must never roll back or block the in-app Notification, which
// is always written first via Repository.Create.
//
// # Preferences are opt-out, not opt-in
//
// Preference (types.go) is a per-(TenantID, UserID, Kind) row with an
// Enabled flag and a Channels list. Service.isEnabled (service.go)
// treats the *absence* of an explicit Preference row as "enabled,
// in-app only" — a user must actively disable a Kind to suppress it.
// When disabled, Service.Notify returns (nil, nil): no Notification
// row is written and no channel fires, per task 9 ("suppressed Kind
// isn't stored/delivered").
//
// # Recipient resolution for tenant/case-wide events
//
// packages/annotations.Mention already names its recipient
// (MentionedUserID) directly, so AnnotationsMentionSink needs no extra
// wiring. The other three upstream events do not carry a natural
// single recipient:
//
//   - signoff.PendingSignoffEvent is per-case, not per-user, so
//     SignoffNotificationSink takes a RecipientResolver callback
//     mapping (tenantID, caseID) to the user IDs to notify.
//   - reasoningeval.Alert and accounting.AlertEvent are
//     tenant/system-wide signals with no case or user attached, so
//     ReasoningEvalAlertSink and AccountingAlertSink are constructed
//     with a fixed Recipients list (typically the tenant's
//     admins/auditors who own that monitoring surface).
//
// # Ingestion-complete and task-assignment are documented entrypoints,
// not integrations
//
// Per tasks 2 and 5, this package does not modify packages/ingestion
// or introduce a new task-assignment engine. NotifyIngestionComplete
// (ingestion.go) and NotifyTaskAssignment (taskassignment.go) are
// plain functions wrapping Service.Notify with the right Kind and
// fields — the call site packages/ingestion (once it has a completion
// hook) or any assignment-aware code (packages/caselifecycle,
// packages/signoff) can invoke once it has decided who to notify.
//
// # Access control
//
// Service.Notify (service.go) is deliberately not actor-gated by
// identity permissions — it is called by trusted server-side event
// hooks (the adapters above, or future direct call sites), not
// directly by an end user through an HTTP handler. Every inbox-reading
// or preference-writing Service method (List, UnreadCount, MarkRead,
// MarkAllRead, SetPreference, Preferences) is gated by authorizeSelf
// (access.go): the ctx actor must be the same user whose inbox/
// preferences are being read or changed — there is no
// "view teammate's notifications" permission in this system.
//
// See doc/notifications.md for the full adapter-to-interface mapping
// and a worked example of the web inbox UI's data flow.
package notifications
