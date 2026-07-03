# Discussion (Annotations) UI

## Overview

`AnnotationsPanel` (mounted on the Case Workspace's new "Discussion" tab,
see `docs/case-workspace-ux.md`) is the reviewer UI for Phase 070's
multi-user notes, highlights, and threaded discussion on a case: a compose
box for a new case-level note, a flat list of threads (a root annotation
plus one level of ordered replies), and a resolve/reopen toggle per
thread. It backs `packages/annotations` — see that package's
`doc/annotations.md` for the full data model, threading rule, and access
control this UI assumes.

This phase deliberately keeps the UI scoped to case-level notes only
(`anchorType: 'case'`, `anchorId: ''`) for composing new annotations. The
component itself is anchor-agnostic — `AnnotationsPanel` renders whatever
`AnnotationEntry[]` it is given regardless of `anchorType` — so a future
phase can compose it (or a filtered subset of its props) into the
Reasoning Tree and Evidence Review panels to show/create node- and
segment-anchored annotations in place, without changing this component.

## Data Shape

There is no `/api/v1/cases/:caseId/annotations` endpoint yet — the same
situation `TreeVisualizationPanel` documents for `/tree` in Phase 065 and
`ReasoningOpinionPanel` documents for `/opinion` in Phase 067. The case
workspace page fetches lazily (only once the Discussion tab is opened) and
treats a fetch failure as "no annotations yet" rather than a page-level
error, since this is an optional tab. The types in `src/types/index.ts`
mirror `packages/annotations.Annotation` field-for-field:

```ts
type AnnotationAnchorType = 'case' | 'tree_node' | 'evidence_segment';

interface AnnotationEntry {           // packages/annotations.Annotation
  id: string;
  caseId: string;
  authorId: string;
  authorName?: string;                // resolved display name, if known
  body: string;
  anchorType: AnnotationAnchorType;
  anchorId: string;                   // '' for 'case'; irac node ID or
                                       // evidence segment ID otherwise
  parentId?: string;                  // set on a reply, absent on a root
  resolved: boolean;
  resolvedBy?: string;
  resolvedAt?: string;
  createdAt: string;
  updatedAt: string;
}
```

The case workspace page (`src/app/cases/[caseId]/page.tsx`) expects three
endpoints once built:

- `GET /api/v1/cases/:caseId/annotations` → `AnnotationEntry[]`
- `POST /api/v1/cases/:caseId/annotations` (body: `{ body, anchorType,
  anchorId, parentId? }`) → the created `AnnotationEntry`
- `POST /api/v1/cases/:caseId/annotations/:id/resolve` and
  `.../reopen` → the updated `AnnotationEntry`

## Threading

`AnnotationsPanel` groups the flat `annotations` prop into threads
client-side (`groupThreads`): every entry with no `parentId` is a root,
every entry with `parentId` set is nested directly under its root, sorted
oldest-first at both levels. This mirrors the one-level-deep threading
`packages/annotations.Create` enforces server-side (`ErrParentIsReply`) —
the UI does not attempt to render or compose a reply-to-a-reply, since the
backend would reject it.

## Resolve / reopen

Only a thread's root shows the resolve/reopen control (`AnnotationCard`'s
`isReply` prop suppresses it for replies) — matching
`packages/annotations`' Resolved/ResolvedBy/ResolvedAt fields being
meaningful on either a root or a reply, but the UI treating "is this
discussion done" as a property of the thread as a whole, not each
individual reply. The toggle is hidden entirely when no `currentUserId` is
supplied (an unauthenticated read-only render).

## Testing

`__tests__/AnnotationsPanel.test.tsx` covers the empty state, thread
grouping (root + nested reply), the open-thread count in the header, the
resolved badge, posting a new case-level note (including the blank-post
guard and an inline error on a rejected `onCreate`), replying to a thread
via its own per-thread reply box, and the resolve/reopen round-trip.
