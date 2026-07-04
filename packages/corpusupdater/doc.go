// Package corpusupdater is Phase 089: keeping the statute (Phase 035,
// packages/statute) and precedent (Phase 036, packages/precedent)
// corpora current per jurisdiction as amendments and new rulings are
// published, without duplicating either corpus's own node model, the
// re-embedding machinery already provided by packages/embedding (Phase
// 015), the notification pipeline shape established by
// packages/notifications (Phase 072), or the durable audit sink added
// in Phase 077 (packages/auditlog).
//
// # What is new here
//
//   - CorpusUpdateJob / JobStatus (types.go): a jurisdiction-scoped
//     unit of work describing one batch of incoming changes to a named
//     TargetCorpus (CorpusStatute or CorpusPrecedent), moving through a
//     closed state machine -- StatusPending -> StatusValidating ->
//     StatusApplying -> (StatusApplied | StatusFailed) ->
//     optionally StatusRolledBack -- via IsValidTransition, never a
//     free-form status string (task 1).
//   - Amendment / ChangeType (types.go): a single staged change to a
//     statute section or precedent entry, naming its TargetID (an
//     existing packages/statute.RuleNode.ID or
//     packages/precedent node ID, referenced by string ID/tag -- this
//     package does not import either corpus package) or leaving
//     TargetID empty when ChangeType is ChangeTypeAdd (a brand new
//     entry). Carries NewText, Citation, and EffectiveDate (task 2).
//   - IsEffective / Engine.EffectiveAmendments (effective.go): real
//     effective-date handling -- an Amendment is "live" only once
//     EffectiveDate <= the caller-supplied now, and
//     EffectiveAmendments's query path filters staged amendments down
//     to just the effective ones, tested with both past and future
//     dates (task 3).
//   - Embedder (embedder.go): a small interface shaped like
//     packages/embedding.EmbeddingService's own Embed method --
//     Engine.ApplyAmendment calls it exactly once per changed rule/
//     precedent after a text change lands, so retrieval indices never
//     drift from a corpus's current text. A production caller adapts
//     packages/embedding.EmbeddingService directly; this package never
//     reimplements embedding or imports a provider (task 4).
//   - NotificationSink (notification.go): an interface receiving a
//     ChangeNotification when an amendment affecting a rule/precedent
//     goes live, listing the case IDs an caller-supplied
//     AffectedCaseResolver names as referencing that target --
//     composing conceptually with packages/notifications the same way
//     packages/signoff.NotificationSink and
//     packages/annotations.MentionSink already do, by shape rather
//     than by import, keeping this package's dependency footprint thin
//     (task 5).
//   - Validate (validation.go): structural checks on an Amendment
//     before Engine.StageAmendment accepts it -- non-empty citation,
//     a recognized ChangeType, an EffectiveDate within a sane window of
//     now, and (for Amend/Repeal) a non-empty TargetID -- producing
//     the structured ErrInvalidAmendment family rather than silently
//     accepting malformed input (task 6).
//   - Engine.Rollback (engine.go): reverts every applied Amendment in a
//     job back to its PreviousText/PreviousCitation snapshot, captured
//     at ApplyAmendment time before the new text overwrote it, and
//     transitions the job to StatusRolledBack. Real revert logic, not
//     a job-status flip alone (task 7).
//   - AuditSink (audit.go): records every job status transition and
//     amendment stage/apply/rollback into packages/auditlog.Store, the
//     same durable, hash-chained sink the rest of the platform already
//     writes to. No second audit table (task 8).
//   - identity.PermViewCorpusUpdater / identity.PermManageCorpusUpdater
//     (packages/identity/permission.go): the fine-grained permissions
//     Engine gates every operation on, following the exact
//     PermViewIntegration/PermManageIntegration precedent from Phase
//     087.
//
// # What is explicitly reused, not duplicated
//
//   - packages/statute (Phase 035) and packages/precedent (Phase 036)
//     remain the only corpus node models in this codebase. Amendment
//     references a rule/precedent by TargetID string, exactly as
//     packages/compliance.Control.MappedTo references platform
//     features by string tag rather than importing them.
//   - packages/embedding.EmbeddingService (Phase 015) remains the only
//     embedding computation in this codebase. Embedder is a thin,
//     locally-declared interface a real adapter wraps around it; this
//     package never computes a vector itself.
//   - packages/notifications (Phase 072) remains the only persisted,
//     user-facing notification inbox. NotificationSink is this
//     package's outbound event shape, mirroring
//     packages/signoff.NotificationSink and
//     packages/reasoningeval.AlertSink's own seam-not-dependency
//     posture -- this package does not import packages/notifications.
//   - packages/auditlog.Store (Phase 077) is the only durable event
//     sink this package writes to, via AuditSink -- the same
//     composition pattern packages/compliance's and packages/privacy's
//     own AuditSink established.
//   - identity.Role / identity.Permission / identity.HasPermission
//     (Phase 006) remain the coarse RBAC gate every Engine method
//     calls through authorizeManage/authorizeView before doing
//     anything corpus-update-specific.
//
// See doc/corpus-updater.md for the full write-up.
package corpusupdater
