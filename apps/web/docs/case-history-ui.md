# History (Case Versioning) UI

## Overview

`HistoryPanel` (mounted on the Case Workspace's new "History" tab, see
`docs/case-workspace-ux.md`) is the reviewer UI for Phase 071's
case-level version history: a chronological timeline (newest first) of
every recorded `Snapshot` across the case's metadata, reasoning tree,
evidence, and draft opinion, each with a "Diff vs. previous" action and,
for case-metadata snapshots only, a "Restore this version" action. It
backs `packages/caseversioning` — see that package's
`doc/case-versioning.md` for the full data model, what it composes from
existing tree-revision/`MetadataVersion` tracking versus what it adds
(opinion history), and the access-control model this UI assumes.

This phase deliberately keeps the UI to a single flat timeline plus
per-entry diff/restore — it does not attempt a branching/graph view of
history, matching how `AnnotationsPanel` deliberately keeps threading to
one level deep in Phase 070.

## Data Shape

There is no `/api/v1/cases/:caseId/versions` endpoint yet — the same
situation `AnnotationsPanel` documents for `/annotations` in Phase 070.
The case workspace page fetches lazily (only once the History tab is
opened) and treats a fetch failure as "no version history yet" rather
than a page-level error, since this is an optional tab. The types in
`src/types/index.ts` mirror `packages/caseversioning.Snapshot`/`.Diff`
field-for-field:

```ts
type SnapshotArtifactKind = 'case-metadata' | 'tree' | 'evidence' | 'opinion';

interface SnapshotEntry {             // packages/caseversioning.Snapshot
  id: string;
  caseId: string;
  artifactKind: SnapshotArtifactKind;
  artifactRevisionRef?: string;       // upstream revision id/version, when one exists
  payload?: Record<string, unknown>;  // present only for 'case-metadata' / 'opinion'
  createdBy: string;
  createdByName?: string;             // resolved display name, if known
  reason?: string;
  label?: string;
  restoredFromId?: string;            // set only on a Restore's own forward-only snapshot
  createdAt: string;
}

interface SnapshotFieldChange {
  field: string;
  before: string;
  after: string;
}

interface SnapshotDiff {              // packages/caseversioning.Diff
  caseId: string;
  artifactKind: SnapshotArtifactKind;
  snapshotAId: string;
  snapshotBId: string;
  fieldChanges?: SnapshotFieldChange[];   // populated for 'case-metadata' pairs
  revisionRefChanged: boolean;
  revisionRefBefore?: string;
  revisionRefAfter?: string;
  identical: boolean;
}
```

The case workspace page (`src/app/cases/[caseId]/page.tsx`) expects
three endpoints once built:

- `GET /api/v1/cases/:caseId/versions` → `SnapshotEntry[]`
- `GET /api/v1/cases/:caseId/versions/diff?a=:snapshotAId&b=:snapshotBId`
  → `SnapshotDiff`
- `POST /api/v1/cases/:caseId/versions/:snapshotId/restore` → the new
  restore `SnapshotEntry`

## Timeline ordering

`HistoryPanel` accepts `snapshots` in any order and sorts by `createdAt`
ascending internally, then renders newest-first (reversing for display)
so the most recent change is always at the top — the direction a
reviewer scanning "what just happened" wants. "Diff vs. previous" always
compares a row against the entry immediately before it in the
chronological (not display) order, regardless of artifact kind — a
reviewer comparing a tree snapshot against the case-metadata snapshot
before it still gets a meaningful (reference-level) result, since `Diff`
requires matching `ArtifactKind` and the panel only ever calls it
between adjacent-in-time entries. If a workflow needs cross-kind or
non-adjacent comparisons, that is a natural follow-up to this panel's
props, not a backend limitation — `packages/caseversioning.Service.Diff`
accepts any two snapshot IDs from the same case.

## Diff rendering

- **Field-level** (`fieldChanges` present, always the case for
  `case-metadata` pairs): a three-column table (`Field` / `Before` /
  `After`), one row per changed field — `title`, `reference`,
  `category_id`, `state`, or a `metadata[<key>]` entry.
- **Reference-level** (`tree` / `evidence` / `opinion` pairs): a single
  line reporting `revisionRefBefore -> revisionRefAfter`, matching
  `packages/caseversioning.ComputeDiff`'s reference-only comparison for
  these kinds (see that package's doc for why: the tree/opinion content
  itself is owned by `packages/treeassembly`/`packages/synthesisagent`,
  not duplicated here).
- **Identical**: a plain "No differences between these versions."
  message when `identical` is true.

## Restore

The "Restore this version" button is shown only when
`artifactKind === 'case-metadata'` and the snapshot is not itself the
record of a prior restore (`!restoredFromId`) — mirroring
`packages/caseversioning.Service.Restore`'s `ErrNotRestorable` for every
other artifact kind exactly, so the UI never offers an action the
backend would reject. A snapshot that resulted from a restore instead
shows a "Restore" badge (via `RestoredFromID`) so the timeline makes
clear which entries are original edits versus restore points.

Restoring calls the page's `handleRestoreSnapshot`, which both appends
the new restore `SnapshotEntry` to local state and re-fetches the whole
workspace payload (`loadCase()`), since a restore mutates the live
`Case` fields `CaseHeader`/`PartiesCategoryPanel` render elsewhere on the
page — those must not go stale after a successful restore.

## Testing

`__tests__/HistoryPanel.test.tsx` covers the empty state, newest-first
timeline ordering with artifact-kind badge and actor attribution,
computing and rendering a field-level diff against the previous
snapshot, Restore being offered only for case-metadata snapshots (never
tree/evidence/opinion, and never on a snapshot that is itself a restore
record), and the `onRestore` callback firing with the correct snapshot
ID. `__tests__/WorkspaceTabs.test.tsx` covers the new History tab
alongside the existing six.
