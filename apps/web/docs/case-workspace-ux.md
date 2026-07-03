# Verdex Case Workspace UX

## Overview

The case workspace is the unified judicial-facing view of a single case, once it exists
past the ingestion wizard (`/cases/new`, documented in `docs/ingestion-ux.md`). It lives at
`/cases/[caseId]` and brings together the case's identity, parties, evidence, timeline,
lifecycle state, and (eventually) its reasoning output into one tabbed workspace.

The workspace's `Case` shape mirrors `packages/caselifecycle.Case` field-for-field (see
`CaseLifecycle` in `src/types/index.ts`): `id`, `tenantId`, `jurisdictionId`, `categoryId`,
`title`, `reference`, `state`, `metadata`/`metadataVersion`, `createdBy`/`createdAt`/
`updatedAt`, `archivedAt`. The five lifecycle states — `draft`, `active`, `under_review`,
`closed`, `archived` — and the transitions between them are taken directly from that
package's `State` constants and `allowedTransitions`/`permittedActions` maps, not
reinvented here.

As with every other panel in the app, any reasoning or opinion content surfaced in this
workspace carries the draft-analysis framing (via `Disclaimer.tsx`) and never reads as a
verdict, ruling, or final decision.

## Directory Structure

```
apps/web/src/
├── app/cases/[caseId]/
│   └── page.tsx                       # Case workspace route
├── components/workspace/
│   ├── CaseHeader.tsx                 # Title, reference, state badge, category, jurisdiction
│   ├── StatusActionsBar.tsx           # Lifecycle state + state-appropriate actions
│   ├── WorkspaceTabs.tsx              # Overview / Evidence / Tree / Reasoning tab strip
│   ├── PartiesCategoryPanel.tsx       # Parties + category/subcategory (Overview tab)
│   ├── EvidenceTimelinePanel.tsx      # Evidence segments + chronological timeline
│   ├── TreeVisualizationPanel.tsx     # IRAC reasoning tree entry point (Phase 065 fills in)
│   ├── ReasoningOpinionPanel.tsx      # Draft opinion entry point (Phase 067 fills in)
│   ├── WorkspaceLoading.tsx           # Full-panel loading state
│   └── WorkspaceError.tsx             # Full-panel error / not-found state
├── lib/
│   └── caseLifecycle.ts               # Client-side mirror of transition/action rules
└── types/index.ts                     # CaseLifecycle, CaseState, CaseParty, EvidenceSegment, …
```

## Layout

`page.tsx` composes, top to bottom, inside `AppShell`:

1. **`WorkspaceLoading`** or **`WorkspaceError`** while the case record is being fetched or
   if that fetch fails (including a distinct "Case not found." message for a 404).
2. **`CaseHeader`** — case title, external reference, lifecycle state badge, category /
   subcategory, and jurisdiction, once loaded.
3. **`StatusActionsBar`** — a second, action-oriented view of the same lifecycle state,
   with buttons for whichever transitions are legal from the current state.
4. **`WorkspaceTabs`** — quick navigation between the four panels below, integrated with
   the existing `Sidebar`'s `/cases` nav item (the sidebar highlights `/cases` for any
   route starting with that prefix, including `/cases/[caseId]`).
5. The active tab's panel, rendered inside a `role="tabpanel"` region wired to the
   corresponding tab via `aria-controls`/`aria-labelledby`.

### Tabs

| Tab | Panel | Shows |
|---|---|---|
| Overview | `PartiesCategoryPanel` | Category/subcategory, party list with role + counsel |
| Evidence & Timeline | `EvidenceTimelinePanel` | Evidence segments (type, confidence, source span) and a chronologically sorted event timeline (undated events last) |
| Reasoning Tree | `TreeVisualizationPanel` | Empty/loading placeholder; Phase 065 renders the interactive issue/rule/fact/conclusion tree here |
| Draft Opinion | `ReasoningOpinionPanel` | `Disclaimer` (always rendered first) plus a loading/empty/has-draft placeholder; Phase 067 renders the full per-issue analysis here |

## Lifecycle State & Actions

`src/lib/caseLifecycle.ts` mirrors `packages/caselifecycle` on the client so the status bar
can compute available actions without a round trip:

- **Ordinary transitions** (`canTransition`): `draft -> active -> under_review -> {closed,
  active}`. `closed` and `archived` have no ordinary forward transitions.
