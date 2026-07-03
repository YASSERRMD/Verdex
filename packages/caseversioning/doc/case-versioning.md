# Case versioning & history

`packages/caseversioning` implements Phase 071: a unified, case-level
version history spanning the case's reasoning tree, evidence/annotation
changes, and draft-opinion output, with diff and restore. It is the
case-level aggregator over versioning mechanisms that already exist
elsewhere — not a reimplementation of any of them.

## Why this package exists now

Before this phase, three separate packages each version their own
narrow slice of a case, with no unified view across them:

- `packages/irac` / `packages/treeassembly` version the reasoning tree
  itself (`irac.TreeRevision`, bumped by `irac.NextRevision` /
  `treeassembly.NextRevision` on every re-assembly).
- `packages/caselifecycle` tracks `Case.MetadataVersion`, bumped by
  `SetMetadata`/`MergeMetadata`, and `packages/signoff`'s
  `ReReviewOnCaseUpdate` (`rereview.go`) already knows how to detect
  "case changed since X" by comparing a stored version against the
  live one.
- `packages/annotations` keeps its own per-annotation audit trail for
  edits to notes/highlights.

None of these let a reviewer see "everything that changed on this
case, in order" in one place, or diff/restore across artifact kinds.
`packages/caseversioning` is that aggregator: a `Snapshot` per
artifact-kind change, one chronological timeline per case, and
diff/restore built on top.

## What this package composes vs. adds

| Artifact kind          | Source of truth                          | What `Snapshot` stores                                  |
|-------------------------|-------------------------------------------|----------------------------------------------------------|
| `case-metadata`         | `packages/caselifecycle.Case`             | `ArtifactRevisionRef` = `MetadataVersion`, plus a compact `CaseMetadataPayload` copy (needed for real diff/restore) |
| `tree`                  | `packages/irac` / `packages/treeassembly` | `ArtifactRevisionRef` = `irac.TreeRevision.RevisionNumber` — a **reference only**, never a copy of the tree |
| `evidence`               | `packages/annotations` / `packages/evidence` | `ArtifactRevisionRef` = an upstream annotation/segment ID, when derivable — a **reference only** |
| `opinion`                | *(none — new in this phase)*              | A compact `OpinionPayload` copy (`Conclusions`, `SkippedIssueNodeIDs`, `GeneratedAt`), since `synthesisagent.Opinion` has no revision ID of its own anywhere upstream |

Only `case-metadata` and `opinion` snapshots carry a `Payload`.
`tree` and `evidence` snapshots are pure references — this package
never duplicates the tree store (`packages/treeassembly/persist.go`
already snapshots trees by case/revision) or the annotation/evidence
store.

**Opinion history is the one genuinely new capability.** No package
before this phase assigns `synthesisagent.Opinion` output any
identifier a later run could be compared against; every synthesis run
just produces a value and the previous one is gone. `SnapshotOpinion`
is what makes "what did the draft opinion say three revisions ago"
answerable for the first time.

## The model

```
Snapshot
  ID, CaseID, TenantID
  ArtifactKind:        case-metadata | tree | evidence | opinion
  ArtifactRevisionRef:  upstream revision id/version, when one exists
  Payload:              CaseMetadataPayload | OpinionPayload | nil
  CreatedBy, Reason, Label
  RestoredFromID:        set only on a Restore's own forward-only snapshot
  CreatedAt
```

`Repository.ListByCase` returns every `Snapshot` for a case, across all
four kinds, ordered oldest-first — the version-history timeline the
`apps/web` "History" tab renders directly.

## Diff

`ComputeDiff` (and `Service.Diff`, its access-controlled entrypoint)
compares two snapshots of the same case and the same `ArtifactKind`:

- **`case-metadata`**: a real field-by-field diff. `FieldChanges`
  reports `title`, `reference`, `category_id`, `state`, and every key
  present in either snapshot's `Metadata` map (as
  `metadata[<key>]`), each with a `Before`/`After` string.
- **`tree` / `evidence` / `opinion`**: a reference-level diff —
  `RevisionRefChanged` plus the two `ArtifactRevisionRef` values. This
  deliberately does not diff tree structure or opinion text; a caller
  that needs that resolves the tree revision via
  `packages/treeassembly` directly, or decodes an opinion snapshot's
  `Payload` via `AsOpinionPayload` and compares `Conclusions` itself.

## Restore

`Service.Restore(ctx, tenantID, snapshotID)` reverts a case's *live*
metadata to a prior `case-metadata` snapshot's `Payload`, via
`caselifecycle.Repository.Update` — actually mutating `Case.Title`,
`Reference`, `CategoryID`, `State`, and `Metadata`, and bumping
`MetadataVersion` (which in turn makes `packages/signoff`'s
`ReReviewOnCaseUpdate` correctly re-trigger review on the next check,
without this package needing to know anything about sign-off).

Restore never rewrites history: it appends exactly one new `Snapshot`
documenting the restore itself, with `RestoredFromID` pointing back at
the source snapshot. `History` before and after a restore therefore
grows by one entry — the two (or more) prior snapshots remain exactly
as they were, so "what did the timeline look like before the restore"
stays answerable.

Only `case-metadata` snapshots can be restored — `ErrNotRestorable`
otherwise. Reverting a `tree`, `evidence`, or `opinion` artifact means
acting through its own upstream package (re-running
`packages/treeassembly.Assemble` against an older evidence set, for
example), not through this package, since this package holds only a
reference to those artifacts, not their content.

## Access control / tenant isolation

Every `Service` method requires an authenticated `identity.User`:
reads need `identity.PermViewCase`, snapshotting and restoring need
`identity.PermEditCase`. Every call also re-confirms the target case
is visible to the caller's tenant via
`caselifecycle.Repository.Get` before touching snapshot storage
(`checkCaseAccess`, collapsing "case not found" and "case belongs to
another tenant" into the same `ErrForbidden`, mirroring
`packages/annotations`). `tenant_isolation_test.go` proves a tenant-B
actor cannot `Get`, see in `History`, snapshot against, `Diff`, or
`Restore` a tenant-A case's snapshot, and `PostgresRepository`/
`TenantScopedRepository` layer Row-Level Security on top for defense
in depth (`integration_test.go`).

## Repository

Two implementations, mirroring `packages/annotations`'s split exactly:

- `InMemoryRepository` — tests and fixtures, safe for concurrent use.
- `PostgresRepository` / `TenantScopedRepository` — backed by the
  `case_version_snapshots` table
  (`packages/persistence/migrations/000014_create_case_version_snapshots.up.sql`,
  `000015_enable_rls_case_version_snapshots.up.sql`). `Payload` is
  stored as JSONB behind a small kind-tagged envelope so
  `PostgresRepository` can decode it back into the exact
  `CaseMetadataPayload`/`OpinionPayload` Go type it was before
  persistence (`Snapshot.Payload` is `any` because it varies per
  `ArtifactKind`).

## The `apps/web` UI

`apps/web/src/components/workspace/HistoryPanel.tsx` is a "History" tab
in the case workspace (`apps/web/src/app/cases/[caseId]/page.tsx`): a
chronological list of every snapshot for the case, each showing its
artifact kind, label/reason, and actor, with a "Diff vs. previous" and
(for `case-metadata` snapshots) a "Restore" action. See
`apps/web/docs/case-history-ui.md` for the UI-specific data shape and
component structure.
