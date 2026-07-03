// Package annotations provides multi-user notes, highlights, and
// threaded discussion attached to a case, mirroring
// packages/casesearch's and packages/signoff's composition-over-
// reimplementation posture.
//
// # Composition, not reimplementation
//
// annotations is a thin layer over three existing packages:
//
//   - packages/caselifecycle supplies the Case entity every annotation
//     ultimately belongs to, and the tenant-accessibility check Service
//     performs before any read or write (checkCaseAccess).
//   - packages/identity gates every Service call on an authenticated
//     user holding identity.PermViewCase (reads) or
//     identity.PermEditCase (writes), mirroring
//     packages/casesearch's and packages/caselifecycle's access model.
//   - packages/observability supplies the AuditEvent projection
//     (AuditRecord.ToAuditEvent) so annotation history flows through
//     the same audit channel as the rest of the system.
//
// # Anchoring
//
// Every Annotation carries an AnchorType and AnchorID pair identifying
// what it is attached to:
//
//   - AnchorCase: the annotation applies to the case as a whole.
//     AnchorID is empty.
//   - AnchorTreeNode: the annotation is anchored to one node in the
//     case's packages/irac reasoning tree. AnchorID is that node's
//     string ID (irac.Node.ID's ID space).
//   - AnchorEvidenceSegment: the annotation is anchored to one evidence
//     segment. AnchorID is that segment's string ID, the same ID space
//     apps/web/src/components/workspace/EvidenceReviewPanel.tsx uses.
//
// Repository.ListByCase accepts an AnchorFilter to narrow results to a
// single anchor type, or a single anchor type + specific anchor ID
// (e.g. "every annotation on tree node X").
//
// # Threading
//
// An Annotation with a nil ParentID is a thread root; one with
// ParentID set is a reply. Threading is exactly one level deep — Create
// rejects a reply-to-a-reply with ErrParentIsReply, keeping the model
// simple (flat discussion under each root) rather than arbitrarily
// nested. Repository.Thread(ctx, tenantID, rootID) returns the root
// followed by its replies, ordered oldest first.
//
// # Mentions
//
// ExtractMentions parses "@<userID>" tokens (a hyphenated UUID,
// matching packages/identity.User.ID's type) out of an Annotation's
// Body. Repository implementations persist one Mention row per token
// found on Create/UpdateBody, queryable via
// Repository.MentionsFor/Service.MentionsFor. Separately, Service
// pushes each Mention through an injected MentionSink at write time —
// a real, minimal hook (mirroring packages/signoff.NotificationSink
// and packages/accounting.AlertSink's idiom exactly) that Phase 072's
// full notification system is expected to consume. This package does
// not itself deliver, batch, retry, or persist a notification feed.
//
// # Resolve / reopen
//
// Resolved, ResolvedBy, and ResolvedAt track whether an annotation (a
// thread root or a reply — either can represent an open question) has
// been marked resolved. Resolve/Reopen are idempotency-checked:
// resolving an already-resolved annotation returns ErrAlreadyResolved,
// and reopening a not-resolved one returns ErrNotResolved.
//
// # Access control
//
// Every Service method requires ctx to carry an authenticated
// identity.User (see access.go). Reads require identity.PermViewCase;
// writes require identity.PermEditCase. UpdateBody and Delete
// additionally require the acting user to be the annotation's author
// (ErrNotAuthor otherwise) — Resolve/Reopen do not, since resolution is
// a case-workflow action rather than an authorship one. Every
// operation also re-confirms the target case is visible to the
// caller's tenant via the composed caselifecycle.Repository.Get before
// touching annotation storage, and every Repository implementation
// refuses (via ErrCrossTenantAccess) to read or write an Annotation
// whose TenantID does not match the scope's tenantID — the same
// belt-and-braces posture packages/casesearch and packages/
// caselifecycle take, verified for annotations in
// tenant_isolation_test.go.
//
// # Audit
//
// Service.Create/UpdateBody/Delete/Resolve/Reopen each append exactly
// one AuditRecord (AuditCreated/AuditEdited/AuditDeleted/
// AuditResolved/AuditReopened) via Repository.AppendAudit, queryable
// per-annotation via Repository.ListAudit/Service.AuditTrail (which
// additionally requires identity.PermAuditRead). AuditRecord rows are
// retained even after their annotation is deleted — see
// packages/persistence/migrations/000012_create_annotations.up.sql's
// annotation_audit_events table, which intentionally carries no
// foreign key back to annotations.
//
// See doc/annotations.md for the full data model, access-control
// write-up, and the apps/web UI this package backs.
package annotations