- **Reopen** (`canReopen`): only from `closed`, and only via a distinct, explicit action
  that requires a non-blank justification — `StatusActionsBar` renders an inline textarea
  and disables "Confirm Reopen" until the reviewer types a reason, matching
  `packages/caselifecycle.Reopen`'s `ErrReasonRequired` guard.
- **Archive** (`canArchive`): only from `closed`, moving to the terminal `archived` state.
  Archived cases show no actions at all (`archived.IsTerminal() == true` on the backend).

`permittedActions`/`isActionPermitted` mirror `packages/caselifecycle`'s `Action` /
`permittedActions` map (`ingest_evidence`, `edit_category`, `edit_timeline`,
`generate_reasoning`, `review_opinion`, `edit_metadata`) for future panels that need to gate
edit affordances by state — e.g. a future evidence-review UI (Phase 066) checking
`isActionPermitted(state, 'ingest_evidence')` before showing an upload control.

## Data Fetching

`page.tsx` fetches the full workspace payload in one call via `apiFetch`:

```
GET /api/v1/cases/:caseId
  -> { caseData: CaseLifecycle, parties: CaseParty[], evidence: EvidenceSegment[], events: TimelineEvent[] }
```

This endpoint does not exist yet on the backend as of this phase — `packages/caselifecycle`
has no HTTP handler wired up. The UI is built against the shape a REST handler over that
package's `Case`/`TransitionRecord` types would plausibly expose, consistent with how
`apps/web/src/app/cases/new/page.tsx` already calls `POST /api/v1/cases` ahead of that
endpoint's backend wiring.

State transitions call three further plausible endpoints, matching
`packages/caselifecycle`'s three distinct mutating operations:

- `POST /api/v1/cases/:caseId/transition` — body `{ toState }`, for ordinary transitions.
- `POST /api/v1/cases/:caseId/reopen` — body `{ justification }`.
- `POST /api/v1/cases/:caseId/archive` — body `{ reason }`.

## Loading & Error States

- **Loading**: `WorkspaceLoading` (a centered spinner) while the initial fetch is in
  flight.
- **Not found**: a 404 `ApiError` from `apiFetch` renders `WorkspaceError` with the message
  "Case not found." — verified by `CaseWorkspacePage.test.tsx`.
- **Other failures**: any other error renders `WorkspaceError` with that error's message
  and a "Retry" button that re-runs the fetch.
- **Unauthenticated**: no session redirects to `/login`, matching `dashboard/page.tsx`'s
  pattern.

One implementation note worth calling out: the auth-redirect `useEffect` depends on a
boolean `hasSession` (not the `session` object) and omits `router` from its dependency
array. `useRouter()`'s return value is stable in real Next.js, but including a
non-primitive value there is fragile — anything that hands back a fresh object on each
render (including some test doubles) would otherwise cause the case fetch to re-run on
every render.

## Responsive Layout

Every workspace component uses `flex-wrap` for header/status-bar action rows and
`grid-cols-1 sm:grid-cols-2` for two-column detail blocks, so they stack to a single column
below the `sm` breakpoint. `WorkspaceTabs` scrolls horizontally (`overflow-x-auto`) rather
than wrapping, so the tab strip stays usable on narrow viewports without pushing panel
content down. `Sidebar`/`AppShell` responsiveness is unchanged from the existing dashboard
pattern (out of scope for this phase).

## Testing

Tests live in `__tests__/` and follow the existing Jest + `@testing-library/react`
convention:

- `CaseHeader.test.tsx`, `PartiesCategoryPanel.test.tsx`, `EvidenceTimelinePanel.test.tsx`,
  `TreeVisualizationPanel.test.tsx`, `ReasoningOpinionPanel.test.tsx`,
  `WorkspaceTabs.test.tsx`, `WorkspaceLoading.test.tsx`, `WorkspaceError.test.tsx` — render
  and state tests for each component, including empty states and (for the opinion panel)
  an explicit assertion that no verdict-implying language appears.
- `StatusActionsBar.test.tsx` — the correct action set for every one of the five lifecycle
  states, the required-justification gate on Reopen, and the `onTransition`/`onArchive`
  callbacks.
- `CaseWorkspacePage.test.tsx` — the route-level integration test: login redirect, loading/
  not-found/generic-error states, successful render, and tab-driven panel switching.

Run: `npm test` from `apps/web/`.
