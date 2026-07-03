// Package caseversioning provides a case-level, unified version history
// and diff/restore capability spanning tree revisions, evidence/
// annotation changes, and opinion revisions, mirroring
// packages/annotations's and packages/signoff's composition-over-
// reimplementation posture.
//
// # Composition, not reimplementation
//
// Verdex already versions several individual case artifacts:
//
//   - packages/irac / packages/treeassembly maintain the reasoning
//     tree's own revision sequence (irac.TreeRevision, versioned via
//     irac.NextRevision / treeassembly.NextRevision on every
//     re-assembly).
//   - packages/caselifecycle tracks Case.MetadataVersion, bumped by
//     SetMetadata/MergeMetadata, and packages/signoff's
//     ReReviewOnCaseUpdate (rereview.go) already detects "case changed
//     since X" by comparing a stored CaseVersion against the live
//     MetadataVersion.
//   - packages/annotations keeps its own per-annotation audit trail
//     (AuditRecord) for edits to notes/highlights.
//
// This package does not duplicate any of that. Snapshot's
// ArtifactRevisionRef field is a pointer into whichever of those
// mechanisms already exists for a given ArtifactKind:
//
//   - ArtifactTree snapshots store the irac.TreeRevision.RevisionNumber
//     as a string. The tree content itself lives in
//     packages/treeassembly's own snapshot store (see
//     packages/treeassembly/persist.go); this package never copies it.
//   - ArtifactCaseMetadata snapshots store the case's
//     MetadataVersion at capture time as ArtifactRevisionRef, and — since
//     Diff/Restore for this kind require actual field values, not just a
//     version number — a compact CaseMetadataPayload copy of the
//     mutable Case fields in Payload.
//   - ArtifactEvidence snapshots store an upstream ID (an
//     annotations.Annotation ID or evidence segment ID) as
//     ArtifactRevisionRef when one is derivable from the triggering
//     event; the evidence/annotation content itself stays in
//     packages/evidence / packages/annotations.
//
// # Opinion history is new
//
// No existing package assigns packages/synthesisagent.Opinion output a
// revision identifier — every synthesis run simply produces a value,
// with no persisted history of prior runs. ArtifactOpinion is this
// package's one genuinely new capability: SnapshotOpinion stores a
// compact OpinionPayload copy (CaseID, Conclusions, SkippedIssueNodeIDs,
// GeneratedAt) directly in Payload, since there is no upstream revision
// ID to merely reference.
//
// # Snapshot: the case-level aggregator entity
//
// A Snapshot is one immutable, point-in-time record of a single case
// artifact's state, always attributed to an actor (CreatedBy) and
// optionally a Reason/Label for change attribution (e.g. "manual edit",
// "signoff re-review", "restore to snapshot <id>"). Repository.ListByCase
// returns every Snapshot for a case across all four ArtifactKind values
// in chronological order — the version-history timeline this phase
// exposes in apps/web's case workspace "History" tab.
//
// # Diff
//
// ComputeDiff (and Service.Diff, its access-controlled entrypoint)
// compares two Snapshots of the same case and ArtifactKind:
//
//   - ArtifactCaseMetadata gets a real field-by-field diff
//     (FieldChanges): title, reference, category_id, state, and every
//     key present in either snapshot's Metadata map.
//   - ArtifactTree, ArtifactEvidence, and ArtifactOpinion get a
//     reference-level diff (RevisionRefChanged / RevisionRefBefore /
//     RevisionRefAfter): whether the pointed-to upstream revision
//     changed, not a content diff of the tree or opinion text itself —
//     that belongs to packages/irac/packages/treeassembly and
//     packages/synthesisagent respectively.
//
// # Restore
//
// Service.Restore reverts a case's live metadata to a prior
// ArtifactCaseMetadata snapshot's payload via caselifecycle.
// Repository.Update, then records a brand-new Snapshot documenting the
// restore itself (RestoredFromID pointing back at the source snapshot).
// History is never rewritten or deleted — Restore only ever appends a
// new, forward-only entry, so "what did version history say before the
// restore" remains answerable after it. Only ArtifactCaseMetadata
// snapshots can be restored (ErrNotRestorable otherwise): reverting a
// tree, evidence, or opinion artifact means acting through its own
// upstream package, not this one.
//
// # Access control
//
// Every Service method requires ctx to carry an authenticated
// identity.User (see access.go): reads require identity.PermViewCase,
// writes (snapshotting and restoring) require identity.PermEditCase.
// Every operation also re-confirms the target case is visible to the
// caller's tenant via the composed caselifecycle.Repository.Get before
// touching snapshot storage, and every Repository implementation
// refuses (via ErrCrossTenantAccess) to read or write a Snapshot whose
// TenantID does not match the scope's tenantID — the same
// belt-and-braces posture packages/annotations and
// packages/caselifecycle take, verified here in
// tenant_isolation_test.go.
//
// See doc/case-versioning.md for the full data model and a worked
// example of the relationship to packages/irac/packages/treeassembly
// tree revisions and packages/caselifecycle.MetadataVersion.
package caseversioning
