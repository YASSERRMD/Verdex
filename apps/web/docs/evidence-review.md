# Verdex Evidence Review UI

## Overview

The evidence review UI is the case-workspace-scoped counterpart to the one-time ingestion
review UI documented in `docs/ingestion-ux.md` (Phase 030). It lives at
`src/components/workspace/EvidenceReviewPanel.tsx`, reachable from the case workspace
(`/cases/[caseId]`, documented in `docs/case-workspace-ux.md`) via a dedicated **Evidence
Review** tab.

As with every other panel in the app, evidence classifications and corrections shown here
are draft material only. Nothing in this panel is a finding, ruling, or verdict — it is a
reviewer's working record of how case evidence has been classified, attributed, and
corrected over the life of the case.

## How this differs from the Phase 030 ingestion review UI

Phase 030's `ClassificationCorrectionPanel`, `ExtractedTextReview`, and
`PartyTimelineEditor` are the **one-time, end-of-intake** review step: a reviewer pages
through everything a single ingestion job just produced, corrects it once, and moves on
through the wizard. That UI has no list-level tooling (no search, filter, multi-select, or
persistent audit view) because it only ever needs to handle one job's output in a fixed
wizard step.

`EvidenceReviewPanel` is the **ongoing, case-lifecycle** counterpart: it operates on the
full accumulated set of evidence segments for a case — however many ingestion jobs
contributed them — at any point after intake, for as long as the case is open. Because the
set of segments can be large and grows over the case's life, this phase adds the tooling
Phase 030 does not need:

| Concern | Phase 030 (ingestion) | Phase 066 (case workspace) |
|---|---|---|
| Scope | One ingestion job's segments, once | All of a case's segments, ongoing |
| Entry point | Step 4 of the `/cases/new` wizard | "Evidence Review" tab in `/cases/[caseId]` |
| Classification/party correction | `ClassificationCorrectionPanel` (per-segment `Select`s) | Same pattern, reused inline per segment |
| Dispute tracking | Not present | Disputed/undisputed toggle per segment |
| Bulk edits | Not present | Multi-select + bulk type/party tagging |
| Search/filter | Not present | Free-text search, type/party/dispute filters |
| Change history | Not present | Visible per-segment audit trail (who/when/what) |
| Party/timeline editing | `PartyTimelineEditor` (add/edit parties & events) | Out of scope here — party *reassignment* on a segment reuses `PartyRole`, but adding/editing parties themselves stays in `PartyTimelineEditor` |

Both UIs reuse the same underlying `EvidenceType`/`PartyRole` vocabulary from
`packages/evidence` (see below) and the same "Select + stub `apiFetch` PUT + inline
`role="alert"` error" interaction pattern established by `ClassificationCorrectionPanel`, so
a reviewer sees consistent controls whether they are correcting a segment during intake or
weeks later from the case workspace.

## Directory Structure

```
apps/web/src/
├── app/cases/[caseId]/
│   └── page.tsx                       # Case workspace route (adds the evidence-review tab)
├── components/workspace/
│   ├── WorkspaceTabs.tsx              # Now five tabs: + "Evidence Review"
│   ├── EvidenceTimelinePanel.tsx      # Read-only evidence + timeline (Evidence & Timeline tab)
│   └── EvidenceReviewPanel.tsx        # This phase: review/correct/audit evidence segments
└── types/index.ts                     # EvidenceSegment.disputed, EvidenceAuditEntry
```

## Evidence Type Taxonomy

`EvidenceType` (`src/types/index.ts`) mirrors `packages/evidence`'s classification concept.
The TS label set predates this phase (introduced in Phase 030) and is kept as-is for
compatibility with existing ingestion-review call sites; the panel's badges map each TS
value to a human label that matches the intent of `packages/evidence/taxonomy.go`'s real
`EvidenceType` constants:

| TS `EvidenceType` | Badge label | Closest `packages/evidence` taxonomy constant |
|---|---|---|
| `testimony` | Testimony | `TypeWitnessStatement` |
| `documentary` | Exhibit | `TypeDocumentaryEvidence` (also covers `TypePhysicalExhibit`) |
| `statute_citation` | Statute Citation | `TypeStatutoryCitation` |
| `argument` | Argument | `TypeArgument` |
| `other` | Other | `TypeOther` |

`PartyRole` (`first_party`, `second_party`, `third_party`, `unattributed`) matches
`packages/evidence/party.go`'s `PartyFirst`/`PartySecond`/`PartyUnattributed`, extended with
`third_party` for consistency with `TimelineParty`/`CaseParty` elsewhere in the app (the Go
package does not yet model a third-party role).

## Features

`EvidenceReviewPanel` renders one `Card` with, top to bottom:

1. **Search & filter bar** — free-text search over segment text, plus `Select` filters for
   type, party, and dispute status. Filters compose (AND) and narrow the list live.
2. **Bulk tagging bar** — a "select all" checkbox (scoped to the currently filtered list),
   per-segment checkboxes, and bulk "set type"/"set party" `Select`s applied to every
   selected segment via **Apply to N selected**.
3. **Segment list** — one card per segment showing:
   - Segment text and a color-coded type badge (`data-testid="evidence-type-badge-<id>"`).
   - Provenance/confidence: source file name (when known), source span `[start–end]`, and
     confidence as a rounded percentage — the same fields `EvidenceTimelinePanel` shows,
     kept consistent between the read-only and review views.
   - Inline **Evidence Type** and **Party** `Select`s for single-segment correction.
   - A **Disputed / Undisputed** toggle button; disputed segments get a distinct red-tinted
     card style so they stand out while scanning the list.
   - A collapsible **change history** section listing every correction made to that segment
     in the current session (field, previous value, new value, actor, timestamp).

Every correction (`type`, `party`, or `disputed`) round-trips through the same stub
`PUT /api/v1/segments/:id/classification` endpoint `ClassificationCorrectionPanel` uses,
via `apiFetch`. A failed save surfaces a `role="alert"` inline error on that segment and
leaves its state unchanged; a successful save updates local state, calls the optional
`onSegmentsChange` prop (so the case workspace page can keep its own `evidence` array in
sync), and appends an `EvidenceAuditEntry`.

## Change Audit

`EvidenceAuditEntry` (`src/types/index.ts`) records `segmentId`, the changed `field`
(`'type' | 'party' | 'disputed'`), `previousValue`/`newValue`, `actor`, and `occurredAt`.
There is no real audit API yet — entries are held in the panel's local state and reset on
remount — so this is explicitly a client-side stand-in for a future audit log endpoint,
following the same "UI built ahead of the backend" approach `PartyTimelineEditor` and
`ClassificationCorrectionPanel` already take for their own stub endpoints. The `actor` is
currently hardcoded to `"Current Reviewer"` pending real session-derived attribution.

## Testing

`__tests__/EvidenceReviewPanel.test.tsx` follows the existing Jest + Testing Library
convention and covers:

- List rendering with correct type badges per segment.
- Provenance/confidence display (source file, source span, confidence percentage).
- Inline classification correction updating both local state and the `onSegmentsChange`
  callback.
- Party reassignment.
- Dispute/undisputed toggling.
- Bulk tagging applied only to selected segments (unselected segments are unaffected).
- Search/filter narrowing the list by type, party, dispute status, and free text.
- Audit history appearing after a correction, with the correct field/before/after text.
- Inline error surfacing when a correction's `apiFetch` call rejects.

`__tests__/WorkspaceTabs.test.tsx` and `__tests__/CaseWorkspacePage.test.tsx` were updated
for the new "Evidence Review" tab alongside the existing four.

Run: `npm test` from `apps/web/`.
