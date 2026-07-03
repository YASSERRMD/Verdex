# Case Search UI

## Overview

`/search` (`src/app/search/page.tsx`) is Phase 069's minimal cross-case search
surface: a query box, a mode selector, structured filters (category, party
name, state, date range), a ranked results list with highlighted snippets,
and a saved-searches panel to persist/re-run/delete named queries — matching
the style established by `/dashboard` and the Case Workspace (`AppShell`,
`Card`, `Input`, `Select`, `Button` from `src/components/ui`).

This is intentionally scoped as a working results list, not a full-featured
search experience: no infinite scroll, no per-hit expansion, no issue/rule
node picker UI (the "Issue / Rule" mode exists in the mode selector, but the
UI does not yet expose a way to pick `IssueOrRuleID` — that requires a tree
node picker, which is out of scope for this pass and can reuse
`TreeVisualizationPanel`'s node list in a future phase).

## Data shape

There is no `/api/v1/search` endpoint yet — the same "types model what the
endpoint is expected to serve" situation `TreeVisualizationPanel` and
`ReasoningOpinionPanel` document for their own not-yet-built endpoints. The
types in `src/types/index.ts` (`SearchMode`, `SearchFilters`, `SearchHit`,
`SearchResultItem`, `SearchResults`, `SavedSearchEntry`) mirror
`packages/casesearch`'s Go types field-for-field:

```ts
type SearchMode = '' | 'keyword' | 'semantic' | 'issue_rule'; // '' = ModeAuto

interface SearchFilters {
  categoryCode?: string;
  jurisdictionId?: string;
  partyName?: string;
  state?: string;
  dateFrom?: string; // ISO date
  dateTo?: string;   // ISO date
}

interface SearchResultItem {
  caseId: string;
  title: string;
  reference: string;
  categoryId: string;
  jurisdictionId: string;
  state: string;
  createdAt: string;
  mode: SearchMode;
  score: number;       // 0..1, rendered as a rounded percentage badge
  snippet: string;      // casesearch.ExtractSnippet output, ** delimited
  hits: SearchHit[];
}

interface SearchResults {
  items: SearchResultItem[];
  totalMatches: number;
  page: { number: number; size: number };
  mode: SearchMode;
  skippedCases: number; // surfaced as "N cases could not be searched"
}
```

The expected wire contract once a real handler exists:

- `GET /api/v1/search?q=...&mode=...&category=...&party=...&state=...&date_from=...&date_to=...`
  → `SearchResults`
- `GET /api/v1/search/saved` → `SavedSearchEntry[]`
- `POST /api/v1/search/saved` with `{ name, text, mode, filter }` → `SavedSearchEntry`
- `DELETE /api/v1/search/saved/:id` → 204

## Snippet highlighting

`packages/casesearch.ExtractSnippet` returns a markup-agnostic excerpt with
the matched span wrapped in `**...**` (see
`packages/casesearch/snippet.go`'s `SnippetHighlightOpen`/`Close`), rather
than embedding HTML server-side. `SearchResultsList.renderSnippet` splits on
`**` and wraps the odd-indexed (matched) segments in a `<mark>` element
client-side, so the highlight convention stays renderer-agnostic on the
backend while still rendering as a real visual highlight in this UI.

## Components

- `SearchFilterPanel` (`src/components/search/SearchFilterPanel.tsx`) — the
  query box, mode `Select`, and filter fields (category, party name, state,
  date range), plus Search/Save search buttons. Pure controlled-component
  props (`query`/`mode`/`filters` + `on*Change` callbacks), no fetching of
  its own — testable in isolation with a small harness (see
  `SearchFilterPanel.test.tsx`).
- `SearchResultsList` (`src/components/search/SearchResultsList.tsx`) —
  renders the three states (`hasSearched=false` prompt, `loading`,
  zero-result empty state) plus the ranked list of `Card`-wrapped results,
  each linking to `/cases/:caseId`.
- `SavedSearchesPanel` (`src/components/search/SavedSearchesPanel.tsx`) —
  lists saved searches by name; clicking a name re-runs it, a per-row Delete
  button removes it.
- `src/app/search/page.tsx` — composes the three above, owns all fetch
  state (`apiFetch` from `src/lib/api.ts`, session-gated via `getSession`
  exactly like `/dashboard`), and builds the query string for
  `GET /api/v1/search`.

## Navigation

A "Search" entry (between "Cases" and "Jurisdictions") was added to
`src/components/layout/Sidebar.tsx`'s `NAV_ITEMS`, visible to every role
(no `adminOnly` gate — search is read-scoped the same as
`identity.PermViewCase` in the backend).
